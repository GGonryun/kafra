package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"

	"braekhus-go/internal/backoff"
	"braekhus-go/internal/jwt"
	"braekhus-go/internal/rpc"
	"braekhus-go/pkg/types"
)

const (
	// DefaultBackoffStart is the default starting backoff duration
	DefaultBackoffStart = 1 * time.Second
	// DefaultBackoffMax is the default maximum backoff duration
	DefaultBackoffMax = 30 * time.Second
	// DefaultRequestTimeout is the default timeout for forwarded requests
	DefaultRequestTimeout = 30 * time.Second
)

// Client represents the braekhus client
type Client struct {
	config     *types.Config
	logger     *logrus.Logger
	jwtManager *jwt.Manager
	rpcClient  *rpc.Client
	backoff    *backoff.Backoff
	
	conn           *websocket.Conn
	connMu         sync.RWMutex
	ctx            context.Context
	cancel         context.CancelFunc
	connected      chan struct{}
	isShutdown     bool
	shutdownMu     sync.RWMutex
}

// New creates a new braekhus client
func New(config *types.Config, logger *logrus.Logger) (*Client, error) {
	jwtManager := jwt.NewManager(logger)
	if err := jwtManager.EnsureKey(config.JWKPath); err != nil {
		return nil, fmt.Errorf("failed to ensure JWT key: %w", err)
	}
	
	backoffInstance, err := backoff.New(DefaultBackoffStart, DefaultBackoffMax)
	if err != nil {
		return nil, fmt.Errorf("failed to create backoff: %w", err)
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	client := &Client{
		config:     config,
		logger:     logger,
		jwtManager: jwtManager,
		backoff:    backoffInstance,
		ctx:        ctx,
		cancel:     cancel,
		connected:  make(chan struct{}),
	}
	
	// Create RPC client with send function
	client.rpcClient = rpc.NewClient(client.sendMessage)
	
	// Register the "call" method with placeholder implementation
	client.rpcClient.AddMethod("call", client.handleCallMethod)
	
	return client, nil
}

// Connect establishes connection to the server
func (c *Client) Connect() error {
	return c.connect()
}

// connect establishes WebSocket connection with retry logic
func (c *Client) connect() error {
	for {
		c.shutdownMu.RLock()
		if c.isShutdown {
			c.shutdownMu.RUnlock()
			return fmt.Errorf("client is shutdown")
		}
		c.shutdownMu.RUnlock()
		
		if err := c.connectOnce(); err != nil {
			c.logger.WithError(err).Warn("Connection failed, retrying...")
			
			select {
			case <-c.ctx.Done():
				return c.ctx.Err()
			case <-time.After(c.backoff.Next()):
				continue
			}
		}
		
		c.backoff.Reset()
		return nil
	}
}

// connectOnce attempts a single connection
func (c *Client) connectOnce() error {
	// Create JWT token
	token, err := c.jwtManager.CreateJWT(c.config.ClientID)
	if err != nil {
		return fmt.Errorf("failed to create JWT: %w", err)
	}
	
	// Build WebSocket URL
	scheme := "ws"
	if !c.config.Insecure {
		scheme = "wss"
	}
	
	u := url.URL{
		Scheme: scheme,
		Host:   fmt.Sprintf("%s:%d", c.config.TunnelHost, c.config.TunnelPort),
	}
	
	// Create headers with authentication
	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+token)
	
	// Establish WebSocket connection
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), headers)
	if err != nil {
		return fmt.Errorf("failed to dial WebSocket: %w", err)
	}
	
	c.connMu.Lock()
	c.conn = conn
	c.connMu.Unlock()
	
	c.logger.Info("WebSocket connection established")
	
	// Send setClientId request
	if _, err := c.rpcClient.Call("setClientId", types.SetClientIDRequest{
		ClientID: c.config.ClientID,
	}); err != nil {
		conn.Close()
		return fmt.Errorf("failed to set client ID: %w", err)
	}
	
	c.logger.Info("Client ID set successfully")
	
	// Signal that we're connected
	select {
	case c.connected <- struct{}{}:
	default:
	}
	
	// Start message handling
	go c.handleMessages()
	
	return nil
}

// handleMessages handles incoming WebSocket messages
func (c *Client) handleMessages() {
	defer func() {
		c.connMu.Lock()
		if c.conn != nil {
			c.conn.Close()
			c.conn = nil
		}
		c.connMu.Unlock()
		
		// Attempt reconnection if not shutdown
		c.shutdownMu.RLock()
		isShutdown := c.isShutdown
		c.shutdownMu.RUnlock()
		
		if !isShutdown {
			c.logger.Info("Connection lost, attempting to reconnect...")
			go c.connect()
		}
	}()
	
	for {
		c.connMu.RLock()
		conn := c.conn
		c.connMu.RUnlock()
		
		if conn == nil {
			return
		}
		
		_, message, err := conn.ReadMessage()
		if err != nil {
			c.logger.WithError(err).Warn("Failed to read WebSocket message")
			return
		}
		
		if err := c.rpcClient.HandleMessage(message); err != nil {
			c.logger.WithError(err).Error("Failed to handle RPC message")
		}
	}
}

// sendMessage sends a message through the WebSocket connection
func (c *Client) sendMessage(data []byte) error {
	c.connMu.RLock()
	conn := c.conn
	c.connMu.RUnlock()
	
	if conn == nil {
		return fmt.Errorf("no WebSocket connection")
	}
	
	return conn.WriteMessage(websocket.TextMessage, data)
}

// handleCallMethod handles the "call" method for forwarded requests
func (c *Client) handleCallMethod(params interface{}) (interface{}, error) {
	c.logger.Debug("Received 'call' method")
	
	// Parse the ForwardedRequest from params
	var request types.ForwardedRequest
	
	// Convert params (interface{}) to JSON and then unmarshal to ForwardedRequest
	paramsBytes, err := json.Marshal(params)
	if err != nil {
		c.logger.WithError(err).Error("Failed to marshal params to JSON")
		return nil, fmt.Errorf("failed to marshal params: %w", err)
	}
	
	if err := json.Unmarshal(paramsBytes, &request); err != nil {
		c.logger.WithError(err).Error("Failed to unmarshal params to ForwardedRequest")
		return nil, fmt.Errorf("failed to unmarshal ForwardedRequest: %w", err)
	}
	
	// Log the parsed request (excluding sensitive headers like authorization)
	logHeaders := make(map[string]interface{})
	for key, value := range request.Headers {
		if strings.ToLower(key) != "authorization" {
			logHeaders[key] = value
		}
	}
	
	c.logger.WithFields(logrus.Fields{
		"method":  request.Method,
		"path":    request.Path,
		"headers": logHeaders,
		"params":  request.Params,
	}).Info("Forwarded request received")
	
	// TODO: Implement actual request forwarding to target service
	// This is where you would:
	// 1. Create HTTP request to c.config.TargetURL + request.Path
	// 2. Set method, headers, query params, and body from ForwardedRequest
	// 3. Execute the HTTP request
	// 4. Parse the response and create ForwardedResponse
	
	// For now, create a placeholder response with the parsed request info
	response := types.ForwardedResponse{
		Headers:    map[string]interface{}{"content-type": "application/json"},
		Status:     200,
		StatusText: "OK",
		Data: map[string]interface{}{
			"message":        "Request received and parsed successfully",
			"parsedMethod":   request.Method,
			"parsedPath":     request.Path,
			"parsedParams":   request.Params,
			"headerCount":    len(request.Headers),
			"targetURL":      c.config.TargetURL,
		},
	}
	
	// Log the response
	c.logger.WithFields(logrus.Fields{
		"status":     response.Status,
		"statusText": response.StatusText,
		"headers":    response.Headers,
	}).Info("Sending response")
	
	return response, nil
}

// WaitUntilConnected waits until the client is connected
func (c *Client) WaitUntilConnected() error {
	select {
	case <-c.connected:
		return nil
	case <-c.ctx.Done():
		return c.ctx.Err()
	}
}

// Run runs the client until shutdown
func (c *Client) Run() error {
	if err := c.Connect(); err != nil {
		return err
	}
	
	<-c.ctx.Done()
	return c.ctx.Err()
}

// Shutdown gracefully shuts down the client
func (c *Client) Shutdown() {
	c.shutdownMu.Lock()
	c.isShutdown = true
	c.shutdownMu.Unlock()
	
	c.cancel()
	
	c.connMu.Lock()
	if c.conn != nil {
		c.conn.Close()
	}
	c.connMu.Unlock()
	
	c.logger.Info("Client shutdown completed")
}
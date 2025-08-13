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

	"p0-ssh-agent/internal/backoff"
	"p0-ssh-agent/internal/jwt"
	"p0-ssh-agent/internal/rpc"
	"p0-ssh-agent/scripts"
	"p0-ssh-agent/types"
)

const (
	// DefaultBackoffStart is the default starting backoff duration
	DefaultBackoffStart = 1 * time.Second
	// DefaultBackoffMax is the default maximum backoff duration
	DefaultBackoffMax = 30 * time.Second
	// DefaultRequestTimeout is the default timeout for forwarded requests
	DefaultRequestTimeout = 30 * time.Second
)

// Client represents the p0-ssh-agent client
type Client struct {
	config     *types.Config
	logger     *logrus.Logger
	jwtManager *jwt.Manager
	rpcClient  *rpc.Client
	backoff    *backoff.Backoff

	conn       *websocket.Conn
	connMu     sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
	connected  chan struct{}
	isShutdown bool
	shutdownMu sync.RWMutex
}

// New creates a new p0-ssh-agent client
func New(config *types.Config, logger *logrus.Logger) (*Client, error) {
	jwtManager := jwt.NewManager(logger)
	if err := jwtManager.LoadKey(config.GetKeyPath()); err != nil {
		return nil, fmt.Errorf("failed to load JWT key: %w", err)
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

	// Create RPC client
	client.rpcClient = rpc.NewClient()

	// Register the "call" method with placeholder implementation
	client.rpcClient.AddMethod("call", client.handleCallMethod)

	// Set up connection callback to call setClientId when WebSocket opens
	client.rpcClient.SetOnConnected(func() {
		client.logger.Info("WebSocket connection established, sending setClientId")
		if _, err := client.rpcClient.Call("setClientId", types.SetClientIDRequest{
			ClientID: client.config.GetClientID(),
		}); err != nil {
			client.logger.WithError(err).Error("Failed to set client ID")
			return
		}
		client.logger.Info("Client ID set successfully")

		// Signal that we're connected
		select {
		case client.connected <- struct{}{}:
		default:
		}
	})

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
	token, err := c.jwtManager.CreateJWT(c.config.GetClientID())
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
		Path:   c.config.TunnelPath,
	}

	// Create headers with authentication
	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+token)

	// Establish WebSocket connection
	c.logger.WithFields(logrus.Fields{
		"url":     u.String(),
		"headers": map[string]string{"Authorization": "Bearer <redacted>"},
	}).Debug("Attempting WebSocket connection")

	conn, resp, err := websocket.DefaultDialer.Dial(u.String(), headers)
	if err != nil {
		// Enhanced error logging with HTTP response details
		if resp != nil {
			c.logger.WithFields(logrus.Fields{
				"status_code": resp.StatusCode,
				"status":      resp.Status,
				"headers":     resp.Header,
			}).Error("WebSocket handshake failed with HTTP response")

			// Log specific authentication errors
			if resp.StatusCode == 401 {
				c.logger.Error("🔐 Authentication failed - JWT token rejected by server")
				c.logger.Error("💡 Check: 1) Client ID is registered 2) JWT key is correct 3) Token not expired")
			} else if resp.StatusCode == 403 {
				c.logger.Error("🚫 Forbidden - Client ID may not be authorized")
			} else if resp.StatusCode == 404 {
				c.logger.Error("🔍 Not Found - Check WebSocket endpoint path")
			}

			return fmt.Errorf("WebSocket handshake failed: HTTP %d %s", resp.StatusCode, resp.Status)
		}

		return fmt.Errorf("failed to dial WebSocket: %w", err)
	}

	c.connMu.Lock()
	c.conn = conn
	c.connMu.Unlock()

	c.logger.Info("WebSocket connection established, connecting JSON-RPC client")

	// Connect the JSON-RPC client to the WebSocket
	// This will trigger the onConnected callback which sends setClientId
	if err := c.rpcClient.ConnectWebSocket(conn); err != nil {
		conn.Close()
		return fmt.Errorf("failed to connect JSON-RPC client: %w", err)
	}

	return nil
}

// handleReconnection handles reconnection logic when connection is lost
func (c *Client) handleReconnection() {
	// The JSON-RPC connection will handle its own lifecycle
	// We just need to detect when it's closed and reconnect if not shutdown

	// Wait for the JSON-RPC connection to close
	c.rpcClient.WaitUntilConnected() // This will block until connected, then return when disconnected

	// Check if we should reconnect
	c.shutdownMu.RLock()
	isShutdown := c.isShutdown
	c.shutdownMu.RUnlock()

	if !isShutdown {
		c.logger.Info("JSON-RPC connection lost, attempting to reconnect...")
		go c.connect()
	}
}

// sendMessage is no longer needed - JSON-RPC client handles all messaging
// Kept for backward compatibility but should not be used
func (c *Client) sendMessage(data []byte) error {
	return fmt.Errorf("sendMessage deprecated - use JSON-RPC client methods instead")
}

// handleCallMethod handles the "call" method and executes provisioning scripts
func (c *Client) handleCallMethod(ctx context.Context, params json.RawMessage) (interface{}, error) {
	c.logger.Info("🔄 Received 'call' method - processing provisioning request")

	// Parse the ForwardedRequest from params
	var request types.ForwardedRequest
	if err := json.Unmarshal(params, &request); err != nil {
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
		"method":     request.Method,
		"path":       request.Path,
		"headers":    logHeaders,
		"params":     request.Params,
		"command":    request.Command,
		"target_url": c.config.TargetURL,
		"client_id":  c.config.GetClientID(),
		"has_data":   request.Data != nil,
	}).Info("📥 P0 SSH Agent received provisioning request")

	// Execute provisioning scripts based on command
	var scriptResult scripts.ProvisioningResult
	if request.Command != "" && request.Data != nil {
		scriptResult = c.executeProvisioningScript(request.Command, request.Data)
	} else {
		// Legacy handling - no command specified
		scriptResult = scripts.ProvisioningResult{
			Success: true,
			Message: "Request logged - no command specified",
		}
	}

	// Prepare response based on script execution result
	response := types.ForwardedResponse{
		Headers:    map[string]interface{}{"content-type": "application/json"},
		Status:     200,
		StatusText: "OK",
	}

	if scriptResult.Success {
		response.Data = map[string]interface{}{
			"success":   true,
			"message":   scriptResult.Message,
			"client_id": c.config.GetClientID(),
			"command":   request.Command,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"status":    "completed",
		}
		c.logger.WithFields(logrus.Fields{
			"command": request.Command,
			"message": scriptResult.Message,
		}).Info("✅ Provisioning script executed successfully")
	} else {
		response.Status = 500
		response.StatusText = "Internal Server Error"
		response.Data = map[string]interface{}{
			"success":   false,
			"error":     scriptResult.Error,
			"client_id": c.config.GetClientID(),
			"command":   request.Command,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"status":    "failed",
		}
		c.logger.WithFields(logrus.Fields{
			"command": request.Command,
			"error":   scriptResult.Error,
		}).Error("❌ Provisioning script execution failed")
	}

	c.logger.WithFields(logrus.Fields{
		"status":      response.Status,
		"status_text": response.StatusText,
		"command":     request.Command,
	}).Info("📤 P0 SSH Agent sending response")

	return response, nil
}

// executeProvisioningScript executes the appropriate provisioning script based on command
func (c *Client) executeProvisioningScript(command string, data interface{}) scripts.ProvisioningResult {
	// Convert data to ProvisioningRequest
	dataBytes, err := json.Marshal(data)
	if err != nil {
		c.logger.WithError(err).Error("Failed to marshal script data")
		return scripts.ProvisioningResult{
			Success: false,
			Error:   fmt.Sprintf("failed to marshal script data: %v", err),
		}
	}

	var req scripts.ProvisioningRequest
	if err := json.Unmarshal(dataBytes, &req); err != nil {
		c.logger.WithError(err).Error("Failed to unmarshal script data to ProvisioningRequest")
		return scripts.ProvisioningResult{
			Success: false,
			Error:   fmt.Sprintf("failed to unmarshal ProvisioningRequest: %v", err),
		}
	}

	c.logger.WithFields(logrus.Fields{
		"command":    command,
		"username":   req.UserName,
		"action":     req.Action,
		"request_id": req.RequestID,
		"sudo":       req.Sudo,
		"has_key":    req.PublicKey != "" && req.PublicKey != "N/A",
	}).Info("🚀 Executing provisioning script")

	// Execute the appropriate script function
	switch scripts.Command(command) {
	case scripts.CommandProvisionUser:
		return scripts.ProvisionUser(req, c.logger)
	case scripts.CommandProvisionAuthorizedKeys:
		return scripts.ProvisionAuthorizedKeys(req, c.logger)
	case scripts.CommandProvisionSudo:
		return scripts.ProvisionSudo(req, c.logger)
	default:
		c.logger.WithField("command", command).Error("Unknown provisioning command")
		return scripts.ProvisioningResult{
			Success: false,
			Error:   fmt.Sprintf("unknown command: %s", command),
		}
	}
}

// WaitUntilConnected waits until the client is connected
func (c *Client) WaitUntilConnected() error {
	return c.rpcClient.WaitUntilConnected()
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

	// Close the JSON-RPC client (this will also close the websocket)
	if err := c.rpcClient.Close(); err != nil {
		c.logger.WithError(err).Warn("Error closing RPC client")
	}

	c.logger.Info("Client shutdown completed")
}

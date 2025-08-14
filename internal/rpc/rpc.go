package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/sourcegraph/jsonrpc2"
	jsonrpc2websocket "github.com/sourcegraph/jsonrpc2/websocket"
)

type MethodHandler func(ctx context.Context, params json.RawMessage) (interface{}, error)

type Client struct {
	mu          sync.RWMutex
	methods     map[string]MethodHandler
	conn        *jsonrpc2.Conn
	ctx         context.Context
	cancel      context.CancelFunc
	wsConn      *websocket.Conn
	connected   chan struct{}
	onConnected func()
}

func NewClient() *Client {
	ctx, cancel := context.WithCancel(context.Background())

	return &Client{
		methods:   make(map[string]MethodHandler),
		ctx:       ctx,
		cancel:    cancel,
		connected: make(chan struct{}, 1),
	}
}

func (c *Client) SetOnConnected(callback func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onConnected = callback
}

func (c *Client) ConnectWebSocket(wsConn *websocket.Conn) error {
	c.mu.Lock()
	c.wsConn = wsConn
	c.mu.Unlock()

	stream := jsonrpc2websocket.NewObjectStream(wsConn)

	conn := jsonrpc2.NewConn(c.ctx, stream, c)

	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()

	select {
	case c.connected <- struct{}{}:
	default:
	}

	c.mu.RLock()
	onConnected := c.onConnected
	c.mu.RUnlock()

	if onConnected != nil {
		go onConnected()
	}

	return nil
}

func (c *Client) Handle(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) {
	if req.Method == "" {
		return
	}

	c.mu.RLock()
	handler, exists := c.methods[req.Method]
	c.mu.RUnlock()

	if !exists {
		conn.ReplyWithError(ctx, req.ID, &jsonrpc2.Error{
			Code:    jsonrpc2.CodeMethodNotFound,
			Message: fmt.Sprintf("method %q not found", req.Method),
		})
		return
	}

	var params json.RawMessage
	if req.Params != nil {
		params = *req.Params
	}

	result, err := handler(ctx, params)
	if err != nil {
		conn.ReplyWithError(ctx, req.ID, &jsonrpc2.Error{
			Code:    jsonrpc2.CodeInternalError,
			Message: err.Error(),
		})
		return
	}

	conn.Reply(ctx, req.ID, result)
}

func (c *Client) AddMethod(method string, handler MethodHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.methods[method] = handler
}

func (c *Client) Call(method string, params interface{}) (json.RawMessage, error) {
	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	if conn == nil {
		return nil, fmt.Errorf("not connected")
	}

	var result json.RawMessage
	err := conn.Call(c.ctx, method, params, &result)
	if err != nil {
		return nil, fmt.Errorf("RPC call failed: %w", err)
	}

	return result, nil
}

func (c *Client) WaitUntilConnected() error {
	select {
	case <-c.connected:
		return nil
	case <-c.ctx.Done():
		return c.ctx.Err()
	}
}

func (c *Client) Close() error {
	c.cancel()

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}

	if c.wsConn != nil {
		c.wsConn.Close()
		c.wsConn = nil
	}

	return nil
}

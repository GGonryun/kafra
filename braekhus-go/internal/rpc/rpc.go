package rpc

import (
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
)

// Request represents a JSON-RPC 2.0 request
type Request struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
	ID      interface{} `json:"id"`
}

// Response represents a JSON-RPC 2.0 response
type Response struct {
	JSONRPC string      `json:"jsonrpc"`
	Result  interface{} `json:"result,omitempty"`
	Error   *Error      `json:"error,omitempty"`
	ID      interface{} `json:"id"`
}

// Error represents a JSON-RPC 2.0 error
type Error struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// MethodHandler is a function that handles RPC method calls
type MethodHandler func(params interface{}) (interface{}, error)

// Client handles JSON-RPC communication
type Client struct {
	mu           sync.RWMutex
	nextID       int64
	methods      map[string]MethodHandler
	pendingCalls map[interface{}]chan *Response
	sendFunc     func([]byte) error
}

// NewClient creates a new JSON-RPC client
func NewClient(sendFunc func([]byte) error) *Client {
	return &Client{
		methods:      make(map[string]MethodHandler),
		pendingCalls: make(map[interface{}]chan *Response),
		sendFunc:     sendFunc,
	}
}

// AddMethod registers a method handler
func (c *Client) AddMethod(method string, handler MethodHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.methods[method] = handler
}

// Call makes a JSON-RPC call
func (c *Client) Call(method string, params interface{}) (interface{}, error) {
	id := atomic.AddInt64(&c.nextID, 1)
	
	request := Request{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      id,
	}
	
	data, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	
	// Create response channel
	respChan := make(chan *Response, 1)
	c.mu.Lock()
	c.pendingCalls[id] = respChan
	c.mu.Unlock()
	
	// Send request
	if err := c.sendFunc(data); err != nil {
		c.mu.Lock()
		delete(c.pendingCalls, id)
		c.mu.Unlock()
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	
	// Wait for response
	response := <-respChan
	
	if response.Error != nil {
		return nil, fmt.Errorf("RPC error %d: %s", response.Error.Code, response.Error.Message)
	}
	
	return response.Result, nil
}

// HandleMessage processes incoming JSON-RPC messages
func (c *Client) HandleMessage(data []byte) error {
	// Try to parse as response first
	var response Response
	if err := json.Unmarshal(data, &response); err == nil && response.ID != nil {
		return c.handleResponse(&response)
	}
	
	// Try to parse as request
	var request Request
	if err := json.Unmarshal(data, &request); err == nil && request.Method != "" {
		return c.handleRequest(&request)
	}
	
	return fmt.Errorf("invalid JSON-RPC message")
}

// handleResponse handles incoming responses
func (c *Client) handleResponse(response *Response) error {
	c.mu.Lock()
	respChan, exists := c.pendingCalls[response.ID]
	if exists {
		delete(c.pendingCalls, response.ID)
	}
	c.mu.Unlock()
	
	if exists {
		respChan <- response
		return nil
	}
	
	return fmt.Errorf("unexpected response with ID %v", response.ID)
}

// handleRequest handles incoming method calls
func (c *Client) handleRequest(request *Request) error {
	c.mu.RLock()
	handler, exists := c.methods[request.Method]
	c.mu.RUnlock()
	
	var response Response
	response.JSONRPC = "2.0"
	response.ID = request.ID
	
	if !exists {
		response.Error = &Error{
			Code:    -32601,
			Message: "Method not found",
		}
	} else {
		result, err := handler(request.Params)
		if err != nil {
			response.Error = &Error{
				Code:    -32603,
				Message: "Internal error",
				Data:    err.Error(),
			}
		} else {
			response.Result = result
		}
	}
	
	// Send response
	data, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}
	
	return c.sendFunc(data)
}
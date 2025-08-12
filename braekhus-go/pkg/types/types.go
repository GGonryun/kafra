package types

import "net/http"

// ForwardedRequest represents a request to be forwarded to the target service
type ForwardedRequest struct {
	Headers http.Header            `json:"headers"`
	Method  string                 `json:"method"`
	Path    string                 `json:"path"`
	Params  map[string]interface{} `json:"params"`
	Data    interface{}            `json:"data"`
	Options *ForwardedRequestOptions `json:"options,omitempty"`
}

// ForwardedRequestOptions contains options for forwarded requests
type ForwardedRequestOptions struct {
	TimeoutMillis *int `json:"timeoutMillis,omitempty"`
}

// ForwardedResponse represents a response from the target service
type ForwardedResponse struct {
	Headers    map[string]interface{} `json:"headers"`
	Status     int                    `json:"status"`
	StatusText string                 `json:"statusText"`
	Data       interface{}            `json:"data"`
}

// Config holds the client configuration
type Config struct {
	TargetURL    string `json:"targetUrl"`
	ClientID     string `json:"clientId"`
	JWKPath      string `json:"jwkPath"`
	TunnelHost   string `json:"tunnelHost"`
	TunnelPort   int    `json:"tunnelPort"`
	Insecure     bool   `json:"insecure"`
}

// SetClientIDRequest is used for the setClientId RPC call
type SetClientIDRequest struct {
	ClientID string `json:"clientId"`
}
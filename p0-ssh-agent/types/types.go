package types

// ForwardedRequest represents a request to be forwarded to the target service
type ForwardedRequest struct {
	Headers map[string]interface{}   `json:"headers"`
	Method  string                   `json:"method"`
	Path    string                   `json:"path"`
	Params  map[string]interface{}   `json:"params"`
	Data    interface{}              `json:"data"`
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
	Version         string   `json:"version" yaml:"version"`
	OrgID           string   `json:"orgId" yaml:"orgId"`
	HostID          string   `json:"hostId" yaml:"hostId"`
	KeyPath         string   `json:"keyPath" yaml:"keyPath"`
	LogPath         string   `json:"logPath" yaml:"logPath"`
	TunnelHost      string   `json:"tunnelHost" yaml:"tunnelHost"` // WebSocket URL like ws://localhost:8079 or wss://example.ngrok.app
	Labels          []string `json:"labels" yaml:"labels"`
	Environment     string   `json:"environment" yaml:"environment"`
	TunnelTimeoutMs int      `json:"tunnelTimeoutMs" yaml:"tunnelTimeoutMs"`
	DryRun          bool     `json:"dryRun" yaml:"dryRun"` // If true, log commands but don't execute them
}

// GetClientID returns the computed client ID in the format ${orgId}:${hostId}:ssh
func (c *Config) GetClientID() string {
	return c.OrgID + ":" + c.HostID + ":ssh"
}

// SetClientIDRequest is used for the setClientId RPC call
type SetClientIDRequest struct {
	ClientID string `json:"clientId"`
}

// RegistrationRequest represents the machine registration request
type RegistrationRequest struct {
	HostID               string            `json:"hostId"`
	Hostname             string            `json:"hostname"`
	PublicIP             string            `json:"publicIp"`
	Fingerprint          string            `json:"fingerprint"`
	FingerprintPublicKey string            `json:"fingerprintPublicKey"`
	JWKPublicKey         map[string]string `json:"jwkPublicKey"`
	EnvironmentID        string            `json:"environmentId"`
	OrgID                string            `json:"orgId"`
	Labels               []string          `json:"labels,omitempty"`
	Timestamp            string            `json:"timestamp"`
}

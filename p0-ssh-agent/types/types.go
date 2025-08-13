package types

// ForwardedRequest represents a request to be forwarded to the target service
type ForwardedRequest struct {
	Headers map[string]interface{}   `json:"headers"`
	Method  string                   `json:"method"`
	Path    string                   `json:"path"`
	Params  map[string]interface{}   `json:"params"`
	Data    interface{}              `json:"data"`
	Command string                   `json:"command,omitempty"` // Command type for script execution
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
	TargetURL       string   `json:"targetUrl" yaml:"targetUrl"`
	KeyPath         string   `json:"keyPath" yaml:"keyPath"` // Unified path for both JWK and keygen
	LogPath         string   `json:"logPath" yaml:"logPath"` // Path for daemon log files
	TunnelHost      string   `json:"tunnelHost" yaml:"tunnelHost"`
	TunnelPort      int      `json:"tunnelPort" yaml:"tunnelPort"`
	TunnelPath      string   `json:"tunnelPath" yaml:"tunnelPath"`
	Insecure        bool     `json:"insecure" yaml:"insecure"`
	Labels          []string `json:"labels" yaml:"labels"`
	Environment     string   `json:"environment" yaml:"environment"`
	TunnelTimeoutMs int      `json:"tunnelTimeoutMs" yaml:"tunnelTimeoutMs"`

	// Deprecated fields (for backward compatibility)
	TenantID   string `json:"tenantId,omitempty" yaml:"tenantId,omitempty"`
	JWKPath    string `json:"jwkPath,omitempty" yaml:"jwkPath,omitempty"`
	KeygenPath string `json:"keygenPath,omitempty" yaml:"keygenPath,omitempty"`
}

// GetOrgID returns the organization ID, with backward compatibility for tenantId
func (c *Config) GetOrgID() string {
	if c.OrgID != "" {
		return c.OrgID
	}
	// Backward compatibility: use TenantID if OrgID is not set
	return c.TenantID
}

// GetClientID returns the computed client ID in the format ${orgId}:${hostId}:ssh
func (c *Config) GetClientID() string {
	return c.GetOrgID() + ":" + c.HostID + ":ssh"
}

// GetKeyPath returns the unified key path, with backward compatibility
func (c *Config) GetKeyPath() string {
	if c.KeyPath != "" {
		return c.KeyPath
	}
	// Backward compatibility: prefer JWKPath over KeygenPath
	if c.JWKPath != "" {
		return c.JWKPath
	}
	return c.KeygenPath
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

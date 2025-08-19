package types

import (
	"time"
)

type ForwardedRequest struct {
	Headers map[string]interface{}   `json:"headers"`
	Method  string                   `json:"method"`
	Path    string                   `json:"path"`
	Params  map[string]interface{}   `json:"params"`
	Data    interface{}              `json:"data"`
	Options *ForwardedRequestOptions `json:"options,omitempty"`
}

type ForwardedRequestOptions struct {
	TimeoutMillis *int `json:"timeoutMillis,omitempty"`
}

type ForwardedResponse struct {
	Headers    map[string]interface{} `json:"headers"`
	Status     int                    `json:"status"`
	StatusText string                 `json:"statusText"`
	Data       interface{}            `json:"data"`
}

type Config struct {
	Version                  string   `json:"version" yaml:"version"`
	OrgID                    string   `json:"orgId" yaml:"orgId"`
	HostID                   string   `json:"hostId" yaml:"hostId"`
	Hostname                 string   `json:"hostname" yaml:"hostname"`
	KeyPath                  string   `json:"keyPath" yaml:"keyPath"`
	TunnelHost               string   `json:"tunnelHost" yaml:"tunnelHost"`
	Labels                   []string `json:"labels" yaml:"labels"`
	Environment              string   `json:"environment" yaml:"environment"`
	HeartbeatIntervalSeconds int      `json:"heartbeatIntervalSeconds" yaml:"heartbeatIntervalSeconds"`
	DryRun                   bool     `json:"dryRun" yaml:"dryRun"`
}

func (c *Config) GetClientID() string {
	return c.OrgID + ":" + c.HostID + ":ssh"
}


func (c *Config) GetHeartbeatInterval() time.Duration {
	return time.Duration(c.HeartbeatIntervalSeconds) * time.Second
}


type SetClientIDRequest struct {
	ClientID string `json:"clientId"`
}

type RegistrationRequest struct {
	HostID               string            `json:"hostId"`
	ClientID             string            `json:"clientId"`
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

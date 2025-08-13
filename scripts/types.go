package scripts

// ProvisioningRequest represents the common parameters for all provisioning operations
type ProvisioningRequest struct {
	UserName  string `json:"userName"`
	Action    string `json:"action"`    // "grant" or "revoke"
	RequestID string `json:"requestId"` // P0 access request identifier
	PublicKey string `json:"publicKey,omitempty"`
	Sudo      bool   `json:"sudo,omitempty"`
}

// ProvisioningResult represents the result of a provisioning operation
type ProvisioningResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
}

// Command represents the command type for different provisioning operations
type Command string

const (
	CommandProvisionUser           Command = "provisionUser"
	CommandProvisionAuthorizedKeys Command = "provisionAuthorizedKeys"
	CommandProvisionSudo           Command = "provisionSudo"
	CommandProvisionSession        Command = "provisionSession"
)
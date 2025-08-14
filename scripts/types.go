package scripts

type ProvisioningRequest struct {
	UserName  string `json:"userName"`
	Action    string `json:"action"`
	RequestID string `json:"requestId"`
	PublicKey string `json:"publicKey,omitempty"`
	Sudo      bool   `json:"sudo,omitempty"`
}

type ProvisioningResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
}

type Command string

const (
	CommandProvisionUser           Command = "provisionUser"
	CommandProvisionAuthorizedKeys Command = "provisionAuthorizedKeys"
	CommandProvisionSudo           Command = "provisionSudo"
	CommandProvisionSession        Command = "provisionSession"
)

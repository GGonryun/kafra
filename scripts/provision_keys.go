package scripts

import (
	"fmt"
	"os/user"
	"path/filepath"

	"github.com/sirupsen/logrus"
)

func ProvisionAuthorizedKeys(req ProvisioningRequest, logger *logrus.Logger) ProvisioningResult {
	logger.WithFields(logrus.Fields{
		"username":    req.UserName,
		"action":      req.Action,
		"request_id":  req.RequestID,
		"has_pub_key": req.PublicKey != "" && req.PublicKey != "N/A",
	}).Info("🔑 Provisioning authorized keys")

	if (req.PublicKey == "" || req.PublicKey == "N/A") && req.Action == "grant" {
		return ProvisioningResult{
			Success: true,
			Message: "No public key provided, skipping authorized keys provisioning",
		}
	}

	userInfo, err := user.Lookup(req.UserName)
	if err != nil {
		return ProvisioningResult{
			Success: false,
			Error:   fmt.Sprintf("user %s not found: %v", req.UserName, err),
		}
	}

	authorizedKeysPath := filepath.Join(userInfo.HomeDir, ".ssh", "authorized_keys")

	switch req.Action {
	case "grant":
		return grantAuthorizedKey(req.PublicKey, req.RequestID, authorizedKeysPath, req.UserName, logger)
	case "revoke":
		return revokeAuthorizedKey(req.RequestID, authorizedKeysPath, logger)
	default:
		return ProvisioningResult{
			Success: false,
			Error:   "invalid action: must be 'grant' or 'revoke'",
		}
	}
}

func grantAuthorizedKey(publicKey, requestID, authorizedKeysPath, username string, logger *logrus.Logger) ProvisioningResult {
	logger.WithFields(logrus.Fields{
		"path":       authorizedKeysPath,
		"username":   username,
		"request_id": requestID,
	}).Debug("Granting SSH key access")

	result := ensureContentInFile(publicKey, requestID, authorizedKeysPath, "600", username, logger)
	if !result.Success {
		return result
	}

	return ProvisioningResult{
		Success: true,
		Message: fmt.Sprintf("SSH public key added to %s successfully", authorizedKeysPath),
	}
}

func revokeAuthorizedKey(requestID, authorizedKeysPath string, logger *logrus.Logger) ProvisioningResult {
	logger.WithFields(logrus.Fields{
		"path":       authorizedKeysPath,
		"request_id": requestID,
	}).Debug("Revoking SSH key access")

	result := removeContentFromFile(requestID, authorizedKeysPath, logger)
	if !result.Success {
		return result
	}

	return ProvisioningResult{
		Success: true,
		Message: fmt.Sprintf("SSH public key removed from %s successfully", authorizedKeysPath),
	}
}

// ProvisionCAKeys provisions CA public keys with cert-authority and principals parameters
func ProvisionCAKeys(req ProvisioningRequest, logger *logrus.Logger) ProvisioningResult {
	logger.WithFields(logrus.Fields{
		"username":   req.UserName,
		"action":     req.Action,
		"request_id": req.RequestID,
		"has_ca_key": req.CAPublicKey != "" && req.CAPublicKey != "N/A",
	}).Info("🔐 Provisioning CA keys")

	if (req.CAPublicKey == "" || req.CAPublicKey == "N/A") && req.Action == "grant" {
		return ProvisioningResult{
			Success: false,
			Message: "No CA public key provided, skipping CA keys provisioning",
		}
	}

	userInfo, err := user.Lookup(req.UserName)
	if err != nil {
		return ProvisioningResult{
			Success: false,
			Error:   fmt.Sprintf("user %s not found: %v", req.UserName, err),
		}
	}

	authorizedKeysPath := filepath.Join(userInfo.HomeDir, ".ssh", "authorized_keys")

	switch req.Action {
	case "grant":
		return grantCAKey(req.CAPublicKey, req.RequestID, authorizedKeysPath, req.UserName, logger)
	case "revoke":
		return revokeCAKey(req.RequestID, authorizedKeysPath, logger)
	default:
		return ProvisioningResult{
			Success: false,
			Error:   "invalid action: must be 'grant' or 'revoke'",
		}
	}
}

func grantCAKey(caPublicKey, requestID, authorizedKeysPath, username string, logger *logrus.Logger) ProvisioningResult {
	logger.WithFields(logrus.Fields{
		"path":       authorizedKeysPath,
		"username":   username,
		"request_id": requestID,
	}).Debug("Granting CA key access")

	// Format CA key with cert-authority and principals parameters
	caKeyEntry := fmt.Sprintf("cert-authority,principals=\"%s\" %s", username, caPublicKey)

	result := ensureContentInFile(caKeyEntry, requestID, authorizedKeysPath, "600", username, logger)
	if !result.Success {
		return result
	}

	return ProvisioningResult{
		Success: true,
		Message: fmt.Sprintf("CA public key added to %s successfully with %s", authorizedKeysPath, caKeyEntry),
	}
}

func revokeCAKey(requestID, authorizedKeysPath string, logger *logrus.Logger) ProvisioningResult {
	logger.WithFields(logrus.Fields{
		"path":       authorizedKeysPath,
		"request_id": requestID,
	}).Debug("Revoking CA key access")

	result := removeContentFromFile(requestID, authorizedKeysPath, logger)
	if !result.Success {
		return result
	}

	return ProvisioningResult{
		Success: true,
		Message: fmt.Sprintf("CA public key removed from %s successfully", authorizedKeysPath),
	}
}

// ValidateCAPublicKey validates that a CA public key is properly formatted
func ValidateCAPublicKey(caPublicKey string) error {
	if caPublicKey == "" {
		return fmt.Errorf("CA public key cannot be empty")
	}

	// Basic validation - should start with key type
	validPrefixes := []string{"ssh-rsa", "ssh-ed25519", "ecdsa-sha2-", "ssh-dss"}
	for _, prefix := range validPrefixes {
		if len(caPublicKey) > len(prefix) && caPublicKey[:len(prefix)] == prefix {
			return nil
		}
	}

	return fmt.Errorf("CA public key does not appear to be a valid SSH public key")
}

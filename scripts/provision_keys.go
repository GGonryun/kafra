package scripts

import (
	"fmt"
	"os/user"
	"path/filepath"

	"github.com/sirupsen/logrus"
)

// ProvisionAuthorizedKeys manages SSH authorized keys for users
func ProvisionAuthorizedKeys(req ProvisioningRequest, logger *logrus.Logger) ProvisioningResult {
	logger.WithFields(logrus.Fields{
		"username":    req.UserName,
		"action":      req.Action,
		"request_id":  req.RequestID,
		"has_pub_key": req.PublicKey != "" && req.PublicKey != "N/A",
	}).Info("🔑 Provisioning authorized keys")

	// Skip if no public key provided, but only for grant operations
	// Revoke operations use requestID and don't need the public key
	if (req.PublicKey == "" || req.PublicKey == "N/A") && req.Action == "grant" {
		return ProvisioningResult{
			Success: true,
			Message: "No public key provided, skipping authorized keys provisioning",
		}
	}

	// Get user info
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

// grantAuthorizedKey adds an SSH public key to the user's authorized_keys file
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

// revokeAuthorizedKey removes an SSH public key from the user's authorized_keys file
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
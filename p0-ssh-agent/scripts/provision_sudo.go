package scripts

import (
	"fmt"

	"github.com/sirupsen/logrus"
)

// ProvisionSudo manages sudo access for users
func ProvisionSudo(req ProvisioningRequest, logger *logrus.Logger) ProvisioningResult {
	logger.WithFields(logrus.Fields{
		"username":   req.UserName,
		"action":     req.Action,
		"request_id": req.RequestID,
		"sudo":       req.Sudo,
	}).Info("⚡ Provisioning sudo access")

	// Skip if sudo not requested
	if !req.Sudo {
		return ProvisioningResult{
			Success: true,
			Message: "Sudo access not requested, skipping sudo provisioning",
		}
	}

	sudoersFile := "/etc/sudoers-p0"
	sudoRule := fmt.Sprintf("%s ALL=(ALL) NOPASSWD: ALL", req.UserName)

	switch req.Action {
	case "grant":
		return grantSudoAccess(sudoRule, req.RequestID, sudoersFile, logger)
	case "revoke":
		return revokeSudoAccess(req.RequestID, sudoersFile, logger)
	default:
		return ProvisioningResult{
			Success: false,
			Error:   "invalid action: must be 'grant' or 'revoke'",
		}
	}
}

// grantSudoAccess grants passwordless sudo access to a user
func grantSudoAccess(sudoRule, requestID, sudoersFile string, logger *logrus.Logger) ProvisioningResult {
	logger.WithFields(logrus.Fields{
		"rule":       sudoRule,
		"request_id": requestID,
		"file":       sudoersFile,
	}).Debug("Granting sudo access")

	// Ensure the user has sudo access
	result := ensureContentInFile(sudoRule, requestID, sudoersFile, "440", "root", logger)
	if !result.Success {
		return result
	}

	// Ensure the include line is in /etc/sudoers
	includeResult := ensureLineInFile("#include sudoers-p0", "/etc/sudoers", logger)
	if !includeResult.Success {
		return includeResult
	}

	return ProvisioningResult{
		Success: true,
		Message: fmt.Sprintf("Sudo access granted successfully for rule: %s", sudoRule),
	}
}

// revokeSudoAccess removes sudo access for a user based on RequestID
func revokeSudoAccess(requestID, sudoersFile string, logger *logrus.Logger) ProvisioningResult {
	logger.WithFields(logrus.Fields{
		"request_id": requestID,
		"file":       sudoersFile,
	}).Debug("Revoking sudo access")

	result := removeContentFromFile(requestID, sudoersFile, logger)
	if !result.Success {
		return result
	}

	return ProvisioningResult{
		Success: true,
		Message: fmt.Sprintf("Sudo access revoked successfully for RequestID: %s", requestID),
	}
}
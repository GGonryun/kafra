package scripts

import (
	"fmt"
	"os/user"

	"github.com/sirupsen/logrus"

	"p0-ssh-agent/internal/osplugins"
)

func ProvisionUser(req ProvisioningRequest, logger *logrus.Logger) ProvisioningResult {
	logger.WithFields(logrus.Fields{
		"username":   req.UserName,
		"action":     req.Action,
		"request_id": req.RequestID,
	}).Info("🧑 Provisioning user")

	if !isValidUsername(req.UserName) {
		return ProvisioningResult{
			Success: false,
			Error:   "invalid username format: must match ^[a-z][-a-z0-9_]*$",
		}
	}

	switch req.Action {
	case "grant":
		return ensureUserExists(req, logger)
	case "revoke":
		return ProvisioningResult{
			Success: true,
			Message: "User access revocation handled by other provisioning functions",
		}
	default:
		return ProvisioningResult{
			Success: false,
			Error:   "invalid action: must be 'grant' or 'revoke'",
		}
	}
}

func ensureUserExists(req ProvisioningRequest, logger *logrus.Logger) ProvisioningResult {
	if _, err := user.Lookup(req.UserName); err == nil {
		logger.WithField("username", req.UserName).Debug("User already exists")
		return ProvisioningResult{
			Success: true,
			Message: "User already exists",
		}
	}

	// Get the appropriate OS plugin
	osPlugin, err := osplugins.GetPlugin(logger)
	if err != nil {
		return ProvisioningResult{
			Success: false,
			Error:   fmt.Sprintf("failed to get OS plugin: %v", err),
		}
	}

	logger.WithFields(logrus.Fields{
		"username":  req.UserName,
		"os_plugin": osPlugin.GetName(),
	}).Info("Creating new JIT user")

	// Use the OS plugin to create the JIT user
	if err := osPlugin.CreateUser(req.UserName, logger); err != nil {
		return ProvisioningResult{
			Success: false,
			Error:   fmt.Sprintf("failed to create user with %s plugin: %v", osPlugin.GetName(), err),
		}
	}

	return ProvisioningResult{
		Success: true,
		Message: fmt.Sprintf("User %s created successfully with %s plugin", req.UserName, osPlugin.GetName()),
	}
}

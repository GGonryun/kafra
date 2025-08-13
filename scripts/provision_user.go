package scripts

import (
	"fmt"
	"os/exec"
	"os/user"
	"strconv"

	"github.com/sirupsen/logrus"
)

// ProvisionUser creates or manages user accounts based on the action (grant/revoke)
func ProvisionUser(req ProvisioningRequest, logger *logrus.Logger) ProvisioningResult {
	logger.WithFields(logrus.Fields{
		"username":   req.UserName,
		"action":     req.Action,
		"request_id": req.RequestID,
	}).Info("🧑 Provisioning user")

	// Validate username format
	if !isValidUsername(req.UserName) {
		return ProvisioningResult{
			Success: false,
			Error:   "invalid username format: must match ^[a-z][-a-z0-9_]*$",
		}
	}

	switch req.Action {
	case "grant":
		return ensureUserExists(req.UserName, logger)
	case "revoke":
		// For revoke, we typically don't delete the user, just remove access
		// The actual access removal is handled by other provisioning functions
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

// ensureUserExists creates a user if they don't already exist
func ensureUserExists(username string, logger *logrus.Logger) ProvisioningResult {
	// Check if user already exists
	if _, err := user.Lookup(username); err == nil {
		logger.WithField("username", username).Debug("User already exists")
		return ProvisioningResult{
			Success: true,
			Message: "User already exists",
		}
	}

	// Find available UID
	newUID, err := findNextAvailableUID()
	if err != nil {
		return ProvisioningResult{
			Success: false,
			Error:   err.Error(),
		}
	}

	logger.WithFields(logrus.Fields{
		"username": username,
		"uid":      newUID,
	}).Info("Creating new user")

	// Try different user creation commands based on system
	if err := createUserWithUseradd(username, newUID, logger); err != nil {
		if err := createUserWithAdduser(username, newUID, logger); err != nil {
			return ProvisioningResult{
				Success: false,
				Error:   "failed to create user: neither useradd nor adduser succeeded",
			}
		}
	}

	return ProvisioningResult{
		Success: true,
		Message: fmt.Sprintf("User %s created successfully with UID %d", username, newUID),
	}
}

// createUserWithUseradd creates a user using the useradd/groupadd commands (RHEL/CentOS style)
func createUserWithUseradd(username string, uid int, logger *logrus.Logger) error {
	// Check if commands exist
	if !commandExists("groupadd") || !commandExists("useradd") {
		return fmt.Errorf("groupadd or useradd not found")
	}

	logger.Debug("Creating user with useradd/groupadd")

	// Create group first
	cmd := exec.Command("sudo", "groupadd", "-g", strconv.Itoa(uid), username)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create group: %v", err)
	}

	// Create user
	cmd = exec.Command("sudo", "useradd", "-m", "-u", strconv.Itoa(uid), "-g", strconv.Itoa(uid), username, "-s", "/bin/bash")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create user: %v", err)
	}

	return nil
}

// createUserWithAdduser creates a user using the adduser command (Debian/Ubuntu style)
func createUserWithAdduser(username string, uid int, logger *logrus.Logger) error {
	if !commandExists("adduser") {
		return fmt.Errorf("adduser not found")
	}

	logger.Debug("Creating user with adduser")

	cmd := exec.Command("sudo", "adduser", "-u", strconv.Itoa(uid), "--gecos", username, "--disabled-password", "--shell", "/bin/bash", username)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create user with adduser: %v", err)
	}

	return nil
}
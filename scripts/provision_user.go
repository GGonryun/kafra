package scripts

import (
	"fmt"
	"os/exec"
	"os/user"
	"strconv"

	"github.com/sirupsen/logrus"
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
		return ensureUserExists(req.UserName, logger)
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

func ensureUserExists(username string, logger *logrus.Logger) ProvisioningResult {
	if _, err := user.Lookup(username); err == nil {
		logger.WithField("username", username).Debug("User already exists")
		return ProvisioningResult{
			Success: true,
			Message: "User already exists",
		}
	}

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

func createUserWithUseradd(username string, uid int, logger *logrus.Logger) error {
	if !commandExists("groupadd") || !commandExists("useradd") {
		return fmt.Errorf("groupadd or useradd not found")
	}

	logger.Debug("Creating user with useradd/groupadd")

	cmd := exec.Command("sudo", "groupadd", "-g", strconv.Itoa(uid), username)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create group: %v", err)
	}

	cmd = exec.Command("sudo", "useradd", "-m", "-u", strconv.Itoa(uid), "-g", strconv.Itoa(uid), username, "-s", "/bin/bash")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create user: %v", err)
	}

	return nil
}

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
package scripts

import (
	"fmt"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

func ProvisionSession(req ProvisioningRequest, logger *logrus.Logger) ProvisioningResult {
	logger.WithFields(logrus.Fields{
		"username":   req.UserName,
		"action":     req.Action,
		"request_id": req.RequestID,
	}).Info("🔌 Provisioning SSH session")

	if !isValidUsername(req.UserName) {
		return ProvisioningResult{
			Success: false,
			Error:   "invalid username format: must match ^[a-z][-a-z0-9_]*$",
		}
	}

	if req.Action != "revoke" {
		return ProvisioningResult{
			Success: false,
			Error:   "ProvisionSession only supports 'revoke' action to terminate SSH connections",
		}
	}

	return killUserSSHConnections(req.UserName, logger)
}

func killUserSSHConnections(username string, logger *logrus.Logger) ProvisioningResult {
	logger.WithField("username", username).Info("🔍 Terminating all user sessions and processes")

	// Method 1: Try systemd user slice termination first (most effective on systemd systems)
	terminated := false
	if commandExists("systemctl") {
		logger.Debug("Attempting to terminate user slice via systemctl")
		cmd := exec.Command("sudo", "systemctl", "kill", fmt.Sprintf("user-%s.slice", username))
		if err := cmd.Run(); err != nil {
			logger.WithError(err).Debug("Failed to kill user slice, falling back to process-level termination")
		} else {
			logger.Info("User slice terminated via systemctl")
			terminated = true
		}
	}

	// Method 2: Get user ID and find all processes owned by the user
	userInfo, err := user.Lookup(username)
	if err != nil {
		return ProvisioningResult{
			Success: false,
			Error:   fmt.Sprintf("failed to lookup user %s: %v", username, err),
		}
	}

	// Find all processes owned by the user using pgrep
	cmd := exec.Command("pgrep", "-u", userInfo.Uid)
	output, err := cmd.Output()
	if err != nil {
		// No processes found is not an error
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			logger.WithField("username", username).Info("ℹ️ No active processes found for user")
			if terminated {
				return ProvisioningResult{
					Success: true,
					Message: fmt.Sprintf("Successfully terminated user slice for %s", username),
				}
			}
			return ProvisioningResult{
				Success: true,
				Message: fmt.Sprintf("No active processes found for user %s", username),
			}
		}
		return ProvisioningResult{
			Success: false,
			Error:   fmt.Sprintf("failed to find user processes: %v", err),
		}
	}

	if len(output) == 0 {
		logger.WithField("username", username).Info("ℹ️ No active processes found for user")
		return ProvisioningResult{
			Success: true,
			Message: fmt.Sprintf("No active processes found for user %s", username),
		}
	}

	// Parse PIDs
	pidLines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var validPids []string
	for _, pidStr := range pidLines {
		pidStr = strings.TrimSpace(pidStr)
		if pidStr != "" {
			if _, err := strconv.Atoi(pidStr); err == nil {
				validPids = append(validPids, pidStr)
			}
		}
	}

	if len(validPids) == 0 {
		logger.WithField("username", username).Info("ℹ️ No valid PIDs found for user")
		return ProvisioningResult{
			Success: true,
			Message: fmt.Sprintf("No active processes found for user %s", username),
		}
	}

	logger.WithFields(logrus.Fields{
		"username": username,
		"pid_count": len(validPids),
		"pids": strings.Join(validPids, ","),
	}).Info("🎯 Found user processes to terminate")

	// Kill processes gracefully first (SIGTERM)
	cmd = exec.Command("sudo", "pkill", "-TERM", "-u", userInfo.Uid)
	if err := cmd.Run(); err != nil {
		logger.WithError(err).Debug("SIGTERM failed, trying SIGKILL")
	} else {
		logger.Debug("Sent SIGTERM to user processes")
		// Give processes a moment to terminate gracefully
		time.Sleep(2 * time.Second)
	}

	// Force kill remaining processes (SIGKILL)
	cmd = exec.Command("sudo", "pkill", "-KILL", "-u", userInfo.Uid)
	if err := cmd.Run(); err != nil {
		logger.WithError(err).Debug("SIGKILL failed - processes may have already terminated")
	} else {
		logger.Debug("Sent SIGKILL to remaining user processes")
	}

	// Verify termination by checking if processes still exist
	cmd = exec.Command("pgrep", "-u", userInfo.Uid)
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			logger.WithFields(logrus.Fields{
				"username": username,
				"terminated_count": len(validPids),
			}).Info("✅ All user processes terminated successfully")

			return ProvisioningResult{
				Success: true,
				Message: fmt.Sprintf("Successfully terminated %d processes for user %s", len(validPids), username),
			}
		}
	}

	// Some processes may still be running, but we've done our best
	logger.WithField("username", username).Warn("Some processes may still be running, but termination signals were sent")
	return ProvisioningResult{
		Success: true,
		Message: fmt.Sprintf("Termination signals sent to %d processes for user %s", len(validPids), username),
	}
}
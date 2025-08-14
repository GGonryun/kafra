package scripts

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

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
	logger.WithField("username", username).Info("🔍 Finding SSH connections for user")

	cmd := exec.Command("ps", "aux")
	output, err := cmd.Output()
	if err != nil {
		return ProvisioningResult{
			Success: false,
			Error:   fmt.Sprintf("failed to list processes: %v", err),
		}
	}

	var pidsToKill []string
	lines := strings.Split(string(output), "\n")
	
	for _, line := range lines {
		if strings.Contains(line, "sshd:") && strings.Contains(line, username+"@") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				pid := fields[1]
				if _, err := strconv.Atoi(pid); err == nil {
					pidsToKill = append(pidsToKill, pid)
					logger.WithFields(logrus.Fields{
						"pid":      pid,
						"username": username,
					}).Info("🎯 Found SSH connection to terminate")
				}
			}
		}
	}

	if len(pidsToKill) == 0 {
		logger.WithField("username", username).Info("ℹ️ No active SSH connections found for user")
		return ProvisioningResult{
			Success: true,
			Message: fmt.Sprintf("No active SSH connections found for user %s", username),
		}
	}

	killedCount := 0
	var errors []string

	for _, pid := range pidsToKill {
		logger.WithFields(logrus.Fields{
			"pid":      pid,
			"username": username,
		}).Info("🔪 Terminating SSH connection")

		cmd := exec.Command("kill", "-TERM", pid)
		if err := cmd.Run(); err != nil {
			logger.WithField("pid", pid).Warn("SIGTERM failed, trying SIGKILL")
			cmd = exec.Command("kill", "-KILL", pid)
			if err := cmd.Run(); err != nil {
				errMsg := fmt.Sprintf("failed to kill PID %s: %v", pid, err)
				errors = append(errors, errMsg)
				logger.WithError(err).WithField("pid", pid).Error("Failed to kill SSH connection")
				continue
			}
		}
		killedCount++
		logger.WithField("pid", pid).Info("✅ SSH connection terminated successfully")
	}

	if len(errors) > 0 {
		return ProvisioningResult{
			Success: false,
			Error:   fmt.Sprintf("killed %d connections, but failed to kill some: %s", killedCount, strings.Join(errors, "; ")),
		}
	}

	logger.WithFields(logrus.Fields{
		"username":     username,
		"killed_count": killedCount,
	}).Info("✅ All SSH connections terminated successfully")

	return ProvisioningResult{
		Success: true,
		Message: fmt.Sprintf("Successfully terminated %d SSH connections for user %s", killedCount, username),
	}
}
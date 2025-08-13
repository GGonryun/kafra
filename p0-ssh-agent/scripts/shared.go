package scripts

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
)

// isValidUsername validates username format against P0 requirements
func isValidUsername(username string) bool {
	pattern := `^[a-z][-a-z0-9_]*$`
	matched, _ := regexp.MatchString(pattern, username)
	return matched
}

// findNextAvailableUID finds the next available UID in the range 65536-90000
func findNextAvailableUID() (int, error) {
	const minUID, maxUID = 65536, 90000

	for uid := minUID; uid <= maxUID; uid++ {
		if _, err := user.LookupId(strconv.Itoa(uid)); err != nil {
			// UID is available
			return uid, nil
		}
	}

	return 0, fmt.Errorf("no available UID found in range %d-%d", minUID, maxUID)
}

// commandExists checks if a command is available in the system PATH
func commandExists(command string) bool {
	_, err := exec.LookPath(command)
	return err == nil
}

// ensureContentInFile adds content to a file with proper permissions and ownership
func ensureContentInFile(content, requestID, filePath, permission, owner string, logger *logrus.Logger) ProvisioningResult {
	comment := fmt.Sprintf("# RequestID: %s", requestID)

	logger.WithFields(logrus.Fields{
		"file":       filePath,
		"request_id": requestID,
		"owner":      owner,
	}).Debug("Ensuring content in file")

	// Create directory if it doesn't exist
	dir := filepath.Dir(filePath)
	if err := exec.Command("sudo", "mkdir", "-p", dir).Run(); err != nil {
		return ProvisioningResult{
			Success: false,
			Error:   fmt.Sprintf("failed to create directory %s: %v", dir, err),
		}
	}

	// Create file if it doesn't exist
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		if err := exec.Command("sudo", "touch", filePath).Run(); err != nil {
			return ProvisioningResult{
				Success: false,
				Error:   fmt.Sprintf("failed to create file %s: %v", filePath, err),
			}
		}
		if err := exec.Command("sudo", "chmod", permission, filePath).Run(); err != nil {
			return ProvisioningResult{
				Success: false,
				Error:   fmt.Sprintf("failed to set permissions on %s: %v", filePath, err),
			}
		}
	}

	// Check if content already exists
	grepCmd := exec.Command("sudo", "grep", "-qF", comment, filePath)
	commentExists := grepCmd.Run() == nil

	grepCmd = exec.Command("sudo", "grep", "-qF", content, filePath)
	contentExists := grepCmd.Run() == nil

	if commentExists && contentExists {
		logger.Debug("Content already exists in file")
		return ProvisioningResult{
			Success: true,
			Message: "Content already exists in file",
		}
	}

	// Add content to file
	appendCmd := exec.Command("sudo", "tee", "-a", filePath)
	appendCmd.Stdin = strings.NewReader(comment + "\n" + content + "\n")
	if err := appendCmd.Run(); err != nil {
		return ProvisioningResult{
			Success: false,
			Error:   fmt.Sprintf("failed to append content to %s: %v", filePath, err),
		}
	}

	// Set ownership if specified
	if owner != "root" && owner != "" {
		sshDir := filepath.Dir(filePath)
		if err := exec.Command("sudo", "chown", "-R", owner+":"+owner, sshDir).Run(); err != nil {
			logger.WithError(err).Warn("Failed to set ownership, but content was added successfully")
		}
	}

	return ProvisioningResult{
		Success: true,
		Message: fmt.Sprintf("Content added to %s successfully", filePath),
	}
}

// removeContentFromFile removes content associated with a RequestID from a file
func removeContentFromFile(requestID, filePath string, logger *logrus.Logger) ProvisioningResult {
	comment := fmt.Sprintf("# RequestID: %s", requestID)

	logger.WithFields(logrus.Fields{
		"file":       filePath,
		"request_id": requestID,
	}).Debug("Removing content from file")

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return ProvisioningResult{
			Success: true,
			Message: "File does not exist, nothing to remove",
		}
	}

	// Use sed to remove lines from comment to next empty line
	sedPattern := fmt.Sprintf("/^%s$/,/^$/d", regexp.QuoteMeta(comment))
	cmd := exec.Command("sudo", "sed", "-i", sedPattern, filePath)
	if err := cmd.Run(); err != nil {
		return ProvisioningResult{
			Success: false,
			Error:   fmt.Sprintf("failed to remove content from %s: %v", filePath, err),
		}
	}

	return ProvisioningResult{
		Success: true,
		Message: fmt.Sprintf("Content removed from %s successfully", filePath),
	}
}

// ensureLineInFile adds a line to a file if it doesn't already exist
func ensureLineInFile(line, filePath string, logger *logrus.Logger) ProvisioningResult {
	logger.WithFields(logrus.Fields{
		"file": filePath,
		"line": line,
	}).Debug("Ensuring line in file")

	// Check if line already exists
	grepCmd := exec.Command("sudo", "grep", "-qF", line, filePath)
	if grepCmd.Run() == nil {
		return ProvisioningResult{
			Success: true,
			Message: "Line already exists in file",
		}
	}

	// Add line to file
	appendCmd := exec.Command("sudo", "tee", "-a", filePath)
	appendCmd.Stdin = strings.NewReader(line + "\n")
	if err := appendCmd.Run(); err != nil {
		return ProvisioningResult{
			Success: false,
			Error:   fmt.Sprintf("failed to append line to %s: %v", filePath, err),
		}
	}

	return ProvisioningResult{
		Success: true,
		Message: fmt.Sprintf("Line added to %s successfully", filePath),
	}
}
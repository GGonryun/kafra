package osplugins

import (
	"fmt"
	"os/exec"
	"os/user"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
)

// CreateJITUser creates a user dynamically for JIT access with configurable shell path
func CreateJITUser(username, sshKey, shellPath string, logger *logrus.Logger) error {
	logger.WithField("user", username).Info("Creating JIT user")

	// Check if user already exists
	if _, err := user.Lookup(username); err == nil {
		logger.WithField("user", username).Info("✅ JIT user already exists")
		return nil
	}

	// Find next available UID
	newUID, err := findNextAvailableUID()
	if err != nil {
		return fmt.Errorf("failed to find available UID: %w", err)
	}

	logger.WithFields(logrus.Fields{
		"username": username,
		"uid":      newUID,
	}).Info("Creating new JIT user with UID")

	// Try useradd first, then fallback to adduser
	if err := createUserWithUseradd(username, newUID, shellPath, logger); err != nil {
		if err := createUserWithAdduser(username, newUID, shellPath, logger); err != nil {
			return fmt.Errorf("failed to create user: neither useradd nor adduser succeeded: %w", err)
		}
	}

	// Add SSH key if provided
	if sshKey != "" {
		err = addSSHKeyToUser(username, sshKey, logger)
		if err != nil {
			logger.WithError(err).Warn("Failed to add SSH key, but user was created")
		}
	}

	logger.WithField("user", username).Info("✅ JIT user created successfully")
	return nil
}

// RemoveJITUser removes a dynamically created user
func RemoveJITUser(username string, logger *logrus.Logger) error {
	logger.WithField("user", username).Info("Removing JIT user")

	// Check if user exists
	cmd := exec.Command("id", username)
	if cmd.Run() != nil {
		logger.WithField("user", username).Info("User does not exist, nothing to remove")
		return nil
	}

	// Remove user with userdel
	cmd = exec.Command("sudo", "userdel", "--remove", username)
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.WithError(err).WithField("output", string(output)).Error("Failed to remove JIT user")
		return fmt.Errorf("failed to remove JIT user: %w", err)
	}

	logger.WithField("user", username).Info("✅ JIT user removed successfully")
	return nil
}

// Helper functions

func findNextAvailableUID() (int, error) {
	const minUID, maxUID = 65536, 90000

	for uid := minUID; uid <= maxUID; uid++ {
		if _, err := user.LookupId(strconv.Itoa(uid)); err != nil {
			return uid, nil
		}
	}

	return 0, fmt.Errorf("no available UID found in range %d-%d", minUID, maxUID)
}

func commandExists(command string) bool {
	_, err := exec.LookPath(command)
	return err == nil
}

func createUserWithUseradd(username string, uid int, shellPath string, logger *logrus.Logger) error {
	if !commandExists("groupadd") || !commandExists("useradd") {
		return fmt.Errorf("groupadd or useradd not found")
	}

	logger.Debug("Creating user with useradd/groupadd")

	cmd := exec.Command("sudo", "groupadd", "-g", strconv.Itoa(uid), username)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create group: %v", err)
	}

	cmd = exec.Command("sudo", "useradd", "-m", "-u", strconv.Itoa(uid), "-g", strconv.Itoa(uid), username, "-s", shellPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create user: %v", err)
	}

	return nil
}

func createUserWithAdduser(username string, uid int, shellPath string, logger *logrus.Logger) error {
	if !commandExists("adduser") {
		return fmt.Errorf("adduser not found")
	}

	logger.Debug("Creating user with adduser")

	cmd := exec.Command("sudo", "adduser", "-u", strconv.Itoa(uid), "--gecos", username, "--disabled-password", "--shell", shellPath, username)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create user with adduser: %v", err)
	}

	return nil
}

func addSSHKeyToUser(username, sshKey string, logger *logrus.Logger) error {
	logger.WithField("user", username).Info("Adding SSH key to user")

	// Create authorized_keys file
	homeDir := fmt.Sprintf("/home/%s", username)
	sshDir := fmt.Sprintf("%s/.ssh", homeDir)
	authorizedKeysFile := fmt.Sprintf("%s/authorized_keys", sshDir)

	// Create .ssh directory
	cmd := exec.Command("sudo", "mkdir", "-p", sshDir)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create .ssh directory: %w", err)
	}

	// Write SSH key
	cmd = exec.Command("sudo", "tee", authorizedKeysFile)
	cmd.Stdin = strings.NewReader(sshKey + "\n")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to write SSH key: %w", err)
	}

	// Set proper permissions
	cmd = exec.Command("sudo", "chown", "-R", fmt.Sprintf("%s:%s", username, username), sshDir)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set SSH directory ownership: %w", err)
	}

	cmd = exec.Command("sudo", "chmod", "700", sshDir)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set SSH directory permissions: %w", err)
	}

	cmd = exec.Command("sudo", "chmod", "600", authorizedKeysFile)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set authorized_keys permissions: %w", err)
	}

	logger.WithField("user", username).Info("✅ SSH key added successfully")
	return nil
}
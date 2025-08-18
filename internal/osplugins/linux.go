//go:build !nixos

package osplugins

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
)

type LinuxPlugin struct{}

func init() {
	Register(&LinuxPlugin{})
}

func (p *LinuxPlugin) GetName() string {
	return "linux"
}

// No need for Detect() method - build tags ensure only appropriate plugin is compiled

func (p *LinuxPlugin) GetInstallDirectories() []string {
	return []string{
		"/usr/local/bin",  // Standard on most distributions
		"/usr/bin",        // Fallback
		"/opt/p0/bin",     // Custom location fallback
	}
}

func (p *LinuxPlugin) CreateSystemdService(serviceName, serviceUser, executablePath, configPath string, logger *logrus.Logger) error {
	logger.Info("Creating systemd service file")

	serviceContent := p.generateSystemdService(serviceName, serviceUser, executablePath, configPath)
	serviceFilePath := fmt.Sprintf("/etc/systemd/system/%s.service", serviceName)

	if err := p.writeServiceFile(serviceFilePath, serviceContent, logger); err != nil {
		return fmt.Errorf("failed to write service file: %w", err)
	}

	cmd := exec.Command("sudo", "systemctl", "daemon-reload")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to reload systemd: %w", err)
	}

	logger.Info("✅ Systemd service created successfully")
	return nil
}

func (p *LinuxPlugin) GetConfigDirectory() string {
	return "/etc/p0-ssh-agent"
}

func (p *LinuxPlugin) CreateUser(username, homeDir string, logger *logrus.Logger) error {
	logger.WithField("user", username).Info("Creating service user")

	cmd := exec.Command("id", username)
	if cmd.Run() == nil {
		logger.WithField("user", username).Info("✅ Service user already exists")
		return nil
	}

	cmd = exec.Command("sudo", "useradd",
		"--system",
		"--shell", "/bin/false",
		"--home", homeDir,
		"--create-home",
		username)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create user %s: %w", username, err)
	}

	logger.WithField("user", username).Info("✅ Service user created successfully")
	return nil
}

func (p *LinuxPlugin) SetupDirectories(dirs []string, owner string, logger *logrus.Logger) error {
	for _, dir := range dirs {
		if dir == "" {
			continue
		}

		logger.WithField("dir", dir).Info("Creating directory")

		cmd := exec.Command("sudo", "mkdir", "-p", dir)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}

		cmd = exec.Command("sudo", "chown", "-R", "root:root", dir)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to set ownership for %s: %w", dir, err)
		}

		cmd = exec.Command("sudo", "chmod", "755", dir)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to set permissions for %s: %w", dir, err)
		}

		logger.WithField("dir", dir).Info("✅ Directory created successfully")
	}

	return nil
}

func (p *LinuxPlugin) GetSystemInfo() map[string]string {
	info := make(map[string]string)
	info["os"] = "linux"
	info["package_manager"] = "unknown"
	info["config_method"] = "traditional"

	// Try to detect distribution
	if content, err := os.ReadFile("/etc/os-release"); err == nil {
		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "ID=") {
				info["distribution"] = strings.Trim(strings.TrimPrefix(line, "ID="), "\"")
			}
			if strings.HasPrefix(line, "VERSION_ID=") {
				info["version"] = strings.Trim(strings.TrimPrefix(line, "VERSION_ID="), "\"")
			}
		}
	}

	return info
}

func (p *LinuxPlugin) generateSystemdService(serviceName, _ /* serviceUser */, executablePath, configPath string) string {
	workingDir := filepath.Dir(configPath)

	return fmt.Sprintf(`[Unit]
Description=P0 SSH Agent - Secure SSH access management
Documentation=https://docs.p0.com/
After=network-online.target
Wants=network-online.target
StartLimitInterval=0

[Service]
Type=simple
User=root
Group=root
WorkingDirectory=%s
ExecStart=%s start --config %s
ExecReload=/bin/kill -HUP $MAINPID
Restart=always
RestartSec=5s
StartLimitIntervalSec=60s
StartLimitBurst=10
StandardOutput=journal
StandardError=journal
SyslogIdentifier=%s

# Ensure service runs independently of user sessions  
RemainAfterExit=no
KillMode=mixed

# Security settings - relaxed for root service that needs system access
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true

# Environment
Environment=PATH=/usr/local/bin:/usr/bin:/bin:/sbin:/usr/sbin
Environment=HOME=/root

[Install]
WantedBy=multi-user.target
`, workingDir, executablePath, configPath, serviceName)
}

func (p *LinuxPlugin) writeServiceFile(filePath, content string, logger *logrus.Logger) error {
	logger.WithField("path", filePath).Info("Writing systemd service file")

	tempFile := "/tmp/" + filepath.Base(filePath)
	if err := os.WriteFile(tempFile, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write temporary file: %w", err)
	}

	cmd := exec.Command("sudo", "mv", tempFile, filePath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to move service file: %w", err)
	}

	cmd = exec.Command("sudo", "chmod", "644", filePath)
	if err := cmd.Run(); err != nil {
		logger.WithError(err).Warn("Failed to set service file permissions")
	}

	logger.WithField("path", filePath).Info("✅ Service file written successfully")
	return nil
}

func (p *LinuxPlugin) CreateJITUser(username, sshKey string, logger *logrus.Logger) error {
	logger.WithField("user", username).Info("Creating JIT user")

	// Check if user already exists
	if _, err := user.Lookup(username); err == nil {
		logger.WithField("user", username).Info("✅ JIT user already exists")
		return nil
	}

	// Find next available UID
	newUID, err := p.findNextAvailableUID()
	if err != nil {
		return fmt.Errorf("failed to find available UID: %w", err)
	}

	logger.WithFields(logrus.Fields{
		"username": username,
		"uid":      newUID,
	}).Info("Creating new JIT user with UID")

	// Try useradd first, then fallback to adduser
	if err := p.createUserWithUseradd(username, newUID, logger); err != nil {
		if err := p.createUserWithAdduser(username, newUID, logger); err != nil {
			return fmt.Errorf("failed to create user: neither useradd nor adduser succeeded: %w", err)
		}
	}

	// Add SSH key if provided
	if sshKey != "" {
		err = p.addSSHKeyToUser(username, sshKey, logger)
		if err != nil {
			logger.WithError(err).Warn("Failed to add SSH key, but user was created")
		}
	}

	logger.WithField("user", username).Info("✅ JIT user created successfully")
	return nil
}

func (p *LinuxPlugin) RemoveJITUser(username string, logger *logrus.Logger) error {
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

func (p *LinuxPlugin) addSSHKeyToUser(username, sshKey string, logger *logrus.Logger) error {
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

func (p *LinuxPlugin) findNextAvailableUID() (int, error) {
	const minUID, maxUID = 65536, 90000

	for uid := minUID; uid <= maxUID; uid++ {
		if _, err := user.LookupId(strconv.Itoa(uid)); err != nil {
			return uid, nil
		}
	}

	return 0, fmt.Errorf("no available UID found in range %d-%d", minUID, maxUID)
}

func (p *LinuxPlugin) commandExists(command string) bool {
	_, err := exec.LookPath(command)
	return err == nil
}

func (p *LinuxPlugin) createUserWithUseradd(username string, uid int, logger *logrus.Logger) error {
	if !p.commandExists("groupadd") || !p.commandExists("useradd") {
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

func (p *LinuxPlugin) createUserWithAdduser(username string, uid int, logger *logrus.Logger) error {
	if !p.commandExists("adduser") {
		return fmt.Errorf("adduser not found")
	}

	logger.Debug("Creating user with adduser")

	cmd := exec.Command("sudo", "adduser", "-u", strconv.Itoa(uid), "--gecos", username, "--disabled-password", "--shell", "/bin/bash", username)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create user with adduser: %v", err)
	}

	return nil
}

func (p *LinuxPlugin) UninstallService(serviceName string, logger *logrus.Logger) error {
	logger.WithField("service", serviceName).Info("Uninstalling systemd service")

	// Stop service if running
	cmd := exec.Command("systemctl", "is-active", serviceName)
	if err := cmd.Run(); err == nil {
		logger.Info("Service is running, stopping...")
		cmd = exec.Command("sudo", "systemctl", "stop", serviceName)
		if err := cmd.Run(); err != nil {
			logger.WithError(err).Warn("Failed to stop service")
		} else {
			logger.Info("Service stopped")
		}
	}

	// Disable service if enabled
	cmd = exec.Command("systemctl", "is-enabled", serviceName)
	if err := cmd.Run(); err == nil {
		logger.Info("Service is enabled, disabling...")
		cmd = exec.Command("sudo", "systemctl", "disable", serviceName)
		if err := cmd.Run(); err != nil {
			logger.WithError(err).Warn("Failed to disable service")
		} else {
			logger.Info("Service disabled")
		}
	}

	// Remove service file
	serviceFilePath := fmt.Sprintf("/etc/systemd/system/%s.service", serviceName)
	if _, err := os.Stat(serviceFilePath); err == nil {
		cmd = exec.Command("sudo", "rm", "-f", serviceFilePath)
		if err := cmd.Run(); err != nil {
			logger.WithError(err).Warn("Failed to remove service file")
		} else {
			logger.WithField("path", serviceFilePath).Info("Service file removed")
		}
	}

	// Reload systemd daemon
	cmd = exec.Command("sudo", "systemctl", "daemon-reload")
	if err := cmd.Run(); err != nil {
		logger.WithError(err).Warn("Failed to reload systemd daemon")
	} else {
		logger.Info("Systemd daemon reloaded")
	}

	return nil
}

func (p *LinuxPlugin) CleanupInstallation(serviceName string, logger *logrus.Logger) error {
	logger.Info("Performing Linux-specific cleanup")
	
	// Remove standard directories
	dirs := []string{
		"/etc/p0-ssh-agent",
		"/var/log/p0-ssh-agent",
	}
	
	for _, dir := range dirs {
		if _, err := os.Stat(dir); err == nil {
			cmd := exec.Command("sudo", "rm", "-rf", dir)
			if err := cmd.Run(); err != nil {
				logger.WithError(err).WithField("dir", dir).Warn("Failed to remove directory")
			} else {
				logger.WithField("dir", dir).Info("Directory removed")
			}
		}
	}
	
	// Remove binary from install directories
	installDirs := p.GetInstallDirectories()
	for _, dir := range installDirs {
		binaryPath := fmt.Sprintf("%s/p0-ssh-agent", dir)
		if _, err := os.Stat(binaryPath); err == nil {
			cmd := exec.Command("sudo", "rm", "-f", binaryPath)
			if err := cmd.Run(); err != nil {
				logger.WithError(err).WithField("path", binaryPath).Warn("Failed to remove binary")
			} else {
				logger.WithField("path", binaryPath).Info("Binary removed")
			}
			break // Only remove from the first directory where it's found
		}
	}
	
	return nil
}
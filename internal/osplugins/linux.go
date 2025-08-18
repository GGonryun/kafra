package osplugins

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

func (p *LinuxPlugin) Detect() bool {
	// This is the fallback plugin - always returns true for generic Linux
	// It should be checked last by the plugin manager
	return true
}

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
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

// NewLinuxPlugin creates a new Linux plugin instance
func NewLinuxPlugin() *LinuxPlugin {
	return &LinuxPlugin{}
}

func init() {
	// Register will be called by LoadPlugins() based on OS detection
}

func (p *LinuxPlugin) GetName() string {
	return "linux"
}

// Detect always returns true as Linux is the fallback
func (p *LinuxPlugin) Detect() bool {
	return true // Linux plugin is the fallback for all non-NixOS systems
}

func (p *LinuxPlugin) GetInstallDirectories() []string {
	return []string{
		"/usr/local/bin", // Standard on most distributions
		"/usr/bin",       // Fallback
		"/opt/p0/bin",    // Custom location fallback
	}
}

func (p *LinuxPlugin) CreateSystemdService(serviceName, executablePath, configPath string, logger *logrus.Logger) error {
	logger.Info("Creating systemd service file")

	serviceContent := p.generateSystemdService(serviceName, executablePath, configPath)
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


func (p *LinuxPlugin) generateSystemdService(serviceName, executablePath, configPath string) string {
	workingDir := filepath.Dir(configPath)

	return fmt.Sprintf(`[Unit]
Description=P0 SSH Agent - Secure SSH access management
Documentation=https://docs.p0.com/
After=network-online.target
Wants=network-online.target
StartLimitIntervalSec=60
StartLimitBurst=10

[Service]
Type=simple
User=root
Group=root
WorkingDirectory=%s
ExecStart=%s start --config %s
ExecReload=/bin/kill -HUP $MAINPID
Restart=always
RestartSec=5s
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

func (p *LinuxPlugin) CreateUser(username string, logger *logrus.Logger) error {
	// Use utility function with standard Linux shell path
	return CreateUser(username, "/bin/bash", logger)
}

func (p *LinuxPlugin) RemoveUser(username string, logger *logrus.Logger) error {
	// Use utility function
	return RemoveUser(username, logger)
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

func (p *LinuxPlugin) DisplayInstallationSuccess(serviceName, configPath string, verbose bool) {
	if verbose {
		fmt.Println("\n📊 Installation Summary:")
		fmt.Printf("   ✅ Service Name: %s\n", serviceName)
		fmt.Printf("   ✅ Service User: root (for system operations)\n")
		fmt.Printf("   ✅ Config Path: %s\n", configPath)
		fmt.Printf("   ✅ Systemd Service: Created (not started)\n")
		fmt.Printf("   ✅ JWT Keys: Generated\n")
	}

	fmt.Println("\n🐧 Linux Installation Complete!")
	fmt.Println("\n1. Configure: vi /etc/p0-ssh-agent/config.yaml")
	fmt.Println("2. Register: ./p0-ssh-agent register")
}

func (p *LinuxPlugin) DisplayUninstallationSuccess(hasErrors bool, errors []error) {
	fmt.Println("\n" + strings.Repeat("=", 60))
	if hasErrors {
		fmt.Println("⚠️ Linux Uninstallation Completed with Errors")
	} else {
		fmt.Println("✅ Linux Uninstallation Completed Successfully")
	}
	fmt.Println(strings.Repeat("=", 60))

	fmt.Println("\n📋 What was removed:")
	fmt.Println("   🗑️ Systemd service (p0-ssh-agent)")
	fmt.Println("   🗑️ Configuration directory (/etc/p0-ssh-agent/)")
	fmt.Println("   🗑️ Log directory (/var/log/p0-ssh-agent/)")
	fmt.Println("   🗑️ System binary from install directories")
	fmt.Println("   🗑️ Service files and permissions")

	if hasErrors {
		fmt.Println("\n❌ Errors encountered:")
		for _, err := range errors {
			fmt.Printf("   • %s\n", err.Error())
		}
		fmt.Println("\n💡 You may need to manually clean up remaining files")
		fmt.Println("💡 Check: sudo systemctl status p0-ssh-agent")
		fmt.Println("💡 Check: ls -la /etc/p0-ssh-agent/")
	} else {
		fmt.Println("\n🎉 P0 SSH Agent has been completely removed from your system")
		fmt.Println("💡 You can safely reinstall anytime with: ./p0-ssh-agent install")
	}

	fmt.Println("\n" + strings.Repeat("=", 60))
}

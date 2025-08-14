package install

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"

	"p0-ssh-agent/types"
)

func createServiceUser(serviceUser, keyPath string, logger *logrus.Logger) error {
	logger.WithField("user", serviceUser).Info("Creating service user")

	cmd := exec.Command("id", serviceUser)
	if cmd.Run() == nil {
		logger.WithField("user", serviceUser).Info("✅ Service user already exists")
		return nil
	}

	cmd = exec.Command("sudo", "useradd",
		"--system",
		"--shell", "/bin/false",
		"--home", keyPath,
		"--create-home",
		serviceUser)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create user %s: %w", serviceUser, err)
	}

	logger.WithField("user", serviceUser).Info("✅ Service user created successfully")
	return nil
}

func createDirectories(cfg *types.Config, serviceUser string, logger *logrus.Logger) error {
	directories := []string{
		cfg.KeyPath,
		filepath.Dir(cfg.LogPath),
	}

	for _, dir := range directories {
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

func generateJWTKeys(keyPath, serviceUser, executablePath string, logger *logrus.Logger) error {
	logger.WithField("key_path", keyPath).Info("Generating JWT keys")

	privateKeyPath := filepath.Join(keyPath, "jwk.private.json")
	publicKeyPath := filepath.Join(keyPath, "jwk.public.json")

	if _, err := os.Stat(privateKeyPath); err == nil {
		if _, err := os.Stat(publicKeyPath); err == nil {
			logger.Info("✅ JWT keys already exist")
			return nil
		}
	}

	cmd := exec.Command("sudo", executablePath, "keygen", "--key-path", keyPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to generate JWT keys: %w\nOutput: %s", err, string(output))
	}

	logger.Info("✅ JWT keys generated successfully")
	return nil
}

func createLogFile(logPath, serviceUser string, logger *logrus.Logger) error {
	if logPath == "" {
		logger.Info("No log path specified, using stdout/stderr")
		return nil
	}

	if stat, err := os.Stat(logPath); err == nil && stat.IsDir() {
		logPath = filepath.Join(logPath, "service.log")
	} else if filepath.Ext(logPath) == "" {
		logPath = filepath.Join(logPath, "service.log")
	}

	logger.WithField("log_path", logPath).Info("Creating log file")

	logDir := filepath.Dir(logPath)
	cmd := exec.Command("sudo", "mkdir", "-p", logDir)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create log directory %s: %w", logDir, err)
	}

	cmd = exec.Command("sudo", "touch", logPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create log file %s: %w", logPath, err)
	}

	cmd = exec.Command("sudo", "chown", "root:root", logPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set ownership for log file: %w", err)
	}

	cmd = exec.Command("sudo", "chmod", "644", logPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set permissions for log file: %w", err)
	}

	logger.WithField("log_path", logPath).Info("✅ Log file created successfully")
	return nil
}

func registerWithBackend(configPath, serviceUser, executablePath string, logger *logrus.Logger) error {
	logger.Info("Registering with P0 backend")

	cmd := exec.Command("sudo", executablePath, "register", "--config", configPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("registration failed: %w\nOutput: %s", err, string(output))
	}

	logger.WithField("output", string(output)).Debug("Registration output")
	return nil
}

func createSystemdService(serviceName, serviceUser, executablePath, configPath string, logger *logrus.Logger) error {
	logger.Info("Creating systemd service file")

	serviceContent := generateSystemdService(serviceName, serviceUser, executablePath, configPath)
	serviceFilePath := fmt.Sprintf("/etc/systemd/system/%s.service", serviceName)

	if err := writeServiceFile(serviceFilePath, serviceContent, logger); err != nil {
		return fmt.Errorf("failed to write service file: %w", err)
	}

	cmd := exec.Command("sudo", "systemctl", "daemon-reload")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to reload systemd: %w", err)
	}

	logger.Info("✅ Systemd service created successfully")
	return nil
}

func enableAndStartService(serviceName string, logger *logrus.Logger) error {
	logger.WithField("service", serviceName).Info("Enabling and starting service")

	cmd := exec.Command("sudo", "systemctl", "enable", serviceName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to enable service: %w", err)
	}

	cmd = exec.Command("sudo", "systemctl", "start", serviceName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}

	cmd = exec.Command("sudo", "systemctl", "is-active", serviceName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("service failed to start properly: %w", err)
	}

	logger.WithField("service", serviceName).Info("✅ Service enabled and started successfully")
	return nil
}

func generateSystemdService(serviceName, serviceUser, executablePath, configPath string) string {
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
RestartSec=10
StandardOutput=journal
StandardError=journal
SyslogIdentifier=%s

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

func writeServiceFile(filePath, content string, logger *logrus.Logger) error {
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

func displayInstallationSuccess(serviceName, serviceUser, configPath, executablePath string) {
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("🎉 P0 SSH Agent Installation Complete!")
	fmt.Println(strings.Repeat("=", 60))

	fmt.Println("\n📊 Installation Summary:")
	fmt.Printf("   ✅ Service Name: %s\n", serviceName)
	fmt.Printf("   ✅ Service User: root (for system operations)\n")
	fmt.Printf("   ✅ Config Path: %s\n", configPath)
	fmt.Printf("   ✅ Systemd Service: Created (not started)\n")
	fmt.Printf("   ✅ JWT Keys: Generated\n")

	fmt.Println("\n⚠️  IMPORTANT: Complete These Steps Before Starting the Service")
	fmt.Println(strings.Repeat("-", 60))

	fmt.Println("\n📝 Step 1: Configure Your Settings")
	fmt.Printf("   Edit the configuration file to update your organization settings:\n")
	fmt.Printf("   \033[1msudo vi %s\033[0m\n", configPath)
	fmt.Println("")
	fmt.Println("   Required fields to update:")
	fmt.Println("   • orgId: Your P0 organization ID")
	fmt.Println("   • hostId: Unique identifier for this machine")
	fmt.Println("   • tunnelHost: Your P0 backend WebSocket URL")

	fmt.Println("\n🔑 Step 2: Register This Machine")
	fmt.Printf("   Generate and submit your registration request:\n")
	fmt.Printf("   \033[1m%s register --config %s\033[0m\n", "p0-ssh-agent", configPath)
	fmt.Println("")
	fmt.Println("   The registration command will:")
	fmt.Println("   • Generate a machine-specific registration code")
	fmt.Println("   • Display system information (hostname, fingerprint, keys)")
	fmt.Println("   • Provide a base64-encoded request for your P0 backend")
	fmt.Println("   • Give you instructions to start the service after approval")

	fmt.Println("\n🔧 Service Management Commands:")
	fmt.Printf("   Status:  sudo systemctl status %s\n", serviceName)
	fmt.Printf("   Stop:    sudo systemctl stop %s\n", serviceName)
	fmt.Printf("   Start:   sudo systemctl start %s\n", serviceName)
	fmt.Printf("   Restart: sudo systemctl restart %s\n", serviceName)
	fmt.Printf("   Logs:    sudo journalctl -u %s -f\n", serviceName)

	fmt.Printf("\n💡 Pro Tip: Use 'p0-ssh-agent status' after starting to validate the installation\n")
	fmt.Println("\n" + strings.Repeat("=", 60))
}

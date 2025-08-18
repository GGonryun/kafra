package install

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"

	"p0-ssh-agent/internal/osplugins"
	"p0-ssh-agent/types"
)

func createServiceUser(serviceUser, keyPath string, osPlugin osplugins.OSPlugin, logger *logrus.Logger) error {
	logger.WithField("user", serviceUser).Info("Creating service user")

	// Use the OS plugin to create the user
	return osPlugin.CreateUser(serviceUser, keyPath, logger)
}

func createDirectories(cfg *types.Config, serviceUser string, osPlugin osplugins.OSPlugin, logger *logrus.Logger) error {
	directories := []string{
		cfg.KeyPath,
		filepath.Dir(cfg.LogPath),
	}

	// Use the OS plugin to setup directories
	return osPlugin.SetupDirectories(directories, serviceUser, logger)
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

// Old NixOS-specific functions removed - now handled by NixOS plugin

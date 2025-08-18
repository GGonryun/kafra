//go:build nixos

package osplugins

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
)

type NixOSPlugin struct{}

func init() {
	Register(&NixOSPlugin{})
}

func (p *NixOSPlugin) GetName() string {
	return "nixos"
}

// No need for Detect() method - build tags ensure only appropriate plugin is compiled

func (p *NixOSPlugin) GetInstallDirectories() []string {
	return []string{
		"/usr/bin",    // NixOS doesn't have /usr/local/bin
		"/opt/p0/bin", // Custom location fallback
	}
}

func (p *NixOSPlugin) CreateSystemdService(serviceName, serviceUser, executablePath, configPath string, logger *logrus.Logger) error {
	logger.Info("🐧 NixOS detected - generating configuration snippet instead of direct service creation")
	return p.generateNixOSServiceConfig(serviceName, serviceUser, executablePath, configPath, logger)
}

func (p *NixOSPlugin) GetConfigDirectory() string {
	return "/etc/p0-ssh-agent"
}

func (p *NixOSPlugin) CreateUser(username, homeDir string, logger *logrus.Logger) error {
	logger.WithField("user", username).Info("Creating service user")

	// Check if user already exists
	cmd := exec.Command("id", username)
	if cmd.Run() == nil {
		logger.WithField("user", username).Info("✅ Service user already exists")
		return nil
	}

	// Check if systemd-homed is available - this is required for NixOS
	if !p.isSystemdHomedAvailable() {
		return p.provideSystemdHomedSetupInstructions(username, homeDir, logger)
	}

	// Create user with systemd-homed
	logger.WithField("user", username).Info("Creating user with systemd-homed")
	return p.createUserWithHomed(username, homeDir, logger)
}

func (p *NixOSPlugin) isSystemdHomedAvailable() bool {
	// Check if systemd-homed service is running
	cmd := exec.Command("systemctl", "is-active", "systemd-homed")
	return cmd.Run() == nil
}

// generateRandomPassword creates a random 32-character password
func (p *NixOSPlugin) generateRandomPassword() (string, error) {
	bytes := make([]byte, 16) // 16 bytes = 32 hex characters
	_, err := rand.Read(bytes)
	if err != nil {
		return "", fmt.Errorf("failed to generate random password: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

func (p *NixOSPlugin) createUserWithHomed(username, homeDir string, logger *logrus.Logger) error {
	logger.WithField("user", username).Info("Creating user with systemd-homed")

	// Generate random password
	password, err := p.generateRandomPassword()
	if err != nil {
		logger.WithError(err).Error("Failed to generate random password")
		return fmt.Errorf("failed to generate password: %w", err)
	}

	// Create user with homectl using NEWPASSWORD environment variable
	cmdStr := fmt.Sprintf("sudo NEWPASSWORD=%s homectl create %s --real-name '%s' --shell '/bin/false' --home-dir '%s' --storage=directory",
		password, username, fmt.Sprintf("P0 Service User %s", username), homeDir)
	cmd := exec.Command("bash", "-c", cmdStr)

	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.WithError(err).WithField("output", string(output)).Error("Failed to create user with homectl")
		return fmt.Errorf("homectl create failed: %w", err)
	}

	// Activate the user to mount/prepare the home directory
	logger.WithField("user", username).Info("Activating user home directory")
	cmdStr = fmt.Sprintf("expect -c \"spawn sudo homectl activate %s; expect \\\"Password:\\\"; send \\\"%s\\r\\\"; interact\"", username, password)
	cmd = exec.Command("bash", "-c", cmdStr)
	output, err = cmd.CombinedOutput()
	if err != nil {
		logger.WithError(err).WithField("output", string(output)).Error("Failed to activate user home directory")
		return fmt.Errorf("homectl activate failed: %w", err)
	}

	logger.WithField("user", username).Info("✅ Service user created and activated successfully with systemd-homed")
	return nil
}

func (p *NixOSPlugin) provideSystemdHomedSetupInstructions(username, homeDir string, logger *logrus.Logger) error {
	logger.WithField("user", username).Error("⚠️  systemd-homed is required but not enabled")
	return fmt.Errorf("systemd-homed is not enabled - add 'services.homed.enable = true;' to /etc/nixos/configuration.nix and run 'sudo nixos-rebuild switch'")
}

func (p *NixOSPlugin) SetupDirectories(dirs []string, owner string, logger *logrus.Logger) error {
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

func (p *NixOSPlugin) GetSystemInfo() map[string]string {
	info := make(map[string]string)
	info["os"] = "nixos"
	info["package_manager"] = "nix"
	info["config_method"] = "declarative"

	// Try to get NixOS version
	if content, err := os.ReadFile("/etc/nixos/version"); err == nil {
		info["version"] = strings.TrimSpace(string(content))
	}

	return info
}

func (p *NixOSPlugin) generateNixOSServiceConfig(serviceName, _ /* serviceUser */, executablePath, configPath string, logger *logrus.Logger) error {
	workingDir := filepath.Dir(configPath)

	nixConfig := fmt.Sprintf(`# Add this to your /etc/nixos/configuration.nix file inside the config block:
# If your configuration.nix doesn't have { config, pkgs, lib, ... }: at the top, add lib:
# { config, pkgs, lib, ... }:

# REQUIRED: Enable systemd-homed for JIT user management
services.homed.enable = true;

# P0 SSH Agent systemd service
systemd.services.%s = {
  enable = true;
  description = "P0 SSH Agent - Secure SSH access management";
  documentation = [ "https://docs.p0.com/" ];
  after = [ "network-online.target" "systemd-homed.service" ];
  wants = [ "network-online.target" ];
  wantedBy = [ "multi-user.target" ];
  
  serviceConfig = {
    Type = "simple";
    User = "root";
    Group = "root";
    WorkingDirectory = "%s";
    ExecStart = "%s start --config %s";
    ExecReload = "/bin/kill -HUP $MAINPID";
    Restart = "always";
    RestartSec = "5s";
    StartLimitIntervalSec = "60s";
    StartLimitBurst = 10;
    StandardOutput = "journal";
    StandardError = "journal";
    SyslogIdentifier = "%s";
    
    # Ensure service runs independently of user sessions
    RemainAfterExit = false;
    KillMode = "mixed";
    
    # Security settings
    ProtectKernelTunables = true;
    ProtectKernelModules = true;
    ProtectControlGroups = true;
  };
  
  # Environment variables - extend PATH to include system binaries needed for user management
  environment = {
    PATH = lib.mkForce "/run/current-system/sw/bin:/run/current-system/sw/sbin:/usr/bin:/bin";
    HOME = "/root";
  };
};`, serviceName, workingDir, executablePath, configPath, serviceName)

	// Write the config to a temporary file for user reference
	configFile := fmt.Sprintf("/tmp/%s-nixos-config.nix", serviceName)
	if err := os.WriteFile(configFile, []byte(nixConfig), 0644); err != nil {
		logger.WithError(err).Warn("Failed to write NixOS config file")
	} else {
		logger.WithField("config_file", configFile).Info("📝 NixOS configuration written to temporary file")
	}

	// Display instructions
	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("🐧 NixOS DETECTED - Manual Configuration Required")
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println("\nNixOS uses declarative configuration management.")
	fmt.Println("Please follow these steps to complete the service setup:")
	fmt.Println("\n📝 Step 1: Add Service to Configuration")
	fmt.Printf("   Add the following to your /etc/nixos/configuration.nix file:\n\n")
	fmt.Println(nixConfig)
	fmt.Printf("\n📄 A copy has been saved to: %s\n", configFile)
	fmt.Println("\n🔄 Step 2: Rebuild System")
	fmt.Println("   sudo nixos-rebuild switch")
	fmt.Println("\n✅ Step 3: Service Management")
	fmt.Printf("   Start:   sudo systemctl start %s\n", serviceName)
	fmt.Printf("   Status:  sudo systemctl status %s\n", serviceName)
	fmt.Printf("   Logs:    sudo journalctl -u %s -f\n", serviceName)
	fmt.Println("\n" + strings.Repeat("=", 70))

	logger.Info("✅ NixOS configuration generated successfully")
	return nil
}

func (p *NixOSPlugin) CreateJITUser(username, sshKey string, logger *logrus.Logger) error {
	logger.WithField("user", username).Info("Creating JIT user")

	// Check if user already exists
	cmd := exec.Command("id", username)
	if cmd.Run() == nil {
		logger.WithField("user", username).Info("✅ JIT user already exists")
		return nil
	}

	// Check if systemd-homed is available
	if !p.isSystemdHomedAvailable() {
		return fmt.Errorf("systemd-homed service is not enabled - required for NixOS user management")
	}

	// Generate random password
	password, err := p.generateRandomPassword()
	if err != nil {
		logger.WithError(err).Error("Failed to generate random password")
		return fmt.Errorf("failed to generate password: %w", err)
	}

	// Create user with homectl using NEWPASSWORD environment variable
	cmdStr := fmt.Sprintf("sudo NEWPASSWORD=%s homectl create %s --real-name '%s' --shell '/run/current-system/sw/bin/bash' --home-dir '/home/%s'",
		password, username, fmt.Sprintf("P0 JIT User %s", username), username)
	cmd = exec.Command("bash", "-c", cmdStr)

	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.WithError(err).WithField("output", string(output)).Error("Failed to create JIT user")
		return fmt.Errorf("failed to create JIT user: %w", err)
	}

	// Add SSH key if provided
	if sshKey != "" {
		err = p.addSSHKeyToUser(username, sshKey, logger)
		if err != nil {
			logger.WithError(err).Warn("Failed to add SSH key, but user was created")
		}
	}

	logger.WithField("user", username).Info("✅ JIT user created and activated successfully")
	return nil
}

func (p *NixOSPlugin) RemoveJITUser(username string, logger *logrus.Logger) error {
	logger.WithField("user", username).Info("Removing JIT user")

	// Check if user exists
	cmd := exec.Command("id", username)
	if cmd.Run() != nil {
		logger.WithField("user", username).Info("User does not exist, nothing to remove")
		return nil
	}

	// Check if systemd-homed is available
	if !p.isSystemdHomedAvailable() {
		return fmt.Errorf("systemd-homed service is not enabled - required for NixOS user management")
	}

	// Remove user with homectl
	cmd = exec.Command("sudo", "homectl", "remove", username)
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.WithError(err).WithField("output", string(output)).Error("Failed to remove JIT user")
		return fmt.Errorf("failed to remove JIT user: %w", err)
	}

	logger.WithField("user", username).Info("✅ JIT user removed successfully")
	return nil
}

func (p *NixOSPlugin) addSSHKeyToUser(username, sshKey string, logger *logrus.Logger) error {
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

func (p *NixOSPlugin) UninstallService(serviceName string, logger *logrus.Logger) error {
	logger.WithField("service", serviceName).Info("Handling NixOS service uninstallation")

	// Stop service if running (NixOS still uses systemctl for runtime management)
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

	// Note: No service file removal needed on NixOS - services are managed declaratively
	logger.Info("ℹ️  NixOS services are managed declaratively - no service files to remove")

	return nil
}

func (p *NixOSPlugin) CleanupInstallation(serviceName string, logger *logrus.Logger) error {
	logger.Info("Performing NixOS-specific cleanup")

	// Remove runtime directories that may have been created
	dirs := []string{
		"/etc/p0-ssh-agent",     // Config directory
		"/var/log/p0-ssh-agent", // Log directory
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

	// Provide NixOS-specific cleanup instructions
	nixosInstructions := fmt.Sprintf(`
# NixOS UNINSTALL INSTRUCTIONS

To completely remove P0 SSH Agent from your NixOS system:

1. Remove the service configuration from /etc/nixos/configuration.nix:
   - Delete the 'systemd.services.%s' block
   - Remove 'services.homed.enable = true;' if no longer needed

2. Rebuild your system:
   sudo nixos-rebuild switch

3. The service will be automatically removed from your system.

Note: Runtime files and binaries have been cleaned up automatically.`, serviceName)

	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("🐧 NixOS UNINSTALL COMPLETE")
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println(nixosInstructions)
	fmt.Println("\n" + strings.Repeat("=", 70))

	return nil
}

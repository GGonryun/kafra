package osplugins

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
)

type NixOSPlugin struct{}

// NewNixOSPlugin creates a new NixOS plugin instance
func NewNixOSPlugin() *NixOSPlugin {
	return &NixOSPlugin{}
}

// getNixOSShellPath detects and returns the appropriate shell path for NixOS
func (p *NixOSPlugin) getNixOSShellPath() string {
	nixosShellPath := "/run/current-system/sw/bin/bash"
	if _, err := os.Stat(nixosShellPath); err != nil {
		// Fallback to standard shell if NixOS path doesn't exist
		return "/bin/bash"
	}
	return nixosShellPath
}

func init() {
	// Register will be called by LoadPlugins() based on OS detection
}

func (p *NixOSPlugin) GetName() string {
	return "nixos"
}

// Detect checks if this is a NixOS system
func (p *NixOSPlugin) Detect() bool {
	// Check for NixOS-specific files/directories
	if _, err := os.Stat("/etc/nixos"); err == nil {
		return true
	}
	if _, err := os.Stat("/run/current-system"); err == nil {
		return true
	}
	// Check for nixos in os-release
	if content, err := os.ReadFile("/etc/os-release"); err == nil {
		if strings.Contains(strings.ToLower(string(content)), "nixos") {
			return true
		}
	}
	return false
}

func (p *NixOSPlugin) GetInstallDirectories() []string {
	return []string{
		"/usr/bin",    // NixOS doesn't have /usr/local/bin
		"/opt/p0/bin", // Custom location fallback
	}
}

func (p *NixOSPlugin) CreateSystemdService(serviceName, executablePath, configPath string, logger *logrus.Logger) error {
	logger.Info("🐧 NixOS detected - generating configuration snippet instead of direct service creation")
	return p.generateNixOSServiceConfig(serviceName, executablePath, configPath, logger)
}

func (p *NixOSPlugin) GetConfigDirectory() string {
	return "/etc/p0-ssh-agent"
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

func (p *NixOSPlugin) generateNixOSServiceConfig(serviceName, executablePath, configPath string, logger *logrus.Logger) error {
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
	logger.WithField("user", username).Info("Creating JIT user with NixOS shell path")

	// Use utility function with NixOS-specific shell path
	return CreateJITUser(username, sshKey, p.getNixOSShellPath(), logger)
}

func (p *NixOSPlugin) RemoveJITUser(username string, logger *logrus.Logger) error {
	logger.WithField("user", username).Info("Removing JIT user")

	// Use utility function
	return RemoveJITUser(username, logger)
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

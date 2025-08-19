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
	return "/run/current-system/sw/bin/bash"
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
	// Install in system modules path at jit/p0-ssh-agent.nix
	moduleDestPath := "/run/current-system/sw/share/nixos/modules/jit/p0-ssh-agent.nix"

	// Create the NixOS module content with actual paths from installation
	moduleContent := p.generateNixOSModule(executablePath, configPath)
	
	// Install the module to system modules path
	if err := p.installNixOSModuleDirectly(moduleContent, moduleDestPath, logger); err != nil {
		logger.WithError(err).Error("Failed to install NixOS module")
		return err
	}

	// Display instructions
	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("🐧 NixOS DETECTED")
	fmt.Println(strings.Repeat("=", 70))

	fmt.Println("\n✅ P0 SSH Agent NixOS module installed!")
	fmt.Printf("📄 Module installed at: %s\n", moduleDestPath)

	fmt.Println("\n📝 Add these lines to your /etc/nixos/configuration.nix:")
	fmt.Println("\n{")
	fmt.Println("  imports = [")
	fmt.Println("    # ... your existing imports ...")
	fmt.Println("    \"${modulesPath}/jit/p0-ssh-agent.nix\"")
	fmt.Println("  ];")
	fmt.Println("")
	fmt.Println("  services.p0-ssh-agent.enable = true;")
	fmt.Println("}")

	fmt.Println("\n🔄 Then rebuild your system:")
	fmt.Println("   sudo nixos-rebuild switch")

	fmt.Println("\n🔧 After rebuild, manage the service with:")
	fmt.Printf("   Status:  sudo systemctl status %s\n", serviceName)
	fmt.Printf("   Logs:    sudo journalctl -u %s -f\n", serviceName)

	fmt.Println("\n💡 Configure P0 SSH Agent by editing:")
	fmt.Printf("   %s\n", configPath)
	fmt.Println("\n" + strings.Repeat("=", 70))

	logger.Info("✅ NixOS module installed successfully")
	return nil
}

func (p *NixOSPlugin) generateNixOSModule(executablePath, configPath string) string {
	return fmt.Sprintf(`{ config, lib, ... }:

with lib;

let
  cfg = config.services.p0-ssh-agent;
in {
  options.services.p0-ssh-agent = {
    enable = mkEnableOption "P0 SSH Agent - Secure SSH access management";
  };
  
  config = mkIf cfg.enable {
    # Main systemd service
    systemd.services.p0-ssh-agent = {
      enable = true;
      description = "P0 SSH Agent - Secure SSH access management";
      documentation = [ "https://docs.p0.com/" ];
      after = [ "network-online.target" ];
      wants = [ "network-online.target" ];
      wantedBy = [ "multi-user.target" ];
      
      startLimitIntervalSec = 60;
      startLimitBurst = 10;
      
      serviceConfig = {
        Type = "simple";
        User = "root";
        Group = "root";
        WorkingDirectory = "/etc/p0-ssh-agent";
        ExecStart = "%s start --config %s";
        ExecReload = "/bin/kill -HUP $MAINPID";
        Restart = "always";
        RestartSec = "5s";
        StandardOutput = "journal";
        StandardError = "journal";
        SyslogIdentifier = "p0-ssh-agent";
        
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
        PATH = lib.mkForce "/run/current-system/sw/bin:/run/current-system/sw/sbin:/run/wrappers/bin:/usr/bin:/bin";
        HOME = "/root";
      };
    };
  };
}`, executablePath, configPath)
}

func (p *NixOSPlugin) installNixOSModuleDirectly(moduleContent, destPath string, logger *logrus.Logger) error {
	// Create a temporary file with the module content
	tempPath := "/tmp/p0-ssh-agent-module.nix"
	if err := os.WriteFile(tempPath, []byte(moduleContent), 0644); err != nil {
		return fmt.Errorf("failed to create temporary module file: %w", err)
	}

	// Create destination directory if needed
	moduleDir := filepath.Dir(destPath)
	cmd := exec.Command("sudo", "mkdir", "-p", moduleDir)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create modules directory: %w", err)
	}

	// Copy module file to final location
	cmd = exec.Command("sudo", "cp", tempPath, destPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install module file: %w", err)
	}

	// Set proper permissions
	cmd = exec.Command("sudo", "chmod", "644", destPath)
	if err := cmd.Run(); err != nil {
		logger.WithError(err).Warn("Failed to set module file permissions")
	}

	// Clean up temporary file
	os.Remove(tempPath)

	logger.WithField("module_path", destPath).Info("✅ NixOS module installed successfully")
	return nil
}

func (p *NixOSPlugin) CreateUser(username string, logger *logrus.Logger) error {
	logger.WithField("user", username).Info("Creating JIT user with NixOS shell path")

	// Use utility function with NixOS-specific shell path
	return CreateUser(username, p.getNixOSShellPath(), logger)
}

func (p *NixOSPlugin) RemoveUser(username string, logger *logrus.Logger) error {
	logger.WithField("user", username).Info("Removing JIT user")

	// Use utility function
	return RemoveUser(username, logger)
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

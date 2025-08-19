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
	// Install the NixOS module
	moduleSourcePath := "/tmp/p0-ssh-agent.nix"
	moduleDestPath := "/etc/nixos/modules/p0-ssh-agent.nix"
	
	// Create the NixOS module content
	moduleContent := p.generateNixOSModule()
	if err := os.WriteFile(moduleSourcePath, []byte(moduleContent), 0644); err != nil {
		logger.WithError(err).Warn("Failed to write NixOS module to temporary file")
	} else {
		logger.WithField("module_file", moduleSourcePath).Info("📝 NixOS module written to temporary file")
	}
	
	// Try to install the module if possible
	if err := p.installNixOSModule(moduleSourcePath, moduleDestPath, logger); err != nil {
		logger.WithError(err).Warn("Could not automatically install NixOS module")
	}

	// Generate example configuration
	exampleConfig := p.generateExampleConfig(executablePath, configPath)

	// Display instructions
	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("🐧 NixOS DETECTED - Module Installation")
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println("\nNixOS module has been prepared for easy configuration.")
	fmt.Println("Please follow these steps to complete the service setup:")
	
	fmt.Println("\n📦 Step 1: Install the NixOS Module")
	fmt.Printf("   sudo mkdir -p /etc/nixos/modules\n")
	fmt.Printf("   sudo cp %s %s\n", moduleSourcePath, moduleDestPath)
	
	fmt.Println("\n📝 Step 2: Import and Configure in /etc/nixos/configuration.nix")
	fmt.Println("   Add the following to your configuration.nix:")
	fmt.Println(exampleConfig)
	
	fmt.Println("\n🔄 Step 3: Rebuild System")
	fmt.Println("   sudo nixos-rebuild switch")
	
	fmt.Println("\n✅ Step 4: Service Management")
	fmt.Printf("   Status:  sudo systemctl status %s\n", serviceName)
	fmt.Printf("   Logs:    sudo journalctl -u %s -f\n", serviceName)
	fmt.Println("\n" + strings.Repeat("=", 70))

	logger.Info("✅ NixOS module and configuration generated successfully")
	return nil
}

func (p *NixOSPlugin) generateNixOSModule() string {
	return `{ config, lib, pkgs, ... }:

with lib;

let
  cfg = config.services.p0-ssh-agent;
in {
  options.services.p0-ssh-agent = {
    enable = mkEnableOption "P0 SSH Agent - Secure SSH access management";
    
    config = mkOption {
      type = types.attrs;
      default = {};
      description = "Configuration for P0 SSH Agent as Nix attributes";
      example = {
        version = "1.0";
        orgId = "your-org";
        hostId = "your-host-id";
        environment = "production";
        tunnelHost = "wss://your-tunnel.example.com";
        keyPath = "/etc/p0-ssh-agent/keys";
        logPath = "/var/log/p0-ssh-agent/service.log";
        labels = [ "type=production" ];
        heartbeatIntervalSeconds = 60;
      };
    };
    
    configFile = mkOption {
      type = types.path;
      description = "Path to the configuration file";
      default = "/etc/p0-ssh-agent/config.yaml";
    };
    
    binaryPath = mkOption {
      type = types.path;
      description = "Path to the p0-ssh-agent binary";
      default = "/usr/bin/p0-ssh-agent";
    };
  };
  
  config = mkIf cfg.enable {
    # Enable systemd-homed for JIT user management
    services.homed.enable = mkDefault true;
    
    # Create configuration directory
    system.activationScripts.p0-ssh-agent-setup = ''
      mkdir -p /etc/p0-ssh-agent
      mkdir -p /var/log/p0-ssh-agent
      chown root:root /etc/p0-ssh-agent /var/log/p0-ssh-agent
      chmod 755 /etc/p0-ssh-agent /var/log/p0-ssh-agent
    '';
    
    # Generate YAML config file from Nix configuration
    environment.etc."p0-ssh-agent/config.yaml" = mkIf (cfg.config != {}) {
      text = ''
        version: "${cfg.config.version or "1.0"}"
        orgId: "${cfg.config.orgId or ""}"
        hostId: "${cfg.config.hostId or ""}"
        ${lib.optionalString (cfg.config ? hostname) ''hostname: "${cfg.config.hostname}"''}
        environment: "${cfg.config.environment or "production"}"
        tunnelHost: "${cfg.config.tunnelHost or ""}"
        keyPath: "${cfg.config.keyPath or "/etc/p0-ssh-agent/keys"}"
        logPath: "${cfg.config.logPath or "/var/log/p0-ssh-agent/service.log"}"
        ${lib.optionalString (cfg.config ? labels) ''labels: ${builtins.toJSON cfg.config.labels}''}
        heartbeatIntervalSeconds: ${toString (cfg.config.heartbeatIntervalSeconds or 60)}
        ${lib.optionalString (cfg.config ? dryRun) ''dryRun: ${if cfg.config.dryRun then "true" else "false"}''}
      '';
      mode = "0644";
    };
    
    # Main systemd service
    systemd.services.p0-ssh-agent = {
      enable = true;
      description = "P0 SSH Agent - Secure SSH access management";
      documentation = [ "https://docs.p0.com/" ];
      after = [ "network-online.target" "systemd-homed.service" ];
      wants = [ "network-online.target" ];
      wantedBy = [ "multi-user.target" ];
      
      startLimitIntervalSec = 60;
      startLimitBurst = 10;
      
      serviceConfig = {
        Type = "simple";
        User = "root";
        Group = "root";
        WorkingDirectory = "/etc/p0-ssh-agent";
        ExecStart = "${cfg.binaryPath} start --config ${cfg.configFile}";
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
    
    # Add p0-ssh-agent command to system PATH if binary exists
    environment.systemPackages = mkIf (builtins.pathExists cfg.binaryPath) [
      (pkgs.runCommand "p0-ssh-agent-wrapper" {} ''
        mkdir -p $out/bin
        ln -s ${cfg.binaryPath} $out/bin/p0-ssh-agent
      '')
    ];
  };
}`
}

func (p *NixOSPlugin) generateExampleConfig(executablePath, configPath string) string {
	return fmt.Sprintf(`
{
  imports = [
    ./modules/p0-ssh-agent.nix
  ];

  services.p0-ssh-agent = {
    enable = true;
    binaryPath = "%s";
    configFile = "%s";
    config = {
      version = "1.0";
      orgId = "your-org-id";
      hostId = "your-host-id";
      environment = "production";
      tunnelHost = "wss://your-tunnel-host.example.com";
      keyPath = "/etc/p0-ssh-agent/keys";
      logPath = "/var/log/p0-ssh-agent/service.log";
      labels = [ "type=production" ];
      heartbeatIntervalSeconds = 60;
    };
  };
}`, executablePath, configPath)
}

func (p *NixOSPlugin) installNixOSModule(sourcePath, destPath string, logger *logrus.Logger) error {
	// Create modules directory
	moduleDir := filepath.Dir(destPath)
	cmd := exec.Command("sudo", "mkdir", "-p", moduleDir)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create modules directory: %w", err)
	}
	
	// Copy module file
	cmd = exec.Command("sudo", "cp", sourcePath, destPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to copy module file: %w", err)
	}
	
	// Set permissions
	cmd = exec.Command("sudo", "chmod", "644", destPath)
	if err := cmd.Run(); err != nil {
		logger.WithError(err).Warn("Failed to set module file permissions")
	}
	
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

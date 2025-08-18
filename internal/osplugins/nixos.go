//go:build nixos

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

	// Try systemd-homed first for dynamic user creation
	logger.WithField("user", username).Info("Attempting to create user with systemd-homed")
	
	// Check if systemd-homed is available
	if p.isSystemdHomedAvailable() {
		err := p.createUserWithHomed(username, homeDir, logger)
		if err == nil {
			return nil
		}
		logger.WithError(err).Warn("systemd-homed user creation failed, falling back to configuration instructions")
	}

	// Fall back to configuration instructions
	return p.provideUserConfigInstructions(username, homeDir, logger)
}

func (p *NixOSPlugin) isSystemdHomedAvailable() bool {
	// Check if systemd-homed service is running
	cmd := exec.Command("systemctl", "is-active", "systemd-homed")
	return cmd.Run() == nil
}

func (p *NixOSPlugin) createUserWithHomed(username, homeDir string, logger *logrus.Logger) error {
	logger.WithField("user", username).Info("Creating user with systemd-homed")
	
	// Create user with homectl
	cmd := exec.Command("sudo", "homectl", "create", username,
		"--real-name", fmt.Sprintf("P0 Service User %s", username),
		"--shell", "/bin/false",
		"--home-dir", homeDir,
		"--uid-range", "1000-1999", // System user range
	)
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.WithError(err).WithField("output", string(output)).Error("Failed to create user with homectl")
		return fmt.Errorf("homectl create failed: %w", err)
	}
	
	logger.WithField("user", username).Info("✅ Service user created successfully with systemd-homed")
	return nil
}

func (p *NixOSPlugin) provideUserConfigInstructions(username, homeDir string, logger *logrus.Logger) error {
	logger.WithField("user", username).Warn("⚠️  User creation requires configuration")
	
	userConfig := fmt.Sprintf(`
# OPTION 1: Enable systemd-homed for dynamic user management
# Add this to your /etc/nixos/configuration.nix:

services.homed.enable = true;

# Then rebuild: sudo nixos-rebuild switch
# After that, create users dynamically with:
# sudo homectl create %s --real-name "P0 Service User" --shell /bin/false

# OPTION 2: Declare user statically in configuration.nix
# Add this to your /etc/nixos/configuration.nix:

users.users.%s = {
  isSystemUser = true;
  shell = "/bin/false";
  home = "%s";
  createHome = true;
  group = "%s";
};

users.groups.%s = {};`, username, username, homeDir, username, username)

	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("🐧 NixOS USER CREATION OPTIONS")
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println("Choose one of the following approaches:")
	fmt.Println(userConfig)
	fmt.Println("\nRecommended: Use systemd-homed for JIT user provisioning")
	fmt.Println("Then run: sudo nixos-rebuild switch")
	fmt.Println(strings.Repeat("=", 70))

	return fmt.Errorf("user %s requires configuration - see options above", username)
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

	nixConfig := fmt.Sprintf(`# Add this to your /etc/nixos/configuration.nix file:

systemd.services.%s = {
  enable = true;
  description = "P0 SSH Agent - Secure SSH access management";
  documentation = [ "https://docs.p0.com/" ];
  after = [ "network-online.target" ];
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
		return fmt.Errorf("systemd-homed is not available - enable it in configuration.nix with: services.homed.enable = true")
	}

	// Create user with homectl
	cmd = exec.Command("sudo", "homectl", "create", username,
		"--real-name", fmt.Sprintf("P0 JIT User %s", username),
		"--shell", "/bin/bash",
		"--home-dir", fmt.Sprintf("/home/%s", username),
	)
	
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

	logger.WithField("user", username).Info("✅ JIT user created successfully")
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
		return fmt.Errorf("systemd-homed is not available - cannot remove user")
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

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
		"/usr/bin",        // NixOS doesn't have /usr/local/bin
		"/opt/p0/bin",     // Custom location fallback
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
  
  environment = {
    PATH = "/usr/local/bin:/usr/bin:/bin:/sbin:/usr/sbin";
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
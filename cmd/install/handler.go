package install

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"p0-ssh-agent/internal/config"
	"p0-ssh-agent/utils"
)

// NewInstallCommand creates the install command
func NewInstallCommand(verbose *bool, configPath *string) *cobra.Command {
	var (
		// Install command flags (optional overrides)
		serviceName string
		serviceUser string
	)

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install and setup P0 SSH Agent (merged bootstrap + install)",
		Long: `Install P0 SSH Agent with complete setup including:
- Binary installation to /usr/local/bin/p0-ssh-agent
- Default config creation (if not exists)
- Config validation
- Service user creation  
- JWT key generation
- Systemd service creation
- Setup instructions for manual config editing and service start

This command does NOT automatically start the service - you must manually:
1. Edit the config file with your settings
2. Register the node with P0 backend
3. Start the systemd service yourself`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCompleteInstall(*verbose, *configPath, serviceName, serviceUser)
		},
	}

	// Optional override flags
	cmd.Flags().StringVar(&serviceName, "service-name", "p0-ssh-agent", "Name for the systemd service")
	cmd.Flags().StringVar(&serviceUser, "user", "p0-agent", "User to run the service as")

	return cmd
}

func runCompleteInstall(verbose bool, configPath string, serviceName, serviceUser string) error {
	// Setup logging
	logger := logrus.New()
	if verbose {
		logger.SetLevel(logrus.DebugLevel)
	} else {
		logger.SetLevel(logrus.InfoLevel)
	}

	// Default config path if not specified
	if configPath == "" {
		configPath = "/etc/p0-ssh-agent/config.yaml"
	}

	logger.WithFields(logrus.Fields{
		"service_name": serviceName,
		"service_user": serviceUser,
		"config_path":  configPath,
	}).Info("🚀 Starting complete P0 SSH Agent installation")

	// Step 0: Bootstrap (if needed) - copy binary and create default config
	logger.Info("📦 Step 0: Bootstrap installation")
	if err := runBootstrapSteps(logger); err != nil {
		logger.WithError(err).Error("Failed to bootstrap")
		return fmt.Errorf("failed to bootstrap: %w", err)
	}

	// Step 1: Validate and load configuration  
	logger.Info("📝 Step 1: Validating configuration")
	cfg, err := config.LoadWithOverrides(configPath, nil)
	if err != nil {
		logger.WithError(err).Error("Configuration validation failed")
		logger.Info("💡 Please edit the configuration file and try again:")
		logger.Info("   sudo nano " + configPath)
		return fmt.Errorf("configuration validation failed: %w", err)
	}
	logger.Info("✅ Configuration validated successfully")

	// Step 2: Auto-detect executable path
	logger.Info("🔍 Step 2: Detecting executable path")
	executablePath, err := detectExecutablePath()
	if err != nil {
		logger.WithError(err).Error("Failed to detect executable path")
		return fmt.Errorf("failed to detect executable path: %w", err)
	}
	logger.WithField("path", executablePath).Info("✅ Executable path detected")

	// Step 3: Create service user
	logger.Info("👤 Step 3: Creating service user")
	if err := createServiceUser(serviceUser, cfg.KeyPath, logger); err != nil {
		logger.WithError(err).Error("Failed to create service user")
		return fmt.Errorf("failed to create service user: %w", err)
	}

	// Step 4: Create directories with proper permissions
	logger.Info("📁 Step 4: Creating directories")
	if err := createDirectories(cfg, serviceUser, logger); err != nil {
		logger.WithError(err).Error("Failed to create directories")
		return fmt.Errorf("failed to create directories: %w", err)
	}

	// Step 5: Generate JWT keys
	logger.Info("🔐 Step 5: Generating JWT keys")
	if err := generateJWTKeys(cfg.KeyPath, serviceUser, executablePath, logger); err != nil {
		logger.WithError(err).Error("Failed to generate JWT keys")
		return fmt.Errorf("failed to generate JWT keys: %w", err)
	}

	// Step 6: Create log file with proper permissions
	logger.Info("📄 Step 6: Creating log file")
	if err := createLogFile(cfg.LogPath, serviceUser, logger); err != nil {
		logger.WithError(err).Error("Failed to create log file")
		return fmt.Errorf("failed to create log file: %w", err)
	}

	// Step 7: Generate and install systemd service
	logger.Info("⚙️  Step 7: Creating systemd service")
	if err := createSystemdService(serviceName, serviceUser, executablePath, configPath, logger); err != nil {
		logger.WithError(err).Error("Failed to create systemd service")
		return fmt.Errorf("failed to create systemd service: %w", err)
	}

	// Generate registration code
	registrationCode, err := utils.GenerateRegistrationRequestCode(configPath, logger)
	if err != nil {
		logger.WithError(err).Warn("Failed to generate registration code")
		// Don't fail installation if registration code generation fails
	}

	// Display success message and next steps
	displayInstallationSuccess(serviceName, serviceUser, configPath, registrationCode, executablePath)

	return nil
}

func detectExecutablePath() (string, error) {
	// Try to find the executable in common locations
	possiblePaths := []string{
		"/usr/local/bin/p0-ssh-agent",
		"/usr/bin/p0-ssh-agent",
		"/opt/p0/bin/p0-ssh-agent",
	}

	// Check if running from current directory
	if currentExe, err := os.Executable(); err == nil {
		if filepath.Base(currentExe) == "p0-ssh-agent" {
			possiblePaths = append([]string{currentExe}, possiblePaths...)
		}
	}

	// Check PATH
	if pathExe, err := exec.LookPath("p0-ssh-agent"); err == nil {
		possiblePaths = append([]string{pathExe}, possiblePaths...)
	}

	// Return first existing path
	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("p0-ssh-agent executable not found in common locations")
}

// runBootstrapSteps handles the bootstrap portion of installation
func runBootstrapSteps(logger *logrus.Logger) error {
	// Constants
	const (
		defaultBinaryName = "p0-ssh-agent"
		defaultInstallDir = "/usr/local/bin"
		defaultConfigDir  = "/etc/p0-ssh-agent"
		defaultConfigFile = "/etc/p0-ssh-agent/config.yaml"
	)

	// Check if we're running as root (not allowed)
	if os.Geteuid() == 0 {
		return fmt.Errorf("install command should not be run as root, please run as regular user with sudo privileges")
	}

	// Get current executable path
	currentExe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get current executable path: %w", err)
	}

	// Copy binary to system location (if not already there)
	destPath := filepath.Join(defaultInstallDir, defaultBinaryName)
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		logger.Info("📦 Installing binary to system location...")
		if err := copyBinaryToSystem(currentExe, destPath, logger); err != nil {
			return fmt.Errorf("failed to copy binary: %w", err)
		}
		logger.WithField("path", destPath).Info("✅ Binary installed successfully")
	} else {
		logger.WithField("path", destPath).Info("✅ Binary already exists at system location")
	}

	// Create config directory
	if _, err := os.Stat(defaultConfigDir); os.IsNotExist(err) {
		logger.Info("📁 Creating configuration directory...")
		if err := createConfigDirectory(defaultConfigDir, logger); err != nil {
			return fmt.Errorf("failed to create config directory: %w", err)
		}
		logger.WithField("path", defaultConfigDir).Info("✅ Configuration directory created")
	} else {
		logger.WithField("path", defaultConfigDir).Info("✅ Configuration directory already exists")
	}

	// Create default config file (if it doesn't exist)
	if _, err := os.Stat(defaultConfigFile); os.IsNotExist(err) {
		logger.Info("📝 Creating default configuration file...")
		if err := createDefaultConfig(defaultConfigFile, logger); err != nil {
			return fmt.Errorf("failed to create default config: %w", err)
		}
		logger.WithField("path", defaultConfigFile).Info("✅ Default configuration file created")
	} else {
		logger.WithField("path", defaultConfigFile).Info("✅ Configuration file already exists")
	}

	return nil
}

func copyBinaryToSystem(srcPath, destPath string, logger *logrus.Logger) error {
	logger.WithFields(logrus.Fields{
		"source":      srcPath,
		"destination": destPath,
	}).Debug("Copying binary")

	// Use sudo to copy the binary
	cmd := exec.Command("sudo", "cp", srcPath, destPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		logger.WithError(err).WithField("output", string(output)).Error("Failed to copy binary")
		return fmt.Errorf("failed to copy binary: %w", err)
	}

	// Set executable permissions
	cmd = exec.Command("sudo", "chmod", "+x", destPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		logger.WithError(err).WithField("output", string(output)).Error("Failed to set permissions")
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	return nil
}

func createConfigDirectory(configDir string, logger *logrus.Logger) error {
	cmd := exec.Command("sudo", "mkdir", "-p", configDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		logger.WithError(err).WithField("output", string(output)).Error("Failed to create config directory")
		return fmt.Errorf("failed to create config directory: %w", err)
	}
	return nil
}

func createDefaultConfig(configFile string, logger *logrus.Logger) error {
	configContent := `# P0 SSH Agent Configuration File
# Please update these values for your environment

# Required: Organization and host identification
orgId: "my-organization"           # Replace with your organization ID
hostId: "hostname-goes-here"       # Replace with unique host identifier

# Required: P0 backend connection
tunnelHost: "wss://p0.example.com/websocket"  # Replace with your P0 backend URL

# File paths
keyPath: "/etc/p0-ssh-agent/keys"    # JWT key storage directory
logPath: "/var/log/p0-ssh-agent/service.log"     # Log file path

# Optional: Machine labels for identification
labels:
  - "environment=production"
  - "team=infrastructure"
  - "region=us-west-2"

# Optional: Advanced settings
environment: "production"
tunnelTimeoutMs: 30000
version: "1.0"
`

	// Create temporary file with config content
	tmpFile, err := os.CreateTemp("", "p0-config-*.yaml")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(configContent); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write config content: %w", err)
	}
	tmpFile.Close()

	// Use sudo to copy the config file
	cmd := exec.Command("sudo", "cp", tmpFile.Name(), configFile)
	if output, err := cmd.CombinedOutput(); err != nil {
		logger.WithError(err).WithField("output", string(output)).Error("Failed to copy config file")
		return fmt.Errorf("failed to copy config file: %w", err)
	}

	// Set proper permissions
	cmd = exec.Command("sudo", "chmod", "644", configFile)
	if output, err := cmd.CombinedOutput(); err != nil {
		logger.WithError(err).WithField("output", string(output)).Error("Failed to set config permissions")
		return fmt.Errorf("failed to set config permissions: %w", err)
	}

	return nil
}

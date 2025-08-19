package install

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"p0-ssh-agent/internal/config"
	"p0-ssh-agent/internal/osplugins"
)

func NewInstallCommand(verbose *bool, configPath *string) *cobra.Command {
	var (
		serviceName string
		allowRoot   bool
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
3. Start the systemd service yourself

SECURITY NOTE: By default, this command prevents running as root for security reasons.
Use --allow-root flag only when necessary (e.g., in containers or restricted environments).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCompleteInstall(*verbose, *configPath, serviceName, allowRoot)
		},
	}

	cmd.Flags().StringVar(&serviceName, "service-name", "p0-ssh-agent", "Name for the systemd service")
	cmd.Flags().BoolVar(&allowRoot, "allow-root", false, "Allow installation to run as root (WARNING: Not recommended for security reasons)")

	return cmd
}

func runCompleteInstall(verbose bool, configPath string, serviceName string, allowRoot bool) error {
	logger := logrus.New()
	if verbose {
		logger.SetLevel(logrus.DebugLevel)
	} else {
		logger.SetLevel(logrus.InfoLevel)
	}

	// Select appropriate OS plugin (auto-registered via init() functions)
	osPlugin, err := osplugins.GetPlugin(logger)
	if err != nil {
		logger.WithError(err).Error("Failed to select OS plugin")
		return fmt.Errorf("failed to select OS plugin: %w", err)
	}

	if configPath == "" {
		configPath = filepath.Join(osPlugin.GetConfigDirectory(), "config.yaml")
	}

	logger.WithFields(logrus.Fields{
		"service_name": serviceName,
		"config_path":  configPath,
		"os_plugin":    osPlugin.GetName(),
	}).Info("🚀 Starting complete P0 SSH Agent installation")

	logger.Info("📦 Step 0: Bootstrap installation")
	if err := runBootstrapSteps(logger, allowRoot, osPlugin); err != nil {
		logger.WithError(err).Error("Failed to bootstrap")
		return fmt.Errorf("failed to bootstrap: %w", err)
	}

	logger.Info("📝 Step 1: Validating configuration")
	cfg, err := config.LoadWithOverrides(configPath, nil)
	if err != nil {
		logger.WithError(err).Error("Configuration validation failed")
		logger.Info("💡 Please edit the configuration file and try again:")
		logger.Info("   sudo vi " + configPath)
		return fmt.Errorf("configuration validation failed: %w", err)
	}
	logger.Info("✅ Configuration validated successfully")

	logger.Info("🔍 Step 2: Detecting executable path")
	executablePath, err := detectExecutablePath()
	if err != nil {
		logger.WithError(err).Error("Failed to detect executable path")
		return fmt.Errorf("failed to detect executable path: %w", err)
	}
	logger.WithField("path", executablePath).Info("✅ Executable path detected")

	logger.Info("📁 Step 3: Creating directories")
	if err := createDirectories(cfg, osPlugin, logger); err != nil {
		logger.WithError(err).Error("Failed to create directories")
		return fmt.Errorf("failed to create directories: %w", err)
	}

	logger.Info("🔐 Step 4: Generating JWT keys")
	if err := generateJWTKeys(cfg.KeyPath, executablePath, logger); err != nil {
		logger.WithError(err).Error("Failed to generate JWT keys")
		return fmt.Errorf("failed to generate JWT keys: %w", err)
	}

	// Step 5: Log management handled by systemd/journalctl - no log file creation needed

	logger.Info("⚙️  Step 5: Creating systemd service")
	if err := osPlugin.CreateSystemdService(serviceName, executablePath, configPath, logger); err != nil {
		logger.WithError(err).Error("Failed to create systemd service")
		return fmt.Errorf("failed to create systemd service: %w", err)
	}

	osPlugin.DisplayInstallationSuccess(serviceName, configPath, verbose)

	return nil
}

func detectExecutablePath() (string, error) {
	possiblePaths := []string{
		"/usr/local/bin/p0-ssh-agent",
		"/usr/bin/p0-ssh-agent",
		"/opt/p0/bin/p0-ssh-agent",
	}

	if currentExe, err := os.Executable(); err == nil {
		if filepath.Base(currentExe) == "p0-ssh-agent" {
			possiblePaths = append([]string{currentExe}, possiblePaths...)
		}
	}

	if pathExe, err := exec.LookPath("p0-ssh-agent"); err == nil {
		possiblePaths = append([]string{pathExe}, possiblePaths...)
	}

	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("p0-ssh-agent executable not found in common locations")
}

func runBootstrapSteps(logger *logrus.Logger, allowRoot bool, osPlugin osplugins.OSPlugin) error {
	const (
		defaultBinaryName = "p0-ssh-agent"
	)

	defaultConfigDir := osPlugin.GetConfigDirectory()
	defaultConfigFile := filepath.Join(defaultConfigDir, "config.yaml")

	if os.Geteuid() == 0 && !allowRoot {
		return fmt.Errorf("install command should not be run as root, please run as regular user with sudo privileges (or use --allow-root flag to bypass this check)")
	}

	if os.Geteuid() == 0 && allowRoot {
		logger.Warn("⚠️  Running as root - this bypasses security restrictions and is not recommended")
	}

	currentExe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get current executable path: %w", err)
	}

	// Determine the best installation directory using the plugin
	installDir, err := determineInstallDirFromPlugin(osPlugin, logger)
	if err != nil {
		return fmt.Errorf("failed to determine installation directory: %w", err)
	}

	destPath := filepath.Join(installDir, defaultBinaryName)
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		logger.Info("📦 Installing binary to system location...")
		if err := copyBinaryToSystem(currentExe, destPath, installDir, logger); err != nil {
			return fmt.Errorf("failed to copy binary: %w", err)
		}
		logger.WithField("path", destPath).Info("✅ Binary installed successfully")
	} else {
		logger.WithField("path", destPath).Info("✅ Binary already exists at system location")
	}

	if _, err := os.Stat(defaultConfigDir); os.IsNotExist(err) {
		logger.Info("📁 Creating configuration directory...")
		if err := createConfigDirectory(defaultConfigDir, logger); err != nil {
			return fmt.Errorf("failed to create config directory: %w", err)
		}
		logger.WithField("path", defaultConfigDir).Info("✅ Configuration directory created")
	} else {
		logger.WithField("path", defaultConfigDir).Info("✅ Configuration directory already exists")
	}

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

func determineInstallDirFromPlugin(osPlugin osplugins.OSPlugin, logger *logrus.Logger) (string, error) {
	candidateDirs := osPlugin.GetInstallDirectories()

	for _, dir := range candidateDirs {
		// Check if parent directory exists
		parentDir := filepath.Dir(dir)
		if _, err := os.Stat(parentDir); err == nil {
			// Parent exists, check if target directory exists or can be created
			if _, err := os.Stat(dir); err == nil {
				logger.WithField("dir", dir).Debug("Found existing installation directory")
				return dir, nil
			} else if os.IsNotExist(err) {
				// Directory doesn't exist but parent does, we can create it
				logger.WithField("dir", dir).Debug("Will create installation directory")
				return dir, nil
			}
		}
	}

	return "", fmt.Errorf("no suitable installation directory found")
}

func copyBinaryToSystem(srcPath, destPath, installDir string, logger *logrus.Logger) error {
	logger.WithFields(logrus.Fields{
		"source":      srcPath,
		"destination": destPath,
	}).Debug("Copying binary")

	// Ensure installation directory exists
	if _, err := os.Stat(installDir); os.IsNotExist(err) {
		logger.WithField("dir", installDir).Debug("Creating installation directory")
		cmd := exec.Command("sudo", "mkdir", "-p", installDir)
		if output, err := cmd.CombinedOutput(); err != nil {
			logger.WithError(err).WithField("output", string(output)).Error("Failed to create installation directory")
			return fmt.Errorf("failed to create installation directory: %w", err)
		}
	}

	cmd := exec.Command("sudo", "cp", srcPath, destPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		logger.WithError(err).WithField("output", string(output)).Error("Failed to copy binary")
		return fmt.Errorf("failed to copy binary: %w", err)
	}

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
tunnelHost: "wss://api.p0.app"  # Replace with your P0 backend URL

# File paths
keyPath: "/etc/p0-ssh-agent/keys"    # JWT key storage directory
# Logs managed by systemd/journalctl automatically

# Optional: Machine labels for identification
labels:
  - "environment=production"
  - "team=infrastructure"
  - "region=us-west-2"

# Optional: Advanced settings
environment: "production"
heartbeatIntervalSeconds: 60
version: "1.0"
`

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

	cmd := exec.Command("sudo", "cp", tmpFile.Name(), configFile)
	if output, err := cmd.CombinedOutput(); err != nil {
		logger.WithError(err).WithField("output", string(output)).Error("Failed to copy config file")
		return fmt.Errorf("failed to copy config file: %w", err)
	}

	cmd = exec.Command("sudo", "chmod", "644", configFile)
	if output, err := cmd.CombinedOutput(); err != nil {
		logger.WithError(err).WithField("output", string(output)).Error("Failed to set config permissions")
		return fmt.Errorf("failed to set config permissions: %w", err)
	}

	return nil
}

package bootstrap

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const (
	defaultBinaryName = "p0-ssh-agent"
	defaultInstallDir = "/usr/local/bin"
	defaultConfigDir  = "/etc/p0-ssh-agent"
	defaultConfigFile = "/etc/p0-ssh-agent/config.yaml"
)

func NewBootstrapCommand(verbose *bool, configPath *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bootstrap",
		Short: "Bootstrap P0 SSH Agent installation by copying binary and creating default config",
		Long: `Bootstrap command copies the current executable to the system location and creates
a default configuration file. This eliminates the need for separate bootstrap files.

The bootstrap process:
- Copies the current executable to /usr/local/bin/p0-ssh-agent
- Creates /etc/p0-ssh-agent/ directory
- Generates a default config.yaml file
- Sets proper permissions

After bootstrap, run 'p0-ssh-agent install' to complete the setup.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBootstrap(*verbose)
		},
	}

	return cmd
}

func runBootstrap(verbose bool) error {
	logger := logrus.New()
	if verbose {
		logger.SetLevel(logrus.DebugLevel)
	} else {
		logger.SetLevel(logrus.InfoLevel)
	}

	logger.Info("🚀 P0 SSH Agent Bootstrap")
	logger.Info("==================================================")

	// Check if running as root
	if os.Geteuid() == 0 {
		logger.Error("❌ This command should not be run as root")
		logger.Info("Please run as a regular user with sudo privileges")
		return fmt.Errorf("bootstrap should not be run as root")
	}

	// Get current executable path
	currentExe, err := os.Executable()
	if err != nil {
		logger.WithError(err).Error("❌ Failed to get current executable path")
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	logger.WithField("current_path", currentExe).Debug("Current executable path detected")

	// Copy binary to system location
	logger.Info("📦 Installing P0 SSH Agent binary...")
	destPath := filepath.Join(defaultInstallDir, defaultBinaryName)
	if err := copyBinaryToSystem(currentExe, destPath, logger); err != nil {
		return fmt.Errorf("failed to copy binary: %w", err)
	}
	logger.WithField("path", destPath).Info("✅ Binary installed successfully")

	// Create config directory
	logger.Info("📁 Creating configuration directory...")
	if err := createConfigDirectory(logger); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}
	logger.WithField("path", defaultConfigDir).Info("✅ Configuration directory created")

	// Create default config file
	logger.Info("📝 Creating default configuration file...")
	if err := createDefaultConfig(logger); err != nil {
		return fmt.Errorf("failed to create default config: %w", err)
	}
	logger.WithField("path", defaultConfigFile).Info("✅ Configuration file created")

	// Display next steps
	displayNextSteps(logger)

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

func createConfigDirectory(logger *logrus.Logger) error {
	cmd := exec.Command("sudo", "mkdir", "-p", defaultConfigDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		logger.WithError(err).WithField("output", string(output)).Error("Failed to create config directory")
		return fmt.Errorf("failed to create config directory: %w", err)
	}
	return nil
}

func createDefaultConfig(logger *logrus.Logger) error {
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
	cmd := exec.Command("sudo", "cp", tmpFile.Name(), defaultConfigFile)
	if output, err := cmd.CombinedOutput(); err != nil {
		logger.WithError(err).WithField("output", string(output)).Error("Failed to copy config file")
		return fmt.Errorf("failed to copy config file: %w", err)
	}

	// Set proper permissions
	cmd = exec.Command("sudo", "chmod", "644", defaultConfigFile)
	if output, err := cmd.CombinedOutput(); err != nil {
		logger.WithError(err).WithField("output", string(output)).Error("Failed to set config permissions")
		return fmt.Errorf("failed to set config permissions: %w", err)
	}

	return nil
}

func displayNextSteps(logger *logrus.Logger) {
	logger.Info("")
	logger.Info("📋 Next Steps:")
	logger.Info("==============")
	logger.Info("1. Edit the configuration file:")
	logger.Info("   sudo nano " + defaultConfigFile)
	logger.Info("")
	logger.Info("2. Update the following required fields:")
	logger.Info("   - orgId: Your organization identifier")
	logger.Info("   - hostId: Unique identifier for this machine")
	logger.Info("   - tunnelHost: Your P0 backend WebSocket URL")
	logger.Info("")
	logger.Info("3. Run the installation:")
	logger.Info("   sudo " + defaultBinaryName + " install")
	logger.Info("")
	logger.Info("🎉 Bootstrap complete!")
	logger.Info("")
	logger.Info("ℹ️  The install command will:")
	logger.Info("   • Validate your configuration")
	logger.Info("   • Create service user and directories")
	logger.Info("   • Generate JWT keys")
	logger.Info("   • Register with P0 backend")
	logger.Info("   • Create and start systemd service")
}

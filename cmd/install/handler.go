package install

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"p0-ssh-agent/internal/config"
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
		Short: "Install P0 SSH Agent as a complete systemd service",
		Long: `Install P0 SSH Agent as a systemd service with full setup including:
- Config validation
- Service user creation
- JWT key generation
- Backend registration
- Log file creation with proper permissions
- Systemd service creation and startup

This command reads configuration from /etc/p0-ssh-agent/config.yaml by default.`,
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

	// Step 1: Validate and load configuration
	logger.Info("📝 Step 1: Validating configuration")
	cfg, err := config.LoadWithOverrides(configPath, nil)
	if err != nil {
		logger.WithError(err).Error("Failed to load configuration")
		return fmt.Errorf("failed to load configuration from %s: %w", configPath, err)
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

	// Step 7: Register with backend
	logger.Info("📡 Step 7: Registering with P0 backend")
	if err := registerWithBackend(configPath, serviceUser, executablePath, logger); err != nil {
		logger.WithError(err).Warn("Failed to register with backend - you may need to do this manually")
		// Don't fail the installation if registration fails
	} else {
		logger.Info("✅ Registration completed successfully")
	}

	// Step 8: Generate and install systemd service
	logger.Info("⚙️  Step 8: Creating systemd service")
	if err := createSystemdService(serviceName, serviceUser, executablePath, configPath, logger); err != nil {
		logger.WithError(err).Error("Failed to create systemd service")
		return fmt.Errorf("failed to create systemd service: %w", err)
	}

	// Step 9: Enable and start service
	logger.Info("🚀 Step 9: Starting service")
	if err := enableAndStartService(serviceName, logger); err != nil {
		logger.WithError(err).Error("Failed to start service")
		return fmt.Errorf("failed to start service: %w", err)
	}

	// Display success message and next steps
	displayInstallationSuccess(serviceName, serviceUser, configPath)

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
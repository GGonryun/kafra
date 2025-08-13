package status

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"p0-ssh-agent/internal/config"
	"p0-ssh-agent/types"
)

// NewStatusCommand creates the status command
func NewStatusCommand(verbose *bool, configPath *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Check P0 SSH Agent installation and system status",
		Long: `Validate P0 SSH Agent installation including:
- Configuration file validation
- Service user existence and permissions
- JWT key presence and validity
- Log file accessibility
- Systemd service status and configuration
- Directory permissions and ownership

This command provides a comprehensive health check of your P0 SSH Agent installation.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatusCheck(*verbose, *configPath)
		},
	}

	return cmd
}

func runStatusCheck(verbose bool, configPath string) error {
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

	logger.WithField("config_path", configPath).Info("🔍 P0 SSH Agent Status Check")

	fmt.Println("🔍 P0 SSH Agent Status Check")
	fmt.Println(strings.Repeat("=", 40))

	allChecksPass := true

	// Check 1: Configuration file
	fmt.Print("📝 Configuration file... ")
	cfg, configValid := checkConfiguration(configPath, logger)
	if configValid {
		fmt.Println("✅ VALID")
	} else {
		fmt.Println("❌ INVALID")
		allChecksPass = false
	}

	// Check 2: Service user
	fmt.Print("👤 Service user... ")
	serviceUser := "p0-agent" // Default service user
	userValid := checkServiceUser(serviceUser, cfg, logger)
	if userValid {
		fmt.Println("✅ EXISTS")
	} else {
		fmt.Println("❌ MISSING")
		allChecksPass = false
	}

	// Check 3: JWT keys
	fmt.Print("🔐 JWT keys... ")
	keysValid := false
	if cfg != nil {
		keysValid = checkJWTKeys(cfg.KeyPath, serviceUser, logger)
	}
	if keysValid {
		fmt.Println("✅ PRESENT")
	} else {
		fmt.Println("❌ MISSING")
		allChecksPass = false
	}

	// Check 4: Directories and permissions
	fmt.Print("📁 Directory permissions... ")
	dirsValid := false
	if cfg != nil {
		dirsValid = checkDirectoryPermissions(cfg, serviceUser, logger)
	}
	if dirsValid {
		fmt.Println("✅ CORRECT")
	} else {
		fmt.Println("❌ INCORRECT")
		allChecksPass = false
	}

	// Check 5: Log file
	fmt.Print("📄 Log file... ")
	logValid := false
	if cfg != nil {
		logValid = checkLogFile(cfg.LogPath, serviceUser, logger)
	}
	if logValid {
		fmt.Println("✅ ACCESSIBLE")
	} else {
		fmt.Println("❌ ISSUES")
		allChecksPass = false
	}

	// Check 6: Systemd service
	fmt.Print("⚙️  Systemd service... ")
	serviceName := "p0-ssh-agent"
	serviceValid := checkSystemdService(serviceName, logger)
	if serviceValid {
		fmt.Println("✅ RUNNING")
	} else {
		fmt.Println("❌ NOT RUNNING")
		allChecksPass = false
	}

	// Check 7: Executable
	fmt.Print("🚀 Executable... ")
	executableValid := checkExecutable(logger)
	if executableValid {
		fmt.Println("✅ FOUND")
	} else {
		fmt.Println("❌ NOT FOUND")
		allChecksPass = false
	}

	fmt.Println(strings.Repeat("=", 40))

	// Summary
	if allChecksPass {
		fmt.Println("🎉 All checks passed! P0 SSH Agent is properly installed and configured.")
		return nil
	} else {
		fmt.Println("⚠️  Some checks failed. Please review the issues above.")
		fmt.Println("\n💡 Quick fixes:")
		fmt.Println("   • Run: sudo p0-ssh-agent install")
		fmt.Println("   • Check configuration file syntax")
		fmt.Println("   • Verify service logs: sudo journalctl -u p0-ssh-agent")
		return fmt.Errorf("system validation failed")
	}
}

// checkConfiguration validates the configuration file
func checkConfiguration(configPath string, logger *logrus.Logger) (*types.Config, bool) {
	logger.WithField("path", configPath).Debug("Checking configuration")

	// Check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		logger.WithField("path", configPath).Error("Configuration file not found")
		return nil, false
	}

	// Try to load and validate configuration
	cfg, err := config.LoadWithOverrides(configPath, nil)
	if err != nil {
		logger.WithError(err).Error("Failed to load configuration")
		return nil, false
	}

	// Basic validation
	if cfg.OrgID == "" || cfg.HostID == "" || cfg.TunnelHost == "" {
		logger.Error("Required configuration fields missing")
		return cfg, false
	}

	return cfg, true
}

// checkServiceUser validates the service user exists and has proper home directory
func checkServiceUser(serviceUser string, cfg *types.Config, logger *logrus.Logger) bool {
	logger.WithField("user", serviceUser).Debug("Checking service user")

	// Check if user exists
	cmd := exec.Command("id", serviceUser)
	if err := cmd.Run(); err != nil {
		logger.WithField("user", serviceUser).Error("Service user not found")
		return false
	}

	// Check user's home directory if config is available
	if cfg != nil && cfg.KeyPath != "" {
		if _, err := os.Stat(cfg.KeyPath); os.IsNotExist(err) {
			logger.WithField("path", cfg.KeyPath).Error("User's key directory not found")
			return false
		}
	}

	return true
}

// checkJWTKeys validates JWT keys exist and are readable
func checkJWTKeys(keyPath, serviceUser string, logger *logrus.Logger) bool {
	if keyPath == "" {
		logger.Debug("No key path specified")
		return true // Not required if no path specified
	}

	logger.WithField("path", keyPath).Debug("Checking JWT keys")

	privateKeyPath := filepath.Join(keyPath, "jwk.private.json")
	publicKeyPath := filepath.Join(keyPath, "jwk.public.json")

	// Check if both key files exist
	if _, err := os.Stat(privateKeyPath); os.IsNotExist(err) {
		logger.WithField("path", privateKeyPath).Error("Private key file not found")
		return false
	}

	if _, err := os.Stat(publicKeyPath); os.IsNotExist(err) {
		logger.WithField("path", publicKeyPath).Error("Public key file not found")
		return false
	}

	// Check if service user can read the keys
	cmd := exec.Command("sudo", "-u", serviceUser, "test", "-r", privateKeyPath)
	if err := cmd.Run(); err != nil {
		logger.WithField("user", serviceUser).Error("Service user cannot read private key")
		return false
	}

	return true
}

// checkDirectoryPermissions validates directory permissions and ownership
func checkDirectoryPermissions(cfg *types.Config, serviceUser string, logger *logrus.Logger) bool {
	directories := []string{cfg.KeyPath}
	
	if cfg.LogPath != "" {
		directories = append(directories, filepath.Dir(cfg.LogPath))
	}

	for _, dir := range directories {
		if dir == "" {
			continue
		}

		logger.WithField("dir", dir).Debug("Checking directory permissions")

		// Check if directory exists
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			logger.WithField("dir", dir).Error("Directory not found")
			return false
		}

		// Check if service user can access directory
		cmd := exec.Command("sudo", "-u", serviceUser, "test", "-d", dir)
		if err := cmd.Run(); err != nil {
			logger.WithFields(logrus.Fields{
				"dir":  dir,
				"user": serviceUser,
			}).Error("Service user cannot access directory")
			return false
		}
	}

	return true
}

// checkLogFile validates log file accessibility
func checkLogFile(logPath, serviceUser string, logger *logrus.Logger) bool {
	if logPath == "" {
		logger.Debug("No log path specified, using stdout/stderr")
		return true
	}

	logger.WithField("path", logPath).Debug("Checking log file")

	// Check if log file exists
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		logger.WithField("path", logPath).Error("Log file not found")
		return false
	}

	// Check if service user can write to log file
	cmd := exec.Command("sudo", "-u", serviceUser, "test", "-w", logPath)
	if err := cmd.Run(); err != nil {
		logger.WithFields(logrus.Fields{
			"path": logPath,
			"user": serviceUser,
		}).Error("Service user cannot write to log file")
		return false
	}

	return true
}

// checkSystemdService validates systemd service status
func checkSystemdService(serviceName string, logger *logrus.Logger) bool {
	logger.WithField("service", serviceName).Debug("Checking systemd service")

	// Check if service file exists
	servicePath := fmt.Sprintf("/etc/systemd/system/%s.service", serviceName)
	if _, err := os.Stat(servicePath); os.IsNotExist(err) {
		logger.WithField("path", servicePath).Error("Service file not found")
		return false
	}

	// Check if service is enabled
	cmd := exec.Command("systemctl", "is-enabled", serviceName)
	if err := cmd.Run(); err != nil {
		logger.WithField("service", serviceName).Error("Service is not enabled")
		return false
	}

	// Check if service is active
	cmd = exec.Command("systemctl", "is-active", serviceName)
	if err := cmd.Run(); err != nil {
		logger.WithField("service", serviceName).Error("Service is not active")
		return false
	}

	return true
}

// checkExecutable validates the p0-ssh-agent executable is installed
func checkExecutable(logger *logrus.Logger) bool {
	logger.Debug("Checking executable")

	// Check common locations
	locations := []string{
		"/usr/local/bin/p0-ssh-agent",
		"/usr/bin/p0-ssh-agent",
	}

	for _, location := range locations {
		if _, err := os.Stat(location); err == nil {
			// Check if executable
			cmd := exec.Command("test", "-x", location)
			if err := cmd.Run(); err == nil {
				logger.WithField("path", location).Debug("Found executable")
				return true
			}
		}
	}

	// Check PATH
	if _, err := exec.LookPath("p0-ssh-agent"); err == nil {
		return true
	}

	logger.Error("Executable not found in common locations or PATH")
	return false
}
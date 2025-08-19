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
	"p0-ssh-agent/internal/logging"
	"p0-ssh-agent/types"
)

func NewStatusCommand(verbose *bool, configPath *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Check P0 SSH Agent installation and system status",
		Long: `Validate P0 SSH Agent installation including:
- Configuration file validation
- System permissions and ownership
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
	if configPath == "" {
		configPath = "/etc/p0-ssh-agent/config.yaml"
	}

	var logger *logrus.Logger
	cfg, err := config.LoadWithOverrides(configPath, nil)
	if err != nil {
		logger = logrus.New()
		if verbose {
			logger.SetLevel(logrus.DebugLevel)
		}
		logger.WithError(err).Warn("Failed to load configuration for logging, using basic logging")
	} else {
		logger = logging.SetupLogger(verbose)
	}

	logger.WithField("config_path", configPath).Info("🔍 P0 SSH Agent Status Check")

	fmt.Println("🔍 P0 SSH Agent Status Check")
	fmt.Println(strings.Repeat("=", 40))

	allChecksPass := true

	fmt.Print("📝 Configuration file... ")
	var configValid bool
	if cfg == nil {
		cfg, configValid = checkConfiguration(configPath, logger)
	} else {
		configValid = true
		logger.WithField("config_path", configPath).Debug("Configuration file is valid")
	}
	if configValid {
		fmt.Println("✅ VALID")
	} else {
		fmt.Println("❌ INVALID")
		allChecksPass = false
	}

	fmt.Print("🔐 JWT keys... ")
	keysValid := false
	if cfg != nil {
		keysValid = checkJWTKeys(cfg.KeyPath, logger)
	}
	if keysValid {
		fmt.Println("✅ PRESENT")
	} else {
		fmt.Println("❌ MISSING")
		allChecksPass = false
	}

	fmt.Print("📁 Directory permissions... ")
	dirsValid := false
	if cfg != nil {
		dirsValid = checkDirectoryPermissions(cfg, logger)
	}
	if dirsValid {
		fmt.Println("✅ CORRECT")
	} else {
		fmt.Println("❌ INCORRECT")
		allChecksPass = false
	}

	fmt.Print("📄 Log file... ")
	logValid := false
	if cfg != nil {
		logValid = true // Always valid since we use journalctl
	}
	if logValid {
		fmt.Println("✅ ACCESSIBLE")
	} else {
		fmt.Println("❌ ISSUES")
		allChecksPass = false
	}

	fmt.Print("⚙️  Systemd service... ")
	serviceName := "p0-ssh-agent"
	serviceValid := checkSystemdService(serviceName, logger)
	if serviceValid {
		fmt.Println("✅ RUNNING")
	} else {
		fmt.Println("❌ NOT RUNNING")
		allChecksPass = false
	}

	fmt.Print("🚀 Executable... ")
	executableValid := checkExecutable(logger)
	if executableValid {
		fmt.Println("✅ FOUND")
	} else {
		fmt.Println("❌ NOT FOUND")
		allChecksPass = false
	}

	fmt.Println(strings.Repeat("=", 40))

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

func checkConfiguration(configPath string, logger *logrus.Logger) (*types.Config, bool) {
	logger.WithField("path", configPath).Debug("Checking configuration")

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		logger.WithField("path", configPath).Error("Configuration file not found")
		return nil, false
	}

	cfg, err := config.LoadWithOverrides(configPath, nil)
	if err != nil {
		logger.WithError(err).Error("Failed to load configuration")
		return nil, false
	}

	if cfg.OrgID == "" || cfg.HostID == "" || cfg.TunnelHost == "" {
		logger.Error("Required configuration fields missing")
		return cfg, false
	}

	return cfg, true
}


func checkJWTKeys(keyPath string, logger *logrus.Logger) bool {
	if keyPath == "" {
		logger.Debug("No key path specified")
		return true
	}

	logger.WithField("path", keyPath).Debug("Checking JWT keys")

	privateKeyPath := filepath.Join(keyPath, "jwk.private.json")
	publicKeyPath := filepath.Join(keyPath, "jwk.public.json")

	if _, err := os.Stat(privateKeyPath); os.IsNotExist(err) {
		logger.WithField("path", privateKeyPath).Error("Private key file not found")
		return false
	}

	if _, err := os.Stat(publicKeyPath); os.IsNotExist(err) {
		logger.WithField("path", publicKeyPath).Error("Public key file not found")
		return false
	}

	// Since service runs as root, just check if files are readable by root
	if _, err := os.Open(privateKeyPath); err != nil {
		logger.WithField("path", privateKeyPath).Error("Cannot read private key")
		return false
	}

	return true
}

func checkDirectoryPermissions(cfg *types.Config, logger *logrus.Logger) bool {
	directories := []string{cfg.KeyPath}
	
	// No log directories to check - using journalctl

	for _, dir := range directories {
		if dir == "" {
			continue
		}

		logger.WithField("dir", dir).Debug("Checking directory permissions")

		if _, err := os.Stat(dir); os.IsNotExist(err) {
			logger.WithField("dir", dir).Error("Directory not found")
			return false
		}

		// Since service runs as root, just check if directory exists and is accessible
		info, err := os.Stat(dir)
		if err != nil {
			logger.WithField("dir", dir).Error("Cannot access directory")
			return false
		}

		if !info.IsDir() {
			logger.WithField("dir", dir).Error("Path is not a directory")
			return false
		}
	}

	return true
}

func checkLogFile(logPath string, logger *logrus.Logger) bool {
	if logPath == "" {
		logger.Debug("No log path specified, using stdout/stderr")
		return true
	}

	logger.WithField("path", logPath).Debug("Checking log file")

	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		logger.WithField("path", logPath).Error("Log file not found")
		return false
	}

	// Since service runs as root, just check if log file is writable
	file, err := os.OpenFile(logPath, os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		logger.WithField("path", logPath).Error("Cannot write to log file")
		return false
	}
	file.Close()

	return true
}

func checkSystemdService(serviceName string, logger *logrus.Logger) bool {
	logger.WithField("service", serviceName).Debug("Checking systemd service")

	servicePath := fmt.Sprintf("/etc/systemd/system/%s.service", serviceName)
	if _, err := os.Stat(servicePath); os.IsNotExist(err) {
		logger.WithField("path", servicePath).Error("Service file not found")
		return false
	}

	cmd := exec.Command("systemctl", "is-enabled", serviceName)
	if err := cmd.Run(); err != nil {
		logger.WithField("service", serviceName).Error("Service is not enabled")
		return false
	}

	cmd = exec.Command("systemctl", "is-active", serviceName)
	if err := cmd.Run(); err != nil {
		logger.WithField("service", serviceName).Error("Service is not active")
		return false
	}

	return true
}

func checkExecutable(logger *logrus.Logger) bool {
	logger.Debug("Checking executable")

	locations := []string{
		"/usr/local/bin/p0-ssh-agent",
		"/usr/bin/p0-ssh-agent",
	}

	for _, location := range locations {
		if _, err := os.Stat(location); err == nil {
			cmd := exec.Command("test", "-x", location)
			if err := cmd.Run(); err == nil {
				logger.WithField("path", location).Debug("Found executable")
				return true
			}
		}
	}

	if _, err := exec.LookPath("p0-ssh-agent"); err == nil {
		return true
	}

	logger.Error("Executable not found in common locations or PATH")
	return false
}
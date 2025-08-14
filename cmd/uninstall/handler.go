package uninstall

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func NewUninstallCommand(verbose *bool, configPath *string) *cobra.Command {
	var (
		serviceName string
		serviceUser string
		force       bool
	)

	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Completely remove P0 SSH Agent installation",
		Long: `Completely uninstall P0 SSH Agent including:
- Stop and disable systemd service
- Remove service files and configuration
- Remove service user and directories
- Remove binary from system location
- Clean up all installation artifacts

This command reverses everything done by the install command.

WARNING: This will permanently delete all configuration, keys, and logs.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUninstall(*verbose, *configPath, serviceName, serviceUser, force)
		},
	}

	cmd.Flags().StringVar(&serviceName, "service-name", "p0-ssh-agent", "Name of the systemd service to remove")
	cmd.Flags().StringVar(&serviceUser, "user", "p0-agent", "Service user to remove")
	cmd.Flags().BoolVar(&force, "force", false, "Force removal without confirmation prompts")

	return cmd
}

func runUninstall(verbose bool, configPath string, serviceName, serviceUser string, force bool) error {
	logger := logrus.New()
	if verbose {
		logger.SetLevel(logrus.DebugLevel)
	} else {
		logger.SetLevel(logrus.InfoLevel)
	}

	if configPath == "" {
		configPath = "/etc/p0-ssh-agent/config.yaml"
	}

	logger.WithFields(logrus.Fields{
		"service_name": serviceName,
		"service_user": serviceUser,
		"config_path":  configPath,
		"force":        force,
	}).Info("🗑️ Starting P0 SSH Agent uninstallation")

	if !force {
		fmt.Printf("⚠️ WARNING: This will completely remove P0 SSH Agent including:\n")
		fmt.Printf("- Systemd service (%s)\n", serviceName)
		fmt.Printf("- Service user (%s)\n", serviceUser)
		fmt.Printf("- Configuration directory (/etc/p0-ssh-agent/)\n")
		fmt.Printf("- Log files and keys\n")
		fmt.Printf("- System binary (/usr/local/bin/p0-ssh-agent)\n\n")
		fmt.Printf("Are you sure you want to continue? (y/N): ")

		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" && response != "yes" && response != "YES" {
			fmt.Println("❌ Uninstall cancelled")
			return nil
		}
	}

	steps := []struct {
		name string
		fn   func() error
	}{
		{"Stop and disable systemd service", func() error { return stopAndDisableService(serviceName, logger) }},
		{"Remove systemd service file", func() error { return removeServiceFile(serviceName, logger) }},
		{"Remove service user", func() error { return removeServiceUser(serviceUser, logger) }},
		{"Remove configuration directory", func() error { return removeDirectory("/etc/p0-ssh-agent", logger) }},
		{"Remove log directory", func() error { return removeDirectory("/var/log/p0-ssh-agent", logger) }},
		{"Remove system binary", func() error { return removeBinary("/usr/local/bin/p0-ssh-agent", logger) }},
		{"Reload systemd daemon", func() error { return reloadSystemd(logger) }},
	}

	var errors []error
	for i, step := range steps {
		logger.WithField("step", i+1).Infof("🔄 Step %d: %s", i+1, step.name)
		if err := step.fn(); err != nil {
			logger.WithError(err).Errorf("❌ Failed: %s", step.name)
			errors = append(errors, fmt.Errorf("%s: %w", step.name, err))
		} else {
			logger.Infof("✅ Completed: %s", step.name)
		}
	}

	if len(errors) > 0 {
		logger.Error("⚠️ Uninstallation completed with errors:")
		for _, err := range errors {
			logger.WithError(err).Error("Error encountered")
		}
		displayUninstallSummary(true, errors)
		return fmt.Errorf("uninstallation completed with %d errors", len(errors))
	}

	displayUninstallSummary(false, nil)
	return nil
}

func stopAndDisableService(serviceName string, logger *logrus.Logger) error {
	logger.WithField("service", serviceName).Debug("Checking service status")

	cmd := exec.Command("systemctl", "is-active", serviceName)
	if err := cmd.Run(); err == nil {
		logger.Info("Service is running, stopping...")
		cmd = exec.Command("sudo", "systemctl", "stop", serviceName)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to stop service: %w", err)
		}
		logger.Info("Service stopped")
	} else {
		logger.Debug("Service not running")
	}

	cmd = exec.Command("systemctl", "is-enabled", serviceName)
	if err := cmd.Run(); err == nil {
		logger.Info("Service is enabled, disabling...")
		cmd = exec.Command("sudo", "systemctl", "disable", serviceName)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to disable service: %w", err)
		}
		logger.Info("Service disabled")
	} else {
		logger.Debug("Service not enabled")
	}

	return nil
}

func removeServiceFile(serviceName string, logger *logrus.Logger) error {
	serviceFilePath := fmt.Sprintf("/etc/systemd/system/%s.service", serviceName)
	
	logger.WithField("path", serviceFilePath).Debug("Checking service file")
	
	if _, err := os.Stat(serviceFilePath); os.IsNotExist(err) {
		logger.Debug("Service file does not exist")
		return nil
	}

	cmd := exec.Command("sudo", "rm", "-f", serviceFilePath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to remove service file: %w", err)
	}

	logger.WithField("path", serviceFilePath).Info("Service file removed")
	return nil
}

func removeServiceUser(serviceUser string, logger *logrus.Logger) error {
	logger.WithField("user", serviceUser).Debug("Checking if user exists")

	cmd := exec.Command("id", serviceUser)
	if err := cmd.Run(); err != nil {
		logger.Debug("User does not exist")
		return nil
	}

	logger.Info("Removing service user and home directory")
	cmd = exec.Command("sudo", "userdel", "-r", serviceUser)
	if err := cmd.Run(); err != nil {
		logger.WithError(err).Warn("Failed to remove user with home directory, trying without -r flag")
		cmd = exec.Command("sudo", "userdel", serviceUser)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to remove user: %w", err)
		}
	}

	logger.WithField("user", serviceUser).Info("Service user removed")
	return nil
}

func removeDirectory(dirPath string, logger *logrus.Logger) error {
	logger.WithField("path", dirPath).Debug("Checking directory")

	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		logger.Debug("Directory does not exist")
		return nil
	}

	cmd := exec.Command("sudo", "rm", "-rf", dirPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to remove directory %s: %w", dirPath, err)
	}

	logger.WithField("path", dirPath).Info("Directory removed")
	return nil
}

func removeBinary(binaryPath string, logger *logrus.Logger) error {
	logger.WithField("path", binaryPath).Debug("Checking binary")

	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		logger.Debug("Binary does not exist")
		return nil
	}

	cmd := exec.Command("sudo", "rm", "-f", binaryPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to remove binary: %w", err)
	}

	logger.WithField("path", binaryPath).Info("Binary removed")
	return nil
}

func reloadSystemd(logger *logrus.Logger) error {
	logger.Debug("Reloading systemd daemon")
	
	cmd := exec.Command("sudo", "systemctl", "daemon-reload")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to reload systemd daemon: %w", err)
	}

	logger.Info("Systemd daemon reloaded")
	return nil
}

func displayUninstallSummary(hasErrors bool, errors []error) {
	fmt.Println("\n" + strings.Repeat("=", 60))
	if hasErrors {
		fmt.Println("⚠️ P0 SSH Agent Uninstallation Completed with Errors")
	} else {
		fmt.Println("✅ P0 SSH Agent Uninstallation Completed Successfully")
	}
	fmt.Println(strings.Repeat("=", 60))

	fmt.Println("\n📋 What was removed:")
	fmt.Println("   🗑️ Systemd service (p0-ssh-agent)")
	fmt.Println("   🗑️ Service user (p0-agent)")
	fmt.Println("   🗑️ Configuration directory (/etc/p0-ssh-agent/)")
	fmt.Println("   🗑️ Log directory (/var/log/p0-ssh-agent/)")
	fmt.Println("   🗑️ System binary (/usr/local/bin/p0-ssh-agent)")
	fmt.Println("   🗑️ Service files and permissions")

	if hasErrors {
		fmt.Println("\n❌ Errors encountered:")
		for _, err := range errors {
			fmt.Printf("   • %s\n", err.Error())
		}
		fmt.Println("\n💡 You may need to manually clean up remaining files")
		fmt.Println("💡 Check: sudo systemctl status p0-ssh-agent")
		fmt.Println("💡 Check: ls -la /etc/p0-ssh-agent/")
	} else {
		fmt.Println("\n🎉 P0 SSH Agent has been completely removed from your system")
		fmt.Println("💡 You can safely reinstall anytime with: ./p0-ssh-agent install")
	}

	fmt.Println("\n" + strings.Repeat("=", 60))
}
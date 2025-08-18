package uninstall

import (
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"p0-ssh-agent/internal/osplugins"
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

	// Get the appropriate OS plugin
	osPlugin, err := osplugins.GetPlugin(logger)
	if err != nil {
		return fmt.Errorf("failed to get OS plugin: %w", err)
	}

	logger.WithFields(logrus.Fields{
		"service_name": serviceName,
		"service_user": serviceUser,
		"config_path":  configPath,
		"force":        force,
		"os_plugin":    osPlugin.GetName(),
	}).Info("🗑️ Starting P0 SSH Agent uninstallation")

	if !force {
		fmt.Printf("⚠️ WARNING: This will completely remove P0 SSH Agent including:\n")
		fmt.Printf("- Systemd service (%s)\n", serviceName)
		fmt.Printf("- Configuration directory (/etc/p0-ssh-agent/)\n")
		fmt.Printf("- Log files and keys\n")
		
		// Show OS-specific binary paths
		installDirs := osPlugin.GetInstallDirectories()
		for _, dir := range installDirs {
			fmt.Printf("- System binary (%s/p0-ssh-agent)\n", dir)
		}
		fmt.Printf("\n")
		
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
		{"Uninstall service", func() error { return osPlugin.UninstallService(serviceName, logger) }},
		{"Clean up installation", func() error { return osPlugin.CleanupInstallation(serviceName, logger) }},
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
package install

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// NewInstallCommand creates the install command
func NewInstallCommand(verbose *bool, configPath *string) *cobra.Command {
	var (
		// Install command flags
		serviceName    string
		serviceUser    string
		executablePath string
		workingDir     string
		configFile     string
	)

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install P0 SSH Agent as a systemd service",
		Long: `Install P0 SSH Agent as a systemd service for automatic startup and management.
This command generates a systemd service file and provides installation instructions.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInstall(
				*verbose, *configPath,
				serviceName, serviceUser, executablePath, workingDir, configFile,
			)
		},
	}

	// Install command flags
	cmd.Flags().StringVar(&serviceName, "service-name", "p0-ssh-agent", "Name for the systemd service")
	cmd.Flags().StringVar(&serviceUser, "user", "p0-agent", "User to run the service as")
	cmd.Flags().StringVar(&executablePath, "executable", "", "Path to p0-ssh-agent executable (auto-detected if empty)")
	cmd.Flags().StringVar(&workingDir, "working-dir", "/etc/p0-ssh-agent", "Working directory for the service")
	cmd.Flags().StringVar(&configFile, "config-file", "/etc/p0-ssh-agent/p0-ssh-agent.yaml", "Configuration file path")

	return cmd
}

func runInstall(
	verbose bool, configPath string,
	serviceName, serviceUser, executablePath, workingDir, configFile string,
) error {
	// Setup logging
	logger := logrus.New()
	if verbose {
		logger.SetLevel(logrus.DebugLevel)
	} else {
		logger.SetLevel(logrus.InfoLevel)
	}

	logger.WithFields(logrus.Fields{
		"service_name":    serviceName,
		"service_user":    serviceUser,
		"executable_path": executablePath,
		"working_dir":     workingDir,
		"config_file":     configFile,
	}).Info("🚀 Installing P0 SSH Agent as systemd service")

	// Auto-detect executable path if not provided
	if executablePath == "" {
		var err error
		executablePath, err = detectExecutablePath()
		if err != nil {
			logger.WithError(err).Error("Failed to detect executable path")
			return fmt.Errorf("failed to detect executable path: %w", err)
		}
		logger.WithField("detected_path", executablePath).Info("Auto-detected executable path")
	}

	// Validate executable exists
	if _, err := os.Stat(executablePath); os.IsNotExist(err) {
		return fmt.Errorf("executable not found at %s", executablePath)
	}

	// Generate systemd service content
	serviceContent := generateSystemdService(serviceName, serviceUser, executablePath, workingDir, configFile)
	serviceFilePath := fmt.Sprintf("/etc/systemd/system/%s.service", serviceName)

	// Display the service file content
	fmt.Println("📄 Generated systemd service file:")
	fmt.Println("=" + strings.Repeat("=", 50))
	fmt.Print(serviceContent)
	fmt.Println("=" + strings.Repeat("=", 50))

	// Write service file
	if err := writeServiceFile(serviceFilePath, serviceContent, logger); err != nil {
		logger.WithError(err).Error("Failed to write service file")
		return fmt.Errorf("failed to write service file: %w", err)
	}

	// Create working directory
	if err := createWorkingDirectory(workingDir, serviceUser, logger); err != nil {
		logger.WithError(err).Warn("Failed to create working directory")
	}

	// Display installation instructions
	displayInstallationInstructions(serviceName, serviceUser, workingDir, configFile, executablePath)

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

func generateSystemdService(serviceName, serviceUser, executablePath, workingDir, configFile string) string {
	return fmt.Sprintf(`[Unit]
Description=P0 SSH Agent - Secure SSH access management
Documentation=https://docs.p0.com/
After=network-online.target
Wants=network-online.target
StartLimitInterval=0

[Service]
Type=simple
User=%s
Group=%s
WorkingDirectory=%s
ExecStart=%s start --config %s
ExecReload=/bin/kill -HUP $MAINPID
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal
SyslogIdentifier=%s

# Security settings
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=%s
PrivateTmp=true
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true

# Environment
Environment=PATH=/usr/local/bin:/usr/bin:/bin
Environment=HOME=%s

[Install]
WantedBy=multi-user.target
`, serviceUser, serviceUser, workingDir, executablePath, configFile, serviceName, workingDir, workingDir)
}

func writeServiceFile(filePath, content string, logger *logrus.Logger) error {
	logger.WithField("path", filePath).Info("Writing systemd service file")

	// Check if we need sudo
	if os.Geteuid() != 0 {
		// Write to temporary file first
		tempFile := "/tmp/" + filepath.Base(filePath)
		if err := os.WriteFile(tempFile, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write temporary file: %w", err)
		}

		// Move with sudo
		cmd := exec.Command("sudo", "mv", tempFile, filePath)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to move service file (need sudo): %w", err)
		}

		// Set permissions
		cmd = exec.Command("sudo", "chmod", "644", filePath)
		if err := cmd.Run(); err != nil {
			logger.WithError(err).Warn("Failed to set service file permissions")
		}
	} else {
		// Running as root, write directly
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write service file: %w", err)
		}
	}

	logger.WithField("path", filePath).Info("✅ Service file written successfully")
	return nil
}

func createWorkingDirectory(workingDir, serviceUser string, logger *logrus.Logger) error {
	logger.WithField("dir", workingDir).Info("Creating working directory")

	// Create directory
	cmd := exec.Command("sudo", "mkdir", "-p", workingDir)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Set ownership
	cmd = exec.Command("sudo", "chown", serviceUser+":"+serviceUser, workingDir)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set directory ownership: %w", err)
	}

	// Set permissions
	cmd = exec.Command("sudo", "chmod", "755", workingDir)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set directory permissions: %w", err)
	}

	logger.WithField("dir", workingDir).Info("✅ Working directory created successfully")
	return nil
}

func displayInstallationInstructions(serviceName, serviceUser, workingDir, configFile, executablePath string) {
	fmt.Println("\n🎯 Installation Instructions")
	fmt.Println("=" + strings.Repeat("=", 30))

	fmt.Println("\n1️⃣ Create service user (if it doesn't exist):")
	fmt.Printf("   sudo useradd --system --shell /bin/false --home %s %s\n", workingDir, serviceUser)

	fmt.Println("\n2️⃣ Copy executable to system location:")
	fmt.Printf("   sudo cp %s /usr/local/bin/p0-ssh-agent\n", executablePath)
	fmt.Printf("   sudo chmod +x /usr/local/bin/p0-ssh-agent\n")

	fmt.Println("\n3️⃣ Create configuration file:")
	fmt.Printf("   sudo mkdir -p %s\n", filepath.Dir(configFile))
	fmt.Printf("   sudo cp p0-ssh-agent.yaml %s\n", configFile)
	fmt.Printf("   sudo chown %s:%s %s\n", serviceUser, serviceUser, configFile)

	fmt.Println("\n4️⃣ Generate JWT keys:")
	fmt.Printf("   sudo -u %s p0-ssh-agent keygen --key-path %s\n", serviceUser, workingDir)

	fmt.Println("\n5️⃣ Enable and start the service:")
	fmt.Printf("   sudo systemctl daemon-reload\n")
	fmt.Printf("   sudo systemctl enable %s\n", serviceName)
	fmt.Printf("   sudo systemctl start %s\n", serviceName)

	fmt.Println("\n6️⃣ Check service status:")
	fmt.Printf("   sudo systemctl status %s\n", serviceName)
	fmt.Printf("   sudo journalctl -u %s -f\n", serviceName)

	fmt.Println("\n🔧 Management Commands:")
	fmt.Println("   Start:   sudo systemctl start " + serviceName)
	fmt.Println("   Stop:    sudo systemctl stop " + serviceName)
	fmt.Println("   Restart: sudo systemctl restart " + serviceName)
	fmt.Println("   Status:  sudo systemctl status " + serviceName)
	fmt.Println("   Logs:    sudo journalctl -u " + serviceName + " -f")

	fmt.Println("\n⚠️  Important Notes:")
	fmt.Println("   • The service will automatically restart if it crashes")
	fmt.Println("   • Logs are sent to systemd journal (journalctl)")
	fmt.Printf("   • Configuration file: %s\n", configFile)
	fmt.Printf("   • Working directory: %s\n", workingDir)
	fmt.Printf("   • Service runs as user: %s\n", serviceUser)

	fmt.Println("\n✅ Service installation completed!")
}
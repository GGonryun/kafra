package register

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"p0-ssh-agent/internal/osplugins"
	"p0-ssh-agent/types"
	"p0-ssh-agent/utils"
)

func NewRegisterCommand(verbose *bool, configPath *string) *cobra.Command {
	var (
		auth        string
		url         string
		hostname    string
		labels      []string
		serviceName string
		allowRoot   bool
	)

	cmd := &cobra.Command{
		Use:   "register",
		Short: "Register machine with P0 backend using automatic registration",
		Long: `Register and install P0 SSH Agent using automatic registration.
This command will:
- Install the P0 SSH Agent binary and service files
- Generate JWT keys
- Send registration key to the P0 backend
- Receive configuration from P0 backend
- Save configuration and trusted CA
- Configure SSH daemon to trust the P0 CA
- Set up systemd service

Usage:
  p0 register --auth "bearer-token" --url "https://p0.dev/o/<org-id>/integrations/self-hosted/computers/<environment-id>/register"

Examples:
  # Basic registration
  p0 register --auth "token123" --url "https://p0.dev/o/myorg/integrations/..."
  
  # With custom hostname and labels
  p0 register --auth "token123" --url "https://p0.dev/o/myorg/integrations/..." \
    --hostname "web-server-01" \
    --label "env=production" \
    --label "team=backend" \
    --label "region=us-west-2"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRegister(*verbose, auth, url, hostname, labels, serviceName, allowRoot)
		},
	}

	cmd.Flags().StringVar(&auth, "auth", "", "Bearer token for authentication (required)")
	cmd.Flags().StringVar(&url, "url", "", "Registration URL (required)")
	cmd.Flags().StringVar(&hostname, "hostname", "", "Override machine hostname")
	cmd.Flags().StringSliceVar(&labels, "label", []string{}, "Machine labels in key=value format (can be used multiple times)")
	cmd.Flags().StringVar(&serviceName, "service-name", "p0-ssh-agent", "Name for the systemd service")
	cmd.Flags().BoolVar(&allowRoot, "allow-root", false, "Allow installation to run as root")

	cmd.MarkFlagRequired("auth")
	cmd.MarkFlagRequired("url")

	return cmd
}

type RegistrationResponse struct {
	Ok            bool   `json:"ok"`
	EnvironmentId string `json:"environmentId"`
	HostId        string `json:"hostId"`
	OrgId         string `json:"orgId"`
	TrustedCa     string `json:"trustedCa"`
	TunnelHost    string `json:"tunnelHost"`
}

func runRegister(verbose bool, auth, url, hostname string, labels []string, serviceName string, allowRoot bool) error {
	logger := logrus.New()
	if verbose {
		logger.SetLevel(logrus.DebugLevel)
	} else {
		logger.SetLevel(logrus.InfoLevel)
	}

	logger.Info("🚀 Starting P0 SSH Agent registration and installation...")

	// Step 1: Perform installation steps (merged from install command)
	logger.Info("📦 Step 1: Installing P0 SSH Agent...")
	osPlugin, err := osplugins.GetPlugin(logger)
	if err != nil {
		return fmt.Errorf("failed to select OS plugin: %w", err)
	}

	// Use standard config location for registration (both OS plugins use /etc/p0-ssh-agent)
	configPath := "/etc/p0-ssh-agent/config.yaml"

	// Run installation steps
	if err := runInstallationSteps(logger, osPlugin, serviceName, configPath, allowRoot); err != nil {
		return fmt.Errorf("installation failed: %w", err)
	}

	// Step 2: Send registration request to P0 backend
	logger.Info("🔗 Step 2: Registering with P0 backend...")
	response, err := sendRegistrationRequest(auth, url, hostname, labels, logger)
	if err != nil {
		return fmt.Errorf("registration failed: %w", err)
	}

	if !response.Ok {
		return fmt.Errorf("registration was not successful")
	}

	// Step 3: Save configuration
	logger.Info("💾 Step 3: Saving configuration...")
	if err := saveConfiguration(response, configPath, logger); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	// Step 4: Registration complete
	logger.Info("✅ Step 4: Registration completed successfully")

	// Display OS-specific post-registration instructions
	fmt.Printf("\n✅ Registration successful. Configuration saved to %s\n", configPath)
	osPlugin.DisplayInstallationSuccess(serviceName, configPath, verbose)

	return nil
}

func sendRegistrationRequest(auth, url, hostname string, labels []string, logger *logrus.Logger) (*RegistrationResponse, error) {
	// Generate the registration request using the key path
	keyPath := "/etc/p0-ssh-agent/keys"
	encodedRequest, err := utils.GenerateRegistrationRequestCodeWithOptions(keyPath, hostname, labels, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to generate registration request: %w", err)
	}

	logger.WithFields(logrus.Fields{
		"url":  url,
		"auth": auth[:8] + "...", // Log only first 8 chars for security
	}).Debug("Sending registration request")

	// Wrap the encoded request in a JSON object
	requestBody := map[string]string{
		"key": encodedRequest,
	}

	requestJSON, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	// Create HTTP request with bearer token
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(requestJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+auth)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send registration request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("registration request failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var response RegistrationResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse registration response: %w", err)
	}

	logger.WithFields(logrus.Fields{
		"orgId":      response.OrgId,
		"hostId":     response.HostId,
		"tunnelHost": response.TunnelHost,
	}).Info("Registration response received")

	return &response, nil
}

func saveConfiguration(response *RegistrationResponse, configPath string, logger *logrus.Logger) error {
	config := types.Config{
		Version:                  "1.0",
		OrgID:                    response.OrgId,
		HostID:                   response.HostId,
		TunnelHost:               response.TunnelHost,
		KeyPath:                  "/etc/p0-ssh-agent/keys",
		EnvironmentId:            response.EnvironmentId,
		HeartbeatIntervalSeconds: 60,
		DryRun:                   false,
	}

	// Config will be saved to /etc/p0-ssh-agent/config.yaml (directory already created in runInstallationSteps)

	// Create a temporary file for the config
	tmpFile, err := os.CreateTemp("", "config_*.yaml")
	if err != nil {
		return fmt.Errorf("failed to create temporary config file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	configYAML := fmt.Sprintf(`# P0 SSH Agent Configuration File
# Auto-generated from registration response

version: "%s"
orgId: "%s"
hostId: "%s"
tunnelHost: "%s"
keyPath: "%s"
environmentId: "%s"
heartbeatIntervalSeconds: %d
dryRun: %t
`,
		config.Version,
		config.OrgID,
		config.HostID,
		config.TunnelHost,
		config.KeyPath,
		config.EnvironmentId,
		config.HeartbeatIntervalSeconds,
		config.DryRun,
	)

	if _, err := tmpFile.WriteString(configYAML); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write config to temporary file: %w", err)
	}
	tmpFile.Close()

	// Copy temp file to final location using sudo
	cmd := exec.Command("sudo", "cp", tmpFile.Name(), configPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to copy config file: %w", err)
	}

	// Set proper permissions
	cmd = exec.Command("sudo", "chmod", "644", configPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set config file permissions: %w", err)
	}

	logger.WithField("path", configPath).Info("Configuration saved successfully")
	return nil
}

func runInstallationSteps(logger *logrus.Logger, osPlugin osplugins.OSPlugin, serviceName string, configPath string, allowRoot bool) error {
	// This incorporates the key functionality from the install command

	// Security check
	if os.Geteuid() == 0 && !allowRoot {
		return fmt.Errorf("register command should not be run as root, please run as regular user with sudo privileges (or use --allow-root flag to bypass this check)")
	}

	if os.Geteuid() == 0 && allowRoot {
		logger.Warn("⚠️  Running as root - this bypasses security restrictions and is not recommended")
	}

	// Get current executable
	currentExe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get current executable path: %w", err)
	}

	// Install binary using OS-specific install directories
	installDirs := osPlugin.GetInstallDirectories()
	var destPath string
	var installSuccess bool

	for _, installDir := range installDirs {
		destPath = filepath.Join(installDir, "p0-ssh-agent")

		// Check if binary already exists at this location
		if _, err := os.Stat(destPath); err == nil {
			logger.WithField("path", destPath).Info("✅ Binary already exists at system location")
			installSuccess = true
			break
		}

		// Try to install to this directory
		logger.WithField("installDir", installDir).Info("📦 Attempting to install binary...")
		if err := copyBinary(currentExe, destPath, logger); err != nil {
			logger.WithError(err).WithField("installDir", installDir).Warn("Failed to install to directory, trying next...")
			continue
		}

		logger.WithField("path", destPath).Info("✅ Binary installed successfully")
		installSuccess = true
		break
	}

	if !installSuccess {
		return fmt.Errorf("failed to install binary to any of the available directories: %v", installDirs)
	}

	// Create config and key directories using OS plugin
	configDir := "/etc/p0-ssh-agent"
	keyPath := filepath.Join(configDir, "keys")

	dirsToSetup := []string{configDir, keyPath}
	if err := osPlugin.SetupDirectories(dirsToSetup, "root", logger); err != nil {
		return fmt.Errorf("failed to setup directories: %w", err)
	}

	// Set proper permissions on key directory (readable for public key access, private key will be protected individually)
	cmd := exec.Command("sudo", "chmod", "755", keyPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set key directory permissions: %w", err)
	}

	// Generate JWT keys
	if err := generateJWTKeys(keyPath, destPath, logger); err != nil {
		return fmt.Errorf("failed to generate JWT keys: %w", err)
	}

	// Create systemd service
	if err := osPlugin.CreateSystemdService(serviceName, destPath, configPath, logger); err != nil {
		return fmt.Errorf("failed to create systemd service: %w", err)
	}

	return nil
}

func copyBinary(srcPath, destPath string, logger *logrus.Logger) error {
	logger.WithFields(logrus.Fields{
		"src":  srcPath,
		"dest": destPath,
	}).Debug("Copying binary using sudo")

	// Use sudo to copy the binary to the system location
	cmd := exec.Command("sudo", "cp", srcPath, destPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to copy binary with sudo: %w", err)
	}

	// Use sudo to set executable permissions
	cmd = exec.Command("sudo", "chmod", "755", destPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set executable permissions with sudo: %w", err)
	}

	return nil
}

func generateJWTKeys(keyPath, executablePath string, logger *logrus.Logger) error {
	// Check if keys already exist
	privateKeyPath := filepath.Join(keyPath, "jwk.private.json")
	publicKeyPath := filepath.Join(keyPath, "jwk.public.json")

	if _, err := os.Stat(privateKeyPath); err == nil {
		if _, err := os.Stat(publicKeyPath); err == nil {
			logger.Info("✅ JWT keys already exist")
			return nil
		}
	}

	// Generate new keys using sudo
	cmd := exec.Command("sudo", executablePath, "keygen", "--key-path", keyPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to generate JWT keys: %w (output: %s)", err, string(output))
	}

	// Set appropriate permissions: public key readable by all, private key root-only
	chmodCmd := exec.Command("sudo", "chmod", "644", publicKeyPath)
	if err := chmodCmd.Run(); err != nil {
		return fmt.Errorf("failed to set public key permissions: %w", err)
	}

	chmodPrivateCmd := exec.Command("sudo", "chmod", "600", privateKeyPath)
	if err := chmodPrivateCmd.Run(); err != nil {
		return fmt.Errorf("failed to set private key permissions: %w", err)
	}

	logger.Info("✅ JWT keys generated successfully")
	return nil
}

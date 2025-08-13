package register

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"p0-ssh-agent/internal/config"
	"p0-ssh-agent/internal/jwt"
	"p0-ssh-agent/types"
	"p0-ssh-agent/utils"
)

// NewRegisterCommand creates the register command
func NewRegisterCommand(verbose *bool, configPath *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "register",
		Short: "Generate machine registration request",
		Long: `Generate a machine registration request with system information and configuration.
This command collects system information (hostname, public IP, machine fingerprint)
and creates a base64-encoded registration request that includes both the configured
hostID (unique identifier) and system hostname for P0 backend registration.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRegister(*verbose, *configPath)
		},
	}

	return cmd
}

func runRegister(verbose bool, configPath string) error {
	// Setup logging
	logger := logrus.New()
	if verbose {
		logger.SetLevel(logrus.DebugLevel)
	} else {
		logger.SetLevel(logrus.InfoLevel)
	}

	logger.Info("🔍 Collecting system information for registration...")

	// Load configuration
	cfg, err := config.LoadWithOverrides(configPath, nil)
	if err != nil {
		logger.WithError(err).Error("Failed to load configuration")
		return err
	}

	// Collect system information using utils functions
	// Note: hostname is the system's network name, while hostID (from config) is a unique identifier
	hostname := utils.GetHostname(logger)
	publicIP := utils.GetPublicIP(logger)
	fingerprint := utils.GetMachineFingerprint(logger)
	fingerprintPublicKey := utils.GetMachinePublicKey(logger)
	jwkPublicKey, err := getJWKPublicKey(cfg.GetKeyPath(), logger)
	if err != nil {
		logger.WithError(err).Error("Failed to load JWK public key")
		return fmt.Errorf("failed to load JWK public key: %w", err)
	}

	// Create registration request with both HostID (from config) and Hostname (from system)
	// Labels are kept as string array in "key=value" format
	request := &types.RegistrationRequest{
		HostID:               cfg.HostID,
		Hostname:             hostname,
		PublicIP:             publicIP,
		Fingerprint:          fingerprint,
		FingerprintPublicKey: fingerprintPublicKey,
		JWKPublicKey:         jwkPublicKey,
		EnvironmentID:        cfg.Environment,
		OrgID:                cfg.GetOrgID(),
		Labels:               cfg.Labels,
		Timestamp:            time.Now().UTC().Format(time.RFC3339),
	}

	// Convert to JSON and base64 encode
	jsonData, err := json.Marshal(request)
	if err != nil {
		logger.WithError(err).Error("Failed to marshal registration request")
		return fmt.Errorf("failed to marshal registration request: %w", err)
	}

	encodedRequest := base64.StdEncoding.EncodeToString(jsonData)

	// Display registration information
	displayRegistrationInfo(request, encodedRequest, logger)

	return nil
}

// getJWKPublicKey loads and returns the JWK public key
func getJWKPublicKey(keyPath string, logger *logrus.Logger) (map[string]string, error) {
	publicKeyPath := filepath.Join(keyPath, jwt.PublicKeyFile)

	data, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read public key file: %w", err)
	}

	var jwk map[string]interface{}
	if err := json.Unmarshal(data, &jwk); err != nil {
		return nil, fmt.Errorf("failed to parse JWK: %w", err)
	}

	// Convert to string map
	result := make(map[string]string)
	for k, v := range jwk {
		if str, ok := v.(string); ok {
			result[k] = str
		}
	}

	logger.WithField("keyPath", publicKeyPath).Debug("Loaded JWK public key")
	return result, nil
}

// displayRegistrationInfo displays the registration information in a formatted way
func displayRegistrationInfo(request *types.RegistrationRequest, encodedRequest string, logger *logrus.Logger) {
	fmt.Println("\n🎯 Machine Registration Request Generated")
	fmt.Println("==========================================")

	fmt.Printf("📋 System Information:\n")
	fmt.Printf("   Host ID: %s\n", request.HostID)
	fmt.Printf("   Hostname: %s\n", request.Hostname)
	fmt.Printf("   Public IP: %s\n", request.PublicIP)
	fmt.Printf("   Fingerprint: %s\n", request.Fingerprint)
	fmt.Printf("   Timestamp: %s\n", request.Timestamp)

	fmt.Printf("\n🏢 Configuration:\n")
	fmt.Printf("   Org ID: %s\n", request.OrgID)
	fmt.Printf("   Environment ID: %s\n", request.EnvironmentID)

	if len(request.Labels) > 0 {
		fmt.Printf("   Labels:\n")
		for _, label := range request.Labels {
			fmt.Printf("     %s\n", label)
		}
	}

	fmt.Printf("\n🔑 Keys:\n")
	fmt.Printf("   Fingerprint Public Key: %s...\n", request.FingerprintPublicKey[:32])
	fmt.Printf("   JWK Key Type: %s\n", request.JWKPublicKey["kty"])
	fmt.Printf("   JWK Algorithm: %s\n", request.JWKPublicKey["alg"])

	fmt.Println("\n📦 Base64 Encoded Registration Request:")
	fmt.Println("==========================================")

	// Print in chunks for readability
	const chunkSize = 80
	for i := 0; i < len(encodedRequest); i += chunkSize {
		end := i + chunkSize
		if end > len(encodedRequest) {
			end = len(encodedRequest)
		}
		fmt.Println(encodedRequest[i:end])
	}

	fmt.Println("\n💡 Next Steps:")
	fmt.Println("1. Copy the base64 encoded registration request above")
	fmt.Println("2. Submit it to your P0 backend for machine registration")
	fmt.Println("3. Once approved, you can start the agent with:")
	fmt.Printf("   p0-ssh-agent start --config %s\n", "your-config.yaml")

	fmt.Println("\n⚠️  IMPORTANT: Keep your JWT keys secure!")
}

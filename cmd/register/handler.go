package register

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

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

	// Create registration request using shared utility
	request, err := utils.CreateRegistrationRequest(configPath, logger)
	if err != nil {
		logger.WithError(err).Error("Failed to create registration request")
		return err
	}

	// Generate encoded request using shared utility
	encodedRequest, err := utils.GenerateRegistrationRequestCode(configPath, logger)
	if err != nil {
		logger.WithError(err).Error("Failed to generate registration code")
		return err
	}

	// Display registration information
	displayRegistrationInfo(request, encodedRequest, logger)

	return nil
}


// displayRegistrationInfo displays the registration information in a formatted way
func displayRegistrationInfo(request *types.RegistrationRequest, encodedRequest string, logger *logrus.Logger) {
	fmt.Println("\n🎯 Machine Registration Request Generated")
	fmt.Println("==========================================")

	fmt.Printf("📋 System Information:\n")
	fmt.Printf("   Host ID: %s\n", request.HostID)
	fmt.Printf("   Client ID: %s\n", request.ClientID)
	fmt.Printf("   Hostname: %s\n", request.Hostname)
	fmt.Printf("   Access IP: %s\n", request.AccessIP)
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

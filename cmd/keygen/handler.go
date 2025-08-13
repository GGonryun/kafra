package keygen

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"p0-ssh-agent/internal/config"
	"p0-ssh-agent/internal/jwt"
)

// NewKeygenCommand creates the keygen command
func NewKeygenCommand(verbose *bool, configPath *string) *cobra.Command {
	var (
		// Keygen command flags
		keyPath string
		force   bool
		
		// Deprecated flags (for backward compatibility)
		keygenPath string
	)

	cmd := &cobra.Command{
		Use:   "keygen",
		Short: "Generate JWT keypair for P0 SSH Agent",
		Long: `Generate ES384 JWT keypair for P0 SSH Agent authentication.
This command should be run once to create the keypair that will be registered
with the P0 backend. The public key will be used for machine registration.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runKeygen(*verbose, *configPath, keyPath, force, keygenPath)
		},
	}

	// Keygen command flags
	cmd.Flags().StringVar(&keyPath, "key-path", "", "Directory to store JWT key files")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing keys")
	cmd.Flags().StringVar(&keygenPath, "path", "", "Directory to store JWT key files (deprecated, use --key-path)")

	return cmd
}

func runKeygen(verbose bool, configPath, keyPath string, force bool, keygenPath string) error {
	// Setup logging
	logger := logrus.New()
	if verbose {
		logger.SetLevel(logrus.DebugLevel)
	} else {
		logger.SetLevel(logrus.InfoLevel)
	}
	
	// Determine key path - prioritize command line flags
	finalKeyPath := keyPath
	if finalKeyPath == "" && keygenPath != "" {
		finalKeyPath = keygenPath // Backward compatibility
	}
	
	// If no key path specified via flags, try to load from config
	if finalKeyPath == "" {
		// Create flag overrides map
		flagOverrides := map[string]interface{}{}
		
		// Load configuration from file and apply flag overrides
		cfg, err := config.LoadWithOverrides(configPath, flagOverrides)
		if err != nil {
			logger.WithError(err).Error("Failed to load configuration")
			return err
		}
		finalKeyPath = cfg.KeyPath
	}
	
	// Default to current directory if still no path
	if finalKeyPath == "" {
		finalKeyPath = "."
	}
	
	logger.WithField("path", finalKeyPath).Info("P0 SSH Agent Key Generator")
	
	// Check if keys already exist
	privateKeyPath := filepath.Join(finalKeyPath, jwt.PrivateKeyFile)
	publicKeyPath := filepath.Join(finalKeyPath, jwt.PublicKeyFile)
	
	if !force {
		if _, err := os.Stat(privateKeyPath); err == nil {
			logger.WithField("path", privateKeyPath).Error("Private key already exists")
			logger.Error("Use --force to overwrite existing keys")
			logger.Error("⚠️  WARNING: Overwriting keys will break existing registrations!")
			return fmt.Errorf("keys already exist at %s", finalKeyPath)
		}
	}
	
	// Create JWT manager
	jwtManager := jwt.NewManager(logger)
	
	// Generate new keypair
	if err := jwtManager.GenerateKeyPair(finalKeyPath); err != nil {
		logger.WithError(err).Error("Failed to generate keypair")
		return err
	}
	
	// Display the public key for registration
	publicKey, err := os.ReadFile(publicKeyPath)
	if err != nil {
		logger.WithError(err).Error("Failed to read generated public key")
		return err
	}
	
	fmt.Println("\n🔑 JWT Keypair Generated Successfully!")
	fmt.Printf("📁 Location: %s\n", finalKeyPath)
	fmt.Printf("🔒 Private Key: %s\n", privateKeyPath)
	fmt.Printf("🔓 Public Key: %s\n", publicKeyPath)
	fmt.Println("\n📋 Public Key for Registration:")
	fmt.Println("=================================")
	fmt.Print(string(publicKey))
	fmt.Println("=================================")
	fmt.Println("\n💡 Next Steps:")
	fmt.Println("1. Register the public key above with your P0 backend")
	fmt.Println("2. Keep the private key secure and backed up")
	fmt.Printf("3. Run: p0-ssh-agent start --org-id YOUR_ORG --host-id YOUR_HOST --key-path %s\n", finalKeyPath)
	fmt.Println("\n⚠️  IMPORTANT: Back up these keys! Losing them will require re-registration.")
	
	return nil
}
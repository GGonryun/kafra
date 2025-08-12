package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"p0-ssh-agent/internal/jwt"
)

var (
	// Command line flags
	keyPath   string
	force     bool
	verbose   bool
)

var rootCmd = &cobra.Command{
	Use:   "keygen",
	Short: "Generate JWT keypair for P0 SSH Agent",
	Long: `Generate ES384 JWT keypair for P0 SSH Agent authentication.
This command should be run once to create the keypair that will be registered
with the P0 backend. The public key will be used for machine registration.`,
	RunE: runKeygen,
}

func init() {
	rootCmd.Flags().StringVar(&keyPath, "path", ".", "Directory to store JWT key files")
	rootCmd.Flags().BoolVar(&force, "force", false, "Overwrite existing keys")
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging")
}

func runKeygen(cmd *cobra.Command, args []string) error {
	// Setup logging
	logger := logrus.New()
	if verbose {
		logger.SetLevel(logrus.DebugLevel)
	} else {
		logger.SetLevel(logrus.InfoLevel)
	}
	
	logger.WithField("path", keyPath).Info("P0 SSH Agent Key Generator")
	
	// Check if keys already exist
	privateKeyPath := filepath.Join(keyPath, jwt.PrivateKeyFile)
	publicKeyPath := filepath.Join(keyPath, jwt.PublicKeyFile)
	
	if !force {
		if _, err := os.Stat(privateKeyPath); err == nil {
			logger.WithField("path", privateKeyPath).Error("Private key already exists")
			logger.Error("Use --force to overwrite existing keys")
			logger.Error("⚠️  WARNING: Overwriting keys will break existing registrations!")
			return fmt.Errorf("keys already exist at %s", keyPath)
		}
	}
	
	// Create JWT manager
	jwtManager := jwt.NewManager(logger)
	
	// Generate new keypair
	if err := jwtManager.GenerateKeyPair(keyPath); err != nil {
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
	fmt.Printf("📁 Location: %s\n", keyPath)
	fmt.Printf("🔒 Private Key: %s\n", privateKeyPath)
	fmt.Printf("🔓 Public Key: %s\n", publicKeyPath)
	fmt.Println("\n📋 Public Key for Registration:")
	fmt.Println("=================================")
	fmt.Print(string(publicKey))
	fmt.Println("=================================")
	fmt.Println("\n💡 Next Steps:")
	fmt.Println("1. Register the public key above with your P0 backend")
	fmt.Println("2. Keep the private key secure and backed up")
	fmt.Printf("3. Run: p0-ssh-agent --client-id your-machine --jwk-path %s\n", keyPath)
	fmt.Println("\n⚠️  IMPORTANT: Back up these keys! Losing them will require re-registration.")
	
	return nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
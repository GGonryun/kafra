package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"p0-ssh-agent/internal/client"
	"p0-ssh-agent/internal/jwt"
	"p0-ssh-agent/pkg/types"
)

var (
	// Global flags
	verbose bool
	
	// Start command flags
	clientID     string
	tunnelHost   string
	tunnelPort   int
	tunnelPath   string
	insecure     bool
	jwkPath      string
	
	// Keygen command flags
	keygenPath   string
	force        bool
)

var rootCmd = &cobra.Command{
	Use:   "p0-ssh-agent",
	Short: "P0 SSH Agent - connects to P0 backend and manages JWT keys",
	Long: `P0 SSH Agent connects to the P0 backend via WebSocket and logs incoming 
requests for monitoring and debugging purposes. It also provides key generation 
functionality for JWT authentication.`,
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the WebSocket proxy agent",
	Long: `Start the P0 SSH Agent WebSocket proxy that connects to the P0 backend 
and logs incoming requests for monitoring and debugging purposes.`,
	RunE: runAgent,
}

var keygenCmd = &cobra.Command{
	Use:   "keygen",
	Short: "Generate JWT keypair for authentication",
	Long: `Generate ES384 JWT keypair for P0 SSH Agent authentication.
This command should be run once to create the keypair that will be registered
with the P0 backend. The public key will be used for machine registration.`,
	RunE: runKeygen,
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging")
	
	// Start command flags
	startCmd.Flags().StringVar(&clientID, "client-id", "", "Client identifier (required)")
	startCmd.Flags().StringVar(&tunnelHost, "tunnel-host", "localhost", "P0 backend host")
	startCmd.Flags().IntVar(&tunnelPort, "tunnel-port", 8080, "P0 backend port")
	startCmd.Flags().StringVar(&tunnelPath, "tunnel-path", "/", "WebSocket endpoint path")
	startCmd.Flags().BoolVar(&insecure, "insecure", false, "Use insecure WebSocket connection (ws instead of wss)")
	startCmd.Flags().StringVar(&jwkPath, "jwk-path", ".", "Path to store JWT key files")
	startCmd.MarkFlagRequired("client-id")
	
	// Keygen command flags
	keygenCmd.Flags().StringVar(&keygenPath, "path", ".", "Directory to store JWT key files")
	keygenCmd.Flags().BoolVar(&force, "force", false, "Overwrite existing keys")
	
	// Add subcommands
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(keygenCmd)
}

func runAgent(cmd *cobra.Command, args []string) error {
	// Setup logging
	logger := logrus.New()
	if verbose {
		logger.SetLevel(logrus.DebugLevel)
	} else {
		logger.SetLevel(logrus.InfoLevel)
	}
	
	// Create configuration from flags
	config := &types.Config{
		TargetURL:  "", // Not used - placeholder only
		ClientID:   clientID,
		TunnelHost: tunnelHost,
		TunnelPort: tunnelPort,
		TunnelPath: tunnelPath,
		Insecure:   insecure,
		JWKPath:    jwkPath,
	}
	
	// Create and start client
	client, err := client.New(config, logger)
	if err != nil {
		logger.WithError(err).Error("Failed to create P0 SSH Agent client")
		
		// Provide helpful guidance for common errors
		if strings.Contains(err.Error(), "failed to load JWT key") {
			logger.Error("🔑 Keys not found or invalid! Generate them first:")
			logger.Errorf("   1. Generate keys: p0-ssh-agent keygen --path %s", config.JWKPath)
			logger.Error("   2. Register public key with P0 backend")
			logger.Error("   3. Run agent again")
		} else if strings.Contains(err.Error(), "permission denied") {
			logger.Error("💡 Fix: Try running with --jwk-path pointing to a writable directory")
			logger.Error("   Example: --jwk-path $HOME/.p0/keys")
			logger.Error("   Or: mkdir -p ~/.p0/keys && chmod 700 ~/.p0/keys")
		}
		
		return err
	}
	
	// Setup signal handling for graceful shutdown
	var gracefulShutdown bool
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	go func() {
		<-sigChan
		logger.Info("Received shutdown signal, shutting down P0 SSH Agent gracefully...")
		gracefulShutdown = true
		client.Shutdown()
	}()
	
	logger.WithFields(logrus.Fields{
		"clientId":    config.ClientID,
		"tunnelHost":  config.TunnelHost,
		"tunnelPort":  config.TunnelPort,
		"tunnelPath":  config.TunnelPath,
		"insecure":    config.Insecure,
	}).Info("Starting P0 SSH Agent")
	
	// Run agent
	if err := client.Run(); err != nil {
		// Check if it's a graceful shutdown vs actual error
		if gracefulShutdown {
			logger.Info("P0 SSH Agent stopped")
			return nil
		}
		logger.WithError(err).Error("P0 SSH Agent stopped with error")
		return err
	}
	
	logger.Info("P0 SSH Agent stopped")
	return nil
}

func runKeygen(cmd *cobra.Command, args []string) error {
	// Setup logging
	logger := logrus.New()
	if verbose {
		logger.SetLevel(logrus.DebugLevel)
	} else {
		logger.SetLevel(logrus.InfoLevel)
	}
	
	logger.WithField("path", keygenPath).Info("P0 SSH Agent Key Generator")
	
	// Check if keys already exist
	privateKeyPath := filepath.Join(keygenPath, jwt.PrivateKeyFile)
	publicKeyPath := filepath.Join(keygenPath, jwt.PublicKeyFile)
	
	if !force {
		if _, err := os.Stat(privateKeyPath); err == nil {
			logger.WithField("path", privateKeyPath).Error("Private key already exists")
			logger.Error("Use --force to overwrite existing keys")
			logger.Error("⚠️  WARNING: Overwriting keys will break existing registrations!")
			return fmt.Errorf("keys already exist at %s", keygenPath)
		}
	}
	
	// Create JWT manager
	jwtManager := jwt.NewManager(logger)
	
	// Generate new keypair
	if err := jwtManager.GenerateKeyPair(keygenPath); err != nil {
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
	fmt.Printf("📁 Location: %s\n", keygenPath)
	fmt.Printf("🔒 Private Key: %s\n", privateKeyPath)
	fmt.Printf("🔓 Public Key: %s\n", publicKeyPath)
	fmt.Println("\n📋 Public Key for Registration:")
	fmt.Println("=================================")
	fmt.Print(string(publicKey))
	fmt.Println("=================================")
	fmt.Println("\n💡 Next Steps:")
	fmt.Println("1. Register the public key above with your P0 backend")
	fmt.Println("2. Keep the private key secure and backed up")
	fmt.Printf("3. Run: p0-ssh-agent start --client-id your-machine --jwk-path %s\n", keygenPath)
	fmt.Println("\n⚠️  IMPORTANT: Back up these keys! Losing them will require re-registration.")
	
	return nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
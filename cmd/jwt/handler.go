package jwt

import (
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"p0-ssh-agent/internal/config"
	"p0-ssh-agent/internal/jwt"
	"p0-ssh-agent/internal/logging"
)

func NewJWTCommand(verbose *bool, configPath *string) *cobra.Command {
	var (
		keyPath     string
		clientID    string
		orgID       string
		hostID      string
		tunnelID    string
		expiration  string
	)

	cmd := &cobra.Command{
		Use:   "jwt",
		Short: "Generate a signed JWT token for websocket connections",
		Long: `Generate a signed JWT token for manual websocket connections.
This command creates a JWT using existing keypairs for direct websocket authentication.
Useful for debugging, testing, or custom integrations.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runJWT(*verbose, *configPath, keyPath, clientID, orgID, hostID, tunnelID, expiration)
		},
	}

	cmd.Flags().StringVar(&keyPath, "key-path", "", "Directory containing JWT key files")
	cmd.Flags().StringVar(&clientID, "client-id", "", "Client ID (if not provided, will use orgId:hostId:ssh)")
	cmd.Flags().StringVar(&orgID, "org-id", "", "Organization ID")
	cmd.Flags().StringVar(&hostID, "host-id", "", "Host ID")
	cmd.Flags().StringVar(&tunnelID, "tunnel-id", "my-tunnel-id", "Tunnel ID for the JWT claim")
	cmd.Flags().StringVar(&expiration, "expiration", "168h", "Token expiration duration (e.g., 24h, 7d, 168h)")

	return cmd
}

func runJWT(verbose bool, configPath, keyPath, clientID, orgID, hostID, tunnelID, expiration string) error {
	flagOverrides := map[string]interface{}{
		"keyPath": keyPath,
		"orgId":   orgID,
		"hostId":  hostID,
	}

	var logger *logrus.Logger
	var finalKeyPath string
	var finalClientID string

	cfg, err := config.LoadWithOverrides(configPath, flagOverrides)
	if err != nil {
		logger = logrus.New()
		if verbose {
			logger.SetLevel(logrus.DebugLevel)
		}
		logger.WithError(err).Warn("Failed to load configuration, using command line flags")
	} else {
		logger = logging.SetupLogger(verbose)
	}

	// Determine key path
	finalKeyPath = keyPath
	if finalKeyPath == "" && cfg != nil {
		finalKeyPath = cfg.KeyPath
	}
	if finalKeyPath == "" {
		finalKeyPath = "."
	}

	// Determine client ID
	if clientID != "" {
		finalClientID = clientID
	} else {
		finalOrgID := orgID
		finalHostID := hostID
		
		if finalOrgID == "" && cfg != nil {
			finalOrgID = cfg.OrgID
		}
		if finalHostID == "" && cfg != nil {
			finalHostID = cfg.HostID
		}
		
		if finalOrgID == "" || finalHostID == "" {
			return fmt.Errorf("either --client-id or both --org-id and --host-id must be provided")
		}
		
		finalClientID = finalOrgID + ":" + finalHostID + ":ssh"
	}

	logger.WithFields(logrus.Fields{
		"keyPath":  finalKeyPath,
		"clientID": finalClientID,
		"tunnelID": tunnelID,
	}).Info("P0 SSH Agent JWT Generator")

	// Parse expiration duration
	duration, err := time.ParseDuration(expiration)
	if err != nil {
		return fmt.Errorf("invalid expiration duration '%s': %w", expiration, err)
	}

	// Create JWT manager and load keys
	jwtManager := jwt.NewManager(logger)
	if err := jwtManager.LoadKey(finalKeyPath); err != nil {
		return fmt.Errorf("failed to load JWT keys: %w", err)
	}

	// Create custom JWT with tunnel ID and expiration
	token, err := jwtManager.CreateJWTWithOptions(finalClientID, tunnelID, duration)
	if err != nil {
		return fmt.Errorf("failed to create JWT: %w", err)
	}

	fmt.Println("\n🔐 JWT Token Generated Successfully!")
	fmt.Printf("👤 Client ID: %s\n", finalClientID)
	fmt.Printf("🎯 Tunnel ID: %s\n", tunnelID)
	fmt.Printf("⏰ Expires: %s\n", time.Now().Add(duration).Format(time.RFC3339))
	fmt.Println("\n📋 JWT Token:")
	fmt.Println("=================================")
	fmt.Println(token)
	fmt.Println("=================================")
	fmt.Println("\n💡 Usage Example:")
	fmt.Printf("websocat 'ws://localhost:8080/ws' -H 'Authorization: Bearer %s'\n", token)
	fmt.Println("\n⚠️  SECURITY: This token grants access to your websocket. Keep it secure!")

	return nil
}
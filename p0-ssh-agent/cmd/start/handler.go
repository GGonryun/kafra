package start

import (
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"p0-ssh-agent/internal/client"
	"p0-ssh-agent/internal/config"
)

// NewStartCommand creates the start command
func NewStartCommand(verbose *bool, configPath *string) *cobra.Command {
	var (
		// Start command flags
		orgID           string
		hostID          string
		tunnelHost      string
		tunnelPort      int
		tunnelPath      string
		insecure        bool
		keyPath         string
		logPath         string
		labels          []string
		environment     string
		tunnelTimeoutMs int

		// Deprecated flags (for backward compatibility)
		tenantID string
		jwkPath  string
	)

	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the WebSocket proxy agent",
		Long: `Start the P0 SSH Agent WebSocket proxy that connects to the P0 backend 
and logs incoming requests for monitoring and debugging purposes.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStart(
				*verbose, *configPath,
				orgID, hostID, tunnelHost, tunnelPort, tunnelPath,
				insecure, keyPath, logPath, labels, environment,
				tunnelTimeoutMs, tenantID, jwkPath,
			)
		},
	}

	// Start command flags
	cmd.Flags().StringVar(&orgID, "org-id", "", "Organization identifier (required)")
	cmd.Flags().StringVar(&hostID, "host-id", "", "Host identifier (required)")
	cmd.Flags().StringVar(&tunnelHost, "tunnel-host", "", "P0 backend host")
	cmd.Flags().IntVar(&tunnelPort, "tunnel-port", 0, "P0 backend port")
	cmd.Flags().StringVar(&tunnelPath, "tunnel-path", "", "WebSocket endpoint path")
	cmd.Flags().BoolVar(&insecure, "insecure", false, "Use insecure WebSocket connection (ws instead of wss)")
	cmd.Flags().StringVar(&keyPath, "key-path", "", "Path to store JWT key files")
	cmd.Flags().StringVar(&logPath, "log-path", "", "Path to store log files (for daemon mode)")
	cmd.Flags().StringSliceVar(&labels, "labels", []string{}, "Machine labels for registration (can be used multiple times)")
	cmd.Flags().StringVar(&environment, "environment", "", "Environment ID for registration")
	cmd.Flags().IntVar(&tunnelTimeoutMs, "tunnel-timeout", 0, "Tunnel timeout in milliseconds")

	// Backward compatibility flags
	cmd.Flags().StringVar(&tenantID, "tenant-id", "", "Tenant identifier (deprecated, use --org-id)")
	cmd.Flags().StringVar(&jwkPath, "jwk-path", "", "Path to store JWT key files (deprecated, use --key-path)")

	return cmd
}

func runStart(
	verbose bool, configPath string,
	orgID, hostID, tunnelHost string, tunnelPort int, tunnelPath string,
	insecure bool, keyPath, logPath string, labels []string, environment string,
	tunnelTimeoutMs int, tenantID, jwkPath string,
) error {
	// Setup logging
	logger := logrus.New()
	if verbose {
		logger.SetLevel(logrus.DebugLevel)
	} else {
		logger.SetLevel(logrus.InfoLevel)
	}

	// Create flag overrides map
	flagOverrides := map[string]interface{}{
		"orgId":           orgID,
		"hostId":          hostID,
		"tunnelHost":      tunnelHost,
		"tunnelPort":      tunnelPort,
		"tunnelPath":      tunnelPath,
		"insecure":        insecure,
		"keyPath":         keyPath,
		"logPath":         logPath,
		"labels":          labels,
		"environment":     environment,
		"tunnelTimeoutMs": tunnelTimeoutMs,
		// Backward compatibility
		"tenantId": tenantID,
		"jwkPath":  jwkPath,
	}

	// Load configuration from file and apply flag overrides
	cfg, err := config.LoadWithOverrides(configPath, flagOverrides)
	if err != nil {
		logger.WithError(err).Error("Failed to load configuration")
		return err
	}

	// Note: tenantId and hostId validation is now handled by the config validation

	// Create and start client
	client, err := client.New(cfg, logger)
	if err != nil {
		logger.WithError(err).Error("Failed to create P0 SSH Agent client")

		// Provide helpful guidance for common errors
		if strings.Contains(err.Error(), "failed to load JWT key") {
			logger.Error("🔑 Keys not found or invalid! Generate them first:")
			logger.Errorf("   1. Generate keys: p0-ssh-agent keygen --key-path %s", cfg.GetKeyPath())
			logger.Error("   2. Register public key with P0 backend")
			logger.Error("   3. Run agent again")
		} else if strings.Contains(err.Error(), "permission denied") {
			logger.Error("💡 Fix: Try running with --key-path pointing to a writable directory")
			logger.Error("   Example: --key-path $HOME/.p0/keys")
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
		"version":         cfg.Version,
		"tenantId":        cfg.TenantID,
		"hostId":          cfg.HostID,
		"clientId":        cfg.GetClientID(),
		"tunnelHost":      cfg.TunnelHost,
		"tunnelPort":      cfg.TunnelPort,
		"tunnelPath":      cfg.TunnelPath,
		"insecure":        cfg.Insecure,
		"keyPath":         cfg.GetKeyPath(),
		"logPath":         cfg.LogPath,
		"labels":          cfg.Labels,
		"environment":     cfg.Environment,
		"tunnelTimeoutMs": cfg.TunnelTimeoutMs,
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

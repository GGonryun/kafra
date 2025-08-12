package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"braekhus-go/internal/client"
	"braekhus-go/pkg/types"
)

var (
	// Command line flags
	targetURL    string
	clientID     string
	tunnelHost   string
	tunnelPort   int
	insecure     bool
	jwkPath      string
	verbose      bool
)

var rootCmd = &cobra.Command{
	Use:   "braekhus-client",
	Short: "Braekhus reverse proxy client",
	Long: `Braekhus client connects to a braekhus server and forwards HTTP requests 
to a target service running on the local network.`,
	RunE: runClient,
}

func init() {
	rootCmd.Flags().StringVar(&targetURL, "target-url", "", "Target URL to forward requests to (required)")
	rootCmd.Flags().StringVar(&clientID, "client-id", "", "Client identifier (required)")
	rootCmd.Flags().StringVar(&tunnelHost, "tunnel-host", "localhost", "Tunnel server host")
	rootCmd.Flags().IntVar(&tunnelPort, "tunnel-port", 8080, "Tunnel server port")
	rootCmd.Flags().BoolVar(&insecure, "insecure", false, "Use insecure WebSocket connection (ws instead of wss)")
	rootCmd.Flags().StringVar(&jwkPath, "jwk-path", ".", "Path to store JWT key files")
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging")
	
	rootCmd.MarkFlagRequired("target-url")
	rootCmd.MarkFlagRequired("client-id")
}

func runClient(cmd *cobra.Command, args []string) error {
	// Setup logging
	logger := logrus.New()
	if verbose {
		logger.SetLevel(logrus.DebugLevel)
	} else {
		logger.SetLevel(logrus.InfoLevel)
	}
	
	// Create configuration from flags
	config := &types.Config{
		TargetURL:  targetURL,
		ClientID:   clientID,
		TunnelHost: tunnelHost,
		TunnelPort: tunnelPort,
		Insecure:   insecure,
		JWKPath:    jwkPath,
	}
	
	// Create and start client
	client, err := client.New(config, logger)
	if err != nil {
		logger.WithError(err).Fatal("Failed to create client")
		return err
	}
	
	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	go func() {
		<-sigChan
		logger.Info("Received shutdown signal, shutting down gracefully...")
		client.Shutdown()
	}()
	
	logger.WithFields(logrus.Fields{
		"targetUrl":  config.TargetURL,
		"clientId":   config.ClientID,
		"tunnelHost": config.TunnelHost,
		"tunnelPort": config.TunnelPort,
		"insecure":   config.Insecure,
	}).Info("Starting braekhus client")
	
	// Run client
	if err := client.Run(); err != nil {
		logger.WithError(err).Error("Client stopped with error")
		return err
	}
	
	logger.Info("Client stopped")
	return nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
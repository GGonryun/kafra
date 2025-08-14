package main

import (
	"os"

	"github.com/spf13/cobra"

	"p0-ssh-agent/cmd/command"
	"p0-ssh-agent/cmd/install"
	"p0-ssh-agent/cmd/keygen"
	"p0-ssh-agent/cmd/register"
	"p0-ssh-agent/cmd/start"
	"p0-ssh-agent/cmd/status"
	"p0-ssh-agent/cmd/uninstall"
)

var (
	verbose    bool
	configPath string
)

var rootCmd = &cobra.Command{
	Use:   "p0-ssh-agent",
	Short: "P0 SSH Agent - connects to P0 backend and manages JWT keys",
	Long: `P0 SSH Agent connects to the P0 backend via WebSocket and logs incoming 
requests for monitoring and debugging purposes. It also provides key generation 
functionality for JWT authentication.`,
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging")
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "Path to configuration file")

	rootCmd.AddCommand(start.NewStartCommand(&verbose, &configPath))
	rootCmd.AddCommand(keygen.NewKeygenCommand(&verbose, &configPath))
	rootCmd.AddCommand(register.NewRegisterCommand(&verbose, &configPath))
	rootCmd.AddCommand(install.NewInstallCommand(&verbose, &configPath))
	rootCmd.AddCommand(uninstall.NewUninstallCommand(&verbose, &configPath))
	rootCmd.AddCommand(status.NewStatusCommand(&verbose, &configPath))
	rootCmd.AddCommand(command.NewCommandCommand(&verbose, &configPath))
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

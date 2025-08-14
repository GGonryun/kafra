package command

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"p0-ssh-agent/scripts"
)

func NewCommandCommand(verbose *bool, configPath *string) *cobra.Command {
	var (
		command   string
		userName  string
		action    string
		requestID string
		publicKey string
		sudo      bool
		dryRun    bool
	)

	cmd := &cobra.Command{
		Use:   "command",
		Short: "Execute provisioning scripts directly for testing",
		Long: `Execute P0 SSH Agent provisioning scripts directly for testing and validation.
This command allows you to test user provisioning, SSH key management, and sudo access
without needing a full P0 backend connection.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCommand(
				*verbose, *configPath,
				command, userName, action, requestID, publicKey, sudo, dryRun,
			)
		},
	}

	cmd.Flags().StringVar(&command, "command", "", "Command to execute (provisionUser, provisionAuthorizedKeys, provisionSudo, provisionSession)")
	cmd.Flags().StringVar(&userName, "username", "", "Username for the operation")
	cmd.Flags().StringVar(&action, "action", "grant", "Action to perform (grant or revoke)")
	cmd.Flags().StringVar(&requestID, "request-id", "", "Request ID for tracking (auto-generated if empty)")
	cmd.Flags().StringVar(&publicKey, "public-key", "", "SSH public key for authorized keys operations")
	cmd.Flags().BoolVar(&sudo, "sudo", false, "Grant sudo access")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Log commands but don't execute them (safe testing mode)")

	cmd.MarkFlagRequired("command")
	cmd.MarkFlagRequired("username")

	return cmd
}

func runCommand(
	verbose bool, configPath string,
	command, userName, action, requestID, publicKey string, sudo, dryRun bool,
) error {
	logger := logrus.New()
	if verbose {
		logger.SetLevel(logrus.DebugLevel)
	} else {
		logger.SetLevel(logrus.InfoLevel)
	}

	if requestID == "" {
		requestID = fmt.Sprintf("cmd-%d", generateRequestID(userName))
	}

	logger.WithFields(logrus.Fields{
		"command":    command,
		"username":   userName,
		"action":     action,
		"request_id": requestID,
		"sudo":       sudo,
		"dry_run":    dryRun,
		"has_key":    publicKey != "",
	}).Info("🧪 Executing provisioning command")

	req := scripts.ProvisioningRequest{
		UserName:  userName,
		Action:    action,
		RequestID: requestID,
		PublicKey: publicKey,
		Sudo:      sudo,
	}

	fmt.Println("📋 Provisioning Request:")
	fmt.Println("=" + strings.Repeat("=", 30))
	requestJSON, _ := json.MarshalIndent(req, "", "  ")
	fmt.Println(string(requestJSON))
	fmt.Println("=" + strings.Repeat("=", 30))

	result := scripts.ExecuteScript(command, req, dryRun, logger)

	fmt.Println("\n📊 Execution Result:")
	fmt.Println("=" + strings.Repeat("=", 25))
	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(resultJSON))

	if result.Success {
		fmt.Println("\n✅ Command executed successfully!")
		if dryRun {
			fmt.Println("🔍 DRY-RUN: No actual changes were made to the system")
		}
	} else {
		fmt.Println("\n❌ Command execution failed!")
		fmt.Printf("Error: %s\n", result.Error)
		return fmt.Errorf("command execution failed: %s", result.Error)
	}

	return nil
}

func generateRequestID(userName string) int64 {
	return int64(1000000 + (hash(userName) % 8999999))
}

func hash(s string) int {
	h := 0
	for _, c := range s {
		h = 31*h + int(c)
	}
	if h < 0 {
		h = -h
	}
	return h
}
package register

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"p0-ssh-agent/types"
	"p0-ssh-agent/utils"
)

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
	logger := logrus.New()
	if verbose {
		logger.SetLevel(logrus.DebugLevel)
	} else {
		logger.SetLevel(logrus.InfoLevel)
	}

	logger.Info("🔍 Collecting system information for registration...")

	request, err := utils.CreateRegistrationRequest(configPath, logger)
	if err != nil {
		logger.WithError(err).Error("Failed to create registration request")
		return err
	}

	encodedRequest, err := utils.GenerateRegistrationRequestCode(configPath, logger)
	if err != nil {
		logger.WithError(err).Error("Failed to generate registration code")
		return err
	}

	displayRegistrationInfo(request, encodedRequest, configPath, logger)

	return nil
}


func displayRegistrationInfo(request *types.RegistrationRequest, encodedRequest string, configPath string, logger *logrus.Logger) {
	fmt.Println("\n📦 Base64 Encoded Registration Request:")
	fmt.Println("==========================================")

	const chunkSize = 80
	for i := 0; i < len(encodedRequest); i += chunkSize {
		end := i + chunkSize
		if end > len(encodedRequest) {
			end = len(encodedRequest)
		}
		fmt.Println(encodedRequest[i:end])
	}

	fmt.Println("\n💡 Next Steps:")
	fmt.Println("1. Submit the base64 encoded registration request to P0")
	fmt.Println("2. Enable and start the systemd service:")
	fmt.Printf("   \033[1msudo systemctl enable p0-ssh-agent\033[0m\n")
	fmt.Printf("   \033[1msudo systemctl start p0-ssh-agent\033[0m\n")

	fmt.Println("\n🔧 Service Management Commands:")
	fmt.Println("   Status:  sudo systemctl status p0-ssh-agent")
	fmt.Println("   Stop:    sudo systemctl stop p0-ssh-agent")
	fmt.Println("   Start:   sudo systemctl start p0-ssh-agent")
	fmt.Println("   Restart: sudo systemctl restart p0-ssh-agent")
	fmt.Println("   Logs:    sudo journalctl -u p0-ssh-agent -f")
}

package install

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/sirupsen/logrus"

	"p0-ssh-agent/internal/osplugins"
	"p0-ssh-agent/types"
)

func createDirectories(cfg *types.Config, osPlugin osplugins.OSPlugin, logger *logrus.Logger) error {
	directories := []string{
		cfg.KeyPath,
	}

	// Use the OS plugin to setup directories (with root ownership)
	return osPlugin.SetupDirectories(directories, "root", logger)
}

func generateJWTKeys(keyPath, executablePath string, logger *logrus.Logger) error {
	logger.WithField("key_path", keyPath).Info("Generating JWT keys")

	privateKeyPath := filepath.Join(keyPath, "jwk.private.json")
	publicKeyPath := filepath.Join(keyPath, "jwk.public.json")

	if _, err := os.Stat(privateKeyPath); err == nil {
		if _, err := os.Stat(publicKeyPath); err == nil {
			logger.Info("✅ JWT keys already exist")
			return nil
		}
	}

	cmd := exec.Command("sudo", executablePath, "keygen", "--key-path", keyPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to generate JWT keys: %w\nOutput: %s", err, string(output))
	}

	logger.Info("✅ JWT keys generated successfully")
	return nil
}



// Old NixOS-specific functions removed - now handled by NixOS plugin

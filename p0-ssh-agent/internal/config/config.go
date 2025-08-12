package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
	"p0-ssh-agent/pkg/types"
)

// Load loads configuration from various sources
func Load() (*types.Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("$HOME/.braekhus")
	viper.AddConfigPath("/etc/braekhus")
	
	// Environment variable support
	viper.SetEnvPrefix("BRAEKHUS")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()
	
	// Set defaults
	setDefaults()
	
	// Read config file (optional)
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}
	
	config := &types.Config{}
	if err := viper.Unmarshal(config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}
	
	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}
	
	return config, nil
}

// setDefaults sets default configuration values
func setDefaults() {
	viper.SetDefault("tunnelHost", "localhost")
	viper.SetDefault("tunnelPort", 8080)
	viper.SetDefault("insecure", false)
	viper.SetDefault("jwkPath", ".")
}

// validateConfig validates the configuration
func validateConfig(config *types.Config) error {
	if config.TargetURL == "" {
		return fmt.Errorf("targetUrl is required")
	}
	
	if config.ClientID == "" {
		return fmt.Errorf("clientId is required")
	}
	
	if config.TunnelHost == "" {
		return fmt.Errorf("tunnelHost is required")
	}
	
	if config.TunnelPort <= 0 || config.TunnelPort > 65535 {
		return fmt.Errorf("tunnelPort must be between 1 and 65535")
	}
	
	if config.JWKPath == "" {
		return fmt.Errorf("jwkPath is required")
	}
	
	return nil
}
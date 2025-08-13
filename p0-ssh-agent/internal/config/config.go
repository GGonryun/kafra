package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
	"p0-ssh-agent/types"
)

// LoadWithOverrides loads configuration from various sources with command-line flag overrides
func LoadWithOverrides(configPath string, flagOverrides map[string]interface{}) (*types.Config, error) {
	// Initialize viper
	v := viper.New()
	
	// Set config file if provided
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.SetConfigName("p0-ssh-agent")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
		v.AddConfigPath("$HOME/.p0")
		v.AddConfigPath("/etc/p0")
	}
	
	// Environment variable support
	v.SetEnvPrefix("P0_SSH_AGENT")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	
	// Set defaults
	setDefaults(v)
	
	// Read config file (optional)
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}
	
	// Apply flag overrides (only set non-empty/non-zero values)
	for key, value := range flagOverrides {
		switch val := value.(type) {
		case string:
			if val != "" {
				v.Set(key, value)
			}
		case int:
			if val != 0 {
				v.Set(key, value)
			}
		case bool:
			if val {
				v.Set(key, value)
			}
		case []string:
			if len(val) > 0 {
				v.Set(key, value)
			}
		default:
			if value != nil {
				v.Set(key, value)
			}
		}
	}
	
	config := &types.Config{}
	if err := v.Unmarshal(config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}
	
	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}
	
	return config, nil
}

// Load loads configuration from various sources (backward compatibility)
func Load() (*types.Config, error) {
	return LoadWithOverrides("", nil)
}

// setDefaults sets default configuration values
func setDefaults(v *viper.Viper) {
	v.SetDefault("version", "1.0")
	v.SetDefault("tunnelHost", "localhost")
	v.SetDefault("tunnelPort", 8080)
	v.SetDefault("tunnelPath", "/")
	v.SetDefault("insecure", false)
	v.SetDefault("keyPath", ".")
	v.SetDefault("logPath", "")
	v.SetDefault("environment", "default")
	v.SetDefault("tunnelTimeoutMs", 30000) // 30 seconds default
	v.SetDefault("labels", []string{})
	
	// Backward compatibility defaults
	v.SetDefault("jwkPath", ".")
	v.SetDefault("keygenPath", ".")
}

// validateConfig validates the configuration
func validateConfig(config *types.Config) error {
	if config.TunnelHost == "" {
		return fmt.Errorf("tunnelHost is required")
	}
	
	if config.TunnelPort <= 0 || config.TunnelPort > 65535 {
		return fmt.Errorf("tunnelPort must be between 1 and 65535")
	}
	
	if config.GetKeyPath() == "" {
		return fmt.Errorf("keyPath (or jwkPath for backward compatibility) is required")
	}
	
	if config.TunnelTimeoutMs < 0 {
		return fmt.Errorf("tunnelTimeoutMs must be non-negative")
	}
	
	// Validate that we have required org and host IDs
	if config.GetOrgID() == "" {
		return fmt.Errorf("orgId (or tenantId for backward compatibility) is required")
	}
	
	if config.HostID == "" {
		return fmt.Errorf("hostId is required")
	}
	
	return nil
}
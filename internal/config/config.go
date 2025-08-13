package config

import (
	"fmt"
	"net/url"
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
	v.SetDefault("tunnelHost", "ws://localhost:8080/ws")
	v.SetDefault("keyPath", ".")
	v.SetDefault("logPath", "")
	v.SetDefault("environment", "default")
	v.SetDefault("tunnelTimeoutMs", 30000) // 30 seconds default
	v.SetDefault("labels", []string{})
}

// validateConfig validates the configuration
func validateConfig(config *types.Config) error {
	// Validate tunnel host URL
	if config.TunnelHost == "" {
		return fmt.Errorf("tunnelHost is required")
	}
	
	// Parse and validate the tunnel URL
	u, err := url.Parse(config.TunnelHost)
	if err != nil {
		return fmt.Errorf("invalid tunnelHost URL: %w", err)
	}
	
	// Validate WebSocket scheme
	if u.Scheme != "ws" && u.Scheme != "wss" {
		return fmt.Errorf("tunnelHost URL must use ws:// or wss:// scheme, got %q", u.Scheme)
	}
	
	// Validate host
	if u.Host == "" {
		return fmt.Errorf("tunnelHost URL must include a host")
	}
	
	if config.KeyPath == "" {
		return fmt.Errorf("keyPath is required")
	}
	
	if config.TunnelTimeoutMs < 0 {
		return fmt.Errorf("tunnelTimeoutMs must be non-negative")
	}
	
	// Validate that we have required org and host IDs
	if config.OrgID == "" {
		return fmt.Errorf("orgId is required")
	}
	
	if config.HostID == "" {
		return fmt.Errorf("hostId is required")
	}
	
	return nil
}
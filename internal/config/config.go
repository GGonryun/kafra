package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/viper"
	"p0-ssh-agent/types"
)

func LoadWithOverrides(configPath string, flagOverrides map[string]interface{}) (*types.Config, error) {
	v := viper.New()
	
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		bootstrapConfigPath := "/etc/p0-ssh-agent/config.yaml"
		if _, err := os.Stat(bootstrapConfigPath); err == nil {
			v.SetConfigFile(bootstrapConfigPath)
		} else {
			v.SetConfigName("p0-ssh-agent")
			v.SetConfigType("yaml")
			v.AddConfigPath(".")
			v.AddConfigPath("$HOME/.p0")
			v.AddConfigPath("/etc/p0")
		}
	}
	
	v.SetEnvPrefix("P0_SSH_AGENT")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	
	setDefaults(v)
	
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}
	
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

func Load() (*types.Config, error) {
	return LoadWithOverrides("", nil)
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("version", "1.0")
	v.SetDefault("tunnelHost", "wss://api.p0.app")
	v.SetDefault("keyPath", "/etc/p0-ssh-agent/keys")
	v.SetDefault("environmentId", "default")
	v.SetDefault("heartbeatIntervalSeconds", 60)
	v.SetDefault("labels", []string{})
}

func validateConfig(config *types.Config) error {
	if config.TunnelHost == "" {
		return fmt.Errorf("tunnelHost is required")
	}
	
	u, err := url.Parse(config.TunnelHost)
	if err != nil {
		return fmt.Errorf("invalid tunnelHost URL: %w", err)
	}
	
	if u.Scheme != "ws" && u.Scheme != "wss" {
		return fmt.Errorf("tunnelHost URL must use ws:// or wss:// scheme, got %q", u.Scheme)
	}
	
	if u.Host == "" {
		return fmt.Errorf("tunnelHost URL must include a host")
	}
	
	if config.KeyPath == "" {
		return fmt.Errorf("keyPath is required")
	}
	
	
	if config.HeartbeatIntervalSeconds <= 0 {
		return fmt.Errorf("heartbeatIntervalSeconds must be greater than 0")
	}
	
	if config.OrgID == "" {
		return fmt.Errorf("orgId is required")
	}
	
	if config.HostID == "" {
		return fmt.Errorf("hostId is required")
	}
	
	return nil
}
package osplugins

import (
	"fmt"
	"sync"

	"github.com/sirupsen/logrus"
)

var (
	registry = make(map[string]OSPlugin)
	mutex    sync.RWMutex
)

// Register adds an OS plugin to the registry
func Register(plugin OSPlugin) {
	mutex.Lock()
	defer mutex.Unlock()
	registry[plugin.GetName()] = plugin
}

// GetPlugin returns the appropriate OS plugin for the current system
func GetPlugin(logger *logrus.Logger) (OSPlugin, error) {
	mutex.RLock()
	defer mutex.RUnlock()
	
	// Try to find a plugin that detects as suitable for this system
	for name, plugin := range registry {
		if plugin.Detect() {
			logger.WithField("plugin", name).Info("Selected OS plugin")
			return plugin, nil
		}
	}
	
	// Fall back to generic Linux plugin if available
	if genericPlugin, exists := registry["linux"]; exists {
		logger.Info("Using generic Linux plugin as fallback")
		return genericPlugin, nil
	}
	
	return nil, fmt.Errorf("no suitable OS plugin found")
}

// ListPlugins returns all registered plugins
func ListPlugins() []string {
	mutex.RLock()
	defer mutex.RUnlock()
	
	var plugins []string
	for name := range registry {
		plugins = append(plugins, name)
	}
	return plugins
}

// GetPluginByName returns a specific plugin by name
func GetPluginByName(name string) (OSPlugin, error) {
	mutex.RLock()
	defer mutex.RUnlock()
	
	if plugin, exists := registry[name]; exists {
		return plugin, nil
	}
	
	return nil, fmt.Errorf("plugin '%s' not found", name)
}
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
	
	// Log all available plugins first
	pluginNames := make([]string, 0, len(registry))
	for name := range registry {
		pluginNames = append(pluginNames, name)
	}
	logger.WithField("available_plugins", pluginNames).Info("Available OS plugins in registry")
	
	// With build tags, only the appropriate plugin will be compiled and registered
	// So we just return the first (and only) plugin that's available
	for name, plugin := range registry {
		logger.WithField("plugin", name).Info("Selected OS plugin")
		return plugin, nil
	}
	
	return nil, fmt.Errorf("no OS plugins found in registry - this should not happen with proper build tags")
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
package osplugins

import (
	"fmt"
	"sync"

	"github.com/sirupsen/logrus"
)

var (
	registry = make(map[string]OSPlugin)
	mutex    sync.RWMutex
	loaded   = false
)

// Register adds an OS plugin to the registry
func Register(plugin OSPlugin) {
	mutex.Lock()
	defer mutex.Unlock()
	registry[plugin.GetName()] = plugin
}

// LoadPlugins dynamically loads plugins based on OS detection
func LoadPlugins(logger *logrus.Logger) error {
	mutex.Lock()
	defer mutex.Unlock()

	if loaded {
		return nil // Already loaded
	}

	// Create plugins for detection
	nixosPlugin := NewNixOSPlugin()
	linuxPlugin := NewLinuxPlugin()

	// Register NixOS plugin if detected
	if nixosPlugin.Detect() {
		logger.Info("Detected NixOS system, registering NixOS plugin")
		registry[nixosPlugin.GetName()] = nixosPlugin
	} else {
		// Fallback to Linux plugin
		logger.Info("Using Linux plugin as fallback")
		registry[linuxPlugin.GetName()] = linuxPlugin
	}

	loaded = true
	return nil
}

// GetPlugin returns the appropriate OS plugin for the current system
func GetPlugin(logger *logrus.Logger) (OSPlugin, error) {
	// Ensure plugins are loaded
	if err := LoadPlugins(logger); err != nil {
		return nil, fmt.Errorf("failed to load plugins: %w", err)
	}

	mutex.RLock()
	defer mutex.RUnlock()

	// Log all available plugins first
	pluginNames := make([]string, 0, len(registry))
	for name := range registry {
		pluginNames = append(pluginNames, name)
	}
	logger.WithField("available_plugins", pluginNames).Info("Available OS plugins in registry")

	// Return the registered plugin (should be only one)
	for name, plugin := range registry {
		logger.WithField("plugin", name).Info("Selected OS plugin")
		return plugin, nil
	}

	return nil, fmt.Errorf("no OS plugins found in registry")
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

package osplugins

import (
	"github.com/sirupsen/logrus"
)

// OSPlugin defines the interface for operating system specific implementations
type OSPlugin interface {
	// GetName returns the name of the OS plugin (e.g., "nixos", "linux")
	GetName() string

	// Detect checks if this plugin should be used for the current system
	Detect() bool

	// GetInstallDirectories returns prioritized list of binary installation directories
	GetInstallDirectories() []string

	// CreateSystemdService handles systemd service creation for this OS
	CreateSystemdService(serviceName, executablePath, configPath string, logger *logrus.Logger) error

	// GetConfigDirectory returns the default configuration directory
	GetConfigDirectory() string

	// SetupDirectories creates and configures necessary directories
	SetupDirectories(dirs []string, owner string, logger *logrus.Logger) error

	// GetSystemInfo returns OS-specific system information
	GetSystemInfo() map[string]string

	// CreateUser creates a user dynamically for JIT access (used by P0 scripts)
	CreateUser(username string, logger *logrus.Logger) error

	// RemoveUser removes a dynamically created user (cleanup)
	RemoveUser(username string, logger *logrus.Logger) error

	// UninstallService handles OS-specific service uninstallation
	UninstallService(serviceName string, logger *logrus.Logger) error

	// CleanupInstallation performs OS-specific cleanup during uninstall
	CleanupInstallation(serviceName string, logger *logrus.Logger) error
}

// InstallConfig contains parameters needed for installation
type InstallConfig struct {
	ServiceName    string
	ExecutablePath string
	ConfigPath     string
	KeyPath        string
	LogPath        string
	AllowRoot      bool
}

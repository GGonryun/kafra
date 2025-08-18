package osplugins

import (
	"github.com/sirupsen/logrus"
)

// OSPlugin defines the interface for operating system specific implementations
type OSPlugin interface {
	// GetName returns the name of the OS plugin (e.g., "nixos", "ubuntu", "centos")
	GetName() string
	
	// Detect returns true if this plugin should be used for the current system
	Detect() bool
	
	// GetInstallDirectories returns prioritized list of binary installation directories
	GetInstallDirectories() []string
	
	// CreateSystemdService handles systemd service creation for this OS
	CreateSystemdService(serviceName, serviceUser, executablePath, configPath string, logger *logrus.Logger) error
	
	// GetConfigDirectory returns the default configuration directory
	GetConfigDirectory() string
	
	// CreateUser creates a system user if needed
	CreateUser(username, homeDir string, logger *logrus.Logger) error
	
	// SetupDirectories creates and configures necessary directories
	SetupDirectories(dirs []string, owner string, logger *logrus.Logger) error
	
	// GetSystemInfo returns OS-specific system information
	GetSystemInfo() map[string]string
}

// InstallConfig contains parameters needed for installation
type InstallConfig struct {
	ServiceName    string
	ServiceUser    string
	ExecutablePath string
	ConfigPath     string
	KeyPath        string
	LogPath        string
	AllowRoot      bool
}
package osplugins

// LoadAllPlugins registers all available OS plugins
func LoadAllPlugins() {
	// Plugins register themselves via init() functions when imported
	// This function exists to provide an explicit loading point if needed
	
	// Force registration of all plugins by accessing their types
	_ = &LinuxPlugin{}
	_ = &NixOSPlugin{}
}
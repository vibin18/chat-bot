package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// defaultConfigPath is the default path for the config file
var defaultConfigPath = "config/config.json"

// GetConfigPath returns the path to the config file
func GetConfigPath() string {
	// Check environment variable
	path := os.Getenv("CONFIG_PATH")
	if path != "" {
		return path
	}
	
	// Use default path
	return defaultConfigPath
}

// SaveConfig saves the configuration to a file
func SaveConfig(config *Config, path string) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	
	// Marshal config to JSON
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	
	// Write to file
	return os.WriteFile(path, data, 0644)
}

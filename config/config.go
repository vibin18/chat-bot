package config

import (
	"encoding/json"
	"os"
	"time"
)

// Config represents the application configuration
type Config struct {
	Server ServerConfig `json:"server"`
	LLM    LLMConfig    `json:"llm"`
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Port int `json:"port"`
}

// LLMConfig holds configuration for the LLM backend
type LLMConfig struct {
	Provider        string        `json:"provider"`
	Ollama          OllamaConfig  `json:"ollama"`
	DefaultTimeout  time.Duration `json:"default_timeout"`
	DefaultMaxToken int           `json:"default_max_tokens"`
}

// OllamaConfig holds specific configuration for Ollama integration
type OllamaConfig struct {
	Enabled        bool          `json:"enabled"`
	Endpoint       string        `json:"endpoint"`
	Model          string        `json:"model"`
	MaxTokens      int           `json:"max_tokens"`
	TimeoutSeconds time.Duration `json:"timeout_seconds"`
}

// LoadConfig loads configuration from a JSON file
func LoadConfig(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	config := &Config{
		Server: ServerConfig{
			Port: 8080,
		},
		LLM: LLMConfig{
			Provider: "ollama",
			Ollama: OllamaConfig{
				Enabled:        true,
				Endpoint:       "http://localhost:11434",
				Model:          "gemma3:1b",
				MaxTokens:      256,
				TimeoutSeconds: 100,
			},
			DefaultTimeout:  100 * time.Second,
			DefaultMaxToken: 256,
		},
	}

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(config); err != nil {
		return nil, err
	}

	return config, nil
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port: 8080,
		},
		LLM: LLMConfig{
			Provider: "ollama",
			Ollama: OllamaConfig{
				Enabled:        true,
				Endpoint:       "http://192.168.1.222:11434",
				Model:          "gemma3:1b",
				MaxTokens:      256,
				TimeoutSeconds: 100,
			},
			DefaultTimeout:  100 * time.Second,
			DefaultMaxToken: 256,
		},
	}
}

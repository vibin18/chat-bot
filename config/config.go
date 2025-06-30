package config

import (
	"encoding/json"
	"os"
	"time"
)

// Config represents the application configuration
type Config struct {
	Server       ServerConfig       `json:"server"`
	LLM          LLMConfig          `json:"llm"`
	WebSearch    WebSearchConfig    `json:"websearch"`
	SecondaryLLM SecondaryLLMConfig `json:"secondary_llm"`
	ImageLLM     ImageLLMConfig     `json:"image_llm"`
	WhatsApp     WhatsAppConfig     `json:"whatsapp"`
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Port int `json:"port"`
}

// LLMConfig holds configuration for the LLM backend
type LLMConfig struct {
	Provider        string        `json:"provider"`
	EnableReasoning bool          `json:"enable_reasoning"`
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

// WebSearchConfig holds configuration for web search functionality
type WebSearchConfig struct {
	Enabled        bool     `json:"enabled"`
	Provider       string   `json:"provider"`      // "serpapi" or "brave"
	SerpAPIKey     string   `json:"serpapi_key"`
	BraveAPIKey    string   `json:"brave_api_key"`
	IntentKeywords []string `json:"intent_keywords"`
}

// SecondaryLLMConfig holds configuration for the secondary LLM
type SecondaryLLMConfig struct {
	Provider string      `json:"provider"`
	Ollama   OllamaConfig `json:"ollama"`
}

// ImageLLMConfig holds configuration for the image analysis LLM
type ImageLLMConfig struct {
	Enabled  bool        `json:"enabled"`
	Provider string      `json:"provider"`
	Ollama   OllamaConfig `json:"ollama"`
}

// FamilyServiceConfig holds configuration for the family service webhook
type FamilyServiceConfig struct {
	Enabled        bool          `json:"enabled"`
	WebhookURL     string        `json:"webhook_url"`
	TimeoutSeconds time.Duration `json:"timeout_seconds"`
}

// FoodServiceConfig holds configuration for the food service webhook
type FoodServiceConfig struct {
	Enabled        bool          `json:"enabled"`
	WebhookURL     string        `json:"webhook_url"`
	TimeoutSeconds time.Duration `json:"timeout_seconds"`
}

// WebServiceConfig holds configuration for the web search service webhook
type WebServiceConfig struct {
	Enabled        bool          `json:"enabled"`
	WebhookURL     string        `json:"webhook_url"`
	TimeoutSeconds time.Duration `json:"timeout_seconds"`
}

// ComfyUIServiceConfig holds configuration for the ComfyUI service
type ComfyUIServiceConfig struct {
	Enabled        bool          `json:"enabled"`
	Endpoint       string        `json:"endpoint"`
	WorkflowPath   string        `json:"workflow_path"`
	TimeoutSeconds time.Duration `json:"timeout_seconds"`
}

// WhatsAppConfig holds configuration for the WhatsApp integration
type WhatsAppConfig struct {
	Enabled      bool     `json:"enabled"`
	BotName      string   `json:"bot_name"`
	TriggerWords []string `json:"trigger_words"`
	TriggerWord  string   `json:"trigger_word"` // Deprecated: kept for backward compatibility
	StoreDir     string   `json:"store_dir"`
	AllowedGroups []string `json:"allowed_groups"`
	FamilyService FamilyServiceConfig `json:"family_service"`
	FoodService   FoodServiceConfig   `json:"food_service"`
	WebService    WebServiceConfig    `json:"web_service"`
	ComfyUIService ComfyUIServiceConfig `json:"comfyui_service"`
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
			Provider:        "ollama",
			EnableReasoning: false,
			Ollama: OllamaConfig{
				Enabled:        true,
				Endpoint:       "http://localhost:11434",
				Model:          "qwen3:14b",
				MaxTokens:      4096,
				TimeoutSeconds: 100,
			},
			DefaultTimeout:  100 * time.Second,
			DefaultMaxToken: 4096,
		},
		WebSearch: WebSearchConfig{
			Enabled: true,
			Provider: "serpapi", // Default to serpapi, can be changed to "brave"
			SerpAPIKey: "", 
			BraveAPIKey: "",
			IntentKeywords: []string{"now", "today", "latest", "current", "news", "weather", "score", "price", "recent", "update"},
		},
		SecondaryLLM: SecondaryLLMConfig{
			Provider: "ollama",
			Ollama: OllamaConfig{
				Enabled:        true,
				Endpoint:       "http://localhost:11434",
				Model:          "gemma3:1b",
				MaxTokens:      256,
				TimeoutSeconds: 30,
			},
		},
		ImageLLM: ImageLLMConfig{
			Enabled:  true,
			Provider: "ollama",
			Ollama: OllamaConfig{
				Enabled:        true,
				Endpoint:       "http://localhost:11434",
				Model:          "llava:7b",  // Default vision model
				MaxTokens:      1024,
				TimeoutSeconds: 60,
			},
		},
		WhatsApp: WhatsAppConfig{
			Enabled:      false,
			BotName:      "Sasi",
			TriggerWords: []string{"@sasi", "sasi", "Sasi"},
			TriggerWord:  "@sasi", // Deprecated: kept for backward compatibility
			StoreDir:     "./data/whatsapp",
			AllowedGroups: []string{},
			FamilyService: FamilyServiceConfig{
				Enabled:        true,
				WebhookURL:     "http://192.168.1.132:5678/webhook/f65ba2b8-582c-4575-b4b9-02b26edc3ea0/chat",
				TimeoutSeconds: 30,
			},
			FoodService: FoodServiceConfig{
				Enabled:        true,
				WebhookURL:     "http://192.168.1.132:5678/webhook/appify",
				TimeoutSeconds: 30,
			},
			WebService: WebServiceConfig{
				Enabled:        true,
				WebhookURL:     "http://192.168.1.132:7000/search",
				TimeoutSeconds: 30,
			},
			ComfyUIService: ComfyUIServiceConfig{
				Enabled:        true,
				Endpoint:       "http://192.168.1.245:9901",
				WorkflowPath:   "./comfyui/flux_8_steps.json",
				TimeoutSeconds: 60,
			},
		},
	}
}

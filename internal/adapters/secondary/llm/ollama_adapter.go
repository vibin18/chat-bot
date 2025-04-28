package llm

import (
	"context"
	"time"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/ollama"
	"github.com/vibin/chat-bot/config"
	"github.com/vibin/chat-bot/internal/core/domain"
	"github.com/vibin/chat-bot/internal/logger"
)

// OllamaAdapter implements the LLMPort interface for the Ollama LLM provider
type OllamaAdapter struct {
	client *ollama.LLM
	config *config.LLMConfig
	logger logger.Logger
}

// NewOllamaAdapter creates a new OllamaAdapter
func NewOllamaAdapter(config *config.LLMConfig, log logger.Logger) (*OllamaAdapter, error) {
	log.Info("Initializing Ollama adapter", "endpoint", config.Ollama.Endpoint, "model", config.Ollama.Model)
	
	client, err := ollama.New(
		ollama.WithServerURL(config.Ollama.Endpoint),
		ollama.WithModel(config.Ollama.Model),
	)
	
	if err != nil {
		log.Error("Failed to initialize Ollama client", "error", err)
		return nil, err
	}
	
	return &OllamaAdapter{
		client: client,
		config: config,
		logger: log,
	}, nil
}

// GenerateResponse generates a response from the LLM for a given chat history
func (a *OllamaAdapter) GenerateResponse(ctx context.Context, messages []domain.Message) (string, error) {
	a.logger.Info("Generating response with Ollama", "model", a.config.Ollama.Model)
	
	// Convert domain messages to LangChain messages
	prompt := formatMessagesAsPrompt(messages)
	
	// Set generation options
	opts := []llms.CallOption{
		llms.WithMaxTokens(a.config.Ollama.MaxTokens),
		llms.WithTemperature(0.7),
	}
	
	// Create a context with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(a.config.Ollama.TimeoutSeconds)*time.Second)
	defer cancel()
	
	// Generate completion
	result, err := a.client.Call(timeoutCtx, prompt, opts...)
	if err != nil {
		a.logger.Error("Ollama generation failed", "error", err)
		return "", err
	}
	
	return result, nil
}

// GetModelInfo returns information about the current LLM model
func (a *OllamaAdapter) GetModelInfo(ctx context.Context) (map[string]interface{}, error) {
	a.logger.Info("Getting model info for Ollama", "model", a.config.Ollama.Model)
	
	// In a real implementation, we would call the Ollama API to get model info
	// For now, return static info
	return map[string]interface{}{
		"name":      a.config.Ollama.Model,
		"provider":  "ollama",
		"endpoint":  a.config.Ollama.Endpoint,
		"maxTokens": a.config.Ollama.MaxTokens,
	}, nil
}

// formatMessagesAsPrompt converts a slice of domain messages to a prompt string for Ollama
func formatMessagesAsPrompt(messages []domain.Message) string {
	var prompt string
	
	for _, msg := range messages {
		if msg.Role == "user" {
			prompt += "User: " + msg.Content + "\n"
		} else if msg.Role == "assistant" {
			prompt += "Assistant: " + msg.Content + "\n"
		}
	}
	
	// Add the final prompt for the assistant to respond
	prompt += "Assistant: "
	
	return prompt
}

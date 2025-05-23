package llm

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
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
	
	// Create Ollama client with the proper configuration
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

// cleanThinkingTags removes empty thinking tags from the response
func cleanThinkingTags(input string) string {
	// Regular expression to match empty thinking tags: <think></think> or <think> </think>
	re := regexp.MustCompile(`<think>\s*</think>`)
	cleaned := re.ReplaceAllString(input, "")
	
	// Also trim any leading/trailing whitespace
	return strings.TrimSpace(cleaned)
}

// GenerateResponse generates a response from the LLM for a given chat history
func (a *OllamaAdapter) GenerateResponse(ctx context.Context, messages []domain.Message) (string, error) {
	model := a.config.Ollama.Model
	a.logger.Info("Generating response with Ollama", "model", model)
	
	// Special handling for image analysis requests
	for _, msg := range messages {
		if msg.Type == domain.MessageTypeImageAnalysis && len(msg.Images) > 0 {
			return a.generateImageAnalysis(ctx, msg)
		}
	}
	
	// For regular text messages, use the LangChain client
	// Convert domain messages to LangChain messages
	prompt := formatMessagesAsPrompt(messages, model, a.config.EnableReasoning)
	
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
	
	// Process result based on model and reasoning settings
	if strings.HasPrefix(model, "qwen3") && !a.config.EnableReasoning {
		a.logger.Info("Processing qwen3 response, removing empty thinking tags")
		result = cleanThinkingTags(result)
	}
	
	return result, nil
}

// generateImageAnalysis handles image analysis requests by making direct API calls to Ollama
func (a *OllamaAdapter) generateImageAnalysis(ctx context.Context, message domain.Message) (string, error) {
	a.logger.Info("Processing image analysis using direct API call", "image_size", len(message.Images[0]))
	
	// Create a clear prompt for image analysis with formatting instructions for WhatsApp
	prompt := `Find what is in this image and format your response with emoji bullet points, clear headings, and short paragraphs for better readability in WhatsApp
	`
	
	// If the user provided a custom prompt, append it to our formatting instructions
	if message.Content != "" && message.Content != "Analyze the following image and provide a detailed description." {
		prompt = message.Content + "\n\n" + prompt
	}
	
	// Create the request payload following Ollama's API format
	type ollamaMessage struct {
		Role    string   `json:"role"`
		Content string   `json:"content"`
		Images  []string `json:"images,omitempty"`
	}
	
	type ollamaRequest struct {
		Model    string                 `json:"model"`
		Messages []ollamaMessage        `json:"messages"`
		Options  map[string]interface{} `json:"options,omitempty"`
		Stream   bool                   `json:"stream"`
	}
	
	// Log verification of the image data being sent to the LLM
	imageData := message.Images[0]
	base64Sample := ""
	if len(imageData) > 100 {
		base64Sample = imageData[:100] + "..."
	} else if len(imageData) > 0 {
		base64Sample = imageData
	}

	// Calculate a checksum (using first 8 bytes) to match against what was extracted from WhatsApp
	var checksum string
	if len(imageData) >= 8 {
		// For base64 data, we need to decode the first chunk first
		decodeData, err := base64.StdEncoding.DecodeString(imageData[:12])
		if err == nil && len(decodeData) >= 8 {
			checksum = fmt.Sprintf("%x", decodeData[:8])
		}
	}
	
	a.logger.Info("Sending image to LLM", 
		"base64_length", len(imageData),
		"base64_prefix", base64Sample,
		"checksum_first_8_bytes", checksum,
		"model", a.config.Ollama.Model)

	// Create a timestamp to prevent caching
	timestamp := time.Now().UnixNano()
	
	// Create the request with system message and user message in chat format
	request := ollamaRequest{
		Model: a.config.Ollama.Model,
		Messages: []ollamaMessage{
			{
				Role:    "user",
				// Add a unique request ID to prevent caching
				Content: prompt + " (Request ID: " + fmt.Sprintf("%d", timestamp) + ")",
				Images:  []string{imageData},
			},
		},
		Options: map[string]interface{}{
			"temperature": 0.7,
			"seed": timestamp, // Use a unique seed for each request
		},
		Stream: false,
	}

	a.logger.Info("Sending unique request", "timestamp", timestamp, "model", a.config.Ollama.Model)
	
	// Marshal the request to JSON
	requestJSON, err := json.Marshal(request)
	if err != nil {
		a.logger.Error("Failed to marshal request", "error", err)
		return "", err
	}
	
	// Log the full JSON payload for debugging
	a.logger.Info("Full JSON request payload", "request_json", string(requestJSON))
	
	// Create a timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(a.config.Ollama.TimeoutSeconds)*time.Second)
	defer cancel()
	
	// Ensure we're using the correct API endpoint for Ollama models
	// The proper endpoint for Ollama multimodal models is /api/chat
	url := fmt.Sprintf("%s/api/chat", a.config.Ollama.Endpoint)
	a.logger.Info("Using Ollama API endpoint", "url", url)
	
	httpReq, err := http.NewRequestWithContext(timeoutCtx, "POST", url, bytes.NewBuffer(requestJSON))
	if err != nil {
		a.logger.Error("Failed to create HTTP request", "error", err)
		return "", fmt.Errorf("failed to create HTTP request: %w", err)
	}
	
	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	
	// Send the request
	a.logger.Info("Sending image analysis request to Ollama", "url", url)
	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		a.logger.Error("Failed to send request", "error", err)
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()
	
	// Read the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		a.logger.Error("Failed to read response", "error", err)
		return "", fmt.Errorf("failed to read response: %w", err)
	}
	
	// Check if the response is successful
	if resp.StatusCode != http.StatusOK {
		a.logger.Error("Received error response", "status", resp.Status, "body", string(body))
		return "", fmt.Errorf("received error response: %s", resp.Status)
	}
	
	a.logger.Info("Received image analysis response", "status", resp.Status, "length", len(body))
	
	// Log the complete raw response to help debug issues
	a.logger.Info("Raw LLM response", "raw_response", string(body))
	
	// Also log important parts of it to a separate logger level for better visualization
	if len(body) > 500 {
		a.logger.Debug("Raw LLM response (sample)", "raw_response_sample", string(body[:500]) + "...")
	}
	
	// Parse the response - based on the actual Ollama API response format
	type ollamaResponseMessage struct {
		Role    string   `json:"role"`
		Content string   `json:"content"`
		Images  []string `json:"images"`
	}

	type ollamaApiResponse struct {
		Model            string               `json:"model"`
		CreatedAt        string               `json:"created_at"`
		Message          ollamaResponseMessage `json:"message"`
		Done             bool                 `json:"done"`
		TotalDuration    int64                `json:"total_duration"`
		LoadDuration     int64                `json:"load_duration"`
		PromptEvalCount  int                  `json:"prompt_eval_count"`
		EvalCount        int                  `json:"eval_count"`
	}
	
	// Parse according to the actual Ollama API response format
	var apiResp ollamaApiResponse
	if err := json.Unmarshal(body, &apiResp); err == nil && apiResp.Message.Content != "" {
		// Successfully parsed response
		a.logger.Info("Parsed LLM API response", 
			"model", apiResp.Model,
			"done", apiResp.Done,
			"response_length", len(apiResp.Message.Content),
			"eval_count", apiResp.EvalCount,
			"total_duration_ms", apiResp.TotalDuration/1000000)
		
		// Log the full content for debugging
		a.logger.Info("Extracted response from Ollama API", "full_content", apiResp.Message.Content)
		
		// Also log the first portion for quicker viewing in condensed logs
		if len(apiResp.Message.Content) > 100 {
			a.logger.Debug("Response preview", "sample", apiResp.Message.Content[:100] + "...")
		}
		return apiResp.Message.Content, nil
	}
	
	// If generate API format fails, try the chat API format as fallback
	type ollamaResponse struct {
		Model     string         `json:"model"`
		CreatedAt string         `json:"created_at"`
		Message   *ollamaMessage `json:"message,omitempty"`
		Response  string         `json:"response,omitempty"`
	}
	
	var responseObj ollamaResponse
	if err := json.Unmarshal(body, &responseObj); err != nil {
		a.logger.Warn("Failed to parse response JSON for both formats", "error", err, "response_text", string(body))
		// Return the raw response if parsing fails
		return string(body), nil
	}
	
	// Log the parsed response structure
	a.logger.Info("Parsed LLM chat response structure", 
		"model", responseObj.Model,
		"has_message", responseObj.Message != nil,
		"has_response_field", responseObj.Response != "",
		"response_length", len(responseObj.Response))
	
	// Extract the content
	var result string
	if responseObj.Response != "" {
		// Newer Ollama API format with Response field
		result = responseObj.Response
		a.logger.Info("Extracted response from 'response' field", "full_content", responseObj.Response)
		
		// Log a preview for easier reading in condensed logs
		if len(responseObj.Response) > 100 {
			a.logger.Debug("Response preview", "sample", responseObj.Response[:100] + "...")
		}
	} else if responseObj.Message != nil && responseObj.Message.Content != "" {
		// Older Ollama API format with Message.Content field
		result = responseObj.Message.Content
		a.logger.Info("Extracted response from 'message.content' field", "full_content", responseObj.Message.Content)
		
		// Log a preview for easier reading in condensed logs
		if len(responseObj.Message.Content) > 100 {
			a.logger.Debug("Response preview", "sample", responseObj.Message.Content[:100] + "...")
		}
	} else {
		// Fallback to the raw response
		a.logger.Warn("Could not extract structured content from response")
		result = string(body)
	}
	
	// Remove any references to base64 encoding
	result = removeBase64References(result)
	
	return result, nil
}

// GetModelInfo returns information about the current LLM model
func (a *OllamaAdapter) GetModelInfo(ctx context.Context) (map[string]interface{}, error) {
	a.logger.Info("Getting model info for Ollama", "model", a.config.Ollama.Model)
	
	// In a real implementation, we would call the Ollama API to get model info
	// For now, return static info
	return map[string]interface{}{
		"name":            a.config.Ollama.Model,
		"provider":        "ollama",
		"endpoint":        a.config.Ollama.Endpoint,
		"maxTokens":       a.config.Ollama.MaxTokens,
		"enableReasoning": a.config.EnableReasoning,
	}, nil
}

// removeBase64References removes references to base64 encoding from the response
func removeBase64References(text string) string {
	// Define patterns to remove
	patterns := []string{
		"This is a base64 encoded image",
		"base64 encoded image",
		"base64 encoding",
		"based on the base64 image",
		"I can see a base64 encoded image",
		"Decoding it reveals",
		"decoding the image",
		"encoded in base64",
		"The image appears to be",
	}
	
	// Remove each pattern
	result := text
	for _, pattern := range patterns {
		result = strings.ReplaceAll(result, pattern, "")
	}
	
	// Clean up any double spaces created from removals
	spaceRegex := regexp.MustCompile(`\s+`)
	result = spaceRegex.ReplaceAllString(result, " ")
	
	// Clean up the start by removing any leading space and periods
	result = strings.TrimSpace(result)
	result = strings.TrimPrefix(result, ".")
	result = strings.TrimSpace(result)
	
	return result
}

// formatMessagesAsPrompt converts a slice of domain messages to a prompt string for Ollama
func formatMessagesAsPrompt(messages []domain.Message, model string, enableReasoning bool) string {
	// Special handling for image analysis
	for _, msg := range messages {
		if msg.Type == domain.MessageTypeImageAnalysis && len(msg.Images) > 0 {
			// Enhanced prompt for detailed image analysis
			analysisPrompt := "Describe in detail what you see in this image. Include information about objects, people, scenes, colors, text, and any other important elements."
			
			// If user provided specific content, use it instead of our default prompt
			if msg.Content != "" && msg.Content != "Describe what is in this image in detail." {
				analysisPrompt = msg.Content
			}
			
			// Format exactly as the Ollama API docs for image analysis
			// This is the correct format with a messages array and images inside the content
			jsonPayload := fmt.Sprintf(`{
`+
				`  "model": "%s",
`+
				`  "messages": [
`+
				`    {
`+
				`      "role": "user",
`+
				`      "content": "%s",
`+
				`      "images": ["%s"]
`+
				`    }
`+
				`  ],
`+
				`  "stream": false
`+
				`}`,
				model,
				escape(analysisPrompt),
				escape(msg.Images[0]),
			)
			return jsonPayload
		}
	}

	// For regular text-based message handling, convert to proper chat format
	// Create proper messages array for Ollama
	type chatMessage struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}

	type chatRequest struct {
		Model    string        `json:"model"`
		Messages []chatMessage `json:"messages"`
		Stream   bool          `json:"stream"`
		Options  map[string]interface{} `json:"options,omitempty"`
	}

	// Build chat messages array
	chatMessages := make([]chatMessage, 0, len(messages)+1)
	
	// Add system message
	chatMessages = append(chatMessages, chatMessage{
		Role:    "system",
		Content: "You are a helpful assistant. Keep your responses concise and to the point unless the user specifically asks for detailed explanations or descriptions.",
	})
	
	// Convert domain messages to chat messages
	for i, msg := range messages {
		role := msg.Role
		content := msg.Content
		
		// For qwen3 models, apply reasoning toggle
		if strings.HasPrefix(model, "qwen3") && !enableReasoning && 
		   role == "user" && i == len(messages)-1 {
			content = content + "/no_think"
		}
		
		chatMessages = append(chatMessages, chatMessage{
			Role:    role,
			Content: content,
		})
	}
	
	// Create request
	request := chatRequest{
		Model:    model,
		Messages: chatMessages,
		Stream:   false,
		Options: map[string]interface{}{
			"temperature": 0.7,
			"num_predict": 1024, // Default max tokens
		},
	}
	
	// Marshal to JSON
	jsonData, err := json.Marshal(request)
	if err != nil {
		return fmt.Sprintf("Error formatting messages: %v", err)
	}
	
	return string(jsonData)
}

// escape escapes special characters in strings to make them safe for JSON
func escape(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\r")
	s = strings.ReplaceAll(s, "\t", "\\t")
	return s
}

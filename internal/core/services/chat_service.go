package services

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/vibin/chat-bot/config"
	"github.com/vibin/chat-bot/internal/core/domain"
	"github.com/vibin/chat-bot/internal/core/ports"
	"github.com/vibin/chat-bot/internal/logger"
)

// ChatService is the core service that implements the business logic for chat interactions
type ChatService struct {
	llm        ports.LLMPort
	imageLLM   ports.LLMPort  // Dedicated LLM for image analysis
	repository ports.ChatRepositoryPort
	webSearch  ports.WebSearchPort
	logger     logger.Logger
	config     *config.Config
}

// NewChatService creates a new ChatService
func NewChatService(llm ports.LLMPort, imageLLM ports.LLMPort, repository ports.ChatRepositoryPort, webSearch ports.WebSearchPort, config *config.Config, logger logger.Logger) *ChatService {
	return &ChatService{
		llm:        llm,
		imageLLM:   imageLLM,
		repository: repository,
		webSearch:  webSearch,
		logger:     logger,
		config:     config,
	}
}

// CreateChat creates a new chat
func (s *ChatService) CreateChat(ctx context.Context, title string) (*domain.Chat, error) {
	s.logger.Info("Creating new chat", "title", title)
	chat := domain.NewChat(title)
	err := s.repository.SaveChat(ctx, chat)
	if err != nil {
		s.logger.Error("Failed to save chat", "error", err)
		return nil, err
	}
	return chat, nil
}

// SendMessage sends a user message to a chat and generates a response
func (s *ChatService) SendMessage(ctx context.Context, chatID, content string) (*domain.Chat, error) {
	s.logger.Info("Sending message to chat", "chat_id", chatID)
	
	// Get the chat
	chat, err := s.repository.GetChat(ctx, chatID)
	if err != nil {
		s.logger.Error("Failed to get chat", "chat_id", chatID, "error", err)
		return nil, err
	}
	
	// Add user message
	userMessage := domain.NewMessage("user", content)
	chat.AddMessage(userMessage)
	
	// Process the response based on the user's message
	var response string
	
	// Check if web search is enabled and if the message indicates a need for search
	if s.config.WebSearch.Enabled && s.webSearch != nil && s.webSearch.DetectSearchIntent(content) {
		// Web search path
		s.logger.Info("Using web search pipeline", "chat_id", chatID)
		response, err = s.processWebSearchRequest(ctx, content, chat.Messages)
	} else {
		// Regular LLM path
		s.logger.Info("Generating standard LLM response", "chat_id", chatID)
		response, err = s.llm.GenerateResponse(ctx, chat.Messages)
	}
	
	if err != nil {
		s.logger.Error("Failed to generate response", "chat_id", chatID, "error", err)
		return nil, err
	}
	
	// Add assistant message
	assistantMessage := domain.NewMessage("assistant", response)
	chat.AddMessage(assistantMessage)
	
	// Save the updated chat
	err = s.repository.SaveChat(ctx, chat)
	if err != nil {
		s.logger.Error("Failed to save chat", "chat_id", chatID, "error", err)
		return nil, err
	}
	
	return chat, nil
}

// GetChat retrieves a chat by ID
func (s *ChatService) GetChat(ctx context.Context, id string) (*domain.Chat, error) {
	s.logger.Info("Getting chat", "chat_id", id)
	return s.repository.GetChat(ctx, id)
}

// ListChats returns all chats
func (s *ChatService) ListChats(ctx context.Context) ([]*domain.Chat, error) {
	s.logger.Info("Listing all chats")
	return s.repository.ListChats(ctx)
}

// DeleteChat deletes a chat by ID
func (s *ChatService) DeleteChat(ctx context.Context, id string) error {
	s.logger.Info("Deleting chat", "chat_id", id)
	return s.repository.DeleteChat(ctx, id)
}

// GetModelInfo returns information about the current LLM model
func (s *ChatService) GetModelInfo(ctx context.Context) (map[string]interface{}, error) {
	s.logger.Info("Getting model information")
	return s.llm.GetModelInfo(ctx)
}

// GetModelName returns the name of the current LLM model
func (s *ChatService) GetModelName() string {
	if s.config.LLM.Provider == "ollama" {
		return s.config.LLM.Ollama.Model
	}
	// Default fallback
	return "unknown"
}

// OllamaImageResponse represents the JSON response structure for Ollama image analysis
type OllamaImageResponse struct {
	Model     string `json:"model"`
	CreatedAt string `json:"created_at"`
	Message   struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"message"`
	Done bool `json:"done"`
}

// CompletionWithImageAnalysis performs image analysis using the dedicated image LLM
func (s *ChatService) CompletionWithImageAnalysis(ctx context.Context, message domain.Message) (string, error) {
	s.logger.Info("Processing image analysis request", "image_count", len(message.Images))
	
	// Create a more detailed prompt for high-quality image analysis
	prompt := "Provide an extremely detailed analysis of this image. Describe all visible elements, people, objects, colors, textures, text, and any other notable aspects. Explain what's happening in the image and provide relevant context. DO NOT mention that this is a base64 encoded image or refer to decoding in any way."
	
	// If user provided custom content, append it to our enhanced prompt
	if message.Content != "" && message.Content != "Analyze the following image and provide a detailed description." {
		prompt = message.Content + " " + prompt
	}
	
	// Update the message with our enhanced prompt
	message.Content = prompt
	
	// Create the message history with a system prompt for image analysis
	messages := []domain.Message{
		{
			Role:    "system",
			Content: "You are an expert image analyst who provides comprehensive, detailed descriptions of image content. Always be thorough and precise. Never mention anything about base64 encoding or image format in your response.",
			Type:    domain.MessageTypeText,
		},
		message,
	}
	
	// Use the dedicated image LLM if available, otherwise fall back to the main LLM
	llm := s.imageLLM
	if llm == nil {
		s.logger.Warn("Image LLM not available, falling back to main LLM")
		llm = s.llm
	}
	
	// Use the model name from config for logging
	var modelName string
	if s.config.ImageLLM.Enabled {
		modelName = s.config.ImageLLM.Ollama.Model
	} else {
		modelName = s.GetModelName()
	}
	s.logger.Info("Using model for image analysis", "model", modelName)
	
	// Generate the response using the appropriate LLM
	rawResponse, err := llm.GenerateResponse(ctx, messages)
	
	if err != nil {
		s.logger.Error("Image analysis failed", "error", err)
		return "", fmt.Errorf("image analysis failed: %v", err)
	}
	
	// Try to parse the response as JSON first (Ollama may return JSON)
	s.logger.Info("Raw image analysis response received", "length", len(rawResponse))
	
	// Post-process the response to remove any references to base64 encoded images
	cleanedResponse := rawResponse
	
	// Check if this looks like JSON
	if strings.HasPrefix(strings.TrimSpace(rawResponse), "{") {
		var ollamaResp OllamaImageResponse
		if err := json.Unmarshal([]byte(rawResponse), &ollamaResp); err == nil {
			// Successfully parsed JSON response
			if ollamaResp.Message.Content != "" {
				cleanedResponse = ollamaResp.Message.Content
				s.logger.Info("Extracted content from JSON response", "json_length", len(rawResponse), "content_length", len(cleanedResponse))
			}
		}
	}
	
	// Remove any references to base64 encoding in the response
	cleanedResponse = removeBase64References(cleanedResponse)
	
	return cleanedResponse, nil
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

// processWebSearchRequest processes a user request that requires web search
func (s *ChatService) processWebSearchRequest(ctx context.Context, userContent string, chatHistory []domain.Message) (string, error) {
	// Step 1: Format the search query using the secondary LLM
	formattedQuery, err := s.webSearch.FormatSearchQuery(ctx, userContent)
	if err != nil {
		s.logger.Error("Failed to format search query", "error", err)
		// Fall back to direct LLM response if search query formatting fails
		return s.llm.GenerateResponse(ctx, chatHistory)
	}
	
	// Step 2: Perform the web search with the formatted query
	searchResults, err := s.webSearch.Search(ctx, formattedQuery)
	if err != nil {
		s.logger.Error("Web search failed", "error", err)
		// Fall back to direct LLM response if search fails
		return s.llm.GenerateResponse(ctx, chatHistory)
	}
	
	// Step 3: Format the search results for the LLM
	contextPrompt := formatSearchResultsForLLM(userContent, searchResults)
	
	// Step 4: Create a new prompt for the main LLM with search results as context
	promptWithContext := domain.NewMessage("user", contextPrompt)
	
	// Replace the last message (user's query) with our enhanced prompt that includes search results
	modifiedHistory := make([]domain.Message, len(chatHistory)-1)
	copy(modifiedHistory, chatHistory[:len(chatHistory)-1])
	modifiedHistory = append(modifiedHistory, promptWithContext)
	
	// Step 5: Generate the final response using the main LLM with search context
	return s.llm.GenerateResponse(ctx, modifiedHistory)
}

// formatSearchResultsForLLM formats search results into a prompt for the LLM
func formatSearchResultsForLLM(userQuery string, searchResults []ports.SearchResult) string {
	var sb strings.Builder
	
	// Add current date and time information
	currentTime := time.Now()
	dateStr := currentTime.Format("Monday, January 2, 2006 at 15:04 MST")
	
	// Add the original user query with date context
	sb.WriteString(fmt.Sprintf("I need information about: %s\n\n", userQuery))
	sb.WriteString(fmt.Sprintf("The current date and time is: %s\n\n", dateStr))
	
	// Add search results as context
	sb.WriteString("Here is the latest information I found from web search:\n\n")
	
	// Add up to 5 search results
	resultCount := len(searchResults)
	if resultCount > 5 {
		resultCount = 5
	}
	
	for i := 0; i < resultCount; i++ {
		result := searchResults[i]
		sb.WriteString(fmt.Sprintf("[%d] %s\n", i+1, result.Title))
		sb.WriteString(fmt.Sprintf("Link: %s\n", result.Link))
		sb.WriteString(fmt.Sprintf("Snippet: %s\n\n", result.Snippet))
	}
	
	// Add final instruction
	sb.WriteString("Based on the above information, please provide a helpful, accurate, and concise response to my query. ")
	sb.WriteString("Cite specific sources where appropriate by referring to the search result number. ")
	sb.WriteString("If the search results don't provide sufficient information, please clearly indicate this and give the best response you can based on your knowledge.")
	
	return sb.String()
}

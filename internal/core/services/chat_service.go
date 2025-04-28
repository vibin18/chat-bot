package services

import (
	"context"

	"github.com/vibin/chat-bot/internal/core/domain"
	"github.com/vibin/chat-bot/internal/core/ports"
	"github.com/vibin/chat-bot/internal/logger"
)

// ChatService is the core service that implements the business logic for chat interactions
type ChatService struct {
	llm        ports.LLMPort
	repository ports.ChatRepositoryPort
	logger     logger.Logger
}

// NewChatService creates a new ChatService
func NewChatService(llm ports.LLMPort, repository ports.ChatRepositoryPort, logger logger.Logger) *ChatService {
	return &ChatService{
		llm:        llm,
		repository: repository,
		logger:     logger,
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
	
	// Generate response using LLM
	s.logger.Info("Generating LLM response", "chat_id", chatID)
	
	response, err := s.llm.GenerateResponse(ctx, chat.Messages)
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

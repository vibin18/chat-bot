package ports

import (
	"context"

	"github.com/vibin/chat-bot/internal/core/domain"
)

// ChatRepositoryPort defines the interface for chat persistence
type ChatRepositoryPort interface {
	// SaveChat saves a chat
	SaveChat(ctx context.Context, chat *domain.Chat) error
	
	// GetChat retrieves a chat by ID
	GetChat(ctx context.Context, id string) (*domain.Chat, error)
	
	// ListChats returns all chats
	ListChats(ctx context.Context) ([]*domain.Chat, error)
	
	// DeleteChat deletes a chat by ID
	DeleteChat(ctx context.Context, id string) error
}

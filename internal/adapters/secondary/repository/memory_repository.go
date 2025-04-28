package repository

import (
	"context"
	"errors"
	"sync"

	"github.com/vibin/chat-bot/internal/core/domain"
	"github.com/vibin/chat-bot/internal/logger"
)

// InMemoryRepository implements the ChatRepositoryPort interface with in-memory storage
type InMemoryRepository struct {
	chats  map[string]*domain.Chat
	mutex  sync.RWMutex
	logger logger.Logger
}

// NewInMemoryRepository creates a new InMemoryRepository
func NewInMemoryRepository(log logger.Logger) *InMemoryRepository {
	return &InMemoryRepository{
		chats:  make(map[string]*domain.Chat),
		logger: log,
	}
}

// SaveChat saves a chat
func (r *InMemoryRepository) SaveChat(ctx context.Context, chat *domain.Chat) error {
	r.logger.Info("Saving chat", "chat_id", chat.ID)
	
	r.mutex.Lock()
	defer r.mutex.Unlock()
	
	r.chats[chat.ID] = chat
	return nil
}

// GetChat retrieves a chat by ID
func (r *InMemoryRepository) GetChat(ctx context.Context, id string) (*domain.Chat, error) {
	r.logger.Info("Getting chat", "chat_id", id)
	
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	
	chat, exists := r.chats[id]
	if !exists {
		r.logger.Warn("Chat not found", "chat_id", id)
		return nil, errors.New("chat not found")
	}
	
	return chat, nil
}

// ListChats returns all chats
func (r *InMemoryRepository) ListChats(ctx context.Context) ([]*domain.Chat, error) {
	r.logger.Info("Listing all chats")
	
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	
	chats := make([]*domain.Chat, 0, len(r.chats))
	for _, chat := range r.chats {
		chats = append(chats, chat)
	}
	
	return chats, nil
}

// DeleteChat deletes a chat by ID
func (r *InMemoryRepository) DeleteChat(ctx context.Context, id string) error {
	r.logger.Info("Deleting chat", "chat_id", id)
	
	r.mutex.Lock()
	defer r.mutex.Unlock()
	
	if _, exists := r.chats[id]; !exists {
		r.logger.Warn("Chat not found for deletion", "chat_id", id)
		return errors.New("chat not found")
	}
	
	delete(r.chats, id)
	return nil
}

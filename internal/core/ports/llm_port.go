package ports

import (
	"context"

	"github.com/vibin/chat-bot/internal/core/domain"
)

// LLMPort defines the interface for interacting with the LLM backend
type LLMPort interface {
	// GenerateResponse generates a response from the LLM for a given chat history
	GenerateResponse(ctx context.Context, messages []domain.Message) (string, error)
	
	// GetModelInfo returns information about the current LLM model
	GetModelInfo(ctx context.Context) (map[string]interface{}, error)
}

package services

import (
	"log"
	"time"

	"github.com/vibin/chat-bot/internal/adapters/secondary/database"
)

// MemoryService provides an interface between the memory database and the application
type MemoryService struct {
	memoryDB *database.MemoryDatabase
}

// NewMemoryService creates a new memory service
func NewMemoryService(memoryDB *database.MemoryDatabase) *MemoryService {
	return &MemoryService{
		memoryDB: memoryDB,
	}
}

// AddMemory adds a memory to the database with retry mechanism
func (s *MemoryService) AddMemory(userID, conversationID, content string) error {
	memory := &database.Memory{
		UserID:        userID,
		ConversationID: conversationID,
		Content:       content,
		CreatedAt:     time.Now(),
		LastUsed:      time.Now(),
		UseCount:      1,
	}
	
	// Implement retry logic for database operations
	var err error
	for retries := 0; retries < 3; retries++ {
		err = s.memoryDB.AddMemory(memory)
		if err == nil {
			return nil // Success
		}
		
		// Log the error and retry
		log.Printf("Warning: Database operation failed (attempt %d/3): %v", retries+1, err)
		time.Sleep(time.Duration(100*(retries+1)) * time.Millisecond) // Exponential backoff
	}
	
	// All retries failed
	log.Printf("ERROR: Failed to add memory to database after 3 attempts: %v", err)
	return err
}

// GetUserMemories retrieves all memories for a specific user in a conversation
func (s *MemoryService) GetUserMemories(userID, conversationID string) ([]*database.Memory, error) {
	return s.memoryDB.GetMemories(userID, conversationID)
}

// GetAllConversationMemories gets all memories for a specific conversation
func (s *MemoryService) GetAllConversationMemories(conversationID string) ([]*database.Memory, error) {
	return s.memoryDB.GetAllMemoriesByConversation(conversationID)
}

// GetMemoryUsers gets all users with memories for a specific conversation
func (s *MemoryService) GetMemoryUsers(conversationID string) (map[string]int, error) {
	return s.memoryDB.GetMemoryUsers(conversationID)
}

// UpdateMemory updates an existing memory
func (s *MemoryService) UpdateMemory(id int64, content string) error {
	memory, err := s.memoryDB.GetMemoryByID(id)
	if err != nil {
		return err
	}
	
	memory.Content = content
	memory.LastUsed = time.Now()
	return s.memoryDB.UpdateMemory(memory)
}

// DeleteMemory deletes a memory by ID
func (s *MemoryService) DeleteMemory(id int64) error {
	return s.memoryDB.DeleteMemory(id)
}

// ClearUserMemories deletes all memories for a specific user in a conversation
func (s *MemoryService) ClearUserMemories(userID, conversationID string) error {
	return s.memoryDB.DeleteAllMemoriesForUser(userID, conversationID)
}

// IncrementMemoryUseCount increments the use count for a memory
func (s *MemoryService) IncrementMemoryUseCount(id int64) error {
	return s.memoryDB.IncrementUseCount(id)
}

// SyncMemoriesFromCache syncs in-memory memories to the database
func (s *MemoryService) SyncMemoriesFromCache(userID, conversationID string, contents []string) error {
	var lastErr error
	for _, content := range contents {
		if err := s.AddMemory(userID, conversationID, content); err != nil {
			log.Printf("Error syncing memory to database: %v", err)
			lastErr = err
		}
	}
	return lastErr
}

// GetMemoriesAsStrings gets all memories for a user/conversation as strings
func (s *MemoryService) GetMemoriesAsStrings(userID, conversationID string) ([]string, error) {
	dbMemories, err := s.GetUserMemories(userID, conversationID)
	if err != nil {
		return nil, err
	}
	
	memories := make([]string, len(dbMemories))
	for i, memory := range dbMemories {
		memories[i] = memory.Content
	}
	
	return memories, nil
}

package whatsapp

import (
	"log"
	"time"

	"github.com/vibin/chat-bot/internal/adapters/secondary/database"
	"github.com/vibin/chat-bot/internal/core/ports"
	"github.com/vibin/chat-bot/internal/core/services"
)

// InitializeMemoryDB initializes the memory database and returns a memory service
func (a *WhatsAppAdapter) InitializeMemoryDB() (*services.MemoryService, error) {
	// Initialize the memory database
	memoryDB, err := database.NewMemoryDatabase()
	if err != nil {
		return nil, err
	}

	// Create a memory service
	memoryService := services.NewMemoryService(memoryDB)
	return memoryService, nil
}

// SetMemoryService sets the memory service for the adapter
func (a *WhatsAppAdapter) SetMemoryService(memoryService *services.MemoryService) {
	a.memoryService = memoryService
}

// SyncMemoryToDatabase syncs in-memory memories to the database
func (a *WhatsAppAdapter) SyncMemoryToDatabase(userID, conversationID string) {
	if a.memoryService == nil {
		return
	}

	// Get memories from in-memory store
	a.mutex.RLock()
	key := createSessionKey(userID, conversationID)
	memories := a.memoryManager.memories[key]
	a.mutex.RUnlock()

	// Convert to string contents and sync to database
	contents := make([]string, len(memories))
	for i, memory := range memories {
		contents[i] = memory.Content
	}

	// Use the memory service to sync to database
	a.memoryService.SyncMemoriesFromCache(userID, conversationID, contents)
}

// PersistMemory adds a new memory to both in-memory and database storage
func (a *WhatsAppAdapter) PersistMemory(userID, conversationID, content string) {
	// Add to in-memory store first
	a.memoryManager.AddMemory(userID, conversationID, content)

	// Then persist to database if memory service is available
	if a.memoryService != nil {
		if err := a.memoryService.AddMemory(userID, conversationID, content); err != nil {
			log.Printf("Error persisting memory to database: %v", err)
		}
	}
}

// GetPersistentMemories gets memories from both in-memory and database storage
func (a *WhatsAppAdapter) GetPersistentMemories(userID, conversationID string) []Memory {
	// Get memories from in-memory store
	inMemMemories := a.memoryManager.GetMemories(userID, conversationID)

	// If memory service is not available, return in-memory memories only
	if a.memoryService == nil {
		return inMemMemories
	}

	// Get memories from database
	dbMemories, err := a.memoryService.GetUserMemories(userID, conversationID)
	if err != nil {
		log.Printf("Error getting memories from database: %v", err)
		return inMemMemories
	}

	// Convert database memories to in-memory format
	dbConvertedMemories := make([]Memory, len(dbMemories))
	for i, dbMemory := range dbMemories {
		dbConvertedMemories[i] = Memory{
			Content:   dbMemory.Content,
			CreatedAt: dbMemory.CreatedAt,
			LastUsed:  dbMemory.LastUsed,
			UseCount:  dbMemory.UseCount,
		}
	}

	// Merge and deduplicate memories
	merged := mergeMemories(inMemMemories, dbConvertedMemories)
	return merged
}

// mergeMemories merges two sets of memories, removing duplicates based on content
func mergeMemories(a, b []Memory) []Memory {
	seen := make(map[string]bool)
	result := []Memory{}

	// Add all memories from a, tracking content
	for _, memory := range a {
		if !seen[memory.Content] {
			seen[memory.Content] = true
			result = append(result, memory)
		}
	}

	// Add memories from b if content is not already included
	for _, memory := range b {
		if !seen[memory.Content] {
			seen[memory.Content] = true
			result = append(result, memory)
		}
	}

	return result
}

// GetUserPersistentMemories gets memories for a specific user from the database
func (a *WhatsAppAdapter) GetUserPersistentMemories(conversationID, userID string) *ports.UserMemories {
	if a.memoryService == nil {
		// Fall back to in-memory if database is not available
		memories := a.memoryManager.GetMemories(userID, conversationID)
		context := a.memoryManager.GetContext(userID, conversationID)
		
		// Convert to port format
		portMemories := make([]ports.Memory, len(memories))
		for i, memory := range memories {
			portMemories[i] = ports.Memory{
				Content:   memory.Content,
				CreatedAt: memory.CreatedAt.Format(time.RFC3339),
				LastUsed:  memory.LastUsed.Format(time.RFC3339),
				UseCount:  memory.UseCount,
			}
		}
		
		// Get conversation details
		a.mutex.RLock()
		conv, exists := a.conversations[conversationID]
		groupName := "Unknown Group"
		if exists {
			groupName = conv.GroupName
		}
		a.mutex.RUnlock()
		
		return &ports.UserMemories{
			ConversationID: conversationID,
			GroupName:      groupName,
			UserID:         userID,
			UserName:       a.getUserDisplayName(userID),
			Memories:       portMemories,
			Context:        context,
		}
	}

	// Get memories from database
	dbMemories, err := a.memoryService.GetUserMemories(userID, conversationID)
	if err != nil {
		log.Printf("Error getting memories from database: %v", err)
		return nil
	}
	
	// Get context from in-memory store
	context := a.memoryManager.GetContext(userID, conversationID)
	
	// Convert to port format
	portMemories := make([]ports.Memory, len(dbMemories))
	for i, memory := range dbMemories {
		portMemories[i] = ports.Memory{
			Content:   memory.Content,
			CreatedAt: memory.CreatedAt.Format(time.RFC3339),
			LastUsed:  memory.LastUsed.Format(time.RFC3339),
			UseCount:  memory.UseCount,
		}
	}
	
	// Get conversation details
	a.mutex.RLock()
	conv, exists := a.conversations[conversationID]
	groupName := "Unknown Group"
	if exists {
		groupName = conv.GroupName
	}
	a.mutex.RUnlock()
	
	return &ports.UserMemories{
		ConversationID: conversationID,
		GroupName:      groupName,
		UserID:         userID,
		UserName:       a.getUserDisplayName(userID),
		Memories:       portMemories,
		Context:        context,
	}
}

// DeletePersistentMemory deletes a memory from the database by index
func (a *WhatsAppAdapter) DeletePersistentMemory(conversationID string, index int) bool {
	if a.memoryService == nil {
		return false
	}
	
	// Get all memories for the conversation
	dbMemories, err := a.memoryService.GetAllConversationMemories(conversationID)
	if err != nil {
		log.Printf("Error getting memories from database: %v", err)
		return false
	}
	
	// Check if index is valid
	if index < 0 || index >= len(dbMemories) {
		return false
	}
	
	// Delete the memory
	err = a.memoryService.DeleteMemory(dbMemories[index].ID)
	if err != nil {
		log.Printf("Error deleting memory from database: %v", err)
		return false
	}
	
	return true
}

// UpdatePersistentMemory updates a memory in the database
func (a *WhatsAppAdapter) UpdatePersistentMemory(conversationID string, index int, newContent string) bool {
	if a.memoryService == nil {
		return false
	}
	
	// Get all memories for the conversation
	dbMemories, err := a.memoryService.GetAllConversationMemories(conversationID)
	if err != nil {
		log.Printf("Error getting memories from database: %v", err)
		return false
	}
	
	// Check if index is valid
	if index < 0 || index >= len(dbMemories) {
		return false
	}
	
	// Update the memory
	err = a.memoryService.UpdateMemory(dbMemories[index].ID, newContent)
	if err != nil {
		log.Printf("Error updating memory in database: %v", err)
		return false
	}
	
	return true
}

// ClearAllPersistentMemories clears all memories for a user in a conversation
func (a *WhatsAppAdapter) ClearAllPersistentMemories(userID, conversationID string) bool {
	if a.memoryService == nil {
		return false
	}
	
	// Delete all memories for the user in the conversation
	err := a.memoryService.ClearUserMemories(userID, conversationID)
	if err != nil {
		log.Printf("Error clearing user memories from database: %v", err)
		return false
	}
	
	return true
}

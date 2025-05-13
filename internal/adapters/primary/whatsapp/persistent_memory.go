package whatsapp

import (
	"fmt"
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
func (a *WhatsAppAdapter) SyncMemoryToDatabase(userID, conversationID string) error {
	if a.memoryService == nil {
		a.log.Error("Cannot sync memories: memory service is not initialized")
		return fmt.Errorf("memory service not initialized")
	}

	// Get memories from in-memory store
	a.mutex.RLock()
	key := createSessionKey(userID, conversationID)
	memories := a.memoryManager.memories[key]
	a.mutex.RUnlock()

	// Log memory sync attempt
	a.log.Info("Syncing memories to database", 
		"user_id", userID, 
		"conversation_id", conversationID, 
		"memory_count", len(memories))

	// Convert to string contents and sync to database
	contents := make([]string, len(memories))
	for i, memory := range memories {
		contents[i] = memory.Content
		a.log.Debug("Memory content to sync", "index", i, "content", memory.Content)
	}

	// Use the memory service to sync to database
	err := a.memoryService.SyncMemoriesFromCache(userID, conversationID, contents)
	if err != nil {
		a.log.Error("Failed to sync memories to database", 
			"error", err, 
			"user_id", userID, 
			"conversation_id", conversationID)
		return err
	}

	a.log.Info("Successfully synced memories to database", 
		"user_id", userID, 
		"conversation_id", conversationID, 
		"count", len(memories))
	return nil
}

// syncAllMemories syncs all in-memory memories to the database
// This is called periodically to ensure persistence
func (a *WhatsAppAdapter) syncAllMemories() {
	if a.memoryService == nil {
		a.log.Error("Cannot sync all memories: memory service is not initialized")
		return
	}

	// Get all active memory keys
	a.mutex.RLock()
	// Create a list of userID/conversationID pairs to sync
	var memoryPairs []struct {
		userID string
		conversationID string
	}

	// Collect all SessionKeys to process
	for key := range a.memoryManager.memories {
		memoryPairs = append(memoryPairs, struct {
			userID string
			conversationID string
		}{
			userID: key.UserID,
			conversationID: key.ConversationID,
		})
	}
	a.mutex.RUnlock()

	a.log.Info("Starting periodic sync of all memories", "pair_count", len(memoryPairs))

	// Track success and failure counts
	successCount := 0
	failureCount := 0

	// Iterate through all memory pairs and sync each one
	for _, pair := range memoryPairs {
		// Sync this specific user's memories
		err := a.SyncMemoryToDatabase(pair.userID, pair.conversationID)
		if err != nil {
			a.log.Error("Failed to sync memories during periodic sync", 
				"error", err, 
				"user_id", pair.userID, 
				"conversation_id", pair.conversationID)
			failureCount++
		} else {
			successCount++
		}
	}

	a.log.Info("Completed periodic sync of all memories", 
		"success_count", successCount, 
		"failure_count", failureCount)
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

package whatsapp

import (
	"log"
)

// AddMemory adds a new memory for a user in a conversation through the admin interface
func (a *WhatsAppAdapter) AddMemory(conversationID string, userID string, content string) bool {
	// First check if the conversation exists
	a.mutex.RLock()
	_, conversationExists := a.conversations[conversationID]
	a.mutex.RUnlock()
	
	if !conversationExists {
		return false
	}
	
	// Add to persistent storage if available
	if a.memoryService != nil {
		err := a.memoryService.AddMemory(userID, conversationID, content)
		if err != nil {
			log.Printf("Error adding memory to database: %v", err)
			return false
		}
		
		// Also add to in-memory storage for immediate use
		a.memoryManager.AddMemory(userID, conversationID, content)
		return true
	}
	
	// Fall back to in-memory only
	a.memoryManager.AddMemory(userID, conversationID, content)
	return true
}

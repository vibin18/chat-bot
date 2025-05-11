package whatsapp

import (
	"time"

	"github.com/vibin/chat-bot/internal/core/ports"
	"go.mau.fi/whatsmeow/types"
)

// convertMemoryToPortMemory converts internal Memory to ports.Memory
func convertMemoryToPortMemory(memory Memory) ports.Memory {
	return ports.Memory{
		Content:   memory.Content,
		CreatedAt: memory.CreatedAt.Format(time.RFC3339),
		LastUsed:  memory.LastUsed.Format(time.RFC3339),
		UseCount:  memory.UseCount,
	}
}

// getUserIDsForConversation returns all user IDs participating in a conversation
func (a *WhatsAppAdapter) getUserIDsForConversation(conversationID string) []string {
	// For now, we'll just use the group members or a default user ID
	// In a real implementation, you might track participants more carefully
	result := []string{"default_user"} // Fallback user ID
	
	// If we can extract a group ID from the conversation ID, try to get participants
	if groupID, ok := a.extractGroupIDFromConversationID(conversationID); ok {
		if jid, err := types.ParseJID(groupID); err == nil {
			if participants, err := a.client.GetGroupInfo(jid); err == nil && len(participants.Participants) > 0 {
				userIDs := make([]string, len(participants.Participants))
				for i, p := range participants.Participants {
					userIDs[i] = p.JID.String()
				}
				return userIDs
			}
		}
	}
	
	return result
}

// extractGroupIDFromConversationID tries to extract a group JID from a conversation ID
func (a *WhatsAppAdapter) extractGroupIDFromConversationID(conversationID string) (string, bool) {
	// This is a simplified implementation
	// In a real system, you might have a more robust way to associate conversation IDs with group IDs
	return conversationID, true // For simplicity, assume conversation ID is the group ID
}

// GetAllMemoryInfo returns summary info for all conversations with memories
func (a *WhatsAppAdapter) GetAllMemoryInfo() []ports.MemoryInfo {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	result := []ports.MemoryInfo{}
	memoryCountMap := make(map[string]int)
	contextCountMap := make(map[string]int)
	
	// Count memories and context for each conversation by combining all user sessions
	for key := range a.memoryManager.memories {
		if _, exists := memoryCountMap[key.ConversationID]; !exists {
			memoryCountMap[key.ConversationID] = 0
		}
		memoryCountMap[key.ConversationID] += len(a.memoryManager.memories[key])
	}
	
	for key := range a.memoryManager.context {
		if _, exists := contextCountMap[key.ConversationID]; !exists {
			contextCountMap[key.ConversationID] = 0
		}
		contextCountMap[key.ConversationID] += len(a.memoryManager.context[key])
	}

	// Create a map of conversation details
	for convID, conv := range a.conversations {
		info := ports.MemoryInfo{
			ConversationID: convID,
			GroupName:      conv.GroupName,
			MemoryCount:    memoryCountMap[convID],
			ContextCount:   contextCountMap[convID],
			LastActivity:   conv.LastActivity.Format(time.RFC3339),
		}
		result = append(result, info)
	}

	return result
}

// GetConversationDetails returns detailed memory and context for a specific conversation
func (a *WhatsAppAdapter) GetConversationDetails(conversationID string) *ports.ConversationDetails {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	// Check if conversation exists
	conv, exists := a.conversations[conversationID]
	if !exists {
		return nil
	}

	// Get all user IDs for this conversation
	userIDs := a.getUserIDsForConversation(conversationID)
	
	// Collect all memories and context across all users for this conversation
	allMemories := []Memory{}
	allContext := []string{}
	
	for _, userID := range userIDs {
		// Get memories and context for this user+conversation
		memories := a.memoryManager.GetMemories(userID, conversationID)
		context := a.memoryManager.GetContext(userID, conversationID)
		
		// Add to our collections
		allMemories = append(allMemories, memories...)
		allContext = append(allContext, context...)
	}

	// Convert memories to port format
	portMemories := make([]ports.Memory, len(allMemories))
	for i, memory := range allMemories {
		portMemories[i] = convertMemoryToPortMemory(memory)
	}

	// Create result
	result := &ports.ConversationDetails{
		ConversationID: conversationID,
		GroupName:      conv.GroupName,
		Memories:       portMemories,
		Context:        allContext,
		LastActivity:   conv.LastActivity.Format(time.RFC3339),
	}

	return result
}

// DeleteMemory deletes a specific memory from a conversation
func (a *WhatsAppAdapter) DeleteMemory(conversationID string, memoryIndex int) bool {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	// Check if conversation exists
	if _, exists := a.conversations[conversationID]; !exists {
		return false
	}
	
	// This is a bit complex now with the session-based memory system
	// We need to find which user's memory we're targeting
	targetUserID := ""
	targetKey := SessionKey{}
	indexWithinSession := 0
	
	// Get all user IDs for this conversation
	userIDs := a.getUserIDsForConversation(conversationID)
	
	// Count memories to find which session contains our target index
	currentIndex := 0
	for _, userID := range userIDs {
		key := createSessionKey(userID, conversationID)
		memories, exists := a.memoryManager.memories[key]
		
		if !exists || len(memories) == 0 {
			continue
		}
		
		if currentIndex <= memoryIndex && memoryIndex < currentIndex+len(memories) {
			// Found the session containing our target memory
			targetUserID = userID
			targetKey = key
			indexWithinSession = memoryIndex - currentIndex
			break
		}
		
		currentIndex += len(memories)
	}
	
	if targetUserID == "" {
		// Didn't find the memory
		return false
	}
	
	// Get memories for this session
	memories := a.memoryManager.memories[targetKey]
	if indexWithinSession < 0 || indexWithinSession >= len(memories) {
		return false
	}

	// Remove memory
	a.memoryManager.memories[targetKey] = append(
		memories[:indexWithinSession],
		memories[indexWithinSession+1:]...,
	)

	return true
}

// ClearAllMemories clears all memories for a conversation
func (a *WhatsAppAdapter) ClearAllMemories(conversationID string) bool {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	// Check if conversation exists
	if _, exists := a.conversations[conversationID]; !exists {
		return false
	}

	// Clear memories and context for all users in this conversation
	userIDs := a.getUserIDsForConversation(conversationID)
	
	for _, userID := range userIDs {
		key := createSessionKey(userID, conversationID)
		delete(a.memoryManager.memories, key)
		delete(a.memoryManager.context, key)
	}
	
	return true
}

// UpdateMemory updates the content of a specific memory
func (a *WhatsAppAdapter) UpdateMemory(conversationID string, memoryIndex int, newContent string) bool {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	// Check if conversation exists
	if _, exists := a.conversations[conversationID]; !exists {
		return false
	}
	
	// Similar to DeleteMemory, we need to find which user's memory we're targeting
	targetUserID := ""
	targetKey := SessionKey{}
	indexWithinSession := 0
	
	// Get all user IDs for this conversation
	userIDs := a.getUserIDsForConversation(conversationID)
	
	// Count memories to find which session contains our target index
	currentIndex := 0
	for _, userID := range userIDs {
		key := createSessionKey(userID, conversationID)
		memories, exists := a.memoryManager.memories[key]
		
		if !exists || len(memories) == 0 {
			continue
		}
		
		if currentIndex <= memoryIndex && memoryIndex < currentIndex+len(memories) {
			// Found the session containing our target memory
			targetUserID = userID
			targetKey = key
			indexWithinSession = memoryIndex - currentIndex
			break
		}
		
		currentIndex += len(memories)
	}
	
	if targetUserID == "" {
		// Didn't find the memory
		return false
	}

	// Get memories for this session
	memories := a.memoryManager.memories[targetKey]
	if indexWithinSession < 0 || indexWithinSession >= len(memories) {
		return false
	}

	// Update memory content
	memories[indexWithinSession].Content = newContent
	memories[indexWithinSession].LastUsed = time.Now()
	a.memoryManager.memories[targetKey] = memories

	return true
}

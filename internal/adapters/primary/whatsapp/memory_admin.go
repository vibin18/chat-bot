package whatsapp

import (
	"strings"
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
	// In this improved implementation, we'll scan our existing memory to find user IDs
	// that have interacted with this conversation
	knownUsers := make(map[string]bool)
	knownUsers["default_user"] = true // Always include default user as fallback
	
	// Scan memory storage for user IDs with this conversation
	for key := range a.memoryManager.memories {
		if key.ConversationID == conversationID {
			knownUsers[key.UserID] = true
		}
	}
	
	// Scan context storage for user IDs with this conversation
	for key := range a.memoryManager.context {
		if key.ConversationID == conversationID {
			knownUsers[key.UserID] = true
		}
	}
	
	// Convert to slice
	userIDs := make([]string, 0, len(knownUsers))
	for userID := range knownUsers {
		userIDs = append(userIDs, userID)
	}
	
	// If we still don't have any users (other than default), try to get from WhatsApp but don't fail if it doesn't work
	if len(userIDs) <= 1 {
		a.tryAddGroupParticipantsAsUsers(conversationID, knownUsers)
		
		// Rebuild the list with any new users
		userIDs = make([]string, 0, len(knownUsers))
		for userID := range knownUsers {
			userIDs = append(userIDs, userID)
		}
	}
	
	return userIDs
}

// tryAddGroupParticipantsAsUsers attempts to add group participants as known users, but doesn't fail if it can't
func (a *WhatsAppAdapter) tryAddGroupParticipantsAsUsers(conversationID string, knownUsers map[string]bool) {
	// Try to get group info, but don't fail if we can't
	groupID, ok := a.extractGroupIDFromConversationID(conversationID)
	if !ok {
		return
	}
	
	jid, err := types.ParseJID(groupID)
	if err != nil {
		return
	}
	
	// Skip if client isn't connected
	if a.client == nil {
		return
	}
	
	participants, err := a.client.GetGroupInfo(jid)
	if err != nil || participants == nil {
		return
	}
	
	// Add any participants we found
	for _, p := range participants.Participants {
		if p.JID.IsEmpty() {
			continue
		}
		knownUsers[p.JID.String()] = true
	}
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

// GetUsersInConversation returns a list of users in a specific conversation
func (a *WhatsAppAdapter) GetUsersInConversation(conversationID string) []ports.UserInfo {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	// Check if conversation exists
	_, exists := a.conversations[conversationID]
	if !exists {
		return nil
	}

	// Get all user IDs for this conversation
	userIDs := a.getUserIDsForConversation(conversationID)
	if len(userIDs) == 0 {
		return []ports.UserInfo{}
	}

	// Get user information
	users := make([]ports.UserInfo, 0, len(userIDs))
	
	for _, userID := range userIDs {
		// Get memory count for this user
		memoryCount := 0
		key := createSessionKey(userID, conversationID)
		
		a.memoryManager.mutex.RLock()
		if memories, exists := a.memoryManager.memories[key]; exists {
			memoryCount = len(memories)
		}
		a.memoryManager.mutex.RUnlock()
		
		// Create a readable name for the user
		name := a.getUserDisplayName(userID)
		
		users = append(users, ports.UserInfo{
			UserID:      userID,
			Name:        name,
			MemoryCount: memoryCount,
		})
	}

	return users
}

// getUserDisplayName returns a human-readable name for a user
func (a *WhatsAppAdapter) getUserDisplayName(userID string) string {
	// If it's default user, return "Default User"
	if userID == "default_user" {
		return "Default User"
	}
	
	// Try to get name from WhatsApp if client is connected
	if a.client != nil {
		// Parse JID
		jid, err := types.ParseJID(userID)
		if err == nil && !jid.IsEmpty() {
			// Try to get contact info
			contact, err := a.client.Store.Contacts.GetContact(jid)
			if err == nil && contact.PushName != "" {
				return contact.PushName
			}
			
			// If contact name isn't available, get the phone number or username
			if jid.User != "" {
				// For user-style JIDs, use the user part
				return jid.User
			}
		}
	}
	
	// Use first part of user ID or the full ID if can't split
	idParts := strings.Split(userID, "@")
	if len(idParts) > 0 && idParts[0] != "" {
		return idParts[0]
	}
	
	// Fallback to user ID if all else fails
	return userID
}

// GetUserMemories returns memories and context for a specific user in a conversation
func (a *WhatsAppAdapter) GetUserMemories(conversationID, userID string) *ports.UserMemories {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	// Check if conversation exists
	conv, exists := a.conversations[conversationID]
	if !exists {
		return nil
	}

	// Get memories and context for this user
	memories := a.memoryManager.GetMemories(userID, conversationID)
	context := a.memoryManager.GetContext(userID, conversationID)

	// Convert memories to port format
	portMemories := make([]ports.Memory, len(memories))
	for i, memory := range memories {
		portMemories[i] = convertMemoryToPortMemory(memory)
	}

	// Create result
	result := &ports.UserMemories{
		ConversationID: conversationID,
		GroupName:      conv.GroupName,
		UserID:         userID,
		UserName:       a.getUserDisplayName(userID),
		Memories:       portMemories,
		Context:        context,
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

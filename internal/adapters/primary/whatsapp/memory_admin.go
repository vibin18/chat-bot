package whatsapp

import (
	"time"

	"github.com/vibin/chat-bot/internal/core/ports"
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

// GetAllMemoryInfo returns summary info for all conversations with memories
func (a *WhatsAppAdapter) GetAllMemoryInfo() []ports.MemoryInfo {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	result := []ports.MemoryInfo{}

	// Create a map of conversation details
	for convID, conv := range a.conversations {
		info := ports.MemoryInfo{
			ConversationID: convID,
			GroupName:      conv.GroupName,
			MemoryCount:    len(a.memoryManager.GetMemories(convID)),
			ContextCount:   len(a.memoryManager.GetContext(convID)),
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

	// Get memories and context
	memories := a.memoryManager.GetMemories(conversationID)
	context := a.memoryManager.GetContext(conversationID)

	// Convert memories to port format
	portMemories := make([]ports.Memory, len(memories))
	for i, memory := range memories {
		portMemories[i] = convertMemoryToPortMemory(memory)
	}

	// Create result
	result := &ports.ConversationDetails{
		ConversationID: conversationID,
		GroupName:      conv.GroupName,
		Memories:       portMemories,
		Context:        context,
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

	// Get memories
	memories := a.memoryManager.memories[conversationID]
	if memoryIndex < 0 || memoryIndex >= len(memories) {
		return false
	}

	// Remove memory
	a.memoryManager.memories[conversationID] = append(
		memories[:memoryIndex],
		memories[memoryIndex+1:]...,
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

	// Clear memories
	delete(a.memoryManager.memories, conversationID)
	
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

	// Get memories
	memories := a.memoryManager.memories[conversationID]
	if memoryIndex < 0 || memoryIndex >= len(memories) {
		return false
	}

	// Update memory content
	memories[memoryIndex].Content = newContent
	memories[memoryIndex].LastUsed = time.Now()
	a.memoryManager.memories[conversationID] = memories

	return true
}

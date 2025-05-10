package whatsapp

import (
	"sync"
	"time"
)

// Memory represents a stored piece of information about a conversation
type Memory struct {
	Content     string
	CreatedAt   time.Time
	LastUsed    time.Time
	UseCount    int
}

// MemoryManager manages in-memory storage of conversation context and memories
type MemoryManager struct {
	// Map of conversation ID to memories
	memories    map[string][]Memory
	// Map of conversation ID to context messages (last 10)
	context     map[string][]string
	mutex       sync.RWMutex
}

// NewMemoryManager creates a new memory manager
func NewMemoryManager() *MemoryManager {
	return &MemoryManager{
		memories: make(map[string][]Memory),
		context:  make(map[string][]string),
		mutex:    sync.RWMutex{},
	}
}

// AddMemory adds a new memory to the conversation
func (m *MemoryManager) AddMemory(conversationID string, content string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Create memory
	memory := Memory{
		Content:   content,
		CreatedAt: time.Now(),
		LastUsed:  time.Now(),
		UseCount:  0,
	}

	// Initialize memories for this conversation if needed
	if _, exists := m.memories[conversationID]; !exists {
		m.memories[conversationID] = []Memory{}
	}

	// Add memory
	m.memories[conversationID] = append(m.memories[conversationID], memory)
}

// GetMemories returns all memories for a conversation
func (m *MemoryManager) GetMemories(conversationID string) []Memory {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if memories, exists := m.memories[conversationID]; exists {
		// Update usage stats for each memory
		for i := range memories {
			memories[i].LastUsed = time.Now()
			memories[i].UseCount++
		}
		return memories
	}

	return []Memory{}
}

// AddContextMessage adds a message to the conversation context
func (m *MemoryManager) AddContextMessage(conversationID string, message string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Initialize context for this conversation if needed
	if _, exists := m.context[conversationID]; !exists {
		m.context[conversationID] = []string{}
	}

	// Add message to context
	m.context[conversationID] = append(m.context[conversationID], message)

	// Limit to last 10 messages
	if len(m.context[conversationID]) > 10 {
		m.context[conversationID] = m.context[conversationID][len(m.context[conversationID])-10:]
	}
}

// GetContext returns the context (last 10 messages) for a conversation
func (m *MemoryManager) GetContext(conversationID string) []string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if context, exists := m.context[conversationID]; exists {
		return context
	}

	return []string{}
}

// ClearContext clears the context for a conversation
func (m *MemoryManager) ClearContext(conversationID string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	delete(m.context, conversationID)
}

// ExtractMemories extracts potential memories from a message
// In a real implementation, this would use NLP or some heuristic to identify
// personal information, preferences, etc.
func (m *MemoryManager) ExtractMemories(conversationID string, userMessage string, botResponse string) {
	// This is a simplified implementation
	// In a real system, you'd use NLP or pattern matching to extract memories
	
	// For now, we'll just add the entire exchange as context
	m.AddContextMessage(conversationID, "User: "+userMessage)
	m.AddContextMessage(conversationID, "Bot: "+botResponse)
}

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

// SessionKey represents a unique identifier for a user session
type SessionKey struct {
	UserID        string
	ConversationID string
}

// MemoryManager manages in-memory storage of conversation context and memories
type MemoryManager struct {
	// Map of session key to memories
	memories    map[SessionKey][]Memory
	// Map of session key to context messages (last 10)
	context     map[SessionKey][]string
	mutex       sync.RWMutex
}

// NewMemoryManager creates a new memory manager
func NewMemoryManager() *MemoryManager {
	return &MemoryManager{
		memories: make(map[SessionKey][]Memory),
		context:  make(map[SessionKey][]string),
		mutex:    sync.RWMutex{},
	}
}

// createSessionKey creates a combined session key from user ID and conversation ID
func createSessionKey(userID, conversationID string) SessionKey {
	return SessionKey{
		UserID:        userID,
		ConversationID: conversationID,
	}
}

// AddMemory adds a new memory to the conversation
func (m *MemoryManager) AddMemory(userID, conversationID string, content string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Create session key
	key := createSessionKey(userID, conversationID)

	// Create memory
	memory := Memory{
		Content:   content,
		CreatedAt: time.Now(),
		LastUsed:  time.Now(),
		UseCount:  0,
	}

	// Initialize memories for this session if needed
	if _, exists := m.memories[key]; !exists {
		m.memories[key] = []Memory{}
	}

	// Add memory
	m.memories[key] = append(m.memories[key], memory)
}

// GetMemories returns all memories for a conversation
func (m *MemoryManager) GetMemories(userID, conversationID string) []Memory {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// Create session key
	key := createSessionKey(userID, conversationID)

	if memories, exists := m.memories[key]; exists {
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
func (m *MemoryManager) AddContextMessage(userID, conversationID string, message string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Create session key
	key := createSessionKey(userID, conversationID)

	// Initialize context for this session if needed
	if _, exists := m.context[key]; !exists {
		m.context[key] = []string{}
	}

	// Add message to context
	m.context[key] = append(m.context[key], message)

	// Limit to last 10 messages
	if len(m.context[key]) > 10 {
		m.context[key] = m.context[key][len(m.context[key])-10:]
	}
}

// GetContext returns the context (last 10 messages) for a conversation
func (m *MemoryManager) GetContext(userID, conversationID string) []string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// Create session key
	key := createSessionKey(userID, conversationID)

	if context, exists := m.context[key]; exists {
		return context
	}

	return []string{}
}

// ClearContext clears the context for a conversation
func (m *MemoryManager) ClearContext(userID, conversationID string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Create session key
	key := createSessionKey(userID, conversationID)
	delete(m.context, key)
}

// ExtractMemories extracts potential memories from a message
// In a real implementation, this would use NLP or some heuristic to identify
// personal information, preferences, etc.
func (m *MemoryManager) ExtractMemories(userID, conversationID string, userMessage string, botResponse string) {
	// This is a simplified implementation
	// In a real system, you'd use NLP or pattern matching to extract memories
	
	// For now, we'll just add the entire exchange as context
	m.AddContextMessage(userID, conversationID, "User: "+userMessage)
	m.AddContextMessage(userID, conversationID, "Bot: "+botResponse)
}

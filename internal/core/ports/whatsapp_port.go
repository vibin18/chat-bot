package ports

import "context"

// GroupInfo contains information about a WhatsApp group
type GroupInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	MemberCount int    `json:"member_count"`
	IsAllowed   bool   `json:"is_allowed"`
}

// MemoryInfo provides a summary of memories for the admin UI
type MemoryInfo struct {
	ConversationID string    `json:"conversation_id"`
	GroupName      string    `json:"group_name"`
	MemoryCount    int       `json:"memory_count"`
	ContextCount   int       `json:"context_count"`
	LastActivity   string    `json:"last_activity"`
}

// Memory represents a stored piece of information about a conversation
type Memory struct {
	Content     string    `json:"content"`
	CreatedAt   string    `json:"created_at"`
	LastUsed    string    `json:"last_used"`
	UseCount    int       `json:"use_count"`
}

// ConversationDetails provides memory and context details for a conversation
type ConversationDetails struct {
	ConversationID string    `json:"conversation_id"`
	GroupName      string    `json:"group_name"`
	Memories       []Memory  `json:"memories"`
	Context        []string  `json:"context"`
	LastActivity   string    `json:"last_activity"`
}

// UserInfo contains basic information about a user in a conversation
type UserInfo struct {
	UserID      string `json:"user_id"`
	Name        string `json:"name"`
	MemoryCount int    `json:"memory_count"`
}

// UserMemories provides memory and context details for a specific user in a conversation
type UserMemories struct {
	ConversationID string   `json:"conversation_id"`
	GroupName      string   `json:"group_name"`
	UserID         string   `json:"user_id"`
	UserName       string   `json:"user_name"`
	Memories       []Memory `json:"memories"`
	Context        []string `json:"context"`
}

// WhatsAppPort is the interface for WhatsApp integration
type WhatsAppPort interface {
	// Connect establishes the connection to WhatsApp
	Connect(ctx context.Context) error
	
	// Disconnect closes the connection to WhatsApp
	Disconnect() error
	
	// IsConnected checks if the client is connected
	IsConnected() bool
	
	// Start listening for messages
	Start(ctx context.Context) error
	
	// GetGroups gets a list of all available groups
	GetGroups() ([]GroupInfo, error)
	
	// UpdateAllowedGroups updates the allowed groups configuration
	UpdateAllowedGroups(groups []string) error
	
	// Memory admin methods
	
	// GetAllMemoryInfo returns summary info for all conversations with memories
	GetAllMemoryInfo() []MemoryInfo
	
	// GetConversationDetails returns detailed memory and context for a specific conversation
	GetConversationDetails(conversationID string) *ConversationDetails
	
	// GetUsersInConversation returns a list of users in a specific conversation
	GetUsersInConversation(conversationID string) []UserInfo
	
	// GetUserMemories returns memories and context for a specific user in a conversation
	GetUserMemories(conversationID, userID string) *UserMemories
	
	// DeleteMemory deletes a specific memory from a conversation
	DeleteMemory(conversationID string, memoryIndex int) bool
	
	// ClearAllMemories clears all memories for a conversation
	ClearAllMemories(conversationID string) bool
	
	// DeleteContextMessage deletes a specific context message for a user in a conversation
	DeleteContextMessage(conversationID, userID string, index int) bool
	
	// UpdateMemory updates the content of a specific memory
	UpdateMemory(conversationID string, memoryIndex int, newContent string) bool
}

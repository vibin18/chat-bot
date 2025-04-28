package domain

import (
	"time"
)

// Message represents a chat message
type Message struct {
	ID        string    `json:"id"`
	Role      string    `json:"role"` // user or assistant
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// Chat represents a conversation between a user and the LLM
type Chat struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Messages  []Message `json:"messages"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// NewMessage creates a new message
func NewMessage(role, content string) Message {
	return Message{
		ID:        generateID(),
		Role:      role,
		Content:   content,
		CreatedAt: time.Now(),
	}
}

// NewChat creates a new chat
func NewChat(title string) *Chat {
	return &Chat{
		ID:        generateID(),
		Title:     title,
		Messages:  []Message{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// AddMessage adds a message to the chat
func (c *Chat) AddMessage(message Message) {
	c.Messages = append(c.Messages, message)
	c.UpdatedAt = time.Now()
}

// generateID generates a simple ID
func generateID() string {
	return time.Now().Format("20060102150405") + randString(6)
}

// randString generates a random string of length n
func randString(n int) string {
	const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[time.Now().UnixNano()%int64(len(letterBytes))]
		time.Sleep(1 * time.Nanosecond) // Ensure different values
	}
	return string(b)
}

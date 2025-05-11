package database

import (
	"database/sql"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// MemoryDatabase handles the persistence of memory data
type MemoryDatabase struct {
	db    *sql.DB
	mutex sync.RWMutex
}

// Memory represents a stored piece of information about a user
type Memory struct {
	ID            int64
	UserID        string
	ConversationID string
	Content       string
	CreatedAt     time.Time
	LastUsed      time.Time
	UseCount      int
}

// NewMemoryDatabase creates a new memory database
func NewMemoryDatabase() (*MemoryDatabase, error) {
	// Ensure data directory exists
	dataDir := "./data"
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, err
	}

	// Open SQLite database
	dbPath := filepath.Join(dataDir, "memories.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	// Create database schema if it doesn't exist
	if err := createSchema(db); err != nil {
		db.Close()
		return nil, err
	}

	return &MemoryDatabase{
		db:    db,
		mutex: sync.RWMutex{},
	}, nil
}

// createSchema creates the memory table if it doesn't exist
func createSchema(db *sql.DB) error {
	// Create memories table
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS memories (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			conversation_id TEXT NOT NULL,
			content TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL,
			last_used TIMESTAMP NOT NULL,
			use_count INTEGER NOT NULL DEFAULT 0,
			UNIQUE(user_id, conversation_id, content)
		)
	`)
	if err != nil {
		return err
	}

	// Create index for faster lookups
	_, err = db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_memories_user_conversation 
		ON memories(user_id, conversation_id)
	`)
	return err
}

// Close closes the database connection
func (m *MemoryDatabase) Close() error {
	return m.db.Close()
}

// AddMemory adds a new memory to the database or updates an existing one
func (m *MemoryDatabase) AddMemory(memory *Memory) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// If no timestamps are set, set them to current time
	now := time.Now()
	if memory.CreatedAt.IsZero() {
		memory.CreatedAt = now
	}
	if memory.LastUsed.IsZero() {
		memory.LastUsed = now
	}

	// Use INSERT OR REPLACE to handle duplicates
	query := `
		INSERT OR REPLACE INTO memories
		(user_id, conversation_id, content, created_at, last_used, use_count)
		VALUES (?, ?, ?, ?, ?, ?)
	`
	result, err := m.db.Exec(
		query,
		memory.UserID,
		memory.ConversationID,
		memory.Content,
		memory.CreatedAt.Format(time.RFC3339),
		memory.LastUsed.Format(time.RFC3339),
		memory.UseCount,
	)
	if err != nil {
		return err
	}

	// If this was an insert (not a replace), get the ID
	id, err := result.LastInsertId()
	if err == nil && id > 0 {
		memory.ID = id
	}

	return nil
}

// GetMemories returns all memories for a conversation and user
func (m *MemoryDatabase) GetMemories(userID, conversationID string) ([]*Memory, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	query := `
		SELECT id, user_id, conversation_id, content, created_at, last_used, use_count
		FROM memories
		WHERE user_id = ? AND conversation_id = ?
		ORDER BY last_used DESC
	`
	rows, err := m.db.Query(query, userID, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var memories []*Memory
	for rows.Next() {
		var memory Memory
		var createdAtStr, lastUsedStr string

		if err := rows.Scan(
			&memory.ID,
			&memory.UserID,
			&memory.ConversationID,
			&memory.Content,
			&createdAtStr,
			&lastUsedStr,
			&memory.UseCount,
		); err != nil {
			return nil, err
		}

		// Parse timestamps
		if memory.CreatedAt, err = time.Parse(time.RFC3339, createdAtStr); err != nil {
			log.Printf("Error parsing created_at timestamp: %v", err)
		}
		if memory.LastUsed, err = time.Parse(time.RFC3339, lastUsedStr); err != nil {
			log.Printf("Error parsing last_used timestamp: %v", err)
		}

		memories = append(memories, &memory)
	}

	return memories, rows.Err()
}

// GetAllMemoriesByConversation gets all memories for a specific conversation
func (m *MemoryDatabase) GetAllMemoriesByConversation(conversationID string) ([]*Memory, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	query := `
		SELECT id, user_id, conversation_id, content, created_at, last_used, use_count
		FROM memories
		WHERE conversation_id = ?
		ORDER BY user_id, last_used DESC
	`
	rows, err := m.db.Query(query, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var memories []*Memory
	for rows.Next() {
		var memory Memory
		var createdAtStr, lastUsedStr string

		if err := rows.Scan(
			&memory.ID,
			&memory.UserID,
			&memory.ConversationID,
			&memory.Content,
			&createdAtStr,
			&lastUsedStr,
			&memory.UseCount,
		); err != nil {
			return nil, err
		}

		// Parse timestamps
		if memory.CreatedAt, err = time.Parse(time.RFC3339, createdAtStr); err != nil {
			log.Printf("Error parsing created_at timestamp: %v", err)
		}
		if memory.LastUsed, err = time.Parse(time.RFC3339, lastUsedStr); err != nil {
			log.Printf("Error parsing last_used timestamp: %v", err)
		}

		memories = append(memories, &memory)
	}

	return memories, rows.Err()
}

// GetMemoryUsers returns all users who have memories for a specific conversation
func (m *MemoryDatabase) GetMemoryUsers(conversationID string) (map[string]int, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	query := `
		SELECT user_id, COUNT(*) as memory_count
		FROM memories
		WHERE conversation_id = ?
		GROUP BY user_id
	`
	rows, err := m.db.Query(query, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := make(map[string]int)
	for rows.Next() {
		var userID string
		var count int
		if err := rows.Scan(&userID, &count); err != nil {
			return nil, err
		}
		users[userID] = count
	}

	return users, rows.Err()
}

// UpdateMemory updates an existing memory
func (m *MemoryDatabase) UpdateMemory(memory *Memory) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Update the last_used timestamp
	memory.LastUsed = time.Now()

	query := `
		UPDATE memories
		SET content = ?, last_used = ?, use_count = ?
		WHERE id = ?
	`
	_, err := m.db.Exec(
		query,
		memory.Content,
		memory.LastUsed.Format(time.RFC3339),
		memory.UseCount,
		memory.ID,
	)
	return err
}

// DeleteMemory deletes a memory by ID
func (m *MemoryDatabase) DeleteMemory(id int64) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	query := "DELETE FROM memories WHERE id = ?"
	_, err := m.db.Exec(query, id)
	return err
}

// DeleteAllMemoriesForUser deletes all memories for a specific user in a conversation
func (m *MemoryDatabase) DeleteAllMemoriesForUser(userID, conversationID string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	query := "DELETE FROM memories WHERE user_id = ? AND conversation_id = ?"
	_, err := m.db.Exec(query, userID, conversationID)
	return err
}

// GetAllConversationsWithMemories gets a list of all conversation IDs that have memories
func (m *MemoryDatabase) GetAllConversationsWithMemories() ([]string, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	query := `
		SELECT DISTINCT conversation_id
		FROM memories
	`
	rows, err := m.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var conversations []string
	for rows.Next() {
		var conversationID string
		if err := rows.Scan(&conversationID); err != nil {
			return nil, err
		}
		conversations = append(conversations, conversationID)
	}

	return conversations, rows.Err()
}

// GetMemoryCount returns the count of memories for a conversation
func (m *MemoryDatabase) GetMemoryCount(conversationID string) (int, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var count int
	query := "SELECT COUNT(*) FROM memories WHERE conversation_id = ?"
	err := m.db.QueryRow(query, conversationID).Scan(&count)
	return count, err
}

// GetMemoryByID retrieves a memory by its ID
func (m *MemoryDatabase) GetMemoryByID(id int64) (*Memory, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	query := `
		SELECT id, user_id, conversation_id, content, created_at, last_used, use_count
		FROM memories
		WHERE id = ?
	`
	var memory Memory
	var createdAtStr, lastUsedStr string

	err := m.db.QueryRow(query, id).Scan(
		&memory.ID,
		&memory.UserID,
		&memory.ConversationID,
		&memory.Content,
		&createdAtStr,
		&lastUsedStr,
		&memory.UseCount,
	)
	if err != nil {
		return nil, err
	}

	// Parse timestamps
	var parseErr error
	if memory.CreatedAt, parseErr = time.Parse(time.RFC3339, createdAtStr); parseErr != nil {
		log.Printf("Error parsing created_at timestamp: %v", parseErr)
	}
	if memory.LastUsed, parseErr = time.Parse(time.RFC3339, lastUsedStr); parseErr != nil {
		log.Printf("Error parsing last_used timestamp: %v", parseErr)
	}

	return &memory, nil
}

// IncrementUseCount increments the use count for a memory
func (m *MemoryDatabase) IncrementUseCount(id int64) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	now := time.Now()
	query := `
		UPDATE memories
		SET use_count = use_count + 1, last_used = ?
		WHERE id = ?
	`
	_, err := m.db.Exec(query, now.Format(time.RFC3339), id)
	return err
}

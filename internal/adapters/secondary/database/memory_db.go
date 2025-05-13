package database

import (
	"database/sql"
	"fmt"
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
	// Use /app/data in Docker, which maps to the persistent volume in docker-compose.yml
	// This is the directory that gets mounted as a volume in Docker
	dataDir := "/app/data"
	
	// Fallback to local ./data if running outside Docker
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		dataDir = "./data"
		log.Printf("INFO: Docker volume not found, using local data directory: %s", dataDir)
	}
	
	// Ensure data directory exists with proper permissions
	if err := os.MkdirAll(dataDir, 0777); err != nil {
		log.Printf("ERROR: Failed to create data directory: %v", err)
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	// Verify the directory exists and has write permissions
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		log.Printf("ERROR: Data directory does not exist after creation attempt: %v", err)
		return nil, fmt.Errorf("data directory does not exist: %w", err)
	}

	// Log the full path for debugging
	absPath, err := filepath.Abs(dataDir)
	if err == nil {
		log.Printf("INFO: Using data directory: %s", absPath)
	}

	// Open SQLite database with explicit journal mode and synchronous settings
	dbPath := filepath.Join(dataDir, "memories.db")
	log.Printf("INFO: Opening SQLite database at: %s", dbPath)
	
	// Create an empty file if it doesn't exist to ensure file is created with proper permissions
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		log.Printf("INFO: Database file does not exist, creating it at: %s", dbPath)
		emptyFile, err := os.Create(dbPath)
		if err != nil {
			log.Printf("ERROR: Failed to create database file: %v", err)
			return nil, fmt.Errorf("failed to create database file: %w", err)
		}
		emptyFile.Close()
		
		// Set permissions to ensure it's writable
		if err := os.Chmod(dbPath, 0666); err != nil {
			log.Printf("WARN: Failed to set database file permissions: %v", err)
		}
	}
	
	// Use connection parameters that ensure durability
	// Use absolute file URI format that's more reliable across platforms
	dbURI := fmt.Sprintf("file:%s?_journal=WAL&_synchronous=NORMAL&cache=shared", dbPath)
	log.Printf("INFO: Using database connection URI: %s", dbURI)
	
	db, err := sql.Open("sqlite3", dbURI)
	if err != nil {
		log.Printf("ERROR: Failed to open SQLite database: %v", err)
		return nil, fmt.Errorf("failed to open SQLite database: %w", err)
	}

	// Verify we can actually connect to the database
	if err := db.Ping(); err != nil {
		log.Printf("ERROR: Failed to ping SQLite database: %v", err)
		db.Close()
		return nil, fmt.Errorf("failed to ping SQLite database: %w", err)
	}

	// Create database schema if it doesn't exist
	if err := createSchema(db); err != nil {
		log.Printf("ERROR: Failed to create database schema: %v", err)
		db.Close()
		return nil, fmt.Errorf("failed to create database schema: %w", err)
	}
	
	log.Printf("INFO: SQLite memory database initialized successfully")

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

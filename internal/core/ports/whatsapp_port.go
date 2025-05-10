package ports

import "context"

// GroupInfo contains information about a WhatsApp group
type GroupInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	MemberCount int    `json:"member_count"`
	IsAllowed   bool   `json:"is_allowed"`
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
}

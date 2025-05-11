package domain

// MessageType defines the type of a message
type MessageType string

const (
	// MessageTypeText is for standard text messages
	MessageTypeText MessageType = "text"
	
	// MessageTypeImageAnalysis is for messages containing images for analysis
	MessageTypeImageAnalysis MessageType = "image_analysis"
)

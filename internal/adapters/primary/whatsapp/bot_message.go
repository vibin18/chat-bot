package whatsapp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types"
)

// SendGroupMessage sends a message to a WhatsApp group on behalf of the bot
func (a *WhatsAppAdapter) SendGroupMessage(groupID string, message string) error {
	// Check if connected
	if !a.IsConnected() {
		return errors.New("WhatsApp is not connected")
	}

	// Validate the group ID
	if !strings.Contains(groupID, "@g.us") {
		return errors.New("invalid group ID format")
	}

	// Check if the group is in the allowed list
	if !a.isGroupAllowed(groupID) {
		return fmt.Errorf("group ID %s is not in the allowed list", groupID)
	}

	// Format message if needed using the formatter
	formattedMessage := message
	if a.formatter != nil {
		// Custom formatting can be applied here if needed
		// Currently using the message as is
	}

	// Parse the JID
	jid, err := types.ParseJID(groupID)
	if err != nil {
		return fmt.Errorf("failed to parse group JID: %v", err)
	}

	// Create a message
	msg := &waProto.Message{
		Conversation: &formattedMessage,
	}

	// Create a context with timeout
	sendCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Send the message
	_, err = a.client.SendMessage(sendCtx, jid, msg)

	if err != nil {
		return fmt.Errorf("failed to send message: %v", err)
	}

	// Log the sent message
	a.log.Info("Bot message sent to group", "group_id", groupID, "message_length", len(message))

	return nil
}

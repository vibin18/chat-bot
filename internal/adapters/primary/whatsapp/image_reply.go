package whatsapp

import (
	"context"
	"time"

	"google.golang.org/protobuf/proto"
	"go.mau.fi/whatsmeow/types/events"
	waProto "go.mau.fi/whatsmeow/binary/proto"
)

// sendImageAnalysisReply sends an image analysis reply without applying text formatting
// This ensures that the image analysis results are sent as-is without any filtering or special formatting
func (a *WhatsAppAdapter) sendImageAnalysisReply(response string, evt *events.Message) {
	if a.client == nil || !a.client.IsConnected() {
		a.log.Error("WhatsApp client not connected")
		return
	}
	
	// Apply rate limiting
	ctx := context.Background()
	err := a.limiter.Wait(ctx)
	if err != nil {
		a.log.Error("Rate limiter error", "error", err)
		return
	}

	// Use the raw response without any formatting - this is important for image analysis
	// as we want to preserve the full analysis without any limitations
	
	// Log debug information
	a.log.Info("Sending WhatsApp image analysis reply", 
		"chat_jid", evt.Info.Chat.String(),
		"response_length", len(response),
		"sender", evt.Info.Sender.String())
	
	// Create a message with proper reply context for threads
	msg := &waProto.Message{
		ExtendedTextMessage: &waProto.ExtendedTextMessage{
			Text: proto.String(response),
			ContextInfo: &waProto.ContextInfo{
				StanzaID:      proto.String(evt.Info.ID),
				Participant:   proto.String(evt.Info.Sender.String()),
				QuotedMessage: &waProto.Message{
					Conversation: proto.String(evt.Message.GetConversation()),
				},
			},
		},
	}
	
	// Send message
	sendCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	_, err = a.client.SendMessage(sendCtx, evt.Info.Chat, msg)
	if err != nil {
		a.log.Error("Failed to send WhatsApp image analysis reply", "error", err)
		return
	}
	
	a.log.Info("WhatsApp image analysis reply sent successfully")
}

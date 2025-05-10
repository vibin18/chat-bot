package whatsapp

import (
	"strings"

	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types/events"
)

// isReplyToBot checks if a message is a reply to a message sent by the bot
func (a *WhatsAppAdapter) isReplyToBot(evt *events.Message) bool {
	// Check if the message has context info indicating it's a reply
	contextInfo := a.getMessageContextInfo(evt)
	if contextInfo == nil {
		return false
	}

	// Check if the message being replied to was from our JID
	if contextInfo.Participant != nil {
		// Get our user ID (phone number) without the device part
		ourUserID := a.client.Store.ID.User
		replyToParticipant := *contextInfo.Participant
		
		// Log for debugging
		a.log.Info("Reply participant check", 
			"our_user_id", ourUserID,
			"reply_to", replyToParticipant,
			"full_jid", a.client.Store.ID.String())
			
		// Compare just the user ID portion (phone number) to handle different JID formats
		if strings.Contains(replyToParticipant, ourUserID) {
			a.log.Info("Detected valid reply to bot")
			return true
		}
	}
	
	return false
}

// getMessageContextInfo extracts context info from a message to determine if it's a reply
func (a *WhatsAppAdapter) getMessageContextInfo(evt *events.Message) *waProto.ContextInfo {
	if evt.Message == nil {
		return nil
	}

	// Check various message types that could contain context info
	if ext := evt.Message.GetExtendedTextMessage(); ext != nil {
		return ext.ContextInfo
	}
	
	if img := evt.Message.GetImageMessage(); img != nil {
		return img.ContextInfo
	}
	
	if vid := evt.Message.GetVideoMessage(); vid != nil {
		return vid.ContextInfo
	}
	
	if aud := evt.Message.GetAudioMessage(); aud != nil {
		return aud.ContextInfo
	}
	
	if doc := evt.Message.GetDocumentMessage(); doc != nil {
		return doc.ContextInfo
	}
	
	// Try other message types as needed
	return nil
}

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

	// Check for StanzaID - this is the most reliable method for replies in groups
	if contextInfo.StanzaID != nil && contextInfo.Participant != nil {
		// If we have a message ID stored in our sent messages, this would be a robust check
		// For now, just log this information
		a.log.Info("Reply stanza check", 
			"stanza_id", *contextInfo.StanzaID,
			"participant", *contextInfo.Participant)
	}

	// Check if it's a reply to one of our messages
	if contextInfo.QuotedMessage != nil {
		// This means it's definitely a reply to something
		a.log.Info("Quoted message detected")

		// Now check who sent the original message
		isFromBot := false
		
		// First, look for our ID in the Quoted message
		if contextInfo.Participant != nil {
			// Method 1: Extract our user ID (phone number) without the device part
			ourUserID := a.client.Store.ID.User
			replyToParticipant := *contextInfo.Participant
			
			// Log for debugging
			a.log.Info("Reply participant check", 
				"our_user_id", ourUserID,
				"reply_to", replyToParticipant,
				"full_jid", a.client.Store.ID.String())
			
			// Method 2: Check for any form of the bot's JID
			if strings.Contains(replyToParticipant, ourUserID) {
				isFromBot = true
			}
			
			// Method 3: Check if the first part of the JID matches (before the @)
			parts := strings.Split(replyToParticipant, "@")
			ourParts := strings.Split(a.client.Store.ID.String(), "@")
			if len(parts) > 0 && len(ourParts) > 0 {
				// Sometimes there's a device ID after the phone number, so compare just the number
				ourPhone := strings.Split(ourParts[0], ":")
				replyPhone := strings.Split(parts[0], ":")
				
				if len(ourPhone) > 0 && len(replyPhone) > 0 && ourPhone[0] == replyPhone[0] {
					isFromBot = true
				}
			}
		}
		
		// For now, assume any reply in our allowed groups is intended for the bot
		// This assumption might be too broad for some use cases
		groupJID := evt.Info.Chat.String()
		if a.isGroupAllowed(groupJID) {
			a.log.Info("Treating reply in allowed group as bot reply")
			isFromBot = true
		}
		
		if isFromBot {
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

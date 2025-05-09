package whatsapp

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/mdp/qrterminal/v3"
	"github.com/vibin/chat-bot/config"
	"github.com/vibin/chat-bot/internal/core/services"
	"github.com/vibin/chat-bot/internal/logger"
	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"golang.org/x/time/rate"
	"google.golang.org/protobuf/proto"

	_ "github.com/mattn/go-sqlite3"
)

// WhatsAppAdapter implements the WhatsApp adapter and the ports.WhatsAppPort interface
type WhatsAppAdapter struct {
	client       *whatsmeow.Client
	store        *store.Device
	storeDir     string
	chatService  *services.ChatService
	log          logger.Logger
	config       *config.WhatsAppConfig
	conversations map[string]*Conversation
	mutex        sync.RWMutex
	limiter      *rate.Limiter // Rate limiter for WhatsApp API calls
}

// Conversation represents an active conversation
type Conversation struct {
	ID        string
	GroupID   string
	GroupName string
	Messages  []string
	LastActivity time.Time
}

// NewWhatsAppAdapter creates a new WhatsApp adapter
func NewWhatsAppAdapter(chatService *services.ChatService, config *config.Config, logger logger.Logger) (*WhatsAppAdapter, error) {
	// Ensure store directory exists
	if _, err := os.Stat(config.WhatsApp.StoreDir); os.IsNotExist(err) {
		err := os.MkdirAll(config.WhatsApp.StoreDir, 0755)
		if err != nil {
			return nil, fmt.Errorf("failed to create WhatsApp store directory: %v", err)
		}
	}

	// Create rate limiter: 60 messages per minute (respecting WhatsApp limits)
	limiter := rate.NewLimiter(rate.Every(time.Second), 10) // 10 burst, 1 per second

	adapter := &WhatsAppAdapter{
		storeDir:     config.WhatsApp.StoreDir,
		chatService:  chatService,
		log:          logger,
		config:       &config.WhatsApp,
		conversations: make(map[string]*Conversation),
		limiter:      limiter,
	}

	return adapter, nil
}

// Connect establishes the connection to WhatsApp
func (a *WhatsAppAdapter) Connect(ctx context.Context) error {
	// Set up the database store for WhatsApp
	dbLog := waLog.Stdout("Database", "DEBUG", true)
	container, err := sqlstore.New("sqlite3", fmt.Sprintf("file:%s/whatsmeow.db?_foreign_keys=on", a.storeDir), dbLog)
	if err != nil {
		return fmt.Errorf("failed to initialize WhatsApp database: %v", err)
	}

	// Get device store
	deviceStore, err := container.GetFirstDevice()
	if err != nil {
		return fmt.Errorf("failed to get device store: %v", err)
	}
	a.store = deviceStore

	// Create WhatsApp client
	clientLog := waLog.Stdout("Client", "INFO", true)
	a.client = whatsmeow.NewClient(deviceStore, clientLog)
	a.client.AddEventHandler(a.eventHandler)

	// Check if we have a stored session
	if a.client.Store.ID == nil {
		// No session found, need to pair and scan QR code
		qrChan, err := a.client.GetQRChannel(ctx)
		if err != nil {
			return fmt.Errorf("error getting QR channel: %v", err)
		}
		
		err = a.client.Connect()
		if err != nil {
			return fmt.Errorf("error connecting to WhatsApp: %v", err)
		}
		
		for evt := range qrChan {
			if evt.Event == "code" {
				// Print the QR code to the console
				qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
				a.log.Info("Scan the QR code with your WhatsApp app")
			} else {
				a.log.Info("QR channel event", "event", evt.Event)
			}
		}
	} else {
		// Session already exists, just connect
		err = a.client.Connect()
		if err != nil {
			return fmt.Errorf("error connecting to WhatsApp: %v", err)
		}
		a.log.Info("Connected to WhatsApp")
	}

	return nil
}

// Disconnect closes the connection to WhatsApp
func (a *WhatsAppAdapter) Disconnect() error {
	if a.client != nil {
		a.client.Disconnect()
		a.log.Info("Disconnected from WhatsApp")
	}
	return nil
}

// IsConnected checks if the client is connected
func (a *WhatsAppAdapter) IsConnected() bool {
	return a.client != nil && a.client.IsConnected()
}

// Start starts listening for messages
func (a *WhatsAppAdapter) Start(ctx context.Context) error {
	a.log.Info("WhatsApp adapter is starting")
	
	if !a.IsConnected() {
		if err := a.Connect(ctx); err != nil {
			return err
		}
	}

	<-ctx.Done()
	return nil
}

// eventHandler handles WhatsApp events
func (a *WhatsAppAdapter) eventHandler(rawEvt interface{}) {
	switch evt := rawEvt.(type) {
	case *events.Message:
		a.handleMessage(evt)
	case *events.Connected:
		a.log.Info("WhatsApp connected")
	case *events.Disconnected:
		a.log.Info("WhatsApp disconnected")
	case *events.LoggedOut:
		a.log.Warn("WhatsApp logged out")
		// Handle logout by clearing the device store
		if a.store != nil {
			err := a.store.Delete()
			if err != nil {
				a.log.Error("Failed to delete device store on logout", "error", err)
			}
		}
	}
}

// handleMessage processes incoming WhatsApp messages
func (a *WhatsAppAdapter) handleMessage(evt *events.Message) {
	// Skip direct messages as requested
	if !evt.Info.IsGroup {
		return
	}

	// Get group ID as string
	groupJID := evt.Info.Chat.String()

	// Check if this group is in allowed groups list
	if !a.isGroupAllowed(groupJID) {
		return
	}

	// Get message content
	message := a.getMessageText(evt)
	if message == "" {
		return
	}

	// Get the conversation ID
	conversationID := a.getOrCreateConversation(evt)
	
	// Check if this is a reply to our bot's message
	isReplyToBot := a.isReplyToBot(evt)
	
	// Process if it's a mention with trigger word OR it's a direct reply to the bot
	isMention := strings.Contains(strings.ToLower(message), strings.ToLower(a.config.TriggerWord))
	
	if !isMention && !isReplyToBot {
		return
	}

	a.log.Info("Received WhatsApp message", 
		"group", groupJID, 
		"message", message, 
		"is_reply", isReplyToBot, 
		"is_mention", isMention)

	// Clean the message by removing the trigger word if present
	cleanMessage := message
	if isMention {
		cleanMessage = strings.ReplaceAll(
			strings.ToLower(message), 
			strings.ToLower(a.config.TriggerWord), 
			"",
		)
	}
	cleanMessage = strings.TrimSpace(cleanMessage)
	
	// Generate response asynchronously
	go a.processAndReply(conversationID, cleanMessage, evt)
}

// processAndReply processes a message and sends a reply
func (a *WhatsAppAdapter) processAndReply(conversationID string, message string, evt *events.Message) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Record in conversation history
	a.recordMessage(conversationID, fmt.Sprintf("User: %s", message))

	// Create a chat if it doesn't exist
	chat, err := a.chatService.GetChat(ctx, conversationID)
	if err != nil {
		// Create a new chat if it doesn't exist
		chatName := fmt.Sprintf("WhatsApp: %s", conversationID)
		chat, err = a.chatService.CreateChat(ctx, chatName)
		if err != nil {
			a.log.Error("Failed to create chat", "error", err)
			return
		}
	}
	
	// Send message through the chat service
	updatedChat, err := a.chatService.SendMessage(ctx, chat.ID, message)
	if err != nil {
		a.log.Error("Failed to process message", "error", err)
		return
	}
	
	// Get the response (last message from the assistant)
	var response string
	if len(updatedChat.Messages) > 0 {
		for i := len(updatedChat.Messages) - 1; i >= 0; i-- {
			msg := updatedChat.Messages[i]
			if msg.Role == "assistant" {
				response = msg.Content
				break
			}
		}
	}
	
	if response == "" {
		a.log.Error("No response generated")
		return
	}
	
	// Record the response in our conversation
	a.recordMessage(conversationID, fmt.Sprintf("Bot: %s", response))
	
	// Send the response
	a.sendReply(response, evt)
}

// recordMessage adds a message to the conversation history
func (a *WhatsAppAdapter) recordMessage(conversationID string, message string) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	conv, exists := a.conversations[conversationID]
	if !exists {
		return
	}

	// Add message to history
	conv.Messages = append(conv.Messages, message)
	
	// Update last activity
	conv.LastActivity = time.Now()
	
	// Limit history to last 50 messages
	if len(conv.Messages) > 50 {
		conv.Messages = conv.Messages[len(conv.Messages)-50:]
	}
}

// getOrCreateConversation gets or creates a conversation for tracking context
func (a *WhatsAppAdapter) getOrCreateConversation(evt *events.Message) string {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	
	groupJID := evt.Info.Chat.String()
	
	// Generate a conversation ID
	conversationID := fmt.Sprintf("whatsapp-%s", groupJID)
	
	// Check if conversation exists
	_, exists := a.conversations[conversationID]
	if !exists {
		// Create new conversation
		a.conversations[conversationID] = &Conversation{
			ID:        conversationID,
			GroupID:   groupJID,
			GroupName: a.getGroupName(evt.Info.Chat),
			Messages:  []string{},
			LastActivity: time.Now(),
		}
	}
	
	return conversationID
}

// getGroupName tries to get a readable group name
func (a *WhatsAppAdapter) getGroupName(jid types.JID) string {
	if a.client == nil {
		return jid.User
	}
	
	groupInfo, err := a.client.GetGroupInfo(jid)
	if err != nil {
		return jid.User
	}
	
	return groupInfo.Name
}

// isGroupAllowed checks if the group is in the allowed list
func (a *WhatsAppAdapter) isGroupAllowed(groupJID string) bool {
	// If no allowed groups configured, don't allow any
	if len(a.config.AllowedGroups) == 0 {
		return false
	}
	
	// If * is in allowed groups, allow all groups
	for _, allowed := range a.config.AllowedGroups {
		if allowed == "*" {
			return true
		}
	}
	
	// Check if this specific group is allowed
	for _, allowed := range a.config.AllowedGroups {
		if strings.Contains(groupJID, allowed) {
			return true
		}
	}
	
	return false
}

// getMessageText extracts text from the message
func (a *WhatsAppAdapter) getMessageText(evt *events.Message) string {
	if evt.Message.GetConversation() != "" {
		return evt.Message.GetConversation()
	}
	
	if evt.Message.GetExtendedTextMessage() != nil {
		return evt.Message.GetExtendedTextMessage().GetText()
	}
	
	return ""
}

// sendReply sends a reply to the message, respecting rate limits
func (a *WhatsAppAdapter) sendReply(response string, evt *events.Message) {
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

	// Log debug information
	a.log.Info("Sending WhatsApp reply", 
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
		a.log.Error("Failed to send WhatsApp reply", "error", err)
		return
	}
	
	a.log.Info("WhatsApp reply sent successfully")
}

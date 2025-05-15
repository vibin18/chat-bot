package whatsapp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"go.mau.fi/whatsmeow/types/events"
)

// FamilyResponse represents the response structure from the family webhook
type FamilyResponse struct {
	Response string `json:"response"`
}

// isFamilyRequest checks if a message is a family request
func (a *WhatsAppAdapter) isFamilyRequest(message string) bool {
	// Check for "@family" keyword in the message
	return strings.Contains(strings.ToLower(message), "@family")
}

// processAndReplyWithFamilyHandler forwards the message to the family webhook and sends back the response
func (a *WhatsAppAdapter) processAndReplyWithFamilyHandler(conversationID string, message string, evt *events.Message) {
	a.log.Info("Processing family request", "conversation_id", conversationID)
	
	// Prepare the request payload with the specified structure
	requestBody, err := json.Marshal(map[string]string{
		"action": "sendMessage",
		"chatInput": message,
	})
	if err != nil {
		a.log.Error("Failed to marshal family request", "error", err)
		a.sendReply("Sorry, there was an error processing your family request.", evt)
		return
	}
	
	// Send request to the family webhook
	client := &http.Client{
		Timeout: time.Second * 30,
	}
	
	webhookURL := "http://192.168.1.132:5678/webhook/f65ba2b8-582c-4575-b4b9-02b26edc3ea0/chat"
	resp, err := client.Post(
		webhookURL,
		"application/json",
		bytes.NewBuffer(requestBody),
	)
	
	if err != nil {
		a.log.Error("Failed to send family webhook request", "error", err)
		a.sendReply("Sorry, I couldn't connect to the family service. Please try again later.", evt)
		return
	}
	defer resp.Body.Close()
	
	// Read and process the response
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		a.log.Error("Failed to read family webhook response", "error", err)
		a.sendReply("Sorry, I couldn't read the response from the family service.", evt)
		return
	}
	
	if resp.StatusCode != http.StatusOK {
		a.log.Error("Family webhook returned error", "status", resp.StatusCode, "body", string(body))
		a.sendReply(fmt.Sprintf("The family service returned an error: %d", resp.StatusCode), evt)
		return
	}
	
	// Parse the response
	var familyResponse FamilyResponse
	if err := json.Unmarshal(body, &familyResponse); err != nil {
		a.log.Error("Failed to unmarshal family webhook response", "error", err)
		a.sendReply("Sorry, I couldn't understand the response from the family service.", evt)
		return
	}
	
	// Record the message in conversation history
	a.recordMessage(conversationID, fmt.Sprintf("User: %s", message))
	a.recordMessage(conversationID, fmt.Sprintf("Bot: %s", familyResponse.Response))
	
	// Send the response from the family webhook
	a.sendReply(familyResponse.Response, evt)
}

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

// WebSearchResponse represents the response structure from the web search service
type WebSearchResponse struct {
	WhatsAppMessage string `json:"whatsapp_message"`
	Topic           string `json:"topic"`
	SearchTime      string `json:"search_time"`
}

// isWebRequest checks if a message is a web search request
func (a *WhatsAppAdapter) isWebRequest(message string) bool {
	// First check if web service is enabled in config
	if !a.config.WebService.Enabled {
		a.log.Info("Web service is disabled in config")
		return false
	}
	
	// Check for "@web" keyword in the message
	isWeb := strings.Contains(strings.ToLower(message), "@web")
	
	// Also make sure it contains at least one trigger word (like @sasi)
	hasTriggerWord := false
	for _, triggerWord := range a.config.TriggerWords {
		if strings.Contains(strings.ToLower(message), strings.ToLower(triggerWord)) {
			hasTriggerWord = true
			break
		}
	}
	
	a.log.Info("Checking for @web keyword", "message", message, "is_web_request", isWeb && hasTriggerWord)
	return isWeb && hasTriggerWord
}

// processAndReplyWithWebHandler forwards the message to the web search service and sends back the response
func (a *WhatsAppAdapter) processAndReplyWithWebHandler(conversationID string, message string, evt *events.Message) {
	a.log.Info("Processing web search request", "conversation_id", conversationID)
	
	// Log the input message for debugging
	a.log.Info("Web search request message (original)", "message", message)
	
	// Clean the message by removing trigger words and @web
	cleanMessage := message
	
	// Remove all trigger words first
	for _, triggerWord := range a.config.TriggerWords {
		cleanMessage = strings.ReplaceAll(
			strings.ToLower(cleanMessage),
			strings.ToLower(triggerWord),
			"",
		)
	}
	
	// Also remove the @web keyword
	cleanMessage = strings.ReplaceAll(
		strings.ToLower(cleanMessage),
		"@web",
		"",
	)
	
	// Trim any extra whitespace
	cleanMessage = strings.TrimSpace(cleanMessage)
	a.log.Info("Web search request message (cleaned)", "message", cleanMessage)
	
	// Prepare the request payload
	requestBody, err := json.Marshal(map[string]string{
		"query": cleanMessage,
	})
	if err != nil {
		a.log.Error("Failed to marshal web search request", "error", err)
		a.sendReply("Sorry, there was an error processing your web search request.", evt)
		return
	}
	
	// Send request to the web search service using configuration values
	client := &http.Client{
		Timeout: time.Second * a.config.WebService.TimeoutSeconds,
	}
	
	webhookURL := a.config.WebService.WebhookURL
	resp, err := client.Post(
		webhookURL,
		"application/json",
		bytes.NewBuffer(requestBody),
	)
	
	if err != nil {
		a.log.Error("Failed to send web search request", "error", err)
		a.sendReply("Sorry, I couldn't connect to the web search service. Please try again later.", evt)
		return
	}
	defer resp.Body.Close()
	
	// Read and process the response
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		a.log.Error("Failed to read web search response", "error", err)
		a.sendReply("Sorry, I couldn't read the response from the web search service.", evt)
		return
	}
	
	if resp.StatusCode != http.StatusOK {
		a.log.Error("Web search service returned error", "status", resp.StatusCode, "body", string(body))
		a.sendReply(fmt.Sprintf("The web search service returned an error: %d", resp.StatusCode), evt)
		return
	}
	
	// Log the raw response for debugging
	a.log.Info("Web search raw response", "body", string(body))
	
	// Parse the response
	var webResponse WebSearchResponse
	if err := json.Unmarshal(body, &webResponse); err != nil {
		a.log.Error("Failed to parse web search response", "error", err)
		a.sendReply("Sorry, I couldn't understand the response from the web search service.", evt)
		return
	}
	
	// Extract the WhatsApp message from the response
	response := webResponse.WhatsAppMessage
	
	if response == "" {
		a.log.Error("Web search response did not contain a valid WhatsApp message")
		a.sendReply("Sorry, the web search service did not return a valid response.", evt)
		return
	}
	
	// Log the extracted message
	a.log.Info("Web search formatted response", "message", response)
	
	// Record the message in conversation history
	a.recordMessage(conversationID, fmt.Sprintf("User: %s", message))
	a.recordMessage(conversationID, fmt.Sprintf("Bot: %s", response))
	
	// Send the response from the web search service
	a.sendReply(response, evt)
}

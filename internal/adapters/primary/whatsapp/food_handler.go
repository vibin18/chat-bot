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

// FoodResponse represents the response structure from the food webhook
type FoodResponse struct {
	Test string `json:"test"`
}

// We need to be flexible with response parsing since the format may vary

// isFoodRequest checks if a message is a food request
func (a *WhatsAppAdapter) isFoodRequest(message string) bool {
	// First check if food service is enabled in config
	if !a.config.FoodService.Enabled {
		a.log.Info("Food service is disabled in config")
		return false
	}
	
	// Check for "@food" keyword in the message
	isFood := strings.Contains(strings.ToLower(message), "@food")
	a.log.Info("Checking for @food keyword", "message", message, "is_food_request", isFood)
	return isFood
}

// processAndReplyWithFoodHandler forwards the message to the food webhook and sends back the response
func (a *WhatsAppAdapter) processAndReplyWithFoodHandler(conversationID string, message string, evt *events.Message) {
	a.log.Info("Processing food request", "conversation_id", conversationID)
	
	// Log the input message for debugging
	a.log.Info("Food request message (original)", "message", message)
	
	// Clean the message by removing trigger words
	cleanMessage := message
	
	// Remove all trigger words first
	for _, triggerWord := range a.config.TriggerWords {
		cleanMessage = strings.ReplaceAll(
			strings.ToLower(cleanMessage),
			strings.ToLower(triggerWord),
			"",
		)
	}
	
	// Also remove the @food keyword
	cleanMessage = strings.ReplaceAll(
		strings.ToLower(cleanMessage),
		"@food",
		"",
	)
	
	// Trim any extra whitespace
	cleanMessage = strings.TrimSpace(cleanMessage)
	a.log.Info("Food request message (cleaned)", "message", cleanMessage)
	
	// Prepare the request payload with the appropriate structure
	requestBody, err := json.Marshal(map[string]string{
		"action": "sendMessage",
		"chatInput": cleanMessage,
	})
	if err != nil {
		a.log.Error("Failed to marshal food request", "error", err)
		a.sendReply("Sorry, there was an error processing your food request.", evt)
		return
	}
	
	// Send request to the food webhook using configuration values
	client := &http.Client{
		Timeout: time.Second * a.config.FoodService.TimeoutSeconds,
	}
	
	webhookURL := a.config.FoodService.WebhookURL
	resp, err := client.Post(
		webhookURL,
		"application/json",
		bytes.NewBuffer(requestBody),
	)
	
	if err != nil {
		a.log.Error("Failed to send food webhook request", "error", err)
		a.sendReply("Sorry, I couldn't connect to the food service. Please try again later.", evt)
		return
	}
	defer resp.Body.Close()
	
	// Read and process the response
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		a.log.Error("Failed to read food webhook response", "error", err)
		a.sendReply("Sorry, I couldn't read the response from the food service.", evt)
		return
	}
	
	if resp.StatusCode != http.StatusOK {
		a.log.Error("Food webhook returned error", "status", resp.StatusCode, "body", string(body))
		a.sendReply(fmt.Sprintf("The food service returned an error: %d", resp.StatusCode), evt)
		return
	}
	
	// Log the raw response for debugging
	a.log.Info("Food webhook raw response", "body", string(body))
	
	// Try to parse the response as different formats
	var response string
	
	// First attempt: try to parse as object with 'test' field
	var foodResponse FoodResponse
	if err := json.Unmarshal(body, &foodResponse); err == nil && foodResponse.Test != "" {
		response = foodResponse.Test
		a.log.Info("Successfully parsed food response as Test object", "response", response)
	} else {
		// Second attempt: try to parse as a simple string
		var rawString string
		if err := json.Unmarshal(body, &rawString); err == nil {
			response = rawString
			a.log.Info("Successfully parsed food response as string", "response", response)
		} else {
			// Third attempt: try to parse as a map
			var mapResponse map[string]interface{}
			if err := json.Unmarshal(body, &mapResponse); err == nil {
				// Convert the map to a string representation
				resBytes, _ := json.MarshalIndent(mapResponse, "", "  ")
				response = string(resBytes)
				a.log.Info("Successfully parsed food response as map", "response", response)
			} else {
				// Fall back to just using the raw response body
				response = string(body)
				a.log.Info("Using raw food response", "response", response)
			}
		}
	}
	
	// Record the message in conversation history
	a.recordMessage(conversationID, fmt.Sprintf("User: %s", message))
	a.recordMessage(conversationID, fmt.Sprintf("Bot: %s", response))
	
	// Send the response from the food webhook
	a.sendReply(response, evt)
}

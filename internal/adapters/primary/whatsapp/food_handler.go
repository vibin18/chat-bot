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
	a.log.Info("Food request message", "message", message)
	
	// Prepare the request payload with the appropriate structure
	requestBody, err := json.Marshal(map[string]string{
		"action": "sendMessage",
		"chatInput": message,
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
	
	// Parse the response
	var foodResponse FoodResponse
	if err := json.Unmarshal(body, &foodResponse); err != nil {
		a.log.Error("Failed to unmarshal food webhook response", "error", err)
		a.sendReply("Sorry, I couldn't understand the response from the food service.", evt)
		return
	}
	
	// Record the message in conversation history
	a.recordMessage(conversationID, fmt.Sprintf("User: %s", message))
	a.recordMessage(conversationID, fmt.Sprintf("Bot: %s", foodResponse.Test))
	
	// Send the response from the food webhook
	a.sendReply(foodResponse.Test, evt)
}

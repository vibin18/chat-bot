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
type FoodArrayItem struct {
	Text string `json:"text"`
}

// We expect an array of FoodArrayItem objects

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
	
	// Parse the response as an array of objects with text field
	var response string
	
	// Try to parse as array of objects with 'text' field
	var foodArray []FoodArrayItem
	if err := json.Unmarshal(body, &foodArray); err == nil && len(foodArray) > 0 && foodArray[0].Text != "" {
		// Extract just the text value from the first item
		response = foodArray[0].Text
		a.log.Info("Successfully parsed food response as array with text field", "response", response)
	} else {
		// Fallback: try other formats
		a.log.Error("Failed to parse food response as expected array format", "error", err)
		
		// Try to parse as a simple array without specific structure
		var genericArray []interface{}
		if err := json.Unmarshal(body, &genericArray); err == nil && len(genericArray) > 0 {
			// Try to get text from first item if it's a map
			if firstItem, ok := genericArray[0].(map[string]interface{}); ok {
				if textValue, ok := firstItem["text"].(string); ok {
					response = textValue
					a.log.Info("Successfully parsed food response from generic array", "response", response)
				} else {
					// Just convert the first item to string
					resBytes, _ := json.Marshal(firstItem)
					response = string(resBytes)
					a.log.Info("Using first array item as response", "response", response)
				}
			} else {
				// Just use the first item directly
				resBytes, _ := json.Marshal(genericArray[0])
				response = string(resBytes)
				a.log.Info("Using first array item as string", "response", response)
			}
		} else {
			// Last resort: just use the raw response
			response = string(body)
			a.log.Info("Using raw food response", "response", response)
		}
	}
	
	// Record the message in conversation history
	a.recordMessage(conversationID, fmt.Sprintf("User: %s", message))
	a.recordMessage(conversationID, fmt.Sprintf("Bot: %s", response))
	
	// Send the response from the food webhook
	a.sendReply(response, evt)
}

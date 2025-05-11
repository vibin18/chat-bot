package http

import (
	"encoding/json"
	"net/http"
)

// BotAdminPage serves the bot admin UI
func (h *Handler) BotAdminPage(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./web/templates/bot_admin.html")
}

// SendMessageRequest represents the request structure for sending a message as the bot
type SendMessageRequest struct {
	GroupID string `json:"group_id"`
	Message string `json:"message"`
}

// handleSendBotMessage handles requests to send a message on behalf of the bot
func (h *Handler) handleSendBotMessage(w http.ResponseWriter, r *http.Request) {
	// Check if WhatsApp is connected
	if !h.whatsappAdapter.IsConnected() {
		h.respondWithError(w, http.StatusServiceUnavailable, "WhatsApp is not connected")
		return
	}

	// Parse request body
	var request SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	// Validate request
	if request.GroupID == "" {
		h.respondWithError(w, http.StatusBadRequest, "Group ID is required")
		return
	}

	if request.Message == "" {
		h.respondWithError(w, http.StatusBadRequest, "Message is required")
		return
	}

	// Send the message
	err := h.whatsappAdapter.SendGroupMessage(request.GroupID, request.Message)
	if err != nil {
		h.respondWithError(w, http.StatusInternalServerError, "Failed to send message: "+err.Error())
		return
	}

	// Respond with success
	h.respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Message sent successfully",
	})
}

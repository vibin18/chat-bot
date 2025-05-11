package http

import (
	"encoding/json"
	"net/http"
)

// AddMemoryRequest represents the request structure for adding a memory
type AddMemoryRequest struct {
	ConversationID string `json:"conversation_id"`
	UserID         string `json:"user_id"`
	Content        string `json:"content"`
}

// handleAddMemory adds a new memory for a specific user in a conversation
func (h *Handler) handleAddMemory(w http.ResponseWriter, r *http.Request) {
	// Check if connected
	if !h.whatsappAdapter.IsConnected() {
		h.respondWithError(w, http.StatusServiceUnavailable, "WhatsApp is not connected")
		return
	}
	
	// Parse request
	var requestData AddMemoryRequest
	err := json.NewDecoder(r.Body).Decode(&requestData)
	if err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	
	// Validate request
	if requestData.ConversationID == "" {
		h.respondWithError(w, http.StatusBadRequest, "Missing conversation ID")
		return
	}
	
	if requestData.UserID == "" {
		h.respondWithError(w, http.StatusBadRequest, "Missing user ID")
		return
	}
	
	if requestData.Content == "" {
		h.respondWithError(w, http.StatusBadRequest, "Missing content")
		return
	}
	
	// Add memory through the adapter
	success := h.whatsappAdapter.AddMemory(requestData.ConversationID, requestData.UserID, requestData.Content)
	if !success {
		h.respondWithError(w, http.StatusInternalServerError, "Failed to add memory")
		return
	}
	
	// Return success
	h.respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Memory added successfully",
	})
}

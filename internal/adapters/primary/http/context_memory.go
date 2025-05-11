package http

import (
	"encoding/json"
	"net/http"
)

// ContextDeleteRequest represents a request to delete a context message
type ContextDeleteRequest struct {
	ConversationID string `json:"conversation_id"`
	UserID         string `json:"user_id"`
	Index          int    `json:"index"`
}

// handleDeleteContextMessage handles requests to delete a specific context message
func (h *Handler) handleDeleteContextMessage(w http.ResponseWriter, r *http.Request) {
	// Check if connected
	if !h.whatsappAdapter.IsConnected() {
		h.respondWithError(w, http.StatusServiceUnavailable, "WhatsApp is not connected")
		return
	}
	
	// Parse request body
	var req ContextDeleteRequest
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&req); err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	defer r.Body.Close()
	
	// Validate request
	if req.ConversationID == "" {
		h.respondWithError(w, http.StatusBadRequest, "Missing conversation ID")
		return
	}
	
	if req.UserID == "" {
		h.respondWithError(w, http.StatusBadRequest, "Missing user ID")
		return
	}
	
	if req.Index < 0 {
		h.respondWithError(w, http.StatusBadRequest, "Invalid index")
		return
	}
	
	// Delete the context message
	success := h.whatsappAdapter.DeleteContextMessage(req.ConversationID, req.UserID, req.Index)
	if !success {
		h.respondWithError(w, http.StatusInternalServerError, "Failed to delete context message")
		return
	}
	
	// Respond with success
	h.respondWithJSON(w, http.StatusOK, map[string]bool{"success": true})
}

package http

import (
	"net/http"
)

// handleGetUsersInConversation returns a list of users in a specific conversation
func (h *Handler) handleGetUsersInConversation(w http.ResponseWriter, r *http.Request) {
	// Check if connected
	if !h.whatsappAdapter.IsConnected() {
		h.respondWithError(w, http.StatusServiceUnavailable, "WhatsApp is not connected")
		return
	}
	
	// Get conversation ID from query parameter
	conversationID := r.URL.Query().Get("conversation_id")
	if conversationID == "" {
		h.respondWithError(w, http.StatusBadRequest, "Missing conversation ID")
		return
	}
	
	// Get users for the conversation
	users := h.whatsappAdapter.GetUsersInConversation(conversationID)
	if users == nil {
		h.respondWithError(w, http.StatusNotFound, "Conversation not found")
		return
	}
	
	h.respondWithJSON(w, http.StatusOK, users)
}

// handleGetUserMemories returns memories and context for a specific user in a conversation
func (h *Handler) handleGetUserMemories(w http.ResponseWriter, r *http.Request) {
	// Check if connected
	if !h.whatsappAdapter.IsConnected() {
		h.respondWithError(w, http.StatusServiceUnavailable, "WhatsApp is not connected")
		return
	}
	
	// Get conversation ID and user ID from query parameters
	conversationID := r.URL.Query().Get("conversation_id")
	userID := r.URL.Query().Get("user_id")
	
	if conversationID == "" {
		h.respondWithError(w, http.StatusBadRequest, "Missing conversation ID")
		return
	}
	
	if userID == "" {
		h.respondWithError(w, http.StatusBadRequest, "Missing user ID")
		return
	}
	
	// Get user memories
	userMemories := h.whatsappAdapter.GetUserMemories(conversationID, userID)
	if userMemories == nil {
		h.respondWithError(w, http.StatusNotFound, "User or conversation not found")
		return
	}
	
	h.respondWithJSON(w, http.StatusOK, userMemories)
}

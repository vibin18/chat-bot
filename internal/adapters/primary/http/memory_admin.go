package http

import (
	"encoding/json"
	"net/http"
)

// handleGetAllMemories returns summary information for all conversations with memories
func (h *Handler) handleGetAllMemories(w http.ResponseWriter, r *http.Request) {
	// Check if connected
	if !h.whatsappAdapter.IsConnected() {
		h.respondWithError(w, http.StatusServiceUnavailable, "WhatsApp is not connected")
		return
	}
	
	// Call the adapter method
	memoryInfo := h.whatsappAdapter.GetAllMemoryInfo()
	h.respondWithJSON(w, http.StatusOK, memoryInfo)
}

// handleGetConversationMemory returns detailed memory and context for a specific conversation
func (h *Handler) handleGetConversationMemory(w http.ResponseWriter, r *http.Request) {
	// Check if connected
	if !h.whatsappAdapter.IsConnected() {
		h.respondWithError(w, http.StatusServiceUnavailable, "WhatsApp is not connected")
		return
	}
	
	// Get conversation ID from query parameter
	conversationID := r.URL.Query().Get("id")
	if conversationID == "" {
		h.respondWithError(w, http.StatusBadRequest, "Missing conversation ID")
		return
	}
	
	// Get conversation details
	details := h.whatsappAdapter.GetConversationDetails(conversationID)
	if details == nil {
		h.respondWithError(w, http.StatusNotFound, "Conversation not found")
		return
	}
	
	h.respondWithJSON(w, http.StatusOK, details)
}

// handleDeleteMemory deletes a specific memory from a conversation
func (h *Handler) handleDeleteMemory(w http.ResponseWriter, r *http.Request) {
	// Check if connected
	if !h.whatsappAdapter.IsConnected() {
		h.respondWithError(w, http.StatusServiceUnavailable, "WhatsApp is not connected")
		return
	}
	
	// Parse request
	var requestData struct {
		ConversationID string `json:"conversation_id"`
		MemoryIndex    int    `json:"memory_index"`
	}
	
	err := json.NewDecoder(r.Body).Decode(&requestData)
	if err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	
	// Delete memory
	success := h.whatsappAdapter.DeleteMemory(requestData.ConversationID, requestData.MemoryIndex)
	if !success {
		h.respondWithError(w, http.StatusNotFound, "Memory not found")
		return
	}
	
	h.respondWithJSON(w, http.StatusOK, map[string]string{"message": "Memory deleted successfully"})
}

// handleClearAllMemories clears all memories for a conversation
func (h *Handler) handleClearAllMemories(w http.ResponseWriter, r *http.Request) {
	// Check if connected
	if !h.whatsappAdapter.IsConnected() {
		h.respondWithError(w, http.StatusServiceUnavailable, "WhatsApp is not connected")
		return
	}
	
	// Parse request
	var requestData struct {
		ConversationID string `json:"conversation_id"`
	}
	
	err := json.NewDecoder(r.Body).Decode(&requestData)
	if err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	
	// Clear all memories
	success := h.whatsappAdapter.ClearAllMemories(requestData.ConversationID)
	if !success {
		h.respondWithError(w, http.StatusNotFound, "Conversation not found")
		return
	}
	
	h.respondWithJSON(w, http.StatusOK, map[string]string{"message": "All memories cleared successfully"})
}

// handleUpdateMemory updates the content of a specific memory
func (h *Handler) handleUpdateMemory(w http.ResponseWriter, r *http.Request) {
	// Check if connected
	if !h.whatsappAdapter.IsConnected() {
		h.respondWithError(w, http.StatusServiceUnavailable, "WhatsApp is not connected")
		return
	}
	
	// Parse request
	var requestData struct {
		ConversationID string `json:"conversation_id"`
		MemoryIndex    int    `json:"memory_index"`
		Content        string `json:"content"`
	}
	
	err := json.NewDecoder(r.Body).Decode(&requestData)
	if err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	
	// Update memory
	success := h.whatsappAdapter.UpdateMemory(requestData.ConversationID, requestData.MemoryIndex, requestData.Content)
	if !success {
		h.respondWithError(w, http.StatusNotFound, "Memory not found")
		return
	}
	
	h.respondWithJSON(w, http.StatusOK, map[string]string{"message": "Memory updated successfully"})
}

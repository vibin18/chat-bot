package whatsapp

import (
	"encoding/json"
	"net/http"

	"github.com/vibin/chat-bot/config"
	"github.com/vibin/chat-bot/internal/logger"
)

// AdminHandler handles the WhatsApp admin API endpoints
type AdminHandler struct {
	adapter *WhatsAppAdapter
	cfg     *config.Config
	log     logger.Logger
}

// NewAdminHandler creates a new WhatsApp admin handler
func NewAdminHandler(adapter *WhatsAppAdapter, cfg *config.Config, log logger.Logger) *AdminHandler {
	return &AdminHandler{
		adapter: adapter,
		cfg:     cfg,
		log:     log,
	}
}

// RegisterRoutes registers admin routes
func (h *AdminHandler) RegisterRoutes(r http.Handler) {
	// This is a placeholder for future direct route registration
	// Currently, we're handling routes in the main HTTP handler
	h.log.Info("WhatsApp admin routes registered")
}

// HandleGetGroups returns a list of WhatsApp groups
func (h *AdminHandler) HandleGetGroups(w http.ResponseWriter, r *http.Request) {
	// Check if connected
	if !h.adapter.IsConnected() {
		h.respondWithError(w, http.StatusServiceUnavailable, "WhatsApp is not connected")
		return
	}
	
	// Get groups
	groups, err := h.adapter.GetGroups()
	if err != nil {
		h.log.Error("Failed to get WhatsApp groups", "error", err)
		h.respondWithError(w, http.StatusInternalServerError, "Failed to get WhatsApp groups")
		return
	}
	
	h.respondWithJSON(w, http.StatusOK, groups)
}

// HandleUpdateGroups updates the list of allowed WhatsApp groups
func (h *AdminHandler) HandleUpdateGroups(w http.ResponseWriter, r *http.Request) {
	var requestData struct {
		AllowedGroups []string `json:"allowed_groups"`
	}
	
	// Parse request
	err := json.NewDecoder(r.Body).Decode(&requestData)
	if err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	
	// Update allowed groups in adapter
	err = h.adapter.UpdateAllowedGroups(requestData.AllowedGroups)
	if err != nil {
		h.respondWithError(w, http.StatusInternalServerError, "Failed to update allowed groups")
		return
	}
	
	// Update config
	h.cfg.WhatsApp.AllowedGroups = requestData.AllowedGroups
	
	// Save config
	err = config.SaveConfig(h.cfg, config.GetConfigPath())
	if err != nil {
		h.log.Error("Failed to save config", "error", err)
		h.respondWithError(w, http.StatusInternalServerError, "Failed to save configuration")
		return
	}
	
	h.respondWithJSON(w, http.StatusOK, map[string]string{"message": "WhatsApp groups updated successfully"})
}

// HandleWhatsAppStatus returns the status of the WhatsApp connection
func (h *AdminHandler) HandleWhatsAppStatus(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"connected": h.adapter.IsConnected(),
		"enabled":   h.cfg.WhatsApp.Enabled,
	}
	
	h.respondWithJSON(w, http.StatusOK, status)
}

// HandleGetAllMemories returns summary information for all conversations with memories
func (h *AdminHandler) HandleGetAllMemories(w http.ResponseWriter, r *http.Request) {
	// Check if connected
	if !h.adapter.IsConnected() {
		h.respondWithError(w, http.StatusServiceUnavailable, "WhatsApp is not connected")
		return
	}
	
	// Get all memory info
	memoryInfo := h.adapter.GetAllMemoryInfo()
	h.respondWithJSON(w, http.StatusOK, memoryInfo)
}

// HandleGetConversationDetails returns detailed memory and context for a specific conversation
func (h *AdminHandler) HandleGetConversationDetails(w http.ResponseWriter, r *http.Request) {
	// Check if connected
	if !h.adapter.IsConnected() {
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
	details := h.adapter.GetConversationDetails(conversationID)
	if details == nil {
		h.respondWithError(w, http.StatusNotFound, "Conversation not found")
		return
	}
	
	h.respondWithJSON(w, http.StatusOK, details)
}

// HandleDeleteMemory deletes a specific memory from a conversation
func (h *AdminHandler) HandleDeleteMemory(w http.ResponseWriter, r *http.Request) {
	// Check if connected
	if !h.adapter.IsConnected() {
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
	success := h.adapter.DeleteMemory(requestData.ConversationID, requestData.MemoryIndex)
	if !success {
		h.respondWithError(w, http.StatusNotFound, "Memory not found")
		return
	}
	
	h.respondWithJSON(w, http.StatusOK, map[string]string{"message": "Memory deleted successfully"})
}

// HandleClearAllMemories clears all memories for a conversation
func (h *AdminHandler) HandleClearAllMemories(w http.ResponseWriter, r *http.Request) {
	// Check if connected
	if !h.adapter.IsConnected() {
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
	success := h.adapter.ClearAllMemories(requestData.ConversationID)
	if !success {
		h.respondWithError(w, http.StatusNotFound, "Conversation not found")
		return
	}
	
	h.respondWithJSON(w, http.StatusOK, map[string]string{"message": "All memories cleared successfully"})
}

// HandleUpdateMemory updates the content of a specific memory
func (h *AdminHandler) HandleUpdateMemory(w http.ResponseWriter, r *http.Request) {
	// Check if connected
	if !h.adapter.IsConnected() {
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
	success := h.adapter.UpdateMemory(requestData.ConversationID, requestData.MemoryIndex, requestData.Content)
	if !success {
		h.respondWithError(w, http.StatusNotFound, "Memory not found")
		return
	}
	
	h.respondWithJSON(w, http.StatusOK, map[string]string{"message": "Memory updated successfully"})
}

// respondWithError sends an error response
func (h *AdminHandler) respondWithError(w http.ResponseWriter, code int, message string) {
	h.respondWithJSON(w, code, map[string]string{"error": message})
}

// respondWithJSON sends a JSON response
func (h *AdminHandler) respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, err := json.Marshal(payload)
	if err != nil {
		h.log.Error("Failed to marshal JSON response", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}

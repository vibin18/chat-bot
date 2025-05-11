package http

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/vibin/chat-bot/config"
)

// setupWhatsAppAdminRoutes sets up routes for WhatsApp admin functionality
func (h *Handler) setupWhatsAppAdminRoutes(r chi.Router) {
	h.logger.Info("Setting up WhatsApp admin routes")

	// Register routes
	r.Route("/whatsapp", func(r chi.Router) {
		// Group list and management
		r.Get("/groups", h.handleGetGroups)
		r.Post("/groups", h.handleUpdateGroups)
		
		// Status endpoint
		r.Get("/status", h.handleWhatsAppStatus)
		
		// Memory management endpoints
		r.Route("/memory", func(r chi.Router) {
			r.Get("/all", h.handleGetAllMemories)
			r.Get("/conversation", h.handleGetConversationMemory)
			r.Get("/users", h.handleGetUsersInConversation)
			r.Get("/user", h.handleGetUserMemories)
			r.Post("/delete", h.handleDeleteMemory)
			r.Post("/clear", h.handleClearAllMemories)
			r.Post("/update", h.handleUpdateMemory)
			r.Post("/context/delete", h.handleDeleteContextMessage)
			r.Post("/add", h.handleAddMemory)
		})
	})
}

// WhatsAppAdminPage serves the WhatsApp admin UI
func (h *Handler) WhatsAppAdminPage(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./web/templates/whatsapp_admin.html")
}

// MemoryAdminPage serves the Memory admin UI
func (h *Handler) MemoryAdminPage(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./web/templates/memory_admin.html")
}

// handleGetGroups returns a list of WhatsApp groups
func (h *Handler) handleGetGroups(w http.ResponseWriter, r *http.Request) {
	// Check if connected
	if !h.whatsappAdapter.IsConnected() {
		h.respondWithError(w, http.StatusServiceUnavailable, "WhatsApp is not connected")
		return
	}
	
	// Get groups
	groups, err := h.whatsappAdapter.GetGroups()
	if err != nil {
		h.logger.Error("Failed to get WhatsApp groups", "error", err)
		h.respondWithError(w, http.StatusInternalServerError, "Failed to get WhatsApp groups")
		return
	}
	
	h.respondWithJSON(w, http.StatusOK, groups)
}

// handleUpdateGroups updates the list of allowed WhatsApp groups
func (h *Handler) handleUpdateGroups(w http.ResponseWriter, r *http.Request) {
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
	err = h.whatsappAdapter.UpdateAllowedGroups(requestData.AllowedGroups)
	if err != nil {
		h.respondWithError(w, http.StatusInternalServerError, "Failed to update allowed groups")
		return
	}
	
	// Update config
	h.config.WhatsApp.AllowedGroups = requestData.AllowedGroups
	
	// Save config
	err = config.SaveConfig(h.config, config.GetConfigPath())
	if err != nil {
		h.logger.Error("Failed to save config", "error", err)
		h.respondWithError(w, http.StatusInternalServerError, "Failed to save configuration")
		return
	}
	
	h.respondWithJSON(w, http.StatusOK, map[string]string{"message": "WhatsApp groups updated successfully"})
}

// handleWhatsAppStatus returns the status of the WhatsApp connection
func (h *Handler) handleWhatsAppStatus(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"connected": h.whatsappAdapter.IsConnected(),
		"enabled":   h.config.WhatsApp.Enabled,
	}
	
	h.respondWithJSON(w, http.StatusOK, status)
}

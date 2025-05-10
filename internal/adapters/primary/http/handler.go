package http

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/vibin/chat-bot/config"
	"github.com/vibin/chat-bot/internal/core/ports"
	"github.com/vibin/chat-bot/internal/core/services"
	"github.com/vibin/chat-bot/internal/logger"
)

// Handler is the HTTP handler for the chat application
type Handler struct {
	service *services.ChatService
	logger  logger.Logger
	router  *chi.Mux
	config  *config.Config
	whatsappAdapter ports.WhatsAppPort
}

// NewHandler creates a new HTTP handler
func NewHandler(service *services.ChatService, cfg *config.Config, whatsappAdapter ports.WhatsAppPort, log logger.Logger) *Handler {
	h := &Handler{
		service: service,
		logger:  log,
		config:  cfg,
		whatsappAdapter: whatsappAdapter,
	}
	
	h.setupRouter()
	return h
}

// setupRouter sets up the Chi router with middleware and routes
func (h *Handler) setupRouter() {
	r := chi.NewRouter()
	
	// Middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(LoggerMiddleware(h.logger))
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))
	
	// CORS middleware
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))
	
	// Static files
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir("./web/static"))))
	
	// API routes
	r.Route("/api", func(r chi.Router) {
		r.Route("/chats", func(r chi.Router) {
			r.Get("/", h.ListChats)
			r.Post("/", h.CreateChat)
			r.Route("/{chatID}", func(r chi.Router) {
				r.Get("/", h.GetChat)
				r.Post("/messages", h.SendMessage)
				r.Delete("/", h.DeleteChat)
			})
		})
		
		r.Get("/model", h.GetModelInfo)
		
		// WhatsApp admin routes
		if h.config.WhatsApp.Enabled && h.whatsappAdapter != nil {
			h.setupWhatsAppAdminRoutes(r)
		}
	})
	
	// Web UI routes
	r.Get("/", h.HomePage)
	r.Get("/chat/{chatID}", h.ChatPage)
	
	// WhatsApp admin UI
	if h.config.WhatsApp.Enabled {
		r.Get("/admin/whatsapp", h.WhatsAppAdminPage)
	}
	
	h.router = r
}

// ServeHTTP implements the http.Handler interface
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.router.ServeHTTP(w, r)
}

// HomePage handles the home page request
func (h *Handler) HomePage(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./web/templates/index.html")
}

// ChatPage handles the chat page request
func (h *Handler) ChatPage(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./web/templates/chat.html")
}

// CreateChat handles the create chat request
func (h *Handler) CreateChat(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title string `json:"title"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	
	chat, err := h.service.CreateChat(r.Context(), req.Title)
	if err != nil {
		h.respondWithError(w, http.StatusInternalServerError, "Failed to create chat")
		return
	}
	
	h.respondWithJSON(w, http.StatusCreated, chat)
}

// GetChat handles the get chat request
func (h *Handler) GetChat(w http.ResponseWriter, r *http.Request) {
	chatID := chi.URLParam(r, "chatID")
	
	chat, err := h.service.GetChat(r.Context(), chatID)
	if err != nil {
		h.respondWithError(w, http.StatusNotFound, "Chat not found")
		return
	}
	
	h.respondWithJSON(w, http.StatusOK, chat)
}

// ListChats handles the list chats request
func (h *Handler) ListChats(w http.ResponseWriter, r *http.Request) {
	chats, err := h.service.ListChats(r.Context())
	if err != nil {
		h.respondWithError(w, http.StatusInternalServerError, "Failed to list chats")
		return
	}
	
	h.respondWithJSON(w, http.StatusOK, chats)
}

// SendMessage handles the send message request
func (h *Handler) SendMessage(w http.ResponseWriter, r *http.Request) {
	chatID := chi.URLParam(r, "chatID")
	
	var req struct {
		Content string `json:"content"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	
	chat, err := h.service.SendMessage(r.Context(), chatID, req.Content)
	if err != nil {
		h.respondWithError(w, http.StatusInternalServerError, "Failed to send message")
		return
	}
	
	h.respondWithJSON(w, http.StatusOK, chat)
}

// DeleteChat handles the delete chat request
func (h *Handler) DeleteChat(w http.ResponseWriter, r *http.Request) {
	chatID := chi.URLParam(r, "chatID")
	
	err := h.service.DeleteChat(r.Context(), chatID)
	if err != nil {
		h.respondWithError(w, http.StatusNotFound, "Chat not found")
		return
	}
	
	h.respondWithJSON(w, http.StatusNoContent, nil)
}

// GetModelInfo handles the get model info request
func (h *Handler) GetModelInfo(w http.ResponseWriter, r *http.Request) {
	info, err := h.service.GetModelInfo(r.Context())
	if err != nil {
		h.respondWithError(w, http.StatusInternalServerError, "Failed to get model info")
		return
	}
	
	h.respondWithJSON(w, http.StatusOK, info)
}

// respondWithError sends an error response
func (h *Handler) respondWithError(w http.ResponseWriter, code int, message string) {
	h.respondWithJSON(w, code, map[string]string{"error": message})
}

// respondWithJSON sends a JSON response
func (h *Handler) respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, err := json.Marshal(payload)
	if err != nil {
		h.logger.Error("Failed to marshal JSON response", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}

// LoggerMiddleware is a middleware that logs HTTP requests
func LoggerMiddleware(log logger.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			
			ctx := context.WithValue(r.Context(), "logger", log)
			
			defer func() {
				log.Info("HTTP request",
					"method", r.Method,
					"path", r.URL.Path,
					"status", ww.Status(),
					"bytes", ww.BytesWritten(),
					"duration", time.Since(start),
				)
			}()
			
			next.ServeHTTP(ww, r.WithContext(ctx))
		})
	}
}

package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/vibin/chat-bot/config"
	httpHandler "github.com/vibin/chat-bot/internal/adapters/primary/http"
	whatsappAdapter "github.com/vibin/chat-bot/internal/adapters/primary/whatsapp"
	"github.com/vibin/chat-bot/internal/adapters/secondary/llm"
	"github.com/vibin/chat-bot/internal/adapters/secondary/repository"
	"github.com/vibin/chat-bot/internal/adapters/secondary/websearch"
	"github.com/vibin/chat-bot/internal/core/ports"
	"github.com/vibin/chat-bot/internal/core/services"
	"github.com/vibin/chat-bot/internal/logger"
)

func main() {
	// Parse command line flags
	configPath := flag.String("config", "", "Path to config file")
	debugMode := flag.Bool("debug", false, "Enable debug logging")
	flag.Parse()

	// Setup logger
	logLevel := slog.LevelInfo
	if *debugMode {
		logLevel = slog.LevelDebug
	}
	log := logger.New(logLevel, os.Stdout)
	log.Info("Starting LLM Chat Bot")

	// Load configuration
	var cfg *config.Config
	var err error

	if *configPath != "" {
		log.Info("Loading configuration", "path", *configPath)
		cfg, err = config.LoadConfig(*configPath)
		if err != nil {
			log.Error("Failed to load configuration", "error", err)
			os.Exit(1)
		}
	} else {
		log.Info("Using default configuration")
		cfg = config.DefaultConfig()
	}

	// Initialize adapters
	log.Info("Initializing adapters")

	// Create main LLM adapter
	llmAdapter, err := llm.NewOllamaAdapter(&cfg.LLM, log)
	if err != nil {
		log.Error("Failed to initialize main LLM adapter", "error", err)
		os.Exit(1)
	}
	
	// Create image analysis LLM adapter if enabled
	var imageLLMAdapter ports.LLMPort
	if cfg.ImageLLM.Enabled {
		log.Info("Initializing image analysis LLM adapter", "provider", cfg.ImageLLM.Provider, "model", cfg.ImageLLM.Ollama.Model)
		// Create a temporary LLMConfig from ImageLLM for adapter initialization
		imageLLMConfig := &config.LLMConfig{
			Provider: cfg.ImageLLM.Provider,
			Ollama:   cfg.ImageLLM.Ollama,
		}
		imageAdapter, err := llm.NewOllamaAdapter(imageLLMConfig, log)
		if err != nil {
			log.Error("Failed to initialize image analysis LLM adapter", "error", err)
			// Fall back to main LLM if image LLM fails
			imageLLMAdapter = llmAdapter
		} else {
			imageLLMAdapter = imageAdapter
		}
	} else {
		// Use main LLM adapter for image analysis if dedicated one is disabled
		imageLLMAdapter = llmAdapter
	}

	// Create repository adapter
	repoAdapter := repository.NewInMemoryRepository(log)
	
	// Create secondary LLM adapter for search query formatting
	var secondaryLLMAdapter *llm.OllamaAdapter
	var webSearchAdapter ports.WebSearchPort
	
	if cfg.WebSearch.Enabled {
		log.Info("Initializing secondary LLM adapter for web search")
		// Create a temporary LLMConfig from SecondaryLLM for adapter initialization
		secondaryLLMConfig := &config.LLMConfig{
			Provider: cfg.SecondaryLLM.Provider,
			Ollama:   cfg.SecondaryLLM.Ollama,
		}
		secondaryLLMAdapter, err = llm.NewOllamaAdapter(secondaryLLMConfig, log)
		if err != nil {
			log.Error("Failed to initialize secondary LLM adapter", "error", err)
			os.Exit(1)
		}
		
		// Create web search adapter based on config
		log.Info("Initializing web search adapter", "provider", cfg.WebSearch.Provider)
		
		switch cfg.WebSearch.Provider {
		case "brave":
			webSearchAdapter = websearch.NewBraveAdapter(&cfg.WebSearch, secondaryLLMAdapter, log)
			log.Info("Using Brave Search adapter")
		case "serpapi", "":
			webSearchAdapter = websearch.NewSerpAPIAdapter(&cfg.WebSearch, secondaryLLMAdapter, log)
			log.Info("Using SerpAPI adapter")
		default:
			log.Warn("Unknown web search provider, falling back to SerpAPI", "provider", cfg.WebSearch.Provider)
			webSearchAdapter = websearch.NewSerpAPIAdapter(&cfg.WebSearch, secondaryLLMAdapter, log)
		}
	}

	// Create chat service with dedicated image LLM adapter
	chatService := services.NewChatService(llmAdapter, imageLLMAdapter, repoAdapter, webSearchAdapter, cfg, log)

	// Initialize WhatsApp adapter if enabled
	var waAdapter ports.WhatsAppPort
	if cfg.WhatsApp.Enabled {
		log.Info("Initializing WhatsApp adapter")
		whatsappAdapter, err := whatsappAdapter.NewWhatsAppAdapter(chatService, cfg, log)
		if err != nil {
			log.Error("Failed to initialize WhatsApp adapter", "error", err)
		} else {
			waAdapter = whatsappAdapter
			
			// Initialize memory database
			log.Info("Initializing memory database")
			memoryService, err := whatsappAdapter.InitializeMemoryDB()
			if err != nil {
				log.Error("Failed to initialize memory database", "error", err)
			} else {
				log.Info("Memory database initialized successfully")
				// Connect the memory service to the WhatsApp adapter
				whatsappAdapter.SetMemoryService(memoryService)
			}
			
			// Start WhatsApp adapter in a goroutine
			go func() {
				log.Info("Starting WhatsApp adapter")
				if err := waAdapter.Connect(context.Background()); err != nil {
					log.Error("Failed to connect to WhatsApp", "error", err)
					return
				}
				
				if err := waAdapter.Start(context.Background()); err != nil {
					log.Error("WhatsApp adapter error", "error", err)
				}
			}()
		}
	}

	// Create HTTP handler
	handler := httpHandler.NewHandler(chatService, cfg, waAdapter, log)

	// Create HTTP server
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 120 * time.Second, // Longer timeout for LLM responses
		IdleTimeout:  60 * time.Second,
	}
	


	// Start the server in a goroutine
	go func() {
		log.Info("Starting HTTP server", "port", cfg.Server.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("HTTP server error", "error", err)
			os.Exit(1)
		}
	}()

	// Create a channel to listen for OS signals
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Block until a signal is received
	<-stop
	log.Info("Shutting down server...")

	// Create a deadline context for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := server.Shutdown(ctx); err != nil {
		log.Error("Server forced to shutdown", "error", err)
	}
	
	// Disconnect WhatsApp if it was enabled
	if cfg.WhatsApp.Enabled && waAdapter != nil && waAdapter.IsConnected() {
		log.Info("Disconnecting WhatsApp adapter")
		if err := waAdapter.Disconnect(); err != nil {
			log.Error("Error disconnecting WhatsApp adapter", "error", err)
		}
	}

	log.Info("Server exited")
}

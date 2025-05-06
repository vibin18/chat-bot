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

	// Create chat service
	chatService := services.NewChatService(llmAdapter, repoAdapter, webSearchAdapter, cfg, log)

	// Create HTTP handler
	handler := httpHandler.NewHandler(chatService, log)

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

	log.Info("Server exited")
}

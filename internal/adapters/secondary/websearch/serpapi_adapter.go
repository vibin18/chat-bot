package websearch

import (
	"context"
	"fmt"
	"strings"

	serpapi "github.com/serpapi/google-search-results-golang"
	"github.com/vibin/chat-bot/config"
	"github.com/vibin/chat-bot/internal/core/domain"
	"github.com/vibin/chat-bot/internal/core/ports"
	"github.com/vibin/chat-bot/internal/logger"
)

// SerpAPIAdapter implements the WebSearchPort interface using SerpAPI
type SerpAPIAdapter struct {
	config *config.WebSearchConfig
	logger logger.Logger
	llm    ports.LLMPort // Secondary LLM for query formatting
}

// NewSerpAPIAdapter creates a new SerpAPIAdapter
func NewSerpAPIAdapter(config *config.WebSearchConfig, secondaryLLM ports.LLMPort, log logger.Logger) *SerpAPIAdapter {
	return &SerpAPIAdapter{
		config: config,
		logger: log,
		llm:    secondaryLLM,
	}
}

// Search performs a web search with the given query and returns results
func (a *SerpAPIAdapter) Search(ctx context.Context, query string) ([]ports.SearchResult, error) {
	a.logger.Info("Performing web search", "query", query)

	if a.config.SerpAPIKey == "" {
		return nil, fmt.Errorf("SerpAPI key is not configured")
	}

	// Create SerpAPI client and set parameters
	a.logger.Info("Setting up SerpAPI parameters", "key_length", len(a.config.SerpAPIKey))
	
	// Create parameter map - don't include the API key here
	parameters := map[string]string{
		"q":             query,
		"engine":        "google",
		"google_domain": "google.com",
		"gl":            "us",
		"hl":            "en",
	}
	
	// Create the search client - API key goes as the second parameter
	client := serpapi.NewGoogleSearch(parameters, a.config.SerpAPIKey)
	a.logger.Info("SerpAPI client created")

	// Execute search
	a.logger.Info("Executing SerpAPI search")
	data, err := client.GetJSON()
	if err != nil {
		a.logger.Error("SerpAPI search failed", "error", err, "error_type", fmt.Sprintf("%T", err))
		
		// Return some mock results for debugging purposes
		return getMockResults(), nil
	}

	// Parse organic results
	var results []ports.SearchResult
	if organicResults, ok := data["organic_results"].([]interface{}); ok {
		for i, result := range organicResults {
			if resultMap, ok := result.(map[string]interface{}); ok {
				// Extract data from result
				title := getStringValue(resultMap, "title")
				link := getStringValue(resultMap, "link")
				snippet := getStringValue(resultMap, "snippet")
				displayedLink := getStringValue(resultMap, "displayed_link")

				// Create SearchResult
				searchResult := ports.SearchResult{
					Title:         title,
					Link:          link,
					Snippet:       snippet,
					DisplayedLink: displayedLink,
					Position:      i + 1,
				}

				results = append(results, searchResult)
			}
		}
	}

	a.logger.Info("Web search completed", "results_count", len(results))
	return results, nil
}

// FormatSearchQuery uses the secondary LLM to format a user query into a more effective search query
func (a *SerpAPIAdapter) FormatSearchQuery(ctx context.Context, userQuery string) (string, error) {
	a.logger.Info("Formatting search query", "user_query", userQuery)

	// Create prompt for the secondary LLM
	prompt := fmt.Sprintf(
		"Your task is to format the following user query into an effective web search query. "+
			"Extract the key information and create a concise, focused search query. "+
			"Do not add any explanation, just return the search query.\n\n"+
			"User query: %s\n\n"+
			"Search query:",
		userQuery,
	)

	// Use the secondary LLM to format the query
	formattedQuery, err := a.llm.GenerateResponse(ctx, []domain.Message{{Role: "user", Content: prompt}})
	if err != nil {
		a.logger.Error("Failed to format search query", "error", err)
		return userQuery, err // Fall back to original query on error
	}

	// Clean up the formatted query
	formattedQuery = strings.TrimSpace(formattedQuery)
	
	a.logger.Info("Formatted search query", "formatted_query", formattedQuery)
	return formattedQuery, nil
}

// DetectSearchIntent detects if a user message indicates a need for web search
func (a *SerpAPIAdapter) DetectSearchIntent(message string) bool {
	// Convert message to lowercase for case-insensitive matching
	lowercaseMessage := strings.ToLower(message)

	// Check for any intent keywords in the message
	for _, keyword := range a.config.IntentKeywords {
		if strings.Contains(lowercaseMessage, strings.ToLower(keyword)) {
			a.logger.Info("Search intent detected", "keyword", keyword)
			return true
		}
	}

	return false
}

// Helper function to safely extract string values from map
func getStringValue(data map[string]interface{}, key string) string {
	if value, ok := data[key]; ok {
		if strValue, ok := value.(string); ok {
			return strValue
		}
	}
	return ""
}

// getMockResults returns mock search results when the real search fails
func getMockResults() []ports.SearchResult {
	return []ports.SearchResult{
		{
			Title:         "Mock Search Result 1",
			Link:          "https://example.com/result1",
			Snippet:       "This is a mock search result for debugging purposes. The actual SerpAPI search failed.",
			DisplayedLink: "example.com/result1",
			Position:      1,
		},
		{
			Title:         "Mock Search Result 2",
			Link:          "https://example.com/result2",
			Snippet:       "Another mock result. Please check your SerpAPI key and network connection.",
			DisplayedLink: "example.com/result2",
			Position:      2,
		},
	}
}

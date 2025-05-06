package websearch

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/vibin/chat-bot/config"
	"github.com/vibin/chat-bot/internal/core/domain"
	"github.com/vibin/chat-bot/internal/core/ports"
	"github.com/vibin/chat-bot/internal/logger"
)

const (
	braveSearchBaseURL = "https://api.search.brave.com/res/v1/web/search"
)

// BraveSearchResponse represents the response from Brave Search API
type BraveSearchResponse struct {
	Query struct {
		Original string `json:"original"`
	} `json:"query"`
	Web struct {
		Results []struct {
			Title       string `json:"title"`
			URL         string `json:"url"`
			Description string `json:"description"`
			Age         string `json:"age,omitempty"`
			Favicon     string `json:"favicon,omitempty"`
		} `json:"results"`
		MoreResultsAvailable bool `json:"more_results_available"`
	} `json:"web"`
	Mixed struct {
		Type    string        `json:"type"`
		Results []interface{} `json:"results"`
	} `json:"mixed,omitempty"`
}

// BraveAdapter implements the WebSearchPort interface using Brave Search API
type BraveAdapter struct {
	config     *config.WebSearchConfig
	logger     logger.Logger
	llm        ports.LLMPort // Secondary LLM for query formatting
	httpClient *http.Client
}

// NewBraveAdapter creates a new BraveAdapter
func NewBraveAdapter(config *config.WebSearchConfig, secondaryLLM ports.LLMPort, log logger.Logger) *BraveAdapter {
	return &BraveAdapter{
		config: config,
		logger: log,
		llm:    secondaryLLM,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Search performs a web search with the given query and returns results
func (a *BraveAdapter) Search(ctx context.Context, query string) ([]ports.SearchResult, error) {
	a.logger.Info("Performing Brave web search", "query", query)

	if a.config.BraveAPIKey == "" {
		a.logger.Error("Brave API key is not configured")
		return getMockResults(), nil
	}

	// Build the search URL with query parameters
	searchURL, err := url.Parse(braveSearchBaseURL)
	if err != nil {
		a.logger.Error("Failed to parse Brave Search URL", "error", err)
		return getMockResults(), nil
	}

	// Add query parameters
	q := searchURL.Query()
	q.Set("q", query)
	// Add additional parameters for better results
	q.Set("count", "10")     // Number of results
	q.Set("offset", "0")     // Start at first result
	q.Set("country", "US")   // Country code
	q.Set("language", "en")  // Language
	searchURL.RawQuery = q.Encode()

	a.logger.Info("Brave Search URL", "url", searchURL.String())

	// Create the request
	req, err := http.NewRequestWithContext(ctx, "GET", searchURL.String(), nil)
	if err != nil {
		a.logger.Error("Failed to create Brave Search request", "error", err)
		return getMockResults(), nil
	}

	// Set headers
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("X-Subscription-Token", a.config.BraveAPIKey)
	// Add extra headers for debugging
	req.Header.Set("User-Agent", "ChatBot/1.0")
	
	// Log request details
	a.logger.Info("Sending Brave Search request", 
		"headers", fmt.Sprintf("%v", req.Header),
		"url", req.URL.String())

	// Execute the request
	resp, err := a.httpClient.Do(req)
	if err != nil {
		a.logger.Error("Brave Search request failed", "error", err)
		return getMockResults(), nil
	}
	defer resp.Body.Close()

	// Log response details
	a.logger.Info("Brave Search response received", 
		"status", resp.StatusCode,
		"headers", fmt.Sprintf("%v", resp.Header))

	// Check response status
	if resp.StatusCode != http.StatusOK {
		a.logger.Error("Brave Search returned non-OK status", "status", resp.StatusCode)
		
		// Try to read the error response
		errorBody, _ := io.ReadAll(resp.Body)
		if len(errorBody) > 0 {
			a.logger.Error("Brave Search error response", "body", string(errorBody))
		}
		
		return getMockResults(), nil
	}

	// Check if the response is gzip compressed
	var reader io.ReadCloser
	switch resp.Header.Get("Content-Encoding") {
	case "gzip":
		a.logger.Info("Response is gzip encoded, decompressing")
		var err error
		reader, err = gzip.NewReader(resp.Body)
		if err != nil {
			a.logger.Error("Failed to create gzip reader", "error", err)
			return getMockResults(), nil
		}
		defer reader.Close()
	default:
		a.logger.Info("Response is not compressed")
		reader = resp.Body
	}
	
	// Read the response body
	body, err := io.ReadAll(reader)
	if err != nil {
		a.logger.Error("Failed to read Brave Search response", "error", err)
		return getMockResults(), nil
	}

	// Debug: Log a snippet of the response
	respSnippet := ""
	if len(body) > 100 {
		respSnippet = string(body[:100]) + "..."
	} else {
		respSnippet = string(body)
	}
	a.logger.Info("Brave Search response snippet", "snippet", respSnippet)

	// Try to parse the response
	var braveResp BraveSearchResponse
	if err := json.Unmarshal(body, &braveResp); err != nil {
		a.logger.Error("Failed to parse Brave Search response", "error", err, "response_start", respSnippet)
		
		// Try to decode as an error response
		var errorResp map[string]interface{}
		if jsonErr := json.Unmarshal(body, &errorResp); jsonErr == nil {
			a.logger.Error("Brave Search returned an error", "response", errorResp)
		}
		
		return getMockResults(), nil
	}

	// Convert Brave results to our format
	var results []ports.SearchResult
	for i, result := range braveResp.Web.Results {
		// For displayed link, use URL if available or extract domain from the URL
		displayedLink := result.URL
		if parsedURL, err := url.Parse(result.URL); err == nil && parsedURL.Host != "" {
			displayedLink = parsedURL.Host
		}
		
		searchResult := ports.SearchResult{
			Title:         result.Title,
			Link:          result.URL,
			Snippet:       result.Description,
			DisplayedLink: displayedLink,
			Position:      i + 1,
		}
		results = append(results, searchResult)
	}

	a.logger.Info("Brave web search completed", "results_count", len(results))
	return results, nil
}

// FormatSearchQuery uses the secondary LLM to format a user query into a more effective search query
func (a *BraveAdapter) FormatSearchQuery(ctx context.Context, userQuery string) (string, error) {
	a.logger.Info("Formatting search query", "user_query", userQuery)

	// Get current date and time for context
	currentTime := time.Now()
	dateContext := currentTime.Format("January 2, 2006 15:04 MST")
	a.logger.Info("Adding date context to query", "date_context", dateContext)

	// Create prompt for the secondary LLM
	prompt := fmt.Sprintf(
		"Your task is to format the following user query into an effective web search query. "+
			"Extract the key information and create a concise, focused search query. "+
			"The current date and time is: %s. "+
			"If the query asks for current information, incorporate time context appropriately. "+
			"Do not add any explanation, just return the search query.\n\n"+
			"User query: %s\n\n"+
			"Search query:",
		dateContext,
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
	
	// Add date directly to the query
	shortDate := currentTime.Format("Jan 2, 2006")
	formattedQuery = fmt.Sprintf("%s as of today: %s", formattedQuery, shortDate)
	
	a.logger.Info("Formatted search query", "formatted_query", formattedQuery)
	return formattedQuery, nil
}

// DetectSearchIntent detects if a user message indicates a need for web search
func (a *BraveAdapter) DetectSearchIntent(message string) bool {
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

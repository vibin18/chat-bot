package ports

import (
	"context"
)

// SearchResult represents a single search result item
type SearchResult struct {
	Title       string `json:"title"`
	Link        string `json:"link"`
	Snippet     string `json:"snippet"`
	DisplayedLink string `json:"displayed_link"`
	Position    int    `json:"position"`
}

// WebSearchPort defines the interface for web search functionality
type WebSearchPort interface {
	// Search performs a web search with the given query and returns results
	Search(ctx context.Context, query string) ([]SearchResult, error)
	
	// FormatSearchQuery uses an LLM to format a user query into a more effective search query
	FormatSearchQuery(ctx context.Context, userQuery string) (string, error)
	
	// DetectSearchIntent detects if a user message indicates a need for web search
	DetectSearchIntent(message string) bool
}

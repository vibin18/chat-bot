package whatsapp

import (
	"regexp"
	"strings"
)

// PredefinedResponses handles special questions with predefined answers
type PredefinedResponses struct {
	// Map of patterns to canned responses
	patterns map[*regexp.Regexp]string
	
	// Sensitive topics that should be filtered
	sensitivePatterns []*regexp.Regexp
}

// NewPredefinedResponses creates a new predefined responses handler
func NewPredefinedResponses() *PredefinedResponses {
	pr := &PredefinedResponses{
		patterns:          make(map[*regexp.Regexp]string),
		sensitivePatterns: make([]*regexp.Regexp, 0),
	}
	
	// Set up predefined responses
	pr.setupResponses()
	pr.setupSensitiveFilters()
	
	return pr
}

// setupResponses initializes predefined patterns and responses
func (pr *PredefinedResponses) setupResponses() {
	// Add predefined responses for specific questions - use simpler patterns
	pr.addResponse(`(?i)\byour\s+name\b`, 
		"👋 I am Sasi, a helpful chat assistant created by Avarachan.")
		
	pr.addResponse(`(?i)\bwho\s+are\s+you\b`, 
		"👋 I am Sasi, a helpful chat assistant created by Avarachan.")
		
	pr.addResponse(`(?i)\bavarachan\b`, 
		"Avarachan is my creator. 🧠")
		
	pr.addResponse(`(?i)\bwhere.*running\b`, 
		"I am running at Avarachan's garage. 🏠")
		
	// Model-related responses - simplified pattern
	pr.addResponse(`(?i)\b(model|ai|llm)\b`, 
		"I am powered by Avarran 007, an advanced AI system. 🤖")
}

// setupSensitiveFilters sets up patterns for sensitive topics that should get filtered responses
func (pr *PredefinedResponses) setupSensitiveFilters() {
	sensitiveTopics := []string{
		// API keys and credentials
		`(?i)(api|access|secret)[\s-]*(key|token|credential)`,
		
		// System prompts
		`(?i)(system|initial)[\s-]*(prompt|instruction|message)`,
		
		// Model technical details
		`(?i)(parameter|token|architecture|training|temperature|weight)`,
		
		// Training data
		`(?i)(training|trained|fine-tun|dataset)`,
		
		// Prompt engineering
		`(?i)(prompt|jailbreak|injection|hack|exploit)`,
		
		// Provider details
		`(?i)(anthropic|openai|google|claude|gpt|gemini|llama|mistral)`,
	}
	
	for _, pattern := range sensitiveTopics {
		pr.sensitivePatterns = append(pr.sensitivePatterns, regexp.MustCompile(pattern))
	}
}

// addResponse adds a pattern-response pair to the predefined responses
func (pr *PredefinedResponses) addResponse(pattern string, response string) {
	regex := regexp.MustCompile(pattern)
	pr.patterns[regex] = response
}

// CheckForPredefinedResponse checks if a message matches any predefined response
// Returns the response and a boolean indicating if a match was found
func (pr *PredefinedResponses) CheckForPredefinedResponse(message string) (string, bool) {
	// Simple keyword matching for critical questions (fallback mechanism)
	messageLower := strings.ToLower(message)
	
	// Direct keyword matching as a fallback
	if strings.Contains(messageLower, "your name") || strings.Contains(messageLower, "who are you") {
		return "👋 I am Sasi, a helpful chat assistant created by Avarachan.", true
	}
	
	if strings.Contains(messageLower, "avarachan") {
		return "Avarachan is my creator. 🧠", true
	}
	
	if strings.Contains(messageLower, "where") && strings.Contains(messageLower, "running") {
		return "I am running at Avarachan's garage. 🏠", true
	}
	
	if (strings.Contains(messageLower, "what") || strings.Contains(messageLower, "which")) && 
	   (strings.Contains(messageLower, "model") || strings.Contains(messageLower, "ai") || 
		strings.Contains(messageLower, "language") || strings.Contains(messageLower, "llm")) {
		return "I am powered by Avarran 007, an advanced AI system. 🤖", true
	}
	
	// Check predefined responses with regex patterns
	for pattern, response := range pr.patterns {
		if pattern.MatchString(message) {
			return response, true
		}
	}
	
	// Check if question is about sensitive topics
	if pr.isSensitiveTopic(message) {
		return "I am powered by Avarran 007, an advanced AI system developed by Avarachan. I'm here to be helpful, accurate, and safe. 🛡️", true
	}
	
	return "", false
}

// isSensitiveTopic checks if the message is about sensitive topics
func (pr *PredefinedResponses) isSensitiveTopic(message string) bool {
	message = strings.ToLower(message)
	
	for _, pattern := range pr.sensitivePatterns {
		if pattern.MatchString(message) {
			return true
		}
	}
	
	return false
}

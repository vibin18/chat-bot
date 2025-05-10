package whatsapp

import (
	"regexp"
	"strconv"
	"strings"
)

// WhatsAppFormatter enhances messages for better display in WhatsApp
type WhatsAppFormatter struct {
	// Add any configuration options here if needed
}

// NewWhatsAppFormatter creates a new WhatsApp message formatter
func NewWhatsAppFormatter() *WhatsAppFormatter {
	return &WhatsAppFormatter{}
}

// Format enhances a message with emojis and better formatting for WhatsApp
func (f *WhatsAppFormatter) Format(message string) string {
	result := message

	// Format headings
	result = f.formatHeadings(result)
	
	// Format lists
	result = f.formatLists(result)
	
	// Format code blocks
	result = f.formatCodeBlocks(result)
	
	// Add emojis to links
	result = f.formatLinks(result)
	
	// Add emojis to key phrases
	result = f.addTopicEmojis(result)
	
	// Format notes and warnings
	result = f.formatNotes(result)
	
	// Format greetings and thanks
	result = f.formatGreetingsAndThanks(result)
	
	return result
}

// formatHeadings adds emojis to headings and formats them properly
func (f *WhatsAppFormatter) formatHeadings(message string) string {
	// Main headings (# Heading)
	h1Regex := regexp.MustCompile(`(?m)^#\s+(.+)$`)
	message = h1Regex.ReplaceAllString(message, "📌 *$1* 📌\n")
	
	// Subheadings (## Heading)
	h2Regex := regexp.MustCompile(`(?m)^##\s+(.+)$`)
	message = h2Regex.ReplaceAllString(message, "🔹 *$1*\n")
	
	// Third level headings (### Heading)
	h3Regex := regexp.MustCompile(`(?m)^###\s+(.+)$`)
	message = h3Regex.ReplaceAllString(message, "✨ _$1_\n")
	
	return message
}

// formatLists enhances bullet points and numbered lists with emojis
func (f *WhatsAppFormatter) formatLists(message string) string {
	// Handle bulleted lists (- item or * item)
	bulletRegex := regexp.MustCompile(`(?m)^[\*\-]\s+(.+)$`)
	message = bulletRegex.ReplaceAllString(message, "• $1")
	
	// Handle numbered lists (1. item)
	numberedRegex := regexp.MustCompile(`(?m)^(\d+)\.\s+(.+)$`)
	
	// Replace with emoji numbers where possible
	message = numberedRegex.ReplaceAllStringFunc(message, func(match string) string {
		submatches := numberedRegex.FindStringSubmatch(match)
		if len(submatches) < 3 {
			return match
		}
		
		number, err := strconv.Atoi(submatches[1])
		if err != nil || number < 1 || number > 10 {
			return match
		}
		
		// Use emoji numbers for 1-10
		numberEmojis := []string{"1️⃣", "2️⃣", "3️⃣", "4️⃣", "5️⃣", "6️⃣", "7️⃣", "8️⃣", "9️⃣", "🔟"}
		return numberEmojis[number-1] + " " + submatches[2]
	})
	
	return message
}

// formatCodeBlocks improves the display of code blocks
func (f *WhatsAppFormatter) formatCodeBlocks(message string) string {
	// Format inline code (`code`)
	inlineCodeRegex := regexp.MustCompile("`([^`]+)`")
	message = inlineCodeRegex.ReplaceAllString(message, "```$1```")
	
	// Format multi-line code blocks (```code```)
	// In WhatsApp, we'll use triple backticks for both start and end
	codeBlockRegex := regexp.MustCompile("(?ms)```[a-zA-Z]*\n(.*?)```")
	message = codeBlockRegex.ReplaceAllString(message, "```$1```")
	
	return message
}

// formatLinks adds link emojis to URLs
func (f *WhatsAppFormatter) formatLinks(message string) string {
	// Add link emoji to URLs
	linkRegex := regexp.MustCompile(`(https?://[^\s]+)`)
	message = linkRegex.ReplaceAllString(message, "🔗 $1")
	
	// Format markdown links: [text](url)
	markdownLinkRegex := regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	message = markdownLinkRegex.ReplaceAllString(message, "🔗 $1: $2")
	
	return message
}

// formatNotes adds emojis to important notes, warnings, and tips
func (f *WhatsAppFormatter) formatNotes(message string) string {
	// Add note emoji
	message = regexp.MustCompile(`(?i)(?m)^Note:(.+)$`).ReplaceAllString(message, "📝 *Note:*$1")
	
	// Add warning emoji
	message = regexp.MustCompile(`(?i)(?m)^Warning:(.+)$`).ReplaceAllString(message, "⚠️ *Warning:*$1")
	
	// Add tip emoji
	message = regexp.MustCompile(`(?i)(?m)^Tip:(.+)$`).ReplaceAllString(message, "💡 *Tip:*$1")
	
	// Add important emoji
	message = regexp.MustCompile(`(?i)(?m)^Important:(.+)$`).ReplaceAllString(message, "❗ *Important:*$1")
	
	return message
}

// formatGreetingsAndThanks adds emojis to greetings and thanks
func (f *WhatsAppFormatter) formatGreetingsAndThanks(message string) string {
	// Format greetings
	greetingPattern := regexp.MustCompile(`(?i)^(hi|hello|hey|greetings)(\s|,|\.|\!|$)`)
	if greetingPattern.MatchString(message) {
		message = greetingPattern.ReplaceAllString(message, "👋 $1$2")
	}
	
	// Format thanks
	thanksPattern := regexp.MustCompile(`(?i)(thank you|thanks)(\s|,|\.|\!|$)`)
	message = thanksPattern.ReplaceAllString(message, "$1 🙏$2")
	
	return message
}

// addTopicEmojis adds relevant emojis based on message topic/content
func (f *WhatsAppFormatter) addTopicEmojis(message string) string {
	topicEmojis := map[string]string{
		// Weather related
		"(?i)\\b(weather|temperature|forecast)\\b": "🌤️",
		
		// News related
		"(?i)\\b(news|headline|article)\\b": "📰",
		
		// Sports related
		"(?i)\\b(sports?|match|game|score)\\b": "🏆",
		
		// Finance related
		"(?i)\\b(finance|money|stock|market|price)\\b": "💰",
		
		// Food related
		"(?i)\\b(food|recipe|cook|eat|restaurant)\\b": "🍽️",
		
		// Travel related
		"(?i)\\b(travel|trip|vacation|flight|hotel)\\b": "✈️",
		
		// Tech related
		"(?i)\\b(tech|technology|computer|software|hardware)\\b": "💻",
		
		// Health related
		"(?i)\\b(health|exercise|fitness|workout|diet)\\b": "🏋️",
		
		// Time related
		"(?i)\\b(time|schedule|calendar|date)\\b": "⏰",
		
		// Location related
		"(?i)\\b(location|place|address|map|direction)\\b": "📍",
		
		// Question related
		"(?i)\\b(how|what|when|where|why|who)\\b.*\\?": "❓",
	}
	
	for pattern, emoji := range topicEmojis {
		regex := regexp.MustCompile(pattern)
		if regex.MatchString(message) {
			// Only add emoji if not already at the start
			if !strings.HasPrefix(strings.TrimSpace(message), emoji) {
				// Add the emoji to the beginning of the first paragraph that mentions the topic
				parts := strings.SplitN(message, "\n", 2)
				if regex.MatchString(parts[0]) {
					message = emoji + " " + message
					break // Only add one emoji at the beginning
				}
			}
		}
	}
	
	return message
}

package whatsapp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/vibin/chat-bot/internal/adapters/secondary/imagegen"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types/events"
)

// Default image sizes
const (
	DefaultImageSize = "512x256" // Landscape as default
	SquareSize       = "512x512"
	PortraitSize     = "256x512"
)

// ImageGenerationCommand represents the command to generate an image
type ImageGenerationCommand struct {
	Prompt string
	Size   string
}

// isImageGenerationRequest checks if the message is a request to generate an image
func (a *WhatsAppAdapter) isImageGenerationRequest(message string) bool {
	lowerMessage := strings.ToLower(message)

	// Check for "@sasi @image" pattern
	return strings.Contains(lowerMessage, "@sasi") && strings.Contains(lowerMessage, "@image")
}

// parseImageGenerationCommand parses the image generation command
func (a *WhatsAppAdapter) parseImageGenerationCommand(message string) *ImageGenerationCommand {
	// Default size
	size := DefaultImageSize

	// Extract size information
	// Landscape is already the default
	if strings.Contains(strings.ToLower(message), "square") {
		size = SquareSize
	} else if strings.Contains(strings.ToLower(message), "portrait") {
		size = PortraitSize
	}

	// Remove the command triggers
	prompt := message

	// Replace case-insensitive
	prompt = strings.ReplaceAll(strings.ToLower(prompt), "@sasi", "")
	prompt = strings.ReplaceAll(strings.ToLower(prompt), "@image", "")

	// Remove size keywords
	prompt = strings.ReplaceAll(strings.ToLower(prompt), "square", "")
	prompt = strings.ReplaceAll(strings.ToLower(prompt), "portrait", "")

	// Trim extra spaces
	prompt = strings.TrimSpace(prompt)

	return &ImageGenerationCommand{
		Prompt: prompt,
		Size:   size,
	}
}

// processAndReplyWithImageGeneration handles image generation request and sends reply
func (a *WhatsAppAdapter) processAndReplyWithImageGeneration(conversationID string, evt *events.Message) {
	// Extract message text and parse command
	messageText := a.getMessageText(evt)
	cmd := a.parseImageGenerationCommand(messageText)

	// Log details
	a.log.Info("Processing image generation request",
		"conversation_id", conversationID,
		"prompt", cmd.Prompt,
		"size", cmd.Size)

	// Send initial response
	a.sendReply("🎨 Generating image from your prompt: \""+cmd.Prompt+"\". Please wait a moment...", evt)

	// Create an image generator
	imageGen := imagegen.NewImageGenerator("http://192.168.1.245:5002/generate", a.log)

	// Generate the image
	ctx, cancel := context.WithTimeout(context.Background(), 160*time.Second)
	defer cancel()

	imageData, err := imageGen.GenerateImage(ctx, cmd.Prompt, cmd.Size)
	if err != nil {
		a.log.Error("Failed to generate image", "error", err)
		a.sendReply("😔 I'm sorry, I wasn't able to create an image from your prompt. Please try again with a different description or try later.", evt)
		return
	}

	// Send the image back to WhatsApp
	err = a.sendGeneratedImage(evt, imageData, cmd.Prompt)
	if err != nil {
		a.log.Error("Failed to send generated image", "error", err)
		a.sendReply("🙏 I created a beautiful image based on your prompt, but I'm having trouble sending it right now. Please try again in a moment.", evt)
	}
}

// sendGeneratedImage sends a generated image back to the WhatsApp chat
func (a *WhatsAppAdapter) sendGeneratedImage(evt *events.Message, imageData []byte, caption string) error {
	// Check if we can respond
	if evt.Info.Chat.String() == "" {
		return fmt.Errorf("no chat info available")
	}

	// Create a context with appropriate timeout for upload
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	
	// Log image details before attempting upload
	a.log.Info("Preparing to upload image to WhatsApp", 
		"image_size", len(imageData), 
		"mime_type", "image/png",
		"chat", evt.Info.Chat.String())
	
	// Verify image data
	if len(imageData) == 0 {
		return fmt.Errorf("empty image data")
	}
	
	// Determine image type for logging purposes
	imageFormat := "png"
	if len(imageData) > 4 {
		// Check header bytes
		if imageData[0] == 0xFF && imageData[1] == 0xD8 && imageData[2] == 0xFF {
			imageFormat = "jpeg"
		} else if imageData[0] == 0x89 && imageData[1] == 0x50 && imageData[2] == 0x4E && imageData[3] == 0x47 {
			imageFormat = "png"
		}
	}
	a.log.Info("Detected image format from data", "format", imageFormat)
	
	// Import WhatsApp-specific media type for image
	// For whatsmeow library, we use a constant rather than a string
	// Prepare upload with appropriate media type
	uploaded, err := a.client.Upload(ctx, imageData, whatsmeow.MediaImage)
	if err != nil {
		a.log.Error("Failed to upload image to WhatsApp", 
			"error_type", fmt.Sprintf("%T", err),
			"error_details", err.Error())
		return fmt.Errorf("failed to upload image: %w", err)
	}

	// Log successful upload details
	a.log.Info("Image uploaded successfully to WhatsApp servers",
		"url_length", len(uploaded.URL),
		"direct_path_length", len(uploaded.DirectPath),
		"has_media_key", uploaded.MediaKey != nil)

	// Update caption with emoji
	captionWithEmoji := "🎨 Generated image: " + caption

	// Add fallback text in case image doesn't display
	imgTypeStr := "image/png"
	mimeType := &imgTypeStr
	
	// Log the full upload details to help diagnose any issues
	a.log.Info("WhatsApp upload details", 
		"uploaded_url", uploaded.URL,
		"uploaded_path", uploaded.DirectPath,
		"file_length", len(imageData))

	// Create the message with all required fields
	imageMsg := &proto.Message{
		ImageMessage: &proto.ImageMessage{
			Caption:       &captionWithEmoji,
			URL:           &uploaded.URL,
			DirectPath:    &uploaded.DirectPath,
			MediaKey:      uploaded.MediaKey,
			Mimetype:      mimeType,
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    &[]uint64{uint64(len(imageData))}[0],
		},
	}
	
	// Create a context with timeout for the send operation
	sendCtx, cancelSend := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelSend()

	// Send the message with detailed error handling
	a.log.Info("Sending image message to WhatsApp chat", "chat_id", evt.Info.Chat.String())
	_, err = a.client.SendMessage(sendCtx, evt.Info.Chat, imageMsg)
	if err != nil {
		a.log.Error("Failed to send image message to WhatsApp", 
			"error_type", fmt.Sprintf("%T", err),
			"error_details", err.Error(),
			"chat_id", evt.Info.Chat.String())
		
		// Try to send as text message with link instead
		a.sendReply(fmt.Sprintf("⚠️ I generated an image but couldn't send it directly. Description: %s", caption), evt)
		return fmt.Errorf("failed to send image: %w", err)
	}

	a.log.Info("Successfully sent generated image",
		"chat", evt.Info.Chat.String(),
		"size", len(imageData))

	return nil
}

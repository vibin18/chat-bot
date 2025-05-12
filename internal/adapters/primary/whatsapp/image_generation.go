package whatsapp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/vibin/chat-bot/internal/adapters/secondary/imagegen"
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
		a.sendReply("❌ Sorry, I couldn't generate that image: "+err.Error(), evt)
		return
	}

	// Send the image back to WhatsApp
	err = a.sendGeneratedImage(evt, imageData, cmd.Prompt)
	if err != nil {
		a.log.Error("Failed to send generated image", "error", err)
		a.sendReply("❌ The image was generated but I couldn't send it: "+err.Error(), evt)
	}
}

// sendGeneratedImage sends a generated image back to the WhatsApp chat
func (a *WhatsAppAdapter) sendGeneratedImage(evt *events.Message, imageData []byte, caption string) error {
	// Check if we can respond
	if evt.Info.Chat.String() == "" {
		return fmt.Errorf("no chat info available")
	}

	// Prepare upload
	uploaded, err := a.client.Upload(context.Background(), imageData, "image/png")
	if err != nil {
		return fmt.Errorf("failed to upload image: %w", err)
	}

	// Update caption with emoji
	captionWithEmoji := "🎨 Generated image: " + caption

	// Create the message
	imageMsg := &proto.Message{
		ImageMessage: &proto.ImageMessage{
			Caption:       &captionWithEmoji,
			URL:           &uploaded.URL,
			DirectPath:    &uploaded.DirectPath,
			MediaKey:      uploaded.MediaKey,
			Mimetype:      &[]string{"image/png"}[0],
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    &[]uint64{uint64(len(imageData))}[0],
		},
	}

	// Send the message
	_, err = a.client.SendMessage(context.Background(), evt.Info.Chat, imageMsg)
	if err != nil {
		return fmt.Errorf("failed to send image: %w", err)
	}

	a.log.Info("Successfully sent generated image",
		"chat", evt.Info.Chat.String(),
		"size", len(imageData))

	return nil
}

package whatsapp

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/vibin/chat-bot/internal/core/domain"
	"go.mau.fi/whatsmeow/types/events"
)

// ImageMessage contains the information for an image message analysis
type ImageMessage struct {
	Base64Image string
	Caption     string
}

// ImageAnalysisResponse represents the response from the LLM for image analysis
type ImageAnalysisResponse struct {
	Model     string `json:"model"`
	CreatedAt string `json:"created_at"`
	Message   struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"message"`
	Done bool `json:"done"`
}

// hasImage checks if a message contains an image
func (a *WhatsAppAdapter) hasImage(evt *events.Message) bool {
	return evt.Message.GetImageMessage() != nil
}

// extractImageData extracts the image data and caption from a message
func (a *WhatsAppAdapter) extractImageData(evt *events.Message) (*ImageMessage, error) {
	imgMsg := evt.Message.GetImageMessage()
	if imgMsg == nil {
		return nil, errors.New("no image in message")
	}

	// Download the image
	img, err := a.client.Download(imgMsg)
	if err != nil {
		return nil, fmt.Errorf("failed to download image: %v", err)
	}

	// Read the image data
	imgData, err := io.ReadAll(bytes.NewReader(img))
	if err != nil {
		return nil, fmt.Errorf("failed to read image data: %v", err)
	}

	// Convert to base64
	base64Img := base64.StdEncoding.EncodeToString(imgData)

	// Get caption if any
	caption := imgMsg.GetCaption()
	if caption == "" {
		caption = "Describe what is in this image in detail."
	}

	return &ImageMessage{
		Base64Image: base64Img,
		Caption:     caption,
	}, nil
}

// analyzeImage sends the image to the LLM for analysis
func (a *WhatsAppAdapter) analyzeImage(imgMsg *ImageMessage) (string, error) {
	// If no caption or very short, use a default prompt
	prompt := imgMsg.Caption
	if len(strings.TrimSpace(prompt)) < 5 {
		prompt = "Describe what is in this image in detail."
	}

	// Use the chat service to send the request to the LLM
	ctx := context.Background()
	message := domain.Message{
		Role:    "user",
		Content: prompt,
		Type:    domain.MessageTypeImageAnalysis,
		Images:  []string{imgMsg.Base64Image},
	}

	// Send the request through the chat service
	response, err := a.chatService.CompletionWithImageAnalysis(ctx, message)
	if err != nil {
		return "", fmt.Errorf("image analysis failed: %v", err)
	}

	return response, nil
}

// processAndReplyWithImageAnalysis processes an image message and sends a reply with the analysis
func (a *WhatsAppAdapter) processAndReplyWithImageAnalysis(conversationID string, evt *events.Message) {
	// Extract image data
	imgData, err := a.extractImageData(evt)
	if err != nil {
		a.log.Error("Failed to extract image data", "error", err)
		a.sendReply("Sorry, I couldn't process that image.", evt)
		return
	}

	a.log.Info("Processing image", 
		"conversation_id", conversationID,
		"caption_length", len(imgData.Caption),
		"image_size", len(imgData.Base64Image))

	// First, send a message that we're analyzing the image
	a.sendReply("I'm analyzing this image, please wait a moment...", evt)

	// Analyze the image
	analysis, err := a.analyzeImage(imgData)
	if err != nil {
		a.log.Error("Failed to analyze image", "error", err)
		a.sendReply("Sorry, I couldn't analyze that image. " + err.Error(), evt)
		return
	}

	// Record the prompt and response in conversation history
	prompt := "📷 [Image with caption: " + imgData.Caption + "]"
	a.recordMessage(conversationID, prompt)
	a.recordMessage(conversationID, analysis)

	// Send the analysis back to the WhatsApp group
	a.sendReply(analysis, evt)
}

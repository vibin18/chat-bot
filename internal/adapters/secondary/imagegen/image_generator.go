package imagegen

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/vibin/chat-bot/internal/logger"
)

// ImageGenerator is an adapter for generating images from text prompts
type ImageGenerator struct {
	endpoint string
	client   *http.Client
	logger   logger.Logger
}

// ImageRequest represents a request to the image generation API
type ImageRequest struct {
	Text      string `json:"text"`
	ImageSize string `json:"image-size"`
}

// ImageResponse represents the response from the image generation API
type ImageResponse struct {
	Image      string     `json:"image"`
	Parameters Parameters `json:"parameters"`
	Error      string     `json:"error,omitempty"`
}

// Parameters represents the parameters used for image generation
type Parameters struct {
	Guidance  float64 `json:"guidance"`
	ImageSize string  `json:"image_size"`
	Seed      *int64  `json:"seed"`
	Steps     int     `json:"steps"`
	Text      string  `json:"text"`
}

// NewImageGenerator creates a new image generator adapter
func NewImageGenerator(endpoint string, log logger.Logger) *ImageGenerator {
	client := &http.Client{
		Timeout: 60 * time.Second, // Image generation might take time
	}

	return &ImageGenerator{
		endpoint: endpoint,
		client:   client,
		logger:   log,
	}
}

// GenerateImage generates an image from a text prompt
func (g *ImageGenerator) GenerateImage(ctx context.Context, prompt string, size string) ([]byte, error) {
	g.logger.Info("Generating image from text", "prompt", prompt, "size", size)

	// Default size if not specified
	if size == "" {
		size = "512x512"
	}

	// Prepare request payload
	request := ImageRequest{
		Text:      prompt,
		ImageSize: size,
	}

	// Convert to JSON
	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	g.logger.Info("Sending request to image generator API", "endpoint", g.endpoint, "request", string(jsonData))

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", g.endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Send request
	resp, err := g.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	g.logger.Info("Received response from image generator API", "status", resp.Status, "length", len(body))

	// Parse response
	var response ImageResponse
	if err := json.Unmarshal(body, &response); err != nil {
		// If JSON parsing fails, check if it's just a raw base64 image
		if len(body) > 100 {
			g.logger.Info("Response parsing failed, but received large response", "response_start", string(body[:100]))
		} else {
			g.logger.Info("Response parsing failed", "response", string(body))
		}
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Check for errors
	if response.Error != "" {
		return nil, fmt.Errorf("image generation failed: %s", response.Error)
	}
	
	// Log the parameters used for generation
	g.logger.Info("Image generation parameters", 
		"size", response.Parameters.ImageSize,
		"steps", response.Parameters.Steps,
		"guidance", response.Parameters.Guidance,
		"prompt", response.Parameters.Text)

	// If the response doesn't follow the expected format, try to return the raw body
	if response.Image == "" {
		g.logger.Warn("No image field in response, trying to use response body as base64 image")
		return body, nil
	}

	// Decode the base64 image
	imageData, err := base64.StdEncoding.DecodeString(response.Image)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64 image: %w", err)
	}

	g.logger.Info("Successfully generated image", "image_size_bytes", len(imageData))
	return imageData, nil
}

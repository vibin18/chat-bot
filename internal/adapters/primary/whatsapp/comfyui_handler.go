package whatsapp

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

// ComfyUIApiRequest represents a request to the ComfyUI API
type ComfyUIApiRequest struct {
	Prompt     map[string]interface{} `json:"prompt"`
	ClientID   string                 `json:"client_id"`
	ExtraData  map[string]interface{} `json:"extra_data,omitempty"`
}

// ComfyUIResponse represents a response from the ComfyUI API
type ComfyUIResponse struct {
	Prompt      map[string]interface{} `json:"prompt"`
	NodeErrors  map[string]interface{} `json:"node_errors"`
	Error       string                 `json:"error,omitempty"`
	PromptID    string                 `json:"prompt_id,omitempty"`
	ExecutionID string                 `json:"execution_id,omitempty"`
}

// ComfyUIStatusResponse represents the status response from the ComfyUI API
type ComfyUIStatusResponse struct {
	Status      map[string]interface{} `json:"status"`
	PromptID    string                 `json:"prompt_id"`
	ExecutionID string                 `json:"execution_id"`
}

// ComfyUIHistoryResponse represents the history response from the ComfyUI API
type ComfyUIHistoryResponse struct {
	Outputs map[string]interface{} `json:"outputs"`
}

// ComfyUIPromptHistoryResponse represents the full history response from the ComfyUI API
type ComfyUIPromptHistoryResponse map[string]struct {
	Outputs map[string]struct {
		Images []ComfyUIOutput `json:"images"`
	} `json:"outputs"`
	Status struct {
		Completed bool   `json:"completed"`
		StatusStr string `json:"status_str"`
	} `json:"status"`
}

// ComfyUIOutput represents an output from the ComfyUI workflow
type ComfyUIOutput struct {
	Filename    string `json:"filename"`
	Type        string `json:"type"`
	SubfolderId string `json:"subfolder_id"`
}

// WhatsAppComfyRequest contains information from a ComfyUI request message
type WhatsAppComfyRequest struct {
	IsValid bool
	Prompt  string
}

// extractComfyUIRequest checks if a message is a ComfyUI request and extracts the prompt
func (a *WhatsAppAdapter) extractComfyUIRequest(message string) WhatsAppComfyRequest {
	// Check for both "@sasi" and "@img" in the message
	messageLower := strings.ToLower(message)
	if !strings.Contains(messageLower, "@sasi") || !strings.Contains(messageLower, "@img") {
		return WhatsAppComfyRequest{IsValid: false}
	}
	
	// Extract the prompt - everything after "@img"
	promptStart := strings.Index(messageLower, "@img") + len("@img")
	prompt := strings.TrimSpace(message[promptStart:])
	
	// If no prompt provided, use empty string (workflow will use its default)
	return WhatsAppComfyRequest{IsValid: true, Prompt: prompt}
}

// isComfyUIRequest checks if a message is a ComfyUI request
func (a *WhatsAppAdapter) isComfyUIRequest(message string) bool {
	request := a.extractComfyUIRequest(message)
	return request.IsValid
}

// processAndReplyWithComfyUI processes an image and sends it to ComfyUI for processing
func (a *WhatsAppAdapter) processAndReplyWithComfyUI(conversationID string, evt *events.Message) {
	// Get the message text
	message := a.getMessageText(evt)
	
	// Extract the ComfyUI request details including any custom prompt
	comfyRequest := a.extractComfyUIRequest(message)
	
	// First, send a message that we're processing the image
	if comfyRequest.Prompt != "" {
		a.sendReply(fmt.Sprintf("avarachan is processing this image using your prompt: '%s'", comfyRequest.Prompt), evt)
	} else {
		a.sendReply("avarachan is processing this image using the default prompt, please wait a moment...", evt)
	}

	// Extract image data
	imgData, err := a.extractImageData(evt)
	if err != nil {
		a.log.Error("Failed to extract image data", "error", err)
		a.sendReply("Sorry, I couldn't process that image.", evt)
		return
	}

	a.log.Info("Processing image with ComfyUI",
		"conversation_id", conversationID,
		"custom_prompt", comfyRequest.Prompt,
		"image_size", len(imgData.Base64Image))

	// Process the image with ComfyUI
	imageURL, err := a.processImageWithComfyUI(imgData.Base64Image, comfyRequest.Prompt)
	if err != nil {
		a.log.Error("Failed to process image with ComfyUI", "error", err)
		a.sendReply("Sorry, I couldn't process that image with ComfyUI. " + err.Error(), evt)
		return
	}

	// Record the prompt and response in conversation history
	promptText := "📷 [Image processed with ComfyUI"
	if comfyRequest.Prompt != "" {
		promptText += fmt.Sprintf(" with prompt: %s", comfyRequest.Prompt)
	}
	promptText += "]"
	a.recordMessage(conversationID, promptText)

	// Send the generated image back to the WhatsApp group
	a.sendImageReply(imageURL, "Generated with ComfyUI", evt)
}

// processImageWithComfyUI sends the image to the ComfyUI service for processing
func (a *WhatsAppAdapter) processImageWithComfyUI(base64Image string, customPrompt string) (string, error) {
	if !a.config.ComfyUIService.Enabled {
		return "", fmt.Errorf("ComfyUI service is not enabled")
	}

	// Read the workflow file
	workflowPath := a.config.ComfyUIService.WorkflowPath
	workflowBytes, err := os.ReadFile(workflowPath)
	if err != nil {
		return "", fmt.Errorf("failed to read workflow file: %v", err)
	}

	// Parse the workflow JSON
	var workflow map[string]interface{}
	if err := json.Unmarshal(workflowBytes, &workflow); err != nil {
		return "", fmt.Errorf("failed to parse workflow JSON: %v", err)
	}

	// Decode base64 image
	imgBytes, err := base64.StdEncoding.DecodeString(base64Image)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64 image: %v", err)
	}

	// Save the input image temporarily
	tempDir := os.TempDir()
	inputImagePath := filepath.Join(tempDir, fmt.Sprintf("input_%s.jpg", uuid.New().String()))
	if err := os.WriteFile(inputImagePath, imgBytes, 0644); err != nil {
		return "", fmt.Errorf("failed to save temporary image: %v", err)
	}
	defer os.Remove(inputImagePath) // Clean up the file when done

	// Upload the image to ComfyUI
	uploadURL := fmt.Sprintf("%s/upload/image", a.config.ComfyUIService.Endpoint)
	uploadedFilename, err := a.uploadImageToComfyUI(inputImagePath, uploadURL)
	if err != nil {
		return "", fmt.Errorf("failed to upload image to ComfyUI: %v", err)
	}

	// Find and update nodes in the workflow
	// First, find the image loader node and update it to use the uploaded image
	imageLoaderFound := false
	
	// Then, if a custom prompt is provided, find step 6 (CLIPTextEncode) and update the prompt
	promptNodeFound := false
	
	for nodeID, nodeData := range workflow {
		nodeMap, ok := nodeData.(map[string]interface{})
		if !ok {
			continue
		}
		
		// Check if this is an image loader node
		class, ok := nodeMap["class_type"].(string)
		if !ok {
			continue
		}
		
		// Update image loader node
		if class == "LoadImage" && !imageLoaderFound {
			// Found an image loader, update it to use our uploaded image
			inputs, ok := nodeMap["inputs"].(map[string]interface{})
			if ok {
				inputs["image"] = uploadedFilename
				imageLoaderFound = true
				a.log.Info("Updated image loader node", "node_id", nodeID)
			}
		}
		
		// Update prompt node if we have a custom prompt - find any CLIPTextEncode node
		if customPrompt != "" && class == "CLIPTextEncode" && !promptNodeFound {
			// This is a positive prompt node (any CLIPTextEncode node)
			inputs, ok := nodeMap["inputs"].(map[string]interface{})
			if ok {
				// Check if this node has a text input field
				if _, hasText := inputs["text"]; hasText {
					// Save the original prompt for logging
					originalPrompt, _ := inputs["text"].(string)
					
					// Update with the custom prompt
					inputs["text"] = customPrompt
					promptNodeFound = true
					
					a.log.Info("Updated prompt in CLIPTextEncode node", 
						"node_id", nodeID,
						"original_prompt", originalPrompt,
						"new_prompt", customPrompt)
				}
			}
		}
	}
	
	// Log the results of our node updates
	found := imageLoaderFound
	if customPrompt != "" {
		a.log.Info("Custom prompt update result", "prompt_node_found", promptNodeFound)
	}
	
	if !found {
		return "", fmt.Errorf("could not find image loader node in workflow")
	}

	// Create a new client ID
	clientID := fmt.Sprintf("whatsapp_bot_%s", uuid.New().String())

	// Create the request payload
	request := ComfyUIApiRequest{
		Prompt:   workflow,
		ClientID: clientID,
	}

	// Send the request to ComfyUI
	queueURL := fmt.Sprintf("%s/prompt", a.config.ComfyUIService.Endpoint)
	jsonBytes, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %v", err)
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: time.Duration(a.config.ComfyUIService.TimeoutSeconds) * time.Second,
	}

	// Send the request
	resp, err := client.Post(queueURL, "application/json", bytes.NewBuffer(jsonBytes))
	if err != nil {
		return "", fmt.Errorf("failed to queue ComfyUI prompt: %v", err)
	}
	defer resp.Body.Close()

	// Read the response
	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %v", err)
	}

	// Parse the response
	var comfyResp ComfyUIResponse
	if err := json.Unmarshal(respBytes, &comfyResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %v", err)
	}

	if comfyResp.Error != "" {
		return "", fmt.Errorf("ComfyUI error: %s", comfyResp.Error)
	}

	// Get the prompt ID from the response
	promptID := comfyResp.PromptID
	if promptID == "" {
		return "", fmt.Errorf("no prompt ID in response")
	}
	a.log.Info("ComfyUI processing request submitted", "prompt_id", promptID, "execution_id", comfyResp.ExecutionID)

	// Poll for completion
	statusURL := fmt.Sprintf("%s/history/%s", a.config.ComfyUIService.Endpoint, promptID)
	a.log.Info("ComfyUI status URL", "url", statusURL)
	
	// Wait for completion with timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(a.config.ComfyUIService.TimeoutSeconds)*time.Second)
	defer cancel()

	// Try to get the status based on timeout configuration - poll every second
	maxRetries := int(a.config.ComfyUIService.TimeoutSeconds)
	a.log.Info("ComfyUI processing started", "timeout_seconds", a.config.ComfyUIService.TimeoutSeconds, "max_retries", maxRetries)
	for retries := 0; retries < maxRetries; retries++ {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("timeout waiting for ComfyUI processing")
		case <-time.After(1 * time.Second):
			// Check status
			resp, err := http.Get(statusURL)
			if err != nil {
				a.log.Error("Failed to check ComfyUI status", "error", err)
				continue
			}

			if resp.StatusCode == http.StatusOK {
				// Read and parse the response
				respBytes, err := io.ReadAll(resp.Body)
				resp.Body.Close()
				if err != nil {
					a.log.Error("Failed to read status response", "error", err)
					continue
				}

				// Log the raw response for debugging - more frequent at first, then every 10 tries
				if retries < 5 || retries % 10 == 0 {
					truncLen := 200
					if len(respBytes) < truncLen {
						truncLen = len(respBytes)
					}
					a.log.Info("ComfyUI status response polling", 
						"prompt_id", promptID, 
						"retry", retries, 
						"response_bytes", len(respBytes),
						"sample", string(respBytes[:truncLen]))
				}

				// Try to parse as the top-level prompt history response first
				var promptHistoryResp ComfyUIPromptHistoryResponse
				if err := json.Unmarshal(respBytes, &promptHistoryResp); err != nil {
					// Show truncated response in case of parse error
					truncLen := 500
					if len(respBytes) < truncLen {
						truncLen = len(respBytes)
					}
					a.log.Error("Failed to parse history response", "error", err, "response", string(respBytes[:truncLen]))
					continue
				}

				// Check if we have outputs in the prompt history response
				a.log.Info("ComfyUI history response received", 
					"prompt_id", promptID, 
					"response_keys", fmt.Sprintf("%v", reflect.ValueOf(promptHistoryResp).MapKeys()))
				
				// Log full response occasionally for debugging
				if retries == 0 || retries % 20 == 0 {
					outputsJson, _ := json.MarshalIndent(promptHistoryResp, "", "  ")
					a.log.Info("ComfyUI full response structure", "response_json", string(outputsJson))
				}
				
				// Look for the prompt entry with our prompt ID
				promptData, exists := promptHistoryResp[promptID]
				if !exists {
					a.log.Debug("Prompt ID not found in history response yet", "prompt_id", promptID)
					continue
				}
				
				// Check if processing is complete
				a.log.Info("ComfyUI processing status", 
					"prompt_id", promptID, 
					"completed", promptData.Status.Completed, 
					"status", promptData.Status.StatusStr,
					"output_nodes", len(promptData.Outputs))
				
				if promptData.Status.Completed && promptData.Status.StatusStr == "success" {
					// Find the first image output
					for nodeId, outputs := range promptData.Outputs {
						a.log.Info("ComfyUI output node found", "node_id", nodeId, "images_count", len(outputs.Images))
						for _, output := range outputs.Images {
							a.log.Info("ComfyUI output image details", "filename", output.Filename, "type", output.Type, "subfolder", output.SubfolderId)
							if output.Type == "output" || output.Type == "image" {
								// Found an image output
								// Use the exact view endpoint format
								imageURL := fmt.Sprintf("%s/view?filename=%s&type=%s",
									a.config.ComfyUIService.Endpoint,
									output.Filename,
									output.Type)
								
								// Download the image to send it via WhatsApp
								a.log.Info("Downloading ComfyUI generated image", "url", imageURL)
								imageBytes, err := a.downloadImage(imageURL)
								if err != nil {
									return "", fmt.Errorf("failed to download generated image: %v", err)
								}
								
								// Save the image locally for WhatsApp to access
								outputImagePath := filepath.Join(tempDir, fmt.Sprintf("comfyui_output_%s.jpg", uuid.New().String()))
								if err := os.WriteFile(outputImagePath, imageBytes, 0644); err != nil {
									return "", fmt.Errorf("failed to save output image: %v", err)
								}
								
								a.log.Info("Successfully saved ComfyUI output image", "path", outputImagePath, "size_bytes", len(imageBytes))
								return outputImagePath, nil
							}
						}
					}
				}
			} else {
				a.log.Warn("Received non-OK status from ComfyUI", "status", resp.Status)
				resp.Body.Close()
			}
		}
	}

	return "", fmt.Errorf("ComfyUI processing timed out or failed")
}

// uploadImageToComfyUI uploads an image to the ComfyUI server
func (a *WhatsAppAdapter) uploadImageToComfyUI(imagePath, uploadURL string) (string, error) {
	// Open the file
	file, err := os.Open(imagePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// Create a buffer to store the form data
	var requestBody bytes.Buffer

	// Create a multipart writer
	multipartWriter := multipart.NewWriter(&requestBody)

	// Create a form file field
	filePart, err := multipartWriter.CreateFormFile("image", filepath.Base(imagePath))
	if err != nil {
		return "", err
	}

	// Copy the file data to the form field
	_, err = io.Copy(filePart, file)
	if err != nil {
		return "", err
	}

	// Close the multipart writer
	multipartWriter.Close()

	// Create the HTTP request
	req, err := http.NewRequest("POST", uploadURL, &requestBody)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", multipartWriter.FormDataContentType())

	// Send the request
	client := &http.Client{
		Timeout: time.Duration(a.config.ComfyUIService.TimeoutSeconds) * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Read the response
	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// Parse the response to get the uploaded filename
	var uploadResp map[string]interface{}
	if err := json.Unmarshal(respBytes, &uploadResp); err != nil {
		return "", err
	}

	// Get the filename
	name, ok := uploadResp["name"].(string)
	if !ok || name == "" {
		return "", fmt.Errorf("invalid upload response")
	}

	return name, nil
}

// downloadImage downloads an image from a URL
func (a *WhatsAppAdapter) downloadImage(url string) ([]byte, error) {
	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: time.Duration(a.config.ComfyUIService.TimeoutSeconds) * time.Second,
	}

	// Send the request
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status: %d", resp.StatusCode)
	}

	// Read the response body
	return io.ReadAll(resp.Body)
}

// sendImageReply sends an image as a reply to a WhatsApp message
func (a *WhatsAppAdapter) sendImageReply(imagePath string, caption string, evt *events.Message) error {
	// Read the image file
	imgBytes, err := os.ReadFile(imagePath)
	if err != nil {
		return fmt.Errorf("failed to read image: %v", err)
	}

	// Wait for rate limiting
	if err := a.limiter.Wait(context.Background()); err != nil {
		a.log.Error("Rate limit error", "error", err)
		return fmt.Errorf("rate limit error: %v", err)
	}

	// Upload and send the image
	resp, err := a.client.Upload(context.Background(), imgBytes, whatsmeow.MediaImage)
	if err != nil {
		return fmt.Errorf("failed to upload image: %v", err)
	}

	// Create the image message
	msg := &waProto.Message{
		ImageMessage: &waProto.ImageMessage{
			URL:           proto.String(resp.URL),
			DirectPath:    proto.String(resp.DirectPath),
			MediaKey:      resp.MediaKey,
			FileEncSHA256: resp.FileEncSHA256,
			FileSHA256:    resp.FileSHA256,
			FileLength:    proto.Uint64(resp.FileLength),
			Caption:       proto.String(caption),
			Mimetype:      proto.String("image/jpeg"),
		},
	}

	// Send the message
	_, err = a.client.SendMessage(context.Background(), evt.Info.Chat, msg)
	if err != nil {
		return fmt.Errorf("failed to send image: %v", err)
	}

	// Clean up the file when done
	defer os.Remove(imagePath)

	return nil
}

// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package llmagent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/elastic/elastic-package/internal/logger"
)

// GeminiProvider implements LLMProvider for Gemini
type GeminiProvider struct {
	apiKey   string
	modelID  string
	endpoint string
	client   *http.Client
}

// GeminiConfig holds configuration for the Gemini provider
type GeminiConfig struct {
	APIKey   string
	ModelID  string
	Endpoint string
}

// NewGeminiProvider creates a new Gemini LLM provider
func NewGeminiProvider(config GeminiConfig) *GeminiProvider {
	if config.ModelID == "" {
		config.ModelID = "gemini-2.5-pro" // Default model
	}
	if config.Endpoint == "" {
		config.Endpoint = "https://generativelanguage.googleapis.com/v1beta"
	}

	// Debug logging with masked API key for security
	logger.Debugf("Creating Gemini provider with model: %s, endpoint: %s",
		config.ModelID, config.Endpoint)
	logger.Debugf("API key (masked for security): %s", maskAPIKey(config.APIKey))

	return &GeminiProvider{
		apiKey:   config.APIKey,
		modelID:  config.ModelID,
		endpoint: config.Endpoint,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// Name returns the provider name
func (g *GeminiProvider) Name() string {
	return "Gemini"
}

// GenerateResponse sends a prompt to Gemini and returns the response
func (g *GeminiProvider) GenerateResponse(ctx context.Context, prompt string, tools []Tool) (*LLMResponse, error) {
	// Convert tools to Google AI format
	googleTools := make([]googleFunctionDeclaration, len(tools))
	for i, tool := range tools {
		googleTools[i] = googleFunctionDeclaration{
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  tool.Parameters,
		}
	}

	// Prepare request payload
	requestPayload := googleRequest{
		Contents: []googleContent{
			{
				Parts: []googlePart{
					{
						Text: prompt,
					},
				},
			},
		},
		GenerationConfig: &googleGenerationConfig{
			MaxOutputTokens: 4096,
		},
	}

	// Add tools if any are provided
	if len(googleTools) > 0 {
		requestPayload.Tools = []googleTool{
			{
				FunctionDeclarations: googleTools,
			},
		}
	}

	jsonPayload, err := json.Marshal(requestPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	url := fmt.Sprintf("%s/models/%s:generateContent", g.endpoint, g.modelID)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-goog-api-key", g.apiKey)

	// Send request
	resp, err := g.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Gemini API returned status %d", resp.StatusCode)
	}

	// Parse response
	var googleResp googleResponse
	if err := json.NewDecoder(resp.Body).Decode(&googleResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Debug logging for the full response
	logger.Debugf("Gemini API response - Candidates count: %d", len(googleResp.Candidates))
	if len(googleResp.Candidates) > 0 {
		candidate := googleResp.Candidates[0]
		logger.Debugf("Gemini API response - FinishReason: %s", candidate.FinishReason)
		logger.Debugf("Gemini API response - Parts count: %d", len(candidate.Content.Parts))
		for i, part := range candidate.Content.Parts {
			if part.Text != "" {
				logger.Debugf("Gemini API response - Part[%d] Text: %s", i, part.Text)
			}
			if part.FunctionCall != nil {
				logger.Debugf("Gemini API response - Part[%d] FunctionCall: name=%s, args=%v",
					i, part.FunctionCall.Name, part.FunctionCall.Args)
			}
		}
	}

	// Convert to our format
	response := &LLMResponse{
		ToolCalls: []ToolCall{},
		Finished:  false,
	}

	if len(googleResp.Candidates) > 0 {
		candidate := googleResp.Candidates[0]

		// Handle different finish reasons
		switch candidate.FinishReason {
		case "STOP":
			response.Finished = true
		case "MALFORMED_FUNCTION_CALL":
			logger.Debugf("Gemini API returned malformed function call - treating as error")
			response.Finished = true
			response.Content = "I encountered an error while trying to call a function. Let me try a different approach."
		case "MAX_TOKENS":
			logger.Debugf("Gemini API hit max tokens limit")
			response.Finished = true
			response.Content = "I reached the maximum response length. Please try breaking this into smaller tasks."
		case "SAFETY":
			logger.Debugf("Gemini API response filtered by safety policies")
			response.Finished = true
			response.Content = "My response was filtered due to safety policies. Please rephrase your request."
		case "RECITATION":
			logger.Debugf("Gemini API response filtered due to recitation")
			response.Finished = true
			response.Content = "My response was filtered due to potential copyright issues. Please rephrase your request."
		case "":
			// Empty finish reason - likely still processing, don't mark as finished
			logger.Debugf("Gemini API returned empty finish reason - continuing")
		default:
			logger.Debugf("Gemini API returned unexpected finish reason: %s - treating as completed", candidate.FinishReason)
			// For unknown finish reasons, mark as finished to prevent infinite loops
			// This is especially important for gemini-2.5-flash which has known instability issues
			response.Finished = true
		}

		// Extract text content and tool calls from parts
		var textParts []string
		for _, part := range candidate.Content.Parts {
			if part.Text != "" {
				textParts = append(textParts, part.Text)
			}
			if part.FunctionCall != nil {
				// Convert function call to our format
				argsJSON, err := json.Marshal(part.FunctionCall.Args)
				if err != nil {
					logger.Debugf("Failed to marshal function call args: %v", err)
					continue
				}

				response.ToolCalls = append(response.ToolCalls, ToolCall{
					ID:        fmt.Sprintf("call_%d", len(response.ToolCalls)),
					Name:      part.FunctionCall.Name,
					Arguments: string(argsJSON),
				})
			}
		}

		// Join all text parts (only override if we don't have error content from finish reason)
		if len(textParts) > 0 && response.Content == "" {
			response.Content = textParts[0] // For simplicity, take the first text part
		}
	}

	return response, nil
}

// Gemini specific types for API communication
type googleRequest struct {
	Contents         []googleContent         `json:"contents"`
	Tools            []googleTool            `json:"tools,omitempty"`
	GenerationConfig *googleGenerationConfig `json:"generationConfig,omitempty"`
}

type googleContent struct {
	Parts []googlePart `json:"parts"`
}

type googlePart struct {
	Text         string              `json:"text,omitempty"`
	FunctionCall *googleFunctionCall `json:"functionCall,omitempty"`
}

type googleFunctionCall struct {
	Name string                 `json:"name"`
	Args map[string]interface{} `json:"args"`
}

type googleTool struct {
	FunctionDeclarations []googleFunctionDeclaration `json:"functionDeclarations"`
}

type googleFunctionDeclaration struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

type googleGenerationConfig struct {
	MaxOutputTokens int `json:"maxOutputTokens,omitempty"`
}

type googleResponse struct {
	Candidates []googleCandidate `json:"candidates"`
}

type googleCandidate struct {
	Content      googleContent `json:"content"`
	FinishReason string        `json:"finishReason"`
}

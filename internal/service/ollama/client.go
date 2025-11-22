package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"ad-tracker/youtube-webhook-ingestion/internal/db/models"
)

// Client is a client for interacting with an Ollama LLM server
type Client struct {
	baseURL    string
	model      string
	apiKey     string
	timeout    time.Duration
	httpClient *http.Client
}

// Config holds the configuration for the Ollama client
type Config struct {
	BaseURL string        // e.g., "http://ollama.example.com:11434"
	Model   string        // e.g., "llama3:8b"
	APIKey  string        // Optional API key for authentication
	Timeout time.Duration // Request timeout (default: 60 seconds)
}

// NewClient creates a new Ollama client
func NewClient(config Config) *Client {
	if config.Timeout == 0 {
		config.Timeout = 60 * time.Second
	}

	return &Client{
		baseURL: strings.TrimSuffix(config.BaseURL, "/"),
		model:   config.Model,
		apiKey:  config.APIKey,
		timeout: config.Timeout,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// ollamaGenerateRequest represents a request to the Ollama /api/generate endpoint
type ollamaGenerateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Format string `json:"format"` // "json" for structured output
	Stream bool   `json:"stream"` // false for non-streaming
}

// ollamaGenerateResponse represents a response from the Ollama /api/generate endpoint
type ollamaGenerateResponse struct {
	Model     string    `json:"model"`
	CreatedAt time.Time `json:"created_at"`
	Response  string    `json:"response"` // The actual JSON response from the LLM
	Done      bool      `json:"done"`
}

// AnalyzeVideoForSponsors sends a video title and description to the LLM for sponsor detection
// Returns the parsed sponsor results, the raw JSON response, and any error
func (c *Client) AnalyzeVideoForSponsors(ctx context.Context, title, description string) (*models.LLMAnalysisResponse, string, error) {
	// Build the prompt
	prompt := buildSponsorDetectionPrompt(title, description)

	// Create request payload
	reqPayload := ollamaGenerateRequest{
		Model:  c.model,
		Prompt: prompt,
		Format: "json",
		Stream: false,
	}

	reqBody, err := json.Marshal(reqPayload)
	if err != nil {
		return nil, "", fmt.Errorf("marshal request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/generate", bytes.NewReader(reqBody))
	if err != nil {
		return nil, "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	// Send request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("send request to Ollama: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, "", fmt.Errorf("ollama API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("read response body: %w", err)
	}

	// Parse Ollama response wrapper
	var ollamaResp ollamaGenerateResponse
	if err := json.Unmarshal(respBody, &ollamaResp); err != nil {
		return nil, "", fmt.Errorf("parse Ollama response: %w", err)
	}

	// The actual LLM response is in the "response" field
	rawLLMResponse := strings.TrimSpace(ollamaResp.Response)

	// Parse the LLM's JSON response into our struct
	var analysisResp models.LLMAnalysisResponse
	if err := json.Unmarshal([]byte(rawLLMResponse), &analysisResp); err != nil {
		return nil, rawLLMResponse, fmt.Errorf("parse LLM JSON response: %w (raw: %s)", err, rawLLMResponse)
	}

	// Validate confidence scores are in range [0, 1]
	for i := range analysisResp.Sponsors {
		if analysisResp.Sponsors[i].Confidence < 0 {
			analysisResp.Sponsors[i].Confidence = 0
		}
		if analysisResp.Sponsors[i].Confidence > 1 {
			analysisResp.Sponsors[i].Confidence = 1
		}
	}

	return &analysisResp, rawLLMResponse, nil
}

// buildSponsorDetectionPrompt constructs the prompt for sponsor detection
func buildSponsorDetectionPrompt(title, description string) string {
	return fmt.Sprintf(`You are analyzing a YouTube video to identify sponsors or brand deals mentioned in the title or description.

Video Title: %s

Video Description:
%s

Identify any sponsors, brand partnerships, or promotional content. For each sponsor found, provide:
1. name: The brand or sponsor name (e.g., "NordVPN", "Skillshare", "Squarespace")
2. confidence: A score from 0.0 to 1.0 indicating how confident you are this is a sponsor (1.0 = definitely a sponsor, 0.5 = possibly a sponsor, use your judgment)
3. evidence: A direct quote from the title or description that indicates sponsorship (e.g., mention of promo codes, affiliate links, "sponsored by", "brought to you by", etc.)

Look for common sponsorship indicators:
- Promo codes or discount codes (e.g., "Use code CREATOR20")
- Affiliate links or special URLs
- Phrases like "This video is sponsored by...", "Thanks to X for sponsoring...", "Brought to you by..."
- References to free trials, discounts, or special offers
- Partnership mentions

Return your response as JSON in this exact format:
{
  "sponsors": [
    {"name": "BrandName", "confidence": 0.95, "evidence": "quote from description"},
    {"name": "AnotherBrand", "confidence": 0.87, "evidence": "another quote"}
  ]
}

If no sponsors are detected, return:
{
  "sponsors": []
}

Only return the JSON, no additional text or explanation.`, title, description)
}

// GetPromptText returns the prompt text that would be sent for a given title and description
// This is useful for storing the prompt in the database
func (c *Client) GetPromptText(title, description string) string {
	return buildSponsorDetectionPrompt(title, description)
}

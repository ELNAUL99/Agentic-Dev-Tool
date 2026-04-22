package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-agent/go-agent/internal/config"
	"github.com/go-agent/go-agent/internal/models"
)

// Client is the interface for LLM providers
type Client interface {
	Complete(ctx context.Context, messages []models.PromptMessage, opts *CompletionOptions) (*models.LLMResponse, error)
	CompleteStream(ctx context.Context, messages []models.PromptMessage, opts *CompletionOptions, handler StreamHandler) error
}

// CompletionOptions configures a single completion request
type CompletionOptions struct {
	Model        string
	Temperature  float64
	MaxTokens    int
	JSONMode     bool
	SystemPrompt string
}

// StreamHandler processes streaming chunks
type StreamHandler func(chunk string) error

// HTTPClient implements Client for OpenAI-compatible APIs
type HTTPClient struct {
	config     config.LLMConfig
	httpClient *http.Client
}

// NewHTTPClient creates a new LLM HTTP client
func NewHTTPClient(cfg config.LLMConfig) *HTTPClient {
	return &HTTPClient{
		config: cfg,
		httpClient: &http.Client{
			Timeout: time.Duration(cfg.Timeout) * time.Second,
		},
	}
}

// openAIRequest matches the OpenAI chat completions API
type openAIRequest struct {
	Model          string          `json:"model"`
	Messages       []openAIMessage `json:"messages"`
	Temperature    float64         `json:"temperature,omitempty"`
	MaxTokens      int             `json:"max_tokens,omitempty"`
	Stream         bool            `json:"stream,omitempty"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
}

type responseFormat struct {
	Type string `json:"type"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// openAIResponse matches the OpenAI chat completions response
type openAIResponse struct {
	ID      string `json:"id"`
	Choices []struct {
		Message      openAIMessage `json:"message"`
		FinishReason string        `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

// Complete sends a non-streaming completion request
func (c *HTTPClient) Complete(ctx context.Context, messages []models.PromptMessage, opts *CompletionOptions) (*models.LLMResponse, error) {
	if opts == nil {
		opts = &CompletionOptions{}
	}

	model := opts.Model
	if model == "" {
		model = c.config.Model
	}

	temp := opts.Temperature
	if temp == 0 && c.config.Temperature != 0 {
		temp = c.config.Temperature
	}

	maxTokens := opts.MaxTokens
	if maxTokens == 0 {
		maxTokens = c.config.MaxTokens
	}

	reqBody := openAIRequest{
		Model:       model,
		Temperature: temp,
		MaxTokens:   maxTokens,
	}

	// Add system prompt if provided
	if opts.SystemPrompt != "" {
		reqBody.Messages = append(reqBody.Messages, openAIMessage{
			Role:    "system",
			Content: opts.SystemPrompt,
		})
	}

	for _, m := range messages {
		reqBody.Messages = append(reqBody.Messages, openAIMessage{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	if opts.JSONMode {
		reqBody.ResponseFormat = &responseFormat{Type: "json_object"}
	}

	return c.doRequest(ctx, reqBody)
}

// CompleteStream sends a streaming completion request
func (c *HTTPClient) CompleteStream(ctx context.Context, messages []models.PromptMessage, opts *CompletionOptions, handler StreamHandler) error {
	if opts == nil {
		opts = &CompletionOptions{}
	}

	model := opts.Model
	if model == "" {
		model = c.config.Model
	}

	reqBody := openAIRequest{
		Model:    model,
		Messages: make([]openAIMessage, 0, len(messages)),
		Stream:   true,
	}

	if opts.SystemPrompt != "" {
		reqBody.Messages = append(reqBody.Messages, openAIMessage{
			Role:    "system",
			Content: opts.SystemPrompt,
		})
	}

	for _, m := range messages {
		reqBody.Messages = append(reqBody.Messages, openAIMessage{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	// Build request
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshaling request: %w", err)
	}

	baseURL := c.config.BaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	req, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.config.APIKey)
	for k, v := range c.config.Headers {
		req.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	return decodeOpenAIStream(resp.Body, handler)
}

func decodeOpenAIStream(r io.Reader, handler StreamHandler) error {
	scanner := bufio.NewScanner(r)
	// Streaming chunks arrive as Server-Sent Events rather than a single JSON
	// document, so each data line has to be decoded independently.
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}

		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "[DONE]" {
			return nil
		}

		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
			Error *struct {
				Message string `json:"message"`
				Type    string `json:"type"`
			} `json:"error,omitempty"`
		}
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			return fmt.Errorf("decoding stream chunk: %w", err)
		}
		if chunk.Error != nil {
			return fmt.Errorf("API error: %s (%s)", chunk.Error.Message, chunk.Error.Type)
		}

		for _, choice := range chunk.Choices {
			if choice.Delta.Content == "" {
				continue
			}
			if err := handler(choice.Delta.Content); err != nil {
				return err
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading stream: %w", err)
	}
	return nil
}

func (c *HTTPClient) doRequest(ctx context.Context, reqBody openAIRequest) (*models.LLMResponse, error) {
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	baseURL := c.config.BaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	req, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.config.APIKey)
	for k, v := range c.config.Headers {
		req.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var apiResp openAIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	if apiResp.Error != nil {
		return nil, fmt.Errorf("API error: %s (%s)", apiResp.Error.Message, apiResp.Error.Type)
	}

	if len(apiResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	result := &models.LLMResponse{
		Content:      apiResp.Choices[0].Message.Content,
		FinishReason: apiResp.Choices[0].FinishReason,
	}
	result.Usage.PromptTokens = apiResp.Usage.PromptTokens
	result.Usage.CompletionTokens = apiResp.Usage.CompletionTokens
	result.Usage.TotalTokens = apiResp.Usage.TotalTokens

	return result, nil
}

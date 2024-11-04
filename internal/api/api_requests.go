// internal/api/api_requests.go

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"KernelSandersBot/internal/types"
)

type APIHandler struct {
	OpenAIKey      string
	OpenAIEndpoint string
	Client         *http.Client
}

func NewAPIHandler(openAIKey, openAIEndpoint string) *APIHandler {
	return &APIHandler{
		OpenAIKey:      openAIKey,
		OpenAIEndpoint: openAIEndpoint,
		Client: &http.Client{
			Timeout: 180 * time.Second, // Set to 3 minutes
		},
	}
}

func (api *APIHandler) QueryOpenAIWithMessages(messages []types.OpenAIMessage) (string, error) {
	fullEndpoint := fmt.Sprintf("%s/chat/completions", api.OpenAIEndpoint)

	query := types.OpenAIQuery{
		Model:       "gpt-4o-mini",
		Messages:    messages,
		Temperature: 0.7,
		MaxTokens:   4096,
	}

	body, err := json.Marshal(query)
	if err != nil {
		return "", fmt.Errorf("failed to marshal OpenAI query: %w", err)
	}

	// Set context timeout to 3 minutes
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", fullEndpoint, bytes.NewBuffer(body))
	if err != nil {
		return "", fmt.Errorf("failed to create OpenAI request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+api.OpenAIKey)

	resp, err := api.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error making request to OpenAI: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("OpenAI returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result types.OpenAIResponse
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return "", fmt.Errorf("error unmarshalling response: %w", err)
	}

	if len(result.Choices) > 0 {
		content := result.Choices[0].Message.Content
		return content, nil
	}

	return "", fmt.Errorf("no choices returned in OpenAI response")
}

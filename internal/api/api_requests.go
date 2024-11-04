// internal/api/api_requests.go

package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"KernelSandersBot/internal/types"
)

// APIHandler handles interactions with the OpenAI API.
type APIHandler struct {
	APIKey      string
	EndpointURL string
	HTTPClient  *http.Client
}

// NewAPIHandler initializes a new APIHandler.
func NewAPIHandler(apiKey, endpointURL string) *APIHandler {
	if endpointURL == "" {
		endpointURL = "https://api.openai.com/v1/chat/completions"
	}
	return &APIHandler{
		APIKey:      apiKey,
		EndpointURL: endpointURL,
		HTTPClient:  &http.Client{Timeout: 30 * time.Second},
	}
}

// QueryOpenAIWithMessages sends a conversation history to OpenAI and retrieves the assistant's response.
func (ah *APIHandler) QueryOpenAIWithMessages(messages []types.OpenAIMessage) (string, error) {
	query := types.OpenAIQuery{
		Model:       "gpt-4o-mini", // Should be set to "gpt-4o-mini"
		Messages:    messages,
		Temperature: 0.7,
		MaxTokens:   1500,
	}

	reqBody, err := json.Marshal(query)
	if err != nil {
		return "", err
	}

	log.Printf("Making OpenAI API request to: %s", ah.EndpointURL) // Added logging

	req, err := http.NewRequest("POST", ah.EndpointURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", ah.APIKey))
	req.Header.Set("Content-Type", "application/json")

	resp, err := ah.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("OpenAI API error: %s - %s", resp.Status, string(bodyBytes))
	}

	var openAIResp types.OpenAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&openAIResp); err != nil {
		return "", err
	}

	if len(openAIResp.Choices) == 0 {
		return "", errors.New("no response from OpenAI")
	}

	return openAIResp.Choices[0].Message.Content, nil
}

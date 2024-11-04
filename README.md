
# KernelSandersBot

**KernelSandersBot** is a Telegram bot integrated with OpenAI's GPT-4 model, designed to provide intelligent and conversational responses to user queries. The bot handles user interactions, maintains conversation contexts, enforces rate limiting, and logs interactions to an AWS S3 bucket. Additionally, it serves full responses via a web interface with enhanced formatting for better readability.

## Table of Contents

- [Features](#features)
- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Configuration](#configuration)
- [Running the Application](#running-the-application)
- [Folder Structure](#folder-structure)
- [Detailed File Descriptions](#detailed-file-descriptions)
  - [cmd/main.go](#cmdmaingo)
  - [internal/api/api_requests.go](#internalapiapi_requestsgo)
  - [internal/app/app.go](#internalappappgo)
  - [internal/app/response_store.go](#internalappresponse_storego)
  - [internal/cache/cache.go](#internalcachecachego)
  - [internal/conversation/conversation_cache.go](#internalconversationconversation_cachego)
  - [internal/handlers/handlers.go](#internalhandlershandlersgo)
  - [internal/s3client/s3client.go](#internals3clients3clientgo)
  - [internal/telegram/telegram_handler.go](#internaltelegramtelegram_handlergo)
  - [internal/types/types.go](#internaltypestypesgo)
  - [internal/usage/usage_cache.go](#internalusageusage_cachego)
  - [internal/utils/utils.go](#internalutilsutilsgo)
- [Logging](#logging)
- [Rate Limiting](#rate-limiting)
- [Response Storage](#response-storage)
- [Web Interface](#web-interface)
- [Dependencies](#dependencies)
- [Contributing](#contributing)
- [License](#license)

## Features

- **Conversational AI:** Leverages OpenAI's GPT-4 model to provide intelligent responses to user queries.
- **Conversation Context:** Maintains context across user interactions to ensure coherent and relevant responses.
- **Rate Limiting:** Implements rate limiting to prevent abuse, allowing only a specified number of messages within a time frame.
- **Logging:** Logs all user interactions to an AWS S3 bucket for monitoring and analysis.
- **Web Interface:** Serves full responses via a web interface with enhanced Markdown formatting for better readability.
- **HTML Parsing:** Utilizes Markdown parsing to display responses with proper formatting, code blocks, and styling.

## Prerequisites

Before setting up **KernelSandersBot**, ensure you have the following installed:

- [Go](https://golang.org/doc/install) (version 1.16 or later)
- An AWS account with access to S3
- A Telegram account to interact with the bot
- An OpenAI API key

## Installation

### Clone the Repository

```bash
git clone https://github.com/yourusername/KernelSandersBot.git
cd KernelSandersBot
```

### Initialize Go Modules

Ensure you're inside the project directory and initialize Go modules:

```bash
go mod tidy
```

This will download all necessary dependencies as specified in the source code.

## Configuration

The bot requires several environment variables to function correctly. You can set these variables in a `.env` file at the root of the project or export them directly in your shell.

### Required Environment Variables

| Variable               | Description                                                       | Example                                |
|------------------------|-------------------------------------------------------------------|----------------------------------------|
| `TELEGRAM_TOKEN`       | Telegram Bot API token obtained from [BotFather](https://t.me/BotFather). | `123456789:ABCDEFGHIJKLMNOPQRSTUVWXYZ`  |
| `OPENAI_KEY`           | OpenAI API key for accessing GPT-4.                              | `sk-XXXXXXXXXXXXXXXXXXXXXXXXXXXX`      |
| `OPENAI_ENDPOINT`      | OpenAI API endpoint URL.                                          | `https://api.openai.com/v1`            |
| `BOT_USERNAME`         | The username of your Telegram bot (without `@`).                 | `KernelSandersBot`                     |
| `AWS_ENDPOINT_URL_S3`  | AWS S3 service endpoint URL.                                      | `https://s3.amazonaws.com`             |
| `AWS_REGION`           | AWS region where your S3 bucket is located.                      | `us-west-2`                            |
| `BUCKET_NAME`          | Name of the AWS S3 bucket for logging interactions.              | `kernel-sanders-logs`                  |
| `NO_LIMIT_USERS`       | Comma-separated list of user IDs exempt from rate limiting.      | `123456789,987654321`                  |
| `BASE_URL`             | Base URL for generating links to full web responses.             | `https://yourdomain.com`               |
| `PORT`                 | Port on which the server will run (default is `8080`).           | `8080`                                 |

### Setting Up the `.env` File

Create a `.env` file in the root directory and populate it with your configurations:

```env
TELEGRAM_TOKEN=your_telegram_bot_token
OPENAI_KEY=your_openai_api_key
OPENAI_ENDPOINT=https://api.openai.com/v1
BOT_USERNAME=KernelSandersBot
AWS_ENDPOINT_URL_S3=https://s3.amazonaws.com
AWS_REGION=us-west-2
BUCKET_NAME=kernel-sanders-logs
NO_LIMIT_USERS=123456789,987654321
BASE_URL=https://yourdomain.com
PORT=8080
```

**Note:** Ensure that the `.env` file is **not** committed to version control as it contains sensitive information.

## Running the Application

After configuring the environment variables, you can run the bot using the following command:

```bash
go run cmd/main.go
```

The server will start on the specified port (default is `8080`). Ensure that the port is open and accessible if deploying to a remote server.

## Folder Structure

```plaintext
KernelSandersBot/
├── cmd/
│   └── main.go
├── internal/
│   ├── api/
│   │   └── api_requests.go
│   ├── app/
│   │   ├── app.go
│   │   └── response_store.go
│   ├── cache/
│   │   └── cache.go
│   ├── conversation/
│   │   └── conversation_cache.go
│   ├── handlers/
│   │   └── handlers.go
│   ├── s3client/
│   │   └── s3client.go
│   ├── telegram/
│   │   └── telegram_handler.go
│   ├── types/
│   │   └── types.go
│   ├── usage/
│   │   └── usage_cache.go
│   └── utils/
│       └── utils.go
├── go.mod
└── go.sum
```

### Overview

- **cmd/**: Contains the entry point of the application.
- **internal/**: Houses the core functionality, segregated into various packages for modularity.
  - **api/**: Manages interactions with external APIs, specifically OpenAI.
  - **app/**: Contains the main application logic, including message processing and response handling.
  - **cache/**: Implements in-memory caching mechanisms.
  - **conversation/**: Manages conversation contexts to maintain state across user interactions.
  - **handlers/**: Defines interfaces and handlers required by other packages.
  - **s3client/**: Handles interactions with AWS S3 for logging purposes.
  - **telegram/**: Manages Telegram-specific functionalities, including message handling.
  - **types/**: Defines data structures and types used across the application.
  - **usage/**: Implements rate limiting to control user interaction frequency.
  - **utils/**: Provides utility functions to support various operations.

## Detailed File Descriptions

### cmd/main.go

```go
// cmd/main.go

package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"

	"KernelSandersBot/internal/app"
	"KernelSandersBot/internal/types"
)

func main() {
	botApp := app.NewApp()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			// Handle web page requests
			botApp.HandleWebRequest(w, r)
			return
		}

		// Handle Telegram updates
		if r.Method != http.MethodPost {
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
			return
		}

		var update types.TelegramUpdate
		if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
			log.Printf("Failed to decode update: %v", err)
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		go botApp.HandleUpdate(&update) // Added HandleUpdate method to process updates

		w.WriteHeader(http.StatusOK)
	})

	port := ":" + os.Getenv("PORT")
	if port == ":" {
		port = ":8080"
	}
	log.Printf("Starting server on port %s...", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
```

**Purpose:**

- **Entry Point:** Serves as the entry point of the application.
- **Server Setup:** Initializes the application and sets up HTTP handlers to manage web requests and Telegram updates.
- **Routing:**
  - **GET Requests:** Handled by `HandleWebRequest` to serve full responses via the web interface.
  - **POST Requests:** Parsed as Telegram updates and processed asynchronously using `HandleUpdate`.

### internal/api/api_requests.go

```go
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
```

**Purpose:**

- **API Integration:** Manages interactions with OpenAI's GPT-4 API.
- **Request Construction:** Constructs and sends HTTP requests to the OpenAI API with the necessary payload.
- **Response Handling:** Parses the API response and extracts the generated content.
- **Error Management:** Handles various error scenarios, including request failures and unexpected responses.

### internal/app/app.go

```go
// internal/app/app.go

package app

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"KernelSandersBot/internal/api"
	"KernelSandersBot/internal/cache"
	"KernelSandersBot/internal/conversation"
	"KernelSandersBot/internal/handlers"
	"KernelSandersBot/internal/s3client"
	"KernelSandersBot/internal/telegram"
	"KernelSandersBot/internal/types"
	"KernelSandersBot/internal/usage"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/joho/godotenv"
	"github.com/russross/blackfriday/v2" // Added for Markdown parsing
	"golang.org/x/time/rate"
)

// Ensure App implements handlers.MessageProcessor
var _ handlers.MessageProcessor = (*App)(nil)

// App represents the main application with all necessary configurations and dependencies.
type App struct {
	TelegramToken        string
	OpenAIKey            string
	OpenAIEndpoint       string
	BotUsername          string
	Cache                *cache.Cache
	HTTPClient           *http.Client
	RateLimiter          *rate.Limiter
	S3BucketName         string
	S3Client             s3client.S3ClientInterface
	UsageCache           *usage.UsageCache
	NoLimitUsers         map[int]struct{}
	ConversationContexts *conversation.ConversationCache
	APIHandler           *api.APIHandler
	TelegramHandler      *telegram.TelegramHandler
	logMutex             sync.Mutex
	ResponseStore        *ResponseStore
}

// NewApp initializes the App with configurations from environment variables.
func NewApp() *App {
	// Load environment variables from .env file if present
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found. Proceeding with environment variables.")
	}

	// Parse NO_LIMIT_USERS
	noLimitUsersRaw := os.Getenv("NO_LIMIT_USERS")
	noLimitUsers := parseNoLimitUsers(noLimitUsersRaw)

	// Initialize AWS S3 Client
	s3Client := s3client.NewS3Client(os.Getenv("AWS_ENDPOINT_URL_S3"), os.Getenv("AWS_REGION"))

	// Initialize APIHandler for OpenAI
	apiHandler := api.NewAPIHandler(os.Getenv("OPENAI_KEY"), os.Getenv("OPENAI_ENDPOINT"))

	app := &App{
		TelegramToken:        os.Getenv("TELEGRAM_TOKEN"),
		OpenAIKey:            os.Getenv("OPENAI_KEY"),
		OpenAIEndpoint:       os.Getenv("OPENAI_ENDPOINT"),
		BotUsername:          os.Getenv("BOT_USERNAME"),
		Cache:                cache.NewCache(),
		HTTPClient:           &http.Client{Timeout: 15 * time.Second},
		RateLimiter:          rate.NewLimiter(rate.Every(time.Second), 5),
		S3BucketName:         os.Getenv("BUCKET_NAME"),
		S3Client:             s3Client,
		UsageCache:           usage.NewUsageCache(),
		NoLimitUsers:         noLimitUsers,
		ConversationContexts: conversation.NewConversationCache(),
		APIHandler:           apiHandler,
		logMutex:             sync.Mutex{},
		ResponseStore:        NewResponseStore(),
	}

	if app.BotUsername == "" {
		log.Println("Warning: BOT_USERNAME environment variable is missing. The bot will not respond to mentions.")
	} else {
		log.Printf("Bot username is set to: %s", app.BotUsername)
	}

	// Initialize TelegramHandler with the App as the MessageProcessor
	app.TelegramHandler = telegram.NewTelegramHandler(app)

	return app
}

// parseNoLimitUsers parses the NO_LIMIT_USERS environment variable into a map of user IDs.
func parseNoLimitUsers(raw string) map[int]struct{} {
	userMap := make(map[int]struct{})
	ids := strings.Split(raw, ",")
	for _, idStr := range ids {
		idStr = strings.TrimSpace(idStr)
		if id, err := strconv.Atoi(idStr); err == nil {
			userMap[id] = struct{}{}
		}
	}
	return userMap
}

// GetBotUsername returns the bot's username.
func (a *App) GetBotUsername() string {
	return a.BotUsername
}

// EscapeHTML escapes all HTML special characters in the text.
func EscapeHTML(text string) string {
	return html.EscapeString(text)
}

// HandleWebRequest handles web requests to serve the full response with enhanced formatting and expiration time.
func (a *App) HandleWebRequest(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	responseText, exists := a.ResponseStore.GetResponse(path)
	expirationTime, _ := a.ResponseStore.GetExpirationTime(path)
	timeRemaining := time.Until(expirationTime)

	if !exists {
		http.Error(w, "Response not found or expired.", http.StatusNotFound)
		return
	}

	// Convert Markdown to HTML using blackfriday
	parsedHTML := blackfriday.Run([]byte(responseText))

	// Enhance HTML formatting for better readability
	formattedText := fmt.Sprintf(`
		<!DOCTYPE html>
		<html lang="en">
		<head>
			<meta charset="UTF-8">
			<title>KernelSanders is finger lickin' good :)</title>
			<style>
				body {
					font-family: Arial, sans-serif;
					margin: 20px;
					background-color: #f9f9f9;
					color: #333;
				}
				h1 {
					color: #4CAF50;
				}
				.container {
					background-color: #fff;
					padding: 20px;
					border-radius: 5px;
					box-shadow: 0 2px 4px rgba(0,0,0,0.1);
				}
				pre {
					background-color: #f4f4f4;
					padding: 10px;
					border-radius: 3px;
					overflow-x: auto;
				}
				code {
					background-color: #f4f4f4;
					padding: 2px 4px;
					border-radius: 3px;
				}
				.note {
					font-style: italic;
					color: #777;
				}
			</style>
		</head>
		<body>
			<div class="container">
				<h1>KernelSanders is finger lickin' good :)</h1>
				<p><strong>Time Remaining:</strong> %s</p>
				<hr>
				<div>%s</div>
				<p class="note">**Note:** Please save this content elsewhere as it will expire soon.</p>
			</div>
		</body>
		</html>
	`, timeRemaining.Truncate(time.Second).String(), string(parsedHTML))

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, formattedText)
}

// ProcessMessage processes a user's message, queries OpenAI, sends the response, and logs the interaction.
func (a *App) ProcessMessage(chatID int64, userID int, username, userQuestion string, messageID int) error {
	// Rate limit check
	isNoLimitUser := false
	if _, ok := a.NoLimitUsers[userID]; ok {
		isNoLimitUser = true
	}

	if !isNoLimitUser && !a.UsageCache.CanUserChat(userID) {
		// Calculate remaining time until limit reset
		timeRemaining := a.UsageCache.TimeUntilLimitReset(userID)
		minutes := int(timeRemaining.Minutes())
		seconds := int(timeRemaining.Seconds()) % 60

		limitMsg := fmt.Sprintf(
			"Rate limit exceeded. Please try again in %d minutes and %d seconds.",
			minutes, seconds,
		)
		if err := a.SendMessage(chatID, limitMsg, messageID); err != nil {
			log.Printf("Failed to send rate limit message to Telegram: %v", err)
		}

		// Log the attempt to S3
		a.logToS3(userID, username, userQuestion, "", isNoLimitUser)
		return fmt.Errorf("user rate limited")
	}

	a.UsageCache.AddUsage(userID)

	// Maintain conversation context
	conversationKey := fmt.Sprintf("user_%d", userID)
	var messages []types.OpenAIMessage
	if history, exists := a.ConversationContexts.Get(conversationKey); exists {
		if err := json.Unmarshal([]byte(history), &messages); err != nil {
			log.Printf("Failed to unmarshal conversation history: %v", err)
			messages = []types.OpenAIMessage{
				{Role: "system", Content: "You are a helpful assistant."},
			}
		}
	} else {
		// Initialize with system prompt
		messages = []types.OpenAIMessage{
			{Role: "system", Content: "You are a helpful assistant."},
		}
	}

	// Append the new user message
	messages = append(messages, types.OpenAIMessage{Role: "user", Content: userQuestion})

	// Query OpenAI
	startTime := time.Now()

	responseText, err := a.APIHandler.QueryOpenAIWithMessages(messages)
	if err != nil {
		log.Printf("OpenAI query failed: %v", err)
		return err
	}

	responseTime := time.Since(startTime).Milliseconds()

	// Append assistant's response to messages
	messages = append(messages, types.OpenAIMessage{Role: "assistant", Content: responseText})

	// Update conversation context
	messagesJSON, _ := json.Marshal(messages)
	a.ConversationContexts.Set(conversationKey, string(messagesJSON))

	// Store the full response in the ResponseStore
	responseID := a.ResponseStore.StoreResponse(responseText)

	// Escape HTML in responseText
	escapedResponse := EscapeHTML(responseText)

	// Prepare final message with truncation if necessary
	var finalMessage string
	link := a.GenerateResponseURL(responseID)
	linkLength := len(link) + len(`<a href=""></a>`) // Account for HTML tags
	maxTelegramLength := 4096
	if len(escapedResponse)+linkLength > maxTelegramLength {
		// Truncate the message to accommodate the link
		truncatedLength := maxTelegramLength - linkLength - len("\n\n")
		if truncatedLength < 0 {
			truncatedLength = 0
		}
		truncatedResponse := escapedResponse
		if len(escapedResponse) > truncatedLength {
			truncatedResponse = escapedResponse[:truncatedLength] + "..."
		}
		finalMessage = fmt.Sprintf("%s\n\n<a href=\"%s\">View full response</a>", truncatedResponse, link)
	} else {
		// Message is within limit; append the link
		finalMessage = fmt.Sprintf("%s\n\n<a href=\"%s\">View full response</a>", escapedResponse, link)
	}

	// Send the message to Telegram with HTML parse mode
	if err := a.SendMessage(chatID, finalMessage, messageID); err != nil {
		log.Printf("Failed to send message to Telegram: %v", err)
		return err
	}

	// Log the interaction in S3
	a.logToS3(userID, username, userQuestion, fmt.Sprintf("%d ms", responseTime), isNoLimitUser)
	return nil
}

// GenerateResponseURL generates the URL for the stored response.
func (a *App) GenerateResponseURL(responseID string) string {
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	return fmt.Sprintf("%s/%s", baseURL, responseID)
}

// SendMessage sends a plain text message to a Telegram chat.
func (a *App) SendMessage(chatID int64, text string, replyToMessageID int) error {
	return a.sendMessage(chatID, text, replyToMessageID)
}

// sendMessage sends a message to a Telegram chat using HTML parse mode.
func (a *App) sendMessage(chatID int64, text string, replyToMessageID int) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", a.TelegramToken)
	payload := map[string]interface{}{
		"chat_id":                  chatID,
		"text":                     text,
		"disable_web_page_preview": false,
		"parse_mode":               "HTML", // Changed from "MarkdownV2" to "HTML"
	}

	if replyToMessageID != 0 {
		payload["reply_to_message_id"] = replyToMessageID
	}

	reqBody, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status: %s - %s", resp.Status, string(bodyBytes))
	}

	return nil
}

// HandleUpdate handles incoming Telegram updates by delegating to TelegramHandler.
func (a *App) HandleUpdate(update *types.TelegramUpdate) {
	_, err := a.TelegramHandler.HandleTelegramMessage(update)
	if err != nil {
		log.Printf("Error handling Telegram update: %v", err)
	}
}

// HandleCommand processes Telegram commands.
func (a *App) HandleCommand(message *types.TelegramMessage, userID int, username string) (string, error) {
	switch message.Text {
	case "/start":
		welcomeMsg := "Welcome to Kernel Sanders Bot! How can I assist you today?"
		err := a.SendMessage(message.Chat.ID, welcomeMsg, message.MessageID)
		return "", err
	case "/help":
		helpMsg := "Available commands:\n/start - Start the bot\n/help - Show this help message"
		err := a.SendMessage(message.Chat.ID, helpMsg, message.MessageID)
		return "", err
	default:
		unknownCmd := "Unknown command. Type /help to see available commands."
		err := a.SendMessage(message.Chat.ID, unknownCmd, message.MessageID)
		return "", err
	}
}

// logToS3 logs user interactions to an S3 bucket with details about rate limiting and usage.
func (a *App) logToS3(userID int, username, userPrompt, responseTime string, isNoLimitUser bool) {
	a.logMutex.Lock()
	defer a.logMutex.Unlock()

	record := []string{
		fmt.Sprintf("%d", userID),
		username,
		userPrompt,
		responseTime,
		fmt.Sprintf("No limit user: %t", isNoLimitUser),
	}

	bucketName := a.S3BucketName
	objectKey := "logs/telegram_logs.csv"

	resp, err := a.S3Client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectKey),
	})

	var existingData [][]string
	if err == nil {
		defer resp.Body.Close()
		bodyBytes, err := io.ReadAll(resp.Body)
		if err == nil && len(bodyBytes) > 0 {
			reader := csv.NewReader(bytes.NewReader(bodyBytes))
			existingData, err = reader.ReadAll()
			if err != nil {
				log.Printf("Failed to parse existing CSV: %v", err)
				existingData = [][]string{}
			}
		}
	} else {
		log.Printf("Failed to get existing CSV from S3: %v. A new CSV will be created.", err)
	}

	if len(existingData) == 0 {
		headers := []string{
			"userID",
			"username",
			"prompt",
			"response_time",
			"no_limit_user",
		}
		existingData = append(existingData, headers)
	}

	existingData = append(existingData, record)

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if err := w.WriteAll(existingData); err != nil {
		log.Printf("Failed to write CSV data to buffer: %v", err)
		return
	}

	_, err = a.S3Client.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectKey),
		Body:   bytes.NewReader(buf.Bytes()),
	})

	if err != nil {
		log.Printf("Failed to upload updated CSV to S3: %v", err)
	} else {
		log.Printf("Successfully appended log data to S3 CSV")
	}
}
```

**Purpose:**

- **Application Logic:** Serves as the core of the bot, handling message processing, interaction with OpenAI, rate limiting, and logging.
- **Web Handling:** Manages HTTP requests for both the web interface and Telegram updates.
- **Message Processing:** Integrates with the Telegram handler to process user messages and commands.
- **Rate Limiting & Logging:** Enforces usage limits and logs interactions to AWS S3 for monitoring.

### internal/app/response_store.go

```go
// internal/app/response_store.go

package app

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// ResponseStore manages stored responses with expiration tracking.
type ResponseStore struct {
	responses map[string]responseEntry
	mutex     sync.RWMutex
}

// responseEntry represents a response's content and expiration time.
type responseEntry struct {
	content   string
	expiresAt time.Time
}

// NewResponseStore initializes the ResponseStore and begins the cleanup routine.
func NewResponseStore() *ResponseStore {
	rs := &ResponseStore{
		responses: make(map[string]responseEntry),
	}
	go rs.cleanupExpiredResponses()
	return rs
}

// StoreResponse stores the response content and returns a unique ID for retrieval.
func (rs *ResponseStore) StoreResponse(content string) string {
	rs.mutex.Lock()
	defer rs.mutex.Unlock()

	id := uuid.New().String()
	rs.responses[id] = responseEntry{
		content:   content,
		expiresAt: time.Now().Add(4 * time.Hour), // Set expiration to 4 hours
	}
	return id
}

// GetResponse retrieves the response content by ID if it hasn't expired.
func (rs *ResponseStore) GetResponse(id string) (string, bool) {
	rs.mutex.RLock()
	defer rs.mutex.RUnlock()

	entry, exists := rs.responses[id]
	if !exists || time.Now().After(entry.expiresAt) {
		return "", false
	}
	return entry.content, true
}

// GetExpirationTime returns the expiration time of a stored response by ID.
func (rs *ResponseStore) GetExpirationTime(id string) (time.Time, bool) {
	rs.mutex.RLock()
	defer rs.mutex.RUnlock()

	entry, exists := rs.responses[id]
	if !exists {
		return time.Time{}, false
	}
	return entry.expiresAt, true
}

// cleanupExpiredResponses periodically removes expired responses from the store.
func (rs *ResponseStore) cleanupExpiredResponses() {
	ticker := time.NewTicker(10 * time.Minute)
	for range ticker.C {
		rs.mutex.Lock()
		for id, entry := range rs.responses {
			if time.Now().After(entry.expiresAt) {
				delete(rs.responses, id)
			}
		}
		rs.mutex.Unlock()
	}
}
```

**Purpose:**

- **Response Management:** Stores full responses from OpenAI and assigns unique IDs for web retrieval.
- **Expiration Tracking:** Ensures that stored responses expire after a set duration (4 hours) to manage storage and privacy.
- **Cleanup Routine:** Periodically cleans up expired responses to free memory and maintain efficiency.

### internal/cache/cache.go

```go
// internal/cache/cache.go

package cache

import (
	"sync"
	"time"
)

// Cache represents a thread-safe in-memory cache.
type Cache struct {
	data  map[string]string
	mutex sync.RWMutex
}

// NewCache initializes and returns a new Cache instance.
func NewCache() *Cache {
	return &Cache{
		data: make(map[string]string),
	}
}

// Get retrieves the value associated with the given key.
func (c *Cache) Get(key string) (string, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	val, exists := c.data[key]
	return val, exists
}

// Set assigns a value to the given key in the cache.
func (c *Cache) Set(key, value string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.data[key] = value
}

// StartEviction periodically removes expired entries from the cache.
// Currently, it deletes all entries at each interval.
// Implement TTL checks or other eviction policies as needed.
func (c *Cache) StartEviction(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	go func() {
		for range ticker.C {
			c.mutex.Lock()
			for key := range c.data {
				// TODO: Implement TTL checks or other eviction policies
				// Example: delete all entries (replace with actual logic)
				delete(c.data, key)
			}
			c.mutex.Unlock()
		}
	}()
}
```

**Purpose:**

- **In-Memory Caching:** Provides a simple thread-safe cache for storing key-value pairs.
- **Eviction Policy:** Currently, the eviction logic removes all entries at specified intervals. Placeholder comments suggest implementing more sophisticated eviction policies like TTL (Time-To-Live) as needed.

### internal/conversation/conversation_cache.go

```go
// internal/conversation/conversation_cache.go

package conversation

import (
	"sync"
	"time"
)

// ConversationCache manages conversation contexts with expiration.
type ConversationCache struct {
	data      map[string]conversationEntry
	mutex     sync.RWMutex
	expiry    time.Duration
	cleanupCh chan struct{}
}

// conversationEntry stores conversation data along with the last updated timestamp.
type conversationEntry struct {
	data     string
	lastSeen time.Time
}

// NewConversationCache initializes a new ConversationCache.
func NewConversationCache() *ConversationCache {
	cc := &ConversationCache{
		data:      make(map[string]conversationEntry),
		expiry:    30 * time.Minute, // Context expires after 30 minutes of inactivity
		cleanupCh: make(chan struct{}),
	}
	go cc.cleanupExpiredContexts()
	return cc
}

// Set stores a conversation context with the current timestamp.
func (cc *ConversationCache) Set(key, value string) {
	cc.mutex.Lock()
	defer cc.mutex.Unlock()
	cc.data[key] = conversationEntry{
		data:     value,
		lastSeen: time.Now(),
	}
}

// Get retrieves a conversation context if it's not expired.
func (cc *ConversationCache) Get(key string) (string, bool) {
	cc.mutex.RLock()
	defer cc.mutex.RUnlock()
	entry, exists := cc.data[key]
	if !exists {
		return "", false
	}
	if time.Since(entry.lastSeen) > cc.expiry {
		return "", false
	}
	return entry.data, true
}

// cleanupExpiredContexts periodically removes expired contexts.
func (cc *ConversationCache) cleanupExpiredContexts() {
	ticker := time.NewTicker(cc.expiry)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			cc.mutex.Lock()
			for key, entry := range cc.data {
				if time.Since(entry.lastSeen) > cc.expiry {
					delete(cc.data, key)
				}
			}
			cc.mutex.Unlock()
		case <-cc.cleanupCh:
			return
		}
	}
}

// Close stops the cleanup goroutine.
func (cc *ConversationCache) Close() {
	close(cc.cleanupCh)
}
```

**Purpose:**

- **Conversation Context Management:** Maintains conversation contexts for each user to ensure coherent interactions.
- **Expiration Tracking:** Conversations expire after 30 minutes of inactivity to manage memory and privacy.
- **Cleanup Routine:** Periodically removes expired conversation contexts to free resources.

### internal/handlers/handlers.go

```go
// internal/handlers/handlers.go

package handlers

import "KernelSandersBot/internal/types"

// MessageProcessor defines the methods that the telegram package requires from the app package.
type MessageProcessor interface {
	ProcessMessage(chatID int64, userID int, username string, userQuestion string, messageID int) error
	HandleCommand(message *types.TelegramMessage, userID int, username string) (string, error)
	SendMessage(chatID int64, text string, replyToMessageID int) error
	GetBotUsername() string
}
```

**Purpose:**

- **Interface Definition:** Defines the `MessageProcessor` interface that specifies the methods required by the Telegram handler.
- **Abstraction:** Allows for decoupling between the Telegram handler and the core application logic, promoting modularity and testability.

### internal/s3client/s3client.go

```go
// internal/s3client/s3client.go

package s3client

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

// S3ClientInterface defines methods for S3 interactions
type S3ClientInterface interface {
	GetObject(input *s3.GetObjectInput) (*s3.GetObjectOutput, error)
	PutObject(input *s3.PutObjectInput) (*s3.PutObjectOutput, error)
}

// S3Client is an implementation of S3ClientInterface for AWS S3
type S3Client struct {
	s3Svc *s3.S3
}

// NewS3Client initializes a new S3 client
func NewS3Client(endpoint, region string) *S3Client {
	sess := session.Must(session.NewSession(&aws.Config{
		Endpoint: aws.String(endpoint),
		Region:   aws.String(region),
	}))
	return &S3Client{
		s3Svc: s3.New(sess),
	}
}

// GetObject retrieves an object from the specified S3 bucket
func (c *S3Client) GetObject(input *s3.GetObjectInput) (*s3.GetObjectOutput, error) {
	return c.s3Svc.GetObject(input)
}

// PutObject uploads an object to the specified S3 bucket
func (c *S3Client) PutObject(input *s3.PutObjectInput) (*s3.PutObjectOutput, error) {
	return c.s3Svc.PutObject(input)
}
```

**Purpose:**

- **AWS S3 Integration:** Provides an interface and implementation for interacting with AWS S3, specifically for logging purposes.
- **Abstraction:** Defines `S3ClientInterface` to allow for easy mocking and testing.
- **Functionality:** Implements methods to retrieve (`GetObject`) and upload (`PutObject`) objects to an S3 bucket.

### internal/telegram/telegram_handler.go

```go
// internal/telegram/telegram_handler.go

package telegram

import (
	"log"
	"strings"

	"KernelSandersBot/internal/handlers"
	"KernelSandersBot/internal/types"
)

type TelegramHandler struct {
	Processor handlers.MessageProcessor
}

func NewTelegramHandler(processor handlers.MessageProcessor) *TelegramHandler {
	return &TelegramHandler{
		Processor: processor,
	}
}

func (th *TelegramHandler) HandleTelegramMessage(update *types.TelegramUpdate) (string, error) {
	var message *types.TelegramMessage

	if update.Message != nil {
		message = update.Message
	} else if update.EditedMessage != nil {
		message = update.EditedMessage
	} else {
		return "", nil
	}

	if message.Chat.ID == 0 || message.Text == "" {
		return "", nil
	}

	chatID := message.Chat.ID
	userQuestion := message.Text
	messageID := message.MessageID
	userID := message.From.ID
	username := message.From.Username

	if strings.HasPrefix(message.Text, "/") {
		_, err := th.Processor.HandleCommand(message, userID, username)
		if err != nil {
			log.Printf("Error handling command: %v", err)
			return "", nil
		}
		return "", nil
	}

	isReply := message.ReplyToMessage != nil
	isTagged := false
	if len(message.Entities) > 0 {
		for _, entity := range message.Entities {
			if entity.Type == "mention" {
				if entity.Offset+entity.Length > len(message.Text) {
					continue
				}
				mention := message.Text[entity.Offset : entity.Offset+entity.Length]
				if isTaggedMention(mention, th.Processor.GetBotUsername()) {
					isTagged = true
					userQuestion = removeMention(userQuestion, mention)
					break
				}
			}
		}
	}

	if !isTagged && !(isReply && message.ReplyToMessage.From.IsBot) && message.Chat.Type != "private" {
		return "", nil
	}

	if err := th.Processor.ProcessMessage(chatID, userID, username, userQuestion, messageID); err != nil {
		log.Printf("Error processing message: %v", err)
		return "", nil
	}

	return "", nil
}

func isTaggedMention(mention, botUsername string) bool {
	return strings.ToLower(mention) == "@"+strings.ToLower(botUsername)
}

func removeMention(text, mention string) string {
	return strings.TrimSpace(strings.Replace(text, mention, "", 1))
}
```

**Purpose:**

- **Telegram Message Handling:** Processes incoming Telegram messages and determines appropriate actions based on the content.
- **Command Processing:** Detects and handles commands (e.g., `/start`, `/help`) by delegating to the `HandleCommand` method.
- **Mention Detection:** Identifies if the bot is mentioned in group chats and processes the message accordingly.
- **Interaction Delegation:** Passes user messages to the `ProcessMessage` method for further handling by the core application.

### internal/types/types.go

```go
// internal/types/types.go

package types

// TelegramUpdate represents an incoming update from Telegram.
type TelegramUpdate struct {
	UpdateID      int              `json:"update_id"`
	Message       *TelegramMessage `json:"message,omitempty"`
	EditedMessage *TelegramMessage `json:"edited_message,omitempty"`
	ChannelPost   *TelegramMessage `json:"channel_post,omitempty"`
}

// TelegramMessage represents a message in Telegram.
type TelegramMessage struct {
	MessageID      int              `json:"message_id"`
	From           TelegramUser     `json:"from"`
	Chat           TelegramChat     `json:"chat"`
	Date           int              `json:"date"`
	Text           string           `json:"text"`
	Entities       []TelegramEntity `json:"entities,omitempty"`
	ReplyToMessage *TelegramMessage `json:"reply_to_message,omitempty"`
}

// TelegramUser represents a user in Telegram.
type TelegramUser struct {
	ID           int    `json:"id"`
	IsBot        bool   `json:"is_bot"`
	FirstName    string `json:"first_name"`
	Username     string `json:"username,omitempty"`
	LanguageCode string `json:"language_code,omitempty"`
}

// TelegramChat represents a chat in Telegram.
type TelegramChat struct {
	ID                          int64  `json:"id"`
	Type                        string `json:"type"`
	Title                       string `json:"title,omitempty"`
	FirstName                   string `json:"first_name,omitempty"`
	Username                    string `json:"username,omitempty"`
	AllMembersAreAdministrators bool   `json:"all_members_are_administrators,omitempty"`
}

// TelegramEntity represents an entity in a Telegram message.
type TelegramEntity struct {
	Offset int    `json:"offset"`
	Length int    `json:"length"`
	Type   string `json:"type"`
}

// OpenAIMessage represents a message in the OpenAI conversation.
type OpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OpenAIQuery represents the payload sent to OpenAI's API.
type OpenAIQuery struct {
	Model       string          `json:"model"`
	Messages    []OpenAIMessage `json:"messages"`
	Temperature float64         `json:"temperature"`
	MaxTokens   int             `json:"max_tokens"`
}

// OpenAIResponse represents the response received from OpenAI's API.
type OpenAIResponse struct {
	ID      string                 `json:"id"`
	Object  string                 `json:"object"`
	Created int                    `json:"created"`
	Model   string                 `json:"model"`
	Choices []OpenAIResponseChoice `json:"choices"`
	Usage   OpenAIUsage            `json:"usage"`
}

// OpenAIResponseChoice represents a single choice in OpenAI's response.
type OpenAIResponseChoice struct {
	Index        int           `json:"index"`
	Message      OpenAIMessage `json:"message"`
	FinishReason string        `json:"finish_reason"`
}

// OpenAIUsage represents token usage information from OpenAI's response.
type OpenAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}
```

**Purpose:**

- **Data Structures:** Defines various data structures used across the application, including representations of Telegram updates, messages, users, chats, and OpenAI API interactions.
- **Type Definitions:** Ensures type safety and consistency when handling data from external APIs and Telegram.

### internal/usage/usage_cache.go

```go
// internal/usage/usage_cache.go

package usage

import (
	"sync"
	"time"
)

// UsageCache tracks user message usage for rate limiting.
type UsageCache struct {
	users    map[int][]time.Time
	mutex    sync.Mutex
	limit    int
	duration time.Duration
}

// NewUsageCache initializes a new UsageCache.
func NewUsageCache() *UsageCache {
	return &UsageCache{
		users:    make(map[int][]time.Time),
		limit:    10,               // Default limit of 10 messages per duration
		duration: 10 * time.Minute, // 10-minute window
	}
}

// CanUserChat checks if a user is allowed to send a message based on usage in the last duration.
func (u *UsageCache) CanUserChat(userID int) bool {
	u.mutex.Lock()
	defer u.mutex.Unlock()

	// Filter out old timestamps.
	validTimes := u.filterRecentMessages(userID)
	u.users[userID] = validTimes

	// Check if user has exceeded the limit.
	return len(validTimes) < u.limit
}

// AddUsage records a new message usage for the user.
func (u *UsageCache) AddUsage(userID int) {
	u.mutex.Lock()
	defer u.mutex.Unlock()

	u.users[userID] = append(u.users[userID], time.Now())
}

// TimeUntilLimitReset calculates the time remaining until the rate limit is lifted.
func (u *UsageCache) TimeUntilLimitReset(userID int) time.Duration {
	u.mutex.Lock()
	defer u.mutex.Unlock()

	validTimes := u.filterRecentMessages(userID)
	if len(validTimes) < u.limit {
		return 0 // No limit currently in place.
	}

	// Calculate time remaining until the oldest timestamp falls outside the duration window.
	oldestTime := validTimes[0]
	return u.duration - time.Since(oldestTime)
}

// filterRecentMessages filters messages within the allowed duration.
func (u *UsageCache) filterRecentMessages(userID int) []time.Time {
	if _, exists := u.users[userID]; !exists {
		u.users[userID] = []time.Time{}
		return u.users[userID]
	}

	validTimes := []time.Time{}
	for _, t := range u.users[userID] {
		if time.Since(t) <= u.duration {
			validTimes = append(validTimes, t)
		}
	}
	return validTimes
}
```

**Purpose:**

- **Rate Limiting:** Implements rate limiting to control the number of messages a user can send within a specified time frame (default is 10 messages per 10 minutes).
- **Usage Tracking:** Tracks the timestamps of user messages to determine if they exceed the allowed limit.
- **Limit Reset Calculation:** Provides functionality to calculate the time remaining until a user's rate limit is lifted.

### internal/utils/utils.go

```go
// internal/utils/utils.go

package utils

// SummarizeToLength trims the text to the specified maximum length.
func SummarizeToLength(text string, maxLength int) string {
	if len(text) <= maxLength {
		return text
	}
	return text[:maxLength]
}
```

**Purpose:**

- **Utility Function:** Provides a simple utility function to truncate text to a specified maximum length, useful for ensuring messages comply with size constraints.

## Logging

**KernelSandersBot** logs all user interactions to an AWS S3 bucket in CSV format. Each log entry includes:

- `userID`: Telegram user ID.
- `username`: Telegram username.
- `prompt`: User's message.
- `response_time`: Time taken by OpenAI to generate a response.
- `no_limit_user`: Boolean indicating if the user is exempt from rate limiting.

**Logging Workflow:**

1. **Interaction Capture:** When a user sends a message, the bot processes it and generates a response.
2. **Logging:** The interaction details are appended to a CSV file stored in the specified S3 bucket.
3. **Duplication Prevention:** The application ensures that log entries are not duplicated and handles CSV parsing and writing efficiently.

**Benefits:**

- **Monitoring:** Enables tracking of user interactions and bot performance.
- **Analytics:** Facilitates analysis of usage patterns and response times.
- **Audit Trails:** Provides an audit trail for compliance and debugging purposes.

## Rate Limiting

To prevent abuse and manage resources effectively, **KernelSandersBot** enforces rate limiting based on user activity.

**Rate Limiting Details:**

- **Default Limit:** 10 messages per 10 minutes.
- **Exempt Users:** Users specified in the `NO_LIMIT_USERS` environment variable are exempt from rate limiting.
- **Limit Reset:** After exceeding the limit, users must wait until their oldest message within the time window expires.

**User Experience:**

- **Exceeded Limit Notification:** If a user exceeds the rate limit, the bot sends a message indicating the remaining time until they can send messages again.
- **No Limit Users:** Exempt users can interact with the bot without restrictions.

**Implementation:**

- **UsageCache:** Utilizes the `UsageCache` struct to track and manage user message counts and timings.
- **Synchronization:** Ensures thread-safe operations using mutexes to handle concurrent access.

## Response Storage

**KernelSandersBot** stores full responses from OpenAI in an in-memory `ResponseStore` with expiration tracking.

**Key Features:**

- **Unique Identification:** Each response is assigned a unique UUID for retrieval via the web interface.
- **Expiration:** Responses expire after 4 hours, ensuring timely cleanup and privacy.
- **Cleanup Routine:** A background goroutine periodically removes expired responses to manage memory usage.

**Web Interface Integration:**

- **Response Links:** If a response exceeds Telegram's 4096-character limit, the bot truncates the message and provides a link to view the full response on the web interface.
- **No Length Limit:** The web interface serves the complete response without any character limitations, ensuring users can access the entire content.

## Web Interface

**KernelSandersBot** includes a web interface that serves full responses with enhanced Markdown formatting for better readability.

**Features:**

- **Markdown Rendering:** Converts Markdown content to HTML using the `blackfriday` library, allowing for formatted text, headings, lists, and code blocks.
- **Styled Layout:** Applies CSS styling to create a clean and user-friendly interface resembling a knowledge base.
- **Expiration Notice:** Displays a note indicating that the content will expire soon, encouraging users to save important information.
- **Custom Title:** The web page title is set to "KernelSanders is finger lickin' good :)", aligning with the bot's branding.

**Accessing Responses:**

- Users receive a link in Telegram to view the full response on the web interface. This ensures that even lengthy responses are accessible without violating Telegram's message size constraints.

## Dependencies

**KernelSandersBot** relies on several Go packages and external services to function effectively.

### Go Packages

- **Standard Library Packages:**
  - `net/http`: For HTTP server and client functionalities.
  - `encoding/json`: For JSON encoding and decoding.
  - `log`: For logging.
  - `sync`: For synchronization primitives.
  - `time`: For time-related operations.
  - `fmt`, `bytes`, `io`, `os`, `strings`, etc.: For various utility functions.

- **Third-Party Packages:**
  - [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go): AWS SDK for interacting with S3.
  - [`github.com/joho/godotenv`](https://github.com/joho/godotenv): For loading environment variables from a `.env` file.
  - [`github.com/russross/blackfriday/v2`](https://github.com/russross/blackfriday): For converting Markdown to HTML.
  - [`golang.org/x/time/rate`](https://pkg.go.dev/golang.org/x/time/rate): For implementing rate limiting.
  - [`github.com/google/uuid`](https://github.com/google/uuid): For generating unique IDs.

### External Services

- **Telegram API:** For receiving and sending messages via the Telegram bot.
- **OpenAI API:** For generating intelligent responses using GPT-4.
- **AWS S3:** For storing logs of user interactions.

## Contributing

Contributions are welcome! If you find any issues or have suggestions for improvements, please open an issue or submit a pull request.

### Steps to Contribute

1. **Fork the Repository**

   Click the "Fork" button at the top-right corner of the repository page to create a personal copy.

2. **Clone Your Fork**

   ```bash
   git clone https://github.com/yourusername/KernelSandersBot.git
   cd KernelSandersBot
   ```

3. **Create a New Branch**

   ```bash
   git checkout -b feature/your-feature-name
   ```

4. **Make Changes**

   Implement your feature or bug fix, ensuring adherence to the project's coding standards.

5. **Commit Your Changes**

   ```bash
   git add .
   git commit -m "Add feature: your feature description"
   ```

6. **Push to Your Fork**

   ```bash
   git push origin feature/your-feature-name
   ```

7. **Create a Pull Request**

   Navigate to your fork on GitHub and click the "Compare & pull request" button. Provide a clear description of your changes.

## License

This project is licensed under the [MIT License](LICENSE). You are free to use, modify, and distribute it as per the terms of the license.
```

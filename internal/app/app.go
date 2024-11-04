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

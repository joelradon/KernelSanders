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
	"KernelSandersBot/internal/utils"

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
	ShutdownChan         chan struct{} // Channel to signal shutdown
	wg                   sync.WaitGroup
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
	log.Printf("OpenAI API Endpoint URL: %s", apiHandler.EndpointURL) // Confirm the endpoint

	// Initialize ResponseStore with S3Client for persistent storage
	responseStore := NewResponseStore(s3Client, os.Getenv("BUCKET_NAME"))

	// Load existing web responses from S3 into ResponseStore to ensure persistence across restarts
	if err := responseStore.LoadResponsesFromS3(); err != nil {
		log.Printf("Failed to load responses from S3: %v", err)
	}

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
		ResponseStore:        responseStore,
		ShutdownChan:         make(chan struct{}),
	}

	if app.BotUsername == "" {
		log.Println("Warning: BOT_USERNAME environment variable is missing. The bot will not respond to mentions.")
	} else {
		log.Printf("Bot username is set to: %s", app.BotUsername)
	}

	// Initialize TelegramHandler with the App as the MessageProcessor
	app.TelegramHandler = telegram.NewTelegramHandler(app)

	// Start the cleanup goroutine for ResponseStore
	// Note: The cleanup is handled within ResponseStore, so no additional cleanup is needed here.

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

// GetTelegramToken returns the Telegram bot token.
func (a *App) GetTelegramToken() string {
	return a.TelegramToken
}

// EscapeHTML escapes all HTML special characters in the text.
func EscapeHTML(text string) string {
	return html.EscapeString(text)
}

// HandleWebRequest handles web requests to serve the full response with enhanced formatting and expiration time.
func (a *App) HandleWebRequest(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	responseText, exists := a.ResponseStore.GetResponse(path)
	createdAt, hasCreated := a.ResponseStore.GetCreationTime(path)
	expiresAt, hasExpires := a.ResponseStore.GetExpirationTime(path)
	timeRemaining := time.Until(expiresAt)

	if !exists || !hasCreated || !hasExpires {
		http.Error(w, "Response not found or expired.", http.StatusNotFound)
		return
	}

	// Convert Markdown to HTML using blackfriday
	parsedHTML := blackfriday.Run([]byte(responseText))

	// Format creation and deletion times
	creationTimeUTC := createdAt.UTC().Format(time.RFC1123)
	creationTimeEDT := createdAt.In(time.FixedZone("EDT", -4*3600)).Format(time.RFC1123)
	deletionTimeUTC := expiresAt.UTC().Format(time.RFC1123)
	deletionTimeEDT := expiresAt.In(time.FixedZone("EDT", -4*3600)).Format(time.RFC1123)

	// Enhance HTML formatting for better readability and add toggle buttons
	// Removed "Toggle Theme" button and renamed "Toggle Raw" to "View RAW" with explanation
	formattedText := fmt.Sprintf(
		`<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="UTF-8">
	<title>KernelSanders is finger lickin' good :)</title>
	<style>
		body {
			font-family: Arial, sans-serif;
			margin: 20px;
			background-color: #121212;
			color: #e0e0e0;
			transition: background-color 0.3s, color 0.3s;
		}
		h1 {
			color: #bb86fc;
		}
		.container {
			background-color: #1e1e1e;
			padding: 20px;
			border-radius: 5px;
			box-shadow: 0 2px 4px rgba(255,255,255,0.1);
		}
		pre {
			background-color: #2c2c2c;
			padding: 10px;
			border-radius: 3px;
			overflow-x: auto;
		}
		code {
			background-color: #2c2c2c;
			padding: 2px 4px;
			border-radius: 3px;
		}
		.note {
			font-style: italic;
			color: #a0a0a0;
		}
		.view-raw-button {
			margin-top: 10px;
			padding: 5px 10px;
			background-color: #bb86fc;
			color: #121212;
			border: none;
			border-radius: 3px;
			cursor: pointer;
		}
	</style>
	<script>
		function toggleRaw() {
			var rawContent = document.getElementById("raw-content");
			if (rawContent.style.display === "none") {
				rawContent.style.display = "block";
			} else {
				rawContent.style.display = "none";
			}
		}
	</script>
</head>
<body>
	<div class="container">
		<h1>KernelSanders is finger lickin' good :)</h1>
		<p><strong>Created At:</strong> UTC: %s | EDT: %s</p>
		<p><strong>Deletion Time:</strong> UTC: %s | EDT: %s</p>
		<p><strong>Time Remaining:</strong> %s</p>
		<button class="view-raw-button" onclick="toggleRaw()">View RAW</button>
		<hr>
		<div id="formatted-content">%s</div>
		<div id="raw-content" style="display:none;">
			<pre><code>%s</code></pre>
		</div>
		<p class="note">**Note:** Please save this content elsewhere as it will expire soon.</p>
		<p class="note">To export or save this response for later, you can copy the RAW view or use your browser's save functionality.</p>
	</div>
</body>
</html>`,
		creationTimeUTC, creationTimeEDT, deletionTimeUTC, deletionTimeEDT, timeRemaining.Truncate(time.Second).String(), string(parsedHTML), html.EscapeString(responseText))

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
			"🚫 *Rate Limit Exceeded*\n\nYou have reached the maximum number of messages allowed within the last 10 minutes. Please try again in %d minutes and %d seconds.",
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

	// Retrieve user's source code if available
	sourceCode, hasSource := a.GetUserSourceCode(userID)
	if hasSource {
		// Prepend source code to the conversation context for context-aware responses
		userQuestion = fmt.Sprintf("Here is my source code:\n%s\n\n%s", sourceCode, userQuestion)
	}

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

	// Store the full response in the ResponseStore (now persisted in S3)
	responseID := a.ResponseStore.StoreResponseForUser(responseText, userID)

	// Escape HTML in responseText
	escapedResponse := EscapeHTML(responseText)

	// Prepare final message with truncation if necessary
	var finalMessage string
	link := a.GenerateResponseURL(responseID)
	linkLength := len(link) + len("<a href=\"\"></a>") // Account for HTML tags
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
		finalMessage = fmt.Sprintf("%s\n\n<a href=\"%s\">View Formatted Response in its entirety</a>", truncatedResponse, link)
	} else {
		// Message is within limit; append the link
		finalMessage = fmt.Sprintf("%s\n\n<a href=\"%s\">View Formatted Response in its entirety</a>", escapedResponse, link)
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

// SendMessage sends a message to a Telegram chat.
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
		"parse_mode":               "HTML",
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
	a.TelegramHandler.HandleTelegramMessage(update)
}

// HandleCommand processes Telegram commands.
func (a *App) HandleCommand(message *types.TelegramMessage, userID int, username string) (string, error) {
	switch {
	case strings.HasPrefix(message.Text, "/mydata@"+a.BotUsername):
		// Extract the command without the bot username
		command := "/mydata"
		return a.HandleSpecificCommand(command, message, userID, username)
	case strings.HasPrefix(message.Text, "/upload@"+a.BotUsername):
		// Removed "/upload" from group chats as per Task 1
		// Any attempt to use "/upload@BOT_USERNAME" in group chats will be handled in telegram_handler.go
		command := "/upload"
		return a.HandleSpecificCommand(command, message, userID, username)
	case strings.HasPrefix(message.Text, "/security@"+a.BotUsername):
		command := "/security"
		return a.HandleSpecificCommand(command, message, userID, username)
	case strings.HasPrefix(message.Text, "/project@"+a.BotUsername):
		command := "/project"
		return a.HandleSpecificCommand(command, message, userID, username)
	case strings.HasPrefix(message.Text, "/my_source_code@"+a.BotUsername):
		command := "/my_source_code"
		return a.HandleSpecificCommand(command, message, userID, username)
	default:
		switch message.Text {
		case "/start":
			welcomeMsg := "🎉 *Welcome to Kernel Sanders Bot!*\n\nYou can ask me questions about your application or upload your source code files for more context."
			err := a.SendMessage(message.Chat.ID, welcomeMsg, message.MessageID)
			return "", err
		case "/help":
			helpMsg := fmt.Sprintf(
				"📚 *Help Menu:*\n\n"+
					"*Commands:*\n"+
					"/start - Start interacting with the bot\n"+
					"/help - Show this help message\n"+
					"/upload - Upload your source code file (only .txt files are supported)\n"+
					"/mydata - View your uploaded files and web responses\n"+
					"/security - Learn about the bot's security measures\n"+
					"/project - Learn about the KernelSanders project and how to contribute\n"+
					"/my_source_code - Get scripts to prepare your source code for upload\n\n"+
					"*File Uploads:*\n"+
					"In group chats, upload .txt files by tagging me in the caption using @%s. In 1-on-1 chats, simply send the .txt file without tagging.\n\n"+
					"These files will be stored for *4 hours* only. Uploading a new file will overwrite the existing one and reset the storage time.\n\n"+
					"*Short-Lived Web Responses:*\n"+
					"The bot provides short-lived web response links for easier reading and navigation of your code outputs. Please save any outputs or files you wish to use for long-term purposes, as the web responses will expire after the specified duration.\n\n"+
					"🔒 *Security:* Only .txt files are accepted to prevent potential security risks associated with other file types.",
				a.BotUsername,
			)
			err := a.SendMessage(message.Chat.ID, helpMsg, message.MessageID)
			return "", err
		case "/upload":
			// Removed "/upload" command from group chats as per Task 1
			uploadMsg := "📤 *Upload Command Removed in Group Chats*\n\nFor privacy reasons, please message me directly by clicking @" + a.BotUsername + " to upload your source code files."
			err := a.SendMessage(message.Chat.ID, uploadMsg, message.MessageID)
			return "", err
		case "/mydata":
			myData, err := a.GetUserData(userID)
			if err != nil {
				log.Printf("Failed to retrieve user data: %v", err)
				errorMsg := "❌ *Error Retrieving Data*\n\nUnable to fetch your data at this time. Please try again later."
				a.SendMessage(message.Chat.ID, errorMsg, message.MessageID)
				return "", err
			}
			err = a.SendMessage(message.Chat.ID, myData, message.MessageID)
			return "", err
		case "/security":
			securityMsg := fmt.Sprintf(
				"🔐 *Security Information:*\n\n" +
					"Your data and responses are handled with the utmost security. Uploaded files are stored securely in S3 with strict access controls and are automatically deleted after 4 hours. All interactions are logged for auditing purposes.\n\n" +
					"The project's source code is open-source, allowing for community review and contributions. You can view the code on GitHub here: [KernelSanders GitHub](https://github.com/joelradon/KernelSanders).\n\n" +
					"Feel free to review the code and contribute to its development!",
			)
			err := a.SendMessage(message.Chat.ID, securityMsg, message.MessageID)
			return "", err
		case "/project":
			projectMsg := "🚀 *KernelSanders Project:*\n\n" +
				"The KernelSanders bot is an open-source project designed to assist you with your coding needs. Contributions are welcome! You can view the source code and contribute on GitHub: <a href=\"https://github.com/joelradon/KernelSanders\">KernelSanders GitHub</a>.\n\n" +
				"If you find this tool useful, consider buying me a coffee: <a href=\"https://paypal.me/joelradon\">Buy me a Coffee</a>. Your support is greatly appreciated! ☕😊"
			err := a.SendMessage(message.Chat.ID, projectMsg, message.MessageID)
			return "", err
		case "/my_source_code":
			mySourceCodeMsg := "💻 *Prepare Your Source Code for Upload:*\n\n" +
				"Use the following scripts to quickly copy and prepare your source code in a directory tree for upload. These scripts exclude README files and only process specified file types.\n\n" +
				"*PowerShell Script:* <a href=\"https://s3.amazonaws.com/your-bucket/powershell_prepare_source.ps1\">Download PowerShell Script</a>\n\n" +
				"*Bash Script:* <a href=\"https://s3.amazonaws.com/your-bucket/bash_prepare_source.sh\">Download Bash Script</a>\n\n" +
				"These scripts will generate a structured output of your code files, making it easier to upload and manage your projects."
			err := a.SendMessage(message.Chat.ID, mySourceCodeMsg, message.MessageID)
			return "", err
		default:
			unknownCmd := "❓ *Unknown command.* Type /help to see available commands."
			err := a.SendMessage(message.Chat.ID, unknownCmd, message.MessageID)
			return "", err
		}
	}
}

// HandleSpecificCommand processes commands with bot mentions.
func (a *App) HandleSpecificCommand(command string, message *types.TelegramMessage, userID int, username string) (string, error) {
	switch command {
	case "/mydata":
		myData, err := a.GetUserData(userID)
		if err != nil {
			log.Printf("Failed to retrieve user data: %v", err)
			errorMsg := "❌ *Error Retrieving Data*\n\nUnable to fetch your data at this time. Please try again later."
			a.SendMessage(message.Chat.ID, errorMsg, message.MessageID)
			return "", err
		}
		err = a.SendMessage(message.Chat.ID, myData, message.MessageID)
		return "", err
	case "/security":
		securityMsg := fmt.Sprintf(
			"🔐 *Security Information:*\n\n" +
				"Your data and responses are handled with the utmost security. Uploaded files are stored securely in S3 with strict access controls and are automatically deleted after 4 hours. All interactions are logged for auditing purposes.\n\n" +
				"The project's source code is open-source, allowing for community review and contributions. You can view the code on GitHub here: [KernelSanders GitHub](https://github.com/joelradon/KernelSanders).\n\n" +
				"Feel free to review the code and contribute to its development!",
		)
		err := a.SendMessage(message.Chat.ID, securityMsg, message.MessageID)
		return "", err
	case "/project":
		projectMsg := "🚀 *KernelSanders Project:*\n\n" +
			"The KernelSanders bot is an open-source project designed to assist you with your coding needs. Contributions are welcome! You can view the source code and contribute on GitHub: <a href=\"https://github.com/joelradon/KernelSanders\">KernelSanders GitHub</a>.\n\n" +
			"If you find this tool useful, consider buying me a coffee: <a href=\"https://paypal.me/joelradon\">Buy me a Coffee</a>. Your support is greatly appreciated! ☕😊"
		err := a.SendMessage(message.Chat.ID, projectMsg, message.MessageID)
		return "", err
	case "/my_source_code":
		mySourceCodeMsg := "💻 *Prepare Your Source Code for Upload:*\n\n" +
			"Use the following scripts to quickly copy and prepare your source code in a directory tree for upload. These scripts exclude README files and only process specified file types.\n\n" +
			"*PowerShell Script:* <a href=\"https://s3.amazonaws.com/your-bucket/powershell_prepare_source.ps1\">Download PowerShell Script</a>\n\n" +
			"*Bash Script:* <a href=\"https://s3.amazonaws.com/your-bucket/bash_prepare_source.sh\">Download Bash Script</a>\n\n" +
			"These scripts will generate a structured output of your code files, making it easier to upload and manage your projects."
		err := a.SendMessage(message.Chat.ID, mySourceCodeMsg, message.MessageID)
		return "", err
	default:
		unknownCmd := "❓ *Unknown command.* Type /help to see available commands."
		err := a.SendMessage(message.Chat.ID, unknownCmd, message.MessageID)
		return "", err
	}
}

// GetUserData retrieves the user's uploaded files and web responses.
func (a *App) GetUserData(userID int) (string, error) {
	// Retrieve uploaded files from S3
	files, err := a.ListUserFiles(userID)
	if err != nil {
		return "", err
	}

	// Retrieve web responses from ResponseStore (now persisted in S3)
	responses, err := a.ResponseStore.GetUserResponsesByUserID(userID)
	if err != nil {
		return "", err
	}

	// Build the response message
	var sb strings.Builder
	sb.WriteString("📊 *Your Data:*\n\n")

	if len(files) > 0 {
		sb.WriteString("*Uploaded Files:*\n")
		sb.WriteString("| File Name | Uploaded At (UTC) | Uploaded At (EDT) | Deletion Time (UTC) | Deletion Time (EDT) |\n")
		sb.WriteString("|-----------|-------------------|-------------------|---------------------|---------------------|\n")
		for _, file := range files {
			fileURL := a.GenerateFileURL(file.FileName)
			sb.WriteString(fmt.Sprintf("| <a href=\"%s\">%s</a> | %s | %s | %s | %s |\n",
				fileURL,
				file.FileName,
				file.UploadedAtUTC.Format(time.RFC1123),
				file.UploadedAtEDT.Format(time.RFC1123),
				file.DeletionTimeUTC.Format(time.RFC1123),
				file.DeletionTimeEDT.Format(time.RFC1123),
			))
		}
		sb.WriteString("\n")
	} else {
		sb.WriteString("*No uploaded files found.*\n\n")
	}

	if len(responses) > 0 {
		sb.WriteString("*Web Responses:*\n")
		sb.WriteString("| Response ID | Created At (UTC) | Created At (EDT) | Deletion Time (UTC) | Deletion Time (EDT) |\n")
		sb.WriteString("|-------------|-------------------|-------------------|---------------------|---------------------|\n")
		for _, resp := range responses {
			responseURL := a.GenerateResponseURL(resp.ID)
			sb.WriteString(fmt.Sprintf("| <a href=\"%s\">%s</a> | %s | %s | %s | %s |\n",
				responseURL,
				resp.ID,
				resp.CreatedAtUTC.Format(time.RFC1123),
				resp.CreatedAtEDT.Format(time.RFC1123),
				resp.DeletionTimeUTC.Format(time.RFC1123),
				resp.DeletionTimeEDT.Format(time.RFC1123),
			))
		}
		sb.WriteString("\n")
	} else {
		sb.WriteString("*No web responses found.*\n")
	}

	return sb.String(), nil
}

// GenerateFileURL generates the download URL for the uploaded file.
func (a *App) GenerateFileURL(fileName string) string {
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	return fmt.Sprintf("%s/files/%s", baseURL, fileName)
}

// ListUserFiles lists all uploaded files for a user by querying S3 directly.
func (a *App) ListUserFiles(userID int) ([]types.UserFile, error) {
	prefix := fmt.Sprintf("user_source_code/%d/", userID)
	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(a.S3BucketName),
		Prefix: aws.String(prefix),
	}

	var files []types.UserFile

	err := a.S3Client.ListObjectsV2Pages(input, func(page *s3.ListObjectsV2Output, lastPage bool) bool {
		for _, obj := range page.Contents {
			// Retrieve metadata to get upload time
			headInput := &s3.HeadObjectInput{
				Bucket: aws.String(a.S3BucketName),
				Key:    aws.String(*obj.Key),
			}
			headResp, err := a.S3Client.HeadObject(headInput)
			if err != nil {
				log.Printf("Failed to retrieve metadata for object %s: %v", *obj.Key, err)
				continue
			}

			uploadedAtStr, exists := headResp.Metadata["uploaded_at"]
			if !exists || uploadedAtStr == nil {
				log.Printf("No 'uploaded_at' metadata for object %s. Skipping.", *obj.Key)
				continue
			}

			uploadedAt, err := time.Parse(time.RFC3339, *uploadedAtStr)
			if err != nil {
				log.Printf("Invalid 'uploaded_at' format for object %s: %v", *obj.Key, err)
				continue
			}

			deletionTime := uploadedAt.Add(types.FileRetentionTime)

			files = append(files, types.UserFile{
				FileName:        *obj.Key,
				UploadedAtUTC:   uploadedAt.UTC(),
				UploadedAtEDT:   uploadedAt.In(time.FixedZone("EDT", -4*3600)),
				DeletionTimeUTC: deletionTime.UTC(),
				DeletionTimeEDT: deletionTime.In(time.FixedZone("EDT", -4*3600)),
			})
		}
		return true // Continue to next page
	})

	if err != nil {
		return nil, err
	}

	return files, nil
}

// Shutdown gracefully shuts down the application, ensuring all goroutines are terminated.
func (a *App) Shutdown() {
	close(a.ShutdownChan)
	a.wg.Wait()
	log.Println("Application has been shut down gracefully.")
}

// logToS3 logs user interactions to an S3 bucket with details about rate limiting and usage.
func (a *App) logToS3(userID int, username, userPrompt, responseTime string, isNoLimitUser bool) {
	a.logMutex.Lock()
	defer a.logMutex.Unlock()

	// Extract keywords from the user prompt
	keywords := utils.ExtractKeywords(userPrompt)

	record := []string{
		fmt.Sprintf("%d", userID),
		username,
		userPrompt,
		keywords, // Added keywords to the CSV record
		responseTime,
		fmt.Sprintf("No limit user: %t", isNoLimitUser),
	}

	bucketName := a.S3BucketName
	objectKey := "logs/telegram_logs.csv"

	// Check if the CSV file exists
	_, err := a.S3Client.HeadObject(&s3.HeadObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectKey),
	})

	var existingData [][]string
	if err == nil {
		// Object exists, get it
		getResp, err := a.S3Client.GetObject(&s3.GetObjectInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(objectKey),
		})
		if err != nil {
			log.Printf("Failed to get existing CSV from S3: %v", err)
		} else {
			defer getResp.Body.Close()
			bodyBytes, err := io.ReadAll(getResp.Body)
			if err == nil && len(bodyBytes) > 0 {
				reader := csv.NewReader(bytes.NewReader(bodyBytes))
				existingData, err = reader.ReadAll()
				if err != nil {
					log.Printf("Failed to parse existing CSV: %v", err)
					existingData = [][]string{}
				}
			}
		}
	} else {
		// Object does not exist, create new
		log.Printf("S3 CSV file does not exist. A new one will be created.")
		existingData = [][]string{}
	}

	if len(existingData) == 0 {
		headers := []string{
			"userID",
			"username",
			"prompt",
			"keywords", // Added keywords header
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

// StoreUserSourceCode stores the user's source code to S3.
func (a *App) StoreUserSourceCode(userID int, code string) error {
	objectKey := fmt.Sprintf("user_source_code/%d/source_code.txt", userID)
	metadata := map[string]*string{
		"uploaded_at": aws.String(time.Now().Format(time.RFC3339)),
	}

	_, err := a.S3Client.PutObject(&s3.PutObjectInput{
		Bucket:   aws.String(a.S3BucketName),
		Key:      aws.String(objectKey),
		Body:     strings.NewReader(code),
		Metadata: metadata,
	})
	if err != nil {
		log.Printf("Failed to upload source code to S3 for user %d: %v", userID, err)
		return err
	}
	return nil
}

// GetUserSourceCode retrieves the user's stored source code from S3.
func (a *App) GetUserSourceCode(userID int) (string, bool) {
	objectKey := fmt.Sprintf("user_source_code/%d/source_code.txt", userID)
	resp, err := a.S3Client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(a.S3BucketName),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		log.Printf("Failed to retrieve source code from S3 for user %d: %v", userID, err)
		return "", false
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Failed to read source code body for user %d: %v", userID, err)
		return "", false
	}

	return string(bodyBytes), true
}

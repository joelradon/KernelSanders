// internal/app/app.go

package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
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
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/russross/blackfriday/v2"
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
	S3BucketName         string
	S3Client             s3client.S3ClientInterface
	UsageCache           *usage.UsageCache
	NoLimitUsers         map[int]struct{}
	ConversationContexts *conversation.ConversationCache
	APIHandler           *api.APIHandler
	TelegramHandler      *telegram.TelegramHandler
	logMutex             sync.Mutex
	ResponseStore        *ResponseStore
	ShutdownChan         chan struct{}
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
	log.Printf("OpenAI API Endpoint URL: %s", apiHandler.EndpointURL)

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

	return app
}

// parseNoLimitUsers parses the NO_LIMIT_USERS environment variable into a map of user IDs.
func parseNoLimitUsers(raw string) map[int]struct{} {
	userMap := make(map[int]struct{})
	if raw == "" {
		return userMap
	}
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
	// Added "Copy to Clipboard" button
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
		.view-raw-button, .copy-clipboard-button {
			margin-top: 10px;
			padding: 5px 10px;
			background-color: #bb86fc;
			color: #121212;
			border: none;
			border-radius: 3px;
			cursor: pointer;
			margin-right: 10px;
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

		function copyToClipboard() {
			var rawContent = document.getElementById("raw-content").innerText;
			navigator.clipboard.writeText(rawContent).then(function() {
				alert("RAW code copied to clipboard!");
			}, function(err) {
				alert("Failed to copy: " + err);
			});
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
		<button class="copy-clipboard-button" onclick="copyToClipboard()">Copy to Clipboard</button>
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

		limitMsg := "✅ *Rate Limit Exceeded*\n\nYou have reached the maximum number of messages allowed within the last 10 minutes. Please try again in " +
			fmt.Sprintf("%d minutes and %d seconds.", minutes, seconds)
		if err := a.SendMessage(chatID, limitMsg, messageID); err != nil {
			log.Printf("Failed to send rate limit message to Telegram: %v", err)
		}

		// Log the attempt to S3
		a.logToS3(userID, username, userQuestion, "", isNoLimitUser)
		return fmt.Errorf("user rate limited")
	}

	a.UsageCache.AddUsage(userID)

	// Detect and replace '#source_code' reference with actual source code
	if strings.Contains(strings.ToLower(userQuestion), "#source_code") {
		sourceCode, hasSource := a.GetUserSourceCode(userID)
		if hasSource {
			// Replace '#source_code' with the actual source code
			userQuestion = strings.ReplaceAll(strings.ToLower(userQuestion), "#source_code", sourceCode)
		} else {
			// Inform the user that no source code is available
			errMsg := "❌ *No Source Code Found*\n\nYou have not uploaded any source code yet. Please upload a `.txt` file using the /upload command."
			if err := a.SendMessage(chatID, errMsg, messageID); err != nil {
				log.Printf("Failed to send no source code message: %v", err)
			}
			return nil
		}
	}

	// Detect and replace '#source_repo' reference with actual repository content
	if strings.Contains(strings.ToLower(userQuestion), "#source_repo") {
		sourceRepo, hasRepo := a.GetUserSourceRepo(userID)
		if hasRepo {
			// Replace '#source_repo' with the actual repository content
			userQuestion = strings.ReplaceAll(strings.ToLower(userQuestion), "#source_repo", sourceRepo)
		} else {
			// Inform the user that no source repository is available
			errMsg := "❌ *No Source Repository Found*\n\nYou have not shared any source repository yet. Please share a repository using the /source_repo command."
			if err := a.SendMessage(chatID, errMsg, messageID); err != nil {
				log.Printf("Failed to send no source repository message: %v", err)
			}
			return nil
		}
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

// HandleCommand processes Telegram commands.
func (a *App) HandleCommand(message *types.TelegramMessage, userID int, username string) (string, error) {
	return a.HandleSpecificCommand(strings.TrimSuffix(message.Text, "@"+a.BotUsername), message, userID, username)
}

// HandleSpecificCommand processes commands with bot mentions.
func (a *App) HandleSpecificCommand(command string, message *types.TelegramMessage, userID int, username string) (string, error) {
	switch command {
	case "/mydata":
		myData, err := a.GetUserData(userID)
		if err != nil {
			log.Printf("Failed to retrieve user data: %v", err)
			errorMsg := "✅ *Error Retrieving Data*\n\nUnable to fetch your data at this time. Please try again later."
			a.SendMessage(message.Chat.ID, errorMsg, message.MessageID)
			return "", err
		}
		err = a.SendMessage(message.Chat.ID, myData, message.MessageID)
		return "", err
	case "/security":
		securityMsg := fmt.Sprintf(
			"🔒 *Security Information:*\n\n" +
				"Your data and responses are handled with the utmost security. Uploaded files and shared repositories are stored securely in S3 with strict access controls and are automatically deleted after 4 hours. All interactions are logged for auditing purposes.\n\n" +
				"The project's source code is open-source, allowing for community review and contributions. You can view the code on GitHub here: [KernelSanders GitHub](https://github.com/joelradon/KernelSanders).\n\n" +
				"Feel free to review the code and contribute to its development!\n\n" +
				"✅ *Delete Your Data:* Use /delete_my_data to remove all your uploaded files, shared repositories, and web responses.",
		)
		err := a.SendMessage(message.Chat.ID, securityMsg, message.MessageID)
		return "", err
	case "/project":
		projectMsg := "🎮 *KernelSanders Project:*\n\n" +
			"The KernelSanders bot is an open-source project designed to assist you with your coding needs. Contributions are welcome! You can view the source code and contribute on GitHub: <a href=\"https://github.com/joelradon/KernelSanders\">KernelSanders GitHub</a>.\n\n" +
			"If you find this tool useful, consider buying me a coffee: <a href=\"https://paypal.me/joelradon\">Buy me a Coffee</a>. Your support is greatly appreciated! ☕"
		err := a.SendMessage(message.Chat.ID, projectMsg, message.MessageID)
		return "", err
	case "/my_source_code":
		mySourceCodeMsg := fmt.Sprintf(
			"# Overview\n\n"+
				"These scripts facilitate the preparation and management of source code files, allowing users to easily gather and format their code for AI interactions with KernelSanders. By excluding certain files and ensuring only relevant file types are processed, they optimize the user’s experience when interacting with the AI bot.\n\n"+
				"Both scripts are designed to:\n"+
				"- List all files in the current directory and its subdirectories.\n"+
				"- Print the contents of each file, excluding README.md.\n"+
				"- Copy the output to the clipboard for easy pasting.\n\n"+
				"Create a source code text file and upload it to Telegram. It will be stored for 4 hours and linked directly to your username. After that, it will be deleted.\n\n"+
				"(short link)https://github.com/joelradon/KernelSanders/blob/main/utility_scripts/copy_source_code.bash\n"+
				"(short link)https://github.com/joelradon/KernelSanders/blob/main/utility_scripts/copy_source_code.ps1\n\n"+
				"**REMEMBER TO NEVER PUT SENSITIVE CODE ANYWHERE.** While this bot has a private store for each user and deletes each file after 4 hours, practice safe coding and don't put any sensitive information in your code base.\n\n"+
				"✅ *Reference Source Code:* After uploading your source code, you can reference it in your messages using `#source_code`. The bot will utilize your uploaded code to provide context-aware responses as long as the file is stored.",
			a.BotUsername,
		)
		err := a.SendMessage(message.Chat.ID, mySourceCodeMsg, message.MessageID)
		return "", err
	case "/source_repo":
		// Handle the /source_repo command
		// Extract the repository URL from the message
		repoURL, err := extractRepoURL(message.Text, "/source_repo")
		if err != nil {
			errMsg := "❌ *Invalid Command Usage*\n\nPlease provide a valid GitHub repository URL after the /source_repo command."
			if sendErr := a.SendMessage(message.Chat.ID, errMsg, message.MessageID); sendErr != nil {
				log.Printf("Failed to send invalid command usage message: %v", sendErr)
			}
			return "", err
		}

		// Process the repository: download, zip, upload to S3
		repoZipURL, err := a.ProcessSourceRepo(userID, repoURL)
		if err != nil {
			log.Printf("Failed to process source repository: %v", err)
			errMsg := "❌ *Repository Processing Error*\n\nFailed to process the shared repository. Please ensure the URL is correct and accessible."
			if sendErr := a.SendMessage(message.Chat.ID, errMsg, message.MessageID); sendErr != nil {
				log.Printf("Failed to send repository processing error message: %v", sendErr)
			}
			return "", err
		}

		// Confirmation message
		confirmationMsg := fmt.Sprintf(
			"✅ *Repository Shared Successfully*\n\nYour repository has been shared and is accessible until:\n\n"+
				"• *Deletion Time:* UTC: %s | EDT: %s\n\n"+
				"Use `#source_repo` to reference this repository in your messages.",
			time.Now().Add(types.FileRetentionTime).UTC().Format(time.RFC1123),
			time.Now().Add(types.FileRetentionTime).In(time.FixedZone("EDT", -4*3600)).Format(time.RFC1123),
		)
		if err := a.SendMessage(message.Chat.ID, confirmationMsg, message.MessageID); err != nil {
			log.Printf("Failed to send repository sharing confirmation message: %v", err)
		}

		// Send the URL to the user
		repoShareMsg := fmt.Sprintf("🚀 *Repository URL:*\n\n<a href=\"%s\">%s</a>", repoZipURL, "source_repo.zip")
		if err := a.SendMessage(message.Chat.ID, repoShareMsg, message.MessageID); err != nil {
			log.Printf("Failed to send repository URL message: %v", err)
		}

		return "", nil
	case "/delete_my_data":
		deleteMsg, err := a.DeleteUserData(userID)
		if err != nil {
			log.Printf("Failed to delete user data: %v", err)
			errorMsg := "✅ *Error Deleting Data*\n\nUnable to delete your data at this time. Please try again later."
			a.SendMessage(message.Chat.ID, errorMsg, message.MessageID)
			return "", err
		}
		err = a.SendMessage(message.Chat.ID, deleteMsg, message.MessageID)
		return "", err
	default:
		unknownCmd := "❔ *Unknown command.* Type /help to see available commands."
		err := a.SendMessage(message.Chat.ID, unknownCmd, message.MessageID)
		return "", err
	}
}

// extractRepoURL extracts the repository URL from the command message.
func extractRepoURL(text, command string) (string, error) {
	parts := strings.SplitN(text, " ", 2)
	if len(parts) < 2 {
		return "", errors.New("repository URL not provided")
	}
	repoURL := strings.TrimSpace(parts[1])
	return repoURL, nil
}

// GetUserData retrieves the user's uploaded files, shared repositories, and web responses.
func (a *App) GetUserData(userID int) (string, error) {
	// Retrieve uploaded source code files from S3
	sourceFiles, err := a.ListUserSourceFiles(userID)
	if err != nil {
		return "", err
	}

	// Retrieve shared repositories from S3
	sourceRepos, err := a.ListUserSourceRepos(userID)
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
	sb.WriteString("✅ *Your Data:*\n\n")

	if len(sourceFiles) > 0 {
		sb.WriteString("*Uploaded Source Code Files:*\n")
		for _, file := range sourceFiles {
			fileURL := a.GenerateFileURL(file.FileName)
			sb.WriteString(fmt.Sprintf("- <a href=\"%s\">%s</a>\n", fileURL, filepath.Base(file.FileName)))
		}
		sb.WriteString("\n")
	} else {
		sb.WriteString("*No uploaded source code files found.*\n\n")
	}

	if len(sourceRepos) > 0 {
		sb.WriteString("*Shared Repositories:*\n")
		for _, repo := range sourceRepos {
			repoURL := a.GenerateRepoURL(repo.FileName)
			sb.WriteString(fmt.Sprintf("- <a href=\"%s\">%s</a>\n", repoURL, "source_repo.zip"))
		}
		sb.WriteString("\n")
	} else {
		sb.WriteString("*No shared repositories found.*\n\n")
	}

	if len(responses) > 0 {
		sb.WriteString("*Web Responses:*\n")
		for _, resp := range responses {
			responseURL := a.GenerateResponseURL(resp.ID)
			sb.WriteString(fmt.Sprintf("- <a href=\"%s\">Response ID: %s</a>\n", responseURL, resp.ID))
		}
		sb.WriteString("\n")
	} else {
		sb.WriteString("*No web responses found.*\n")
	}

	return sb.String(), nil
}

// ListUserSourceFiles lists all uploaded source code files for a user by querying S3.
func (a *App) ListUserSourceFiles(userID int) ([]types.UserFile, error) {
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

// ListUserSourceRepos lists all shared repositories for a user by querying S3.
func (a *App) ListUserSourceRepos(userID int) ([]types.UserFile, error) {
	prefix := fmt.Sprintf("user_source_repo/%d/", userID)
	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(a.S3BucketName),
		Prefix: aws.String(prefix),
	}

	var repos []types.UserFile

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

			repos = append(repos, types.UserFile{
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

	return repos, nil
}

// GenerateRepoURL generates the download URL for the shared repository.
func (a *App) GenerateRepoURL(fileName string) string {
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	return fmt.Sprintf("%s/files/%s", baseURL, fileName)
}

// GenerateFileURL generates the download URL for the uploaded file.
func (a *App) GenerateFileURL(fileName string) string {
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	return fmt.Sprintf("%s/files/%s", baseURL, fileName)
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

// HandleRepositoryShare processes the shared repository URL by the user.
func (a *App) HandleRepositoryShare(chatID int64, userID int, username, repoURL string, messageID int) error {
	// Validate the GitHub repository URL
	if !isValidGitHubURL(repoURL) {
		errMsg := "❌ *Invalid Repository URL*\n\nPlease provide a valid GitHub repository URL after the /source_repo command."
		if err := a.SendMessage(chatID, errMsg, messageID); err != nil {
			log.Printf("Failed to send invalid repository URL message: %v", err)
		}
		return errors.New("invalid repository URL")
	}

	// Process the repository: download, zip, upload to S3
	repoZipURL, err := a.ProcessSourceRepo(userID, repoURL)
	if err != nil {
		log.Printf("Failed to process source repository: %v", err)
		errMsg := "❌ *Repository Processing Error*\n\nFailed to process the shared repository. Please ensure the URL is correct and accessible."
		if err := a.SendMessage(chatID, errMsg, messageID); err != nil {
			log.Printf("Failed to send repository processing error message: %v", err)
		}
		return err
	}

	// Confirmation message
	confirmationMsg := fmt.Sprintf(
		"✅ *Repository Shared Successfully*\n\nYour repository has been shared and is accessible until:\n\n"+
			"• *Deletion Time:* UTC: %s | EDT: %s\n\n"+
			"Use `#source_repo` to reference this repository in your messages.",
		time.Now().Add(types.FileRetentionTime).UTC().Format(time.RFC1123),
		time.Now().Add(types.FileRetentionTime).In(time.FixedZone("EDT", -4*3600)).Format(time.RFC1123),
	)
	if err := a.SendMessage(chatID, confirmationMsg, messageID); err != nil {
		log.Printf("Failed to send repository sharing confirmation message: %v", err)
	}

	// Send the URL to the user
	repoShareMsg := fmt.Sprintf("🚀 *Repository URL:*\n\n<a href=\"%s\">%s</a>", repoZipURL, "source_repo.zip")
	if err := a.SendMessage(chatID, repoShareMsg, messageID); err != nil {
		log.Printf("Failed to send repository URL message: %v", err)
	}

	return nil
}

// isValidGitHubURL validates if the provided URL is a valid GitHub repository URL.
func isValidGitHubURL(url string) bool {
	lowerURL := strings.ToLower(url)
	return strings.HasPrefix(lowerURL, "https://github.com/") || strings.HasPrefix(lowerURL, "http://github.com/")
}

// DeleteUserData deletes all uploaded source code files, shared repositories, and web responses for a user.
func (a *App) DeleteUserData(userID int) (string, error) {
	// Delete source code files
	sourceCodeKey := fmt.Sprintf("user_source_code/%d/source_code.txt", userID)
	_, err := a.S3Client.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(a.S3BucketName),
		Key:    aws.String(sourceCodeKey),
	})
	if err != nil {
		log.Printf("Failed to delete source code from S3 for user %d: %v", userID, err)
		return "", err
	}

	// Delete shared repositories
	prefix := fmt.Sprintf("user_source_repo/%d/", userID)
	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(a.S3BucketName),
		Prefix: aws.String(prefix),
	}

	err = a.S3Client.ListObjectsV2Pages(input, func(page *s3.ListObjectsV2Output, lastPage bool) bool {
		for _, obj := range page.Contents {
			_, delErr := a.S3Client.DeleteObject(&s3.DeleteObjectInput{
				Bucket: aws.String(a.S3BucketName),
				Key:    aws.String(*obj.Key),
			})
			if delErr != nil {
				log.Printf("Failed to delete shared repository from S3 for user %d: %v", userID, delErr)
				continue
			}
		}
		return true // Continue to next page
	})
	if err != nil {
		log.Printf("Failed to delete shared repositories from S3 for user %d: %v", userID, err)
		return "", err
	}

	// Delete all web responses associated with the user
	responses, err := a.ResponseStore.GetUserResponsesByUserID(userID)
	if err != nil {
		log.Printf("Failed to retrieve user responses for deletion: %v", err)
		return "", err
	}

	for _, resp := range responses {
		a.ResponseStore.DeleteResponse(resp.ID)
	}

	deleteMsg := "✅ *Data Deleted Successfully*\n\nAll your uploaded source code files, shared repositories, and web responses have been deleted."
	return deleteMsg, nil
}

// ProcessSourceRepo handles the processing of a shared GitHub repository URL.
func (a *App) ProcessSourceRepo(userID int, repoURL string) (string, error) {
	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "source_repo_*")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tempDir) // Clean up after processing

	// Clone the repository using git
	cmd := exec.Command("git", "clone", repoURL, tempDir)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to clone repository: %v - %s", err, stderr.String())
	}

	// Compress the repository into a ZIP file
	zipFileName := "source_repo.zip"
	zipFilePath := filepath.Join(tempDir, zipFileName)
	err = zipDirectory(tempDir, zipFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to zip repository: %v", err)
	}

	// Read the ZIP file content
	zipContent, err := os.ReadFile(zipFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to read ZIP file: %v", err)
	}

	// Upload the ZIP file to S3
	objectKey := fmt.Sprintf("user_source_repo/%d/%s", userID, zipFileName)
	metadata := map[string]*string{
		"uploaded_at": aws.String(time.Now().Format(time.RFC3339)),
	}

	_, err = a.S3Client.PutObject(&s3.PutObjectInput{
		Bucket:   aws.String(a.S3BucketName),
		Key:      aws.String(objectKey),
		Body:     bytes.NewReader(zipContent),
		Metadata: metadata,
	})
	if err != nil {
		log.Printf("Failed to upload repository ZIP to S3 for user %d: %v", userID, err)
		return "", err
	}

	// Generate a pre-signed URL for the uploaded ZIP file (expires in 4 hours)
	repoZipURL, err := a.S3Client.GeneratePresignedURL(a.S3BucketName, objectKey, types.FileRetentionTime)
	if err != nil {
		return "", fmt.Errorf("failed to generate pre-signed URL: %v", err)
	}

	return repoZipURL, nil
}

// zipDirectory compresses the specified directory into a ZIP file at the given destination path.
func zipDirectory(sourceDir, destZip string) error {
	// Use the zip command to compress the directory
	cmd := exec.Command("zip", "-r", destZip, ".")
	cmd.Dir = sourceDir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("zip command failed: %v - %s", err, stderr.String())
	}
	return nil
}

// StoreUserSourceCode stores the user's uploaded source code to S3.
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

// GetUserSourceCode retrieves the user's uploaded source code from S3.
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

// GetUserSourceRepo retrieves the user's shared repository URL from S3.
func (a *App) GetUserSourceRepo(userID int) (string, bool) {
	objectKey := fmt.Sprintf("user_source_repo/%d/source_repo.zip", userID)
	resp, err := a.S3Client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(a.S3BucketName),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		log.Printf("Failed to retrieve source repository from S3 for user %d: %v", userID, err)
		return "", false
	}
	defer resp.Body.Close()

	// Since the repository is zipped, we can provide the pre-signed URL directly
	repoURL := a.GenerateRepoURL(objectKey)
	return repoURL, true
}

// ListUserFiles lists all uploaded source code files and shared repositories for a user.
func (a *App) ListUserFiles(userID int) ([]types.UserFile, error) {
	var allFiles []types.UserFile

	// List source code files
	sourceFiles, err := a.ListUserSourceFiles(userID)
	if err != nil {
		return nil, err
	}
	allFiles = append(allFiles, sourceFiles...)

	// List shared repositories
	sourceRepos, err := a.ListUserSourceRepos(userID)
	if err != nil {
		return nil, err
	}
	allFiles = append(allFiles, sourceRepos...)

	return allFiles, nil
}

// ProcessRepositoryShare processes the repository sharing.
func (a *App) ProcessRepositoryShare(chatID int64, userID int, username, repoURL string, messageID int) error {
	return a.HandleRepositoryShare(chatID, userID, username, repoURL, messageID)
}

// Shutdown gracefully shuts down the application, ensuring all goroutines are terminated.
func (a *App) Shutdown() {
	close(a.ShutdownChan)
	a.wg.Wait()
	log.Println("Application has been shut down gracefully.")
}

// Run starts the application's HTTP server and listens for incoming Telegram updates.
func (a *App) Run() {
	// Start the HTTP server for web responses
	http.HandleFunc("/", a.HandleWebRequest)
	http.HandleFunc("/files/", a.HandleFileDownload) // Implement HandleFileDownload as needed

	server := &http.Server{
		Addr: ":" + a.GetPort(),
	}

	// Start the HTTP server in a separate goroutine
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		log.Printf("Starting HTTP server on port %s", a.GetPort())
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	// Start Telegram webhook or long polling
	// Choose one based on configuration
	// Example: Starting webhook
	err := a.StartWebhook()
	if err != nil {
		log.Printf("Failed to start webhook: %v", err)
	} else {
		log.Println("Webhook started successfully.")
	}

	// Alternatively, implement long polling
	// Uncomment the following lines to enable long polling
	/*
		a.wg.Add(1)
		go func() {
			defer a.wg.Done()
			a.ListenLongPoll()
		}()
	*/

	// Wait for shutdown signal
	<-a.ShutdownChan

	// Shutdown the HTTP server gracefully
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("HTTP server Shutdown Failed:%+v", err)
	}
}

// GetPort retrieves the port from environment variables or defaults to 8080.
func (a *App) GetPort() string {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	return port
}

// HandleFileDownload handles file download requests.
func (a *App) HandleFileDownload(w http.ResponseWriter, r *http.Request) {
	// Extract the file key from the URL
	fileKey := strings.TrimPrefix(r.URL.Path, "/files/")
	if fileKey == "" {
		http.Error(w, "File key not provided.", http.StatusBadRequest)
		return
	}

	// Retrieve the file from S3
	resp, err := a.S3Client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(a.S3BucketName),
		Key:    aws.String(fileKey),
	})
	if err != nil {
		log.Printf("Failed to retrieve file %s from S3: %v", fileKey, err)
		http.Error(w, "File not found.", http.StatusNotFound)
		return
	}
	defer resp.Body.Close()

	// Set appropriate headers
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filepath.Base(fileKey)))
	w.Header().Set("Content-Type", "application/octet-stream")

	// Stream the file to the response
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		log.Printf("Failed to stream file %s to response: %v", fileKey, err)
	}
}

// StartWebhook sets up the Telegram webhook.
func (a *App) StartWebhook() error {
	// Retrieve webhook URL from environment variables
	webhookURL := os.Getenv("WEBHOOK_URL")
	if webhookURL == "" {
		return errors.New("WEBHOOK_URL environment variable is not set")
	}

	// Set the webhook with Telegram
	setWebhookURL := fmt.Sprintf("https://api.telegram.org/bot%s/setWebhook", a.TelegramToken)
	payload := map[string]interface{}{
		"url": webhookURL,
	}

	reqBody, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook payload: %v", err)
	}

	resp, err := a.HTTPClient.Post(setWebhookURL, "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("failed to set webhook: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to set webhook, status: %s, body: %s", resp.Status, string(bodyBytes))
	}

	log.Println("Telegram webhook set successfully.")
	return nil
}

// ListenLongPoll starts listening for Telegram updates using long polling.
func (a *App) ListenLongPoll() {
	for {
		select {
		case <-a.ShutdownChan:
			log.Println("Shutting down long poll listener.")
			return
		default:
			updates, err := a.TelegramHandler.FetchUpdates()
			if err != nil {
				log.Printf("Failed to fetch updates: %v", err)
				time.Sleep(5 * time.Second)
				continue
			}

			for _, update := range updates {
				a.HandleUpdate(&update)
			}
		}
	}
}

// GetSummary generates a brief summary using the OpenAI API.
func (a *App) GetSummary(prompt string) (string, error) {
	// Prepare the messages for OpenAI
	messages := []types.OpenAIMessage{
		{Role: "user", Content: prompt},
	}

	// Query OpenAI for summary
	summary, err := a.APIHandler.QueryOpenAIWithMessages(messages)
	if err != nil {
		log.Printf("Failed to get summary from OpenAI: %v", err)
		return "", err
	}

	// Trim any extra whitespace from the summary
	summary = strings.TrimSpace(summary)
	return summary, nil
}

// AnalyzeUserCode generates a brief summary of the user's uploaded code.
func (a *App) AnalyzeUserCode(userID int) (string, error) {
	// Retrieve the user's source code
	code, exists := a.GetUserSourceCode(userID)
	if !exists {
		return "", errors.New("no source code found for user")
	}

	// Create a prompt for summarization
	prompt := fmt.Sprintf("Provide a concise two-sentence summary of the following source code:\n\n%s", code)

	// Get the summary using the GetSummary method
	summary, err := a.GetSummary(prompt)
	if err != nil {
		return "", err
	}

	return summary, nil
}

// logToS3 logs user interactions to S3 for auditing purposes.
func (a *App) logToS3(userID int, username, question, responseTime string, isNoLimitUser bool) {
	a.logMutex.Lock()
	defer a.logMutex.Unlock()

	logEntry := map[string]interface{}{
		"user_id":       userID,
		"username":      username,
		"question":      question,
		"response_time": responseTime,
		"no_limit_user": isNoLimitUser,
		"timestamp":     time.Now().Format(time.RFC3339),
	}

	logBytes, err := json.Marshal(logEntry)
	if err != nil {
		log.Printf("Failed to marshal log entry: %v", err)
		return
	}

	logKey := fmt.Sprintf("logs/%d/%s.json", userID, uuid.New().String())
	_, err = a.S3Client.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(a.S3BucketName),
		Key:    aws.String(logKey),
		Body:   bytes.NewReader(logBytes),
	})
	if err != nil {
		log.Printf("Failed to upload log entry to S3: %v", err)
	}
}

// GetUserSourceCode retrieves the user's source code.

// initializeS3Client initializes the AWS S3 client.
// Implemented as needed.
func initializeS3Client() *s3.S3 {
	// Initialization logic here
	return &s3.S3{}
}

// NewAPIHandler initializes and returns a new APIHandler.
func NewAPIHandler() *APIHandler {
	// Initialization logic here
	return &APIHandler{}
}

// APIHandler handles interactions with external APIs like OpenAI.
type APIHandler struct {
	// Fields as needed
}

// QueryOpenAIWithMessages queries the OpenAI API with the given messages.
func (api *APIHandler) QueryOpenAIWithMessages(messages []types.OpenAIMessage) (string, error) {
	// Implementation logic here
	return "", nil
}

// TelegramHandler interface for processing Telegram messages.
// Assuming it's defined elsewhere.
type TelegramHandler struct {
	Processor *handlers.MessageProcessor
	// Other fields as needed
}

// No update needed for other methods and functionalities.

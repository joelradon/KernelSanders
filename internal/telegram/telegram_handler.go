// internal/telegram/telegram_handler.go

package telegram

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"KernelSandersBot/internal/handlers"
	"KernelSandersBot/internal/types"
)

// TelegramHandler handles Telegram updates and delegates processing to the MessageProcessor.
type TelegramHandler struct {
	Processor handlers.MessageProcessor
	client    *http.Client
	offset    int
	mutex     sync.Mutex
}

// NewTelegramHandler initializes a new TelegramHandler.
func NewTelegramHandler(processor handlers.MessageProcessor) *TelegramHandler {
	return &TelegramHandler{
		Processor: processor,
		client:    &http.Client{Timeout: 60 * time.Second}, // Timeout set to 60 seconds for long polling
		offset:    0,
	}
}

// HandleTelegramMessage processes incoming Telegram messages, including text and documents.
func (th *TelegramHandler) HandleTelegramMessage(update *types.TelegramUpdate) (string, error) {
	var message *types.TelegramMessage

	if update.Message != nil {
		message = update.Message
	} else if update.EditedMessage != nil {
		message = update.EditedMessage
	} else {
		return "", nil
	}

	if message.Chat.ID == 0 {
		return "", nil
	}

	// Check if the message contains a document (file)
	if message.Document != nil {
		return th.HandleDocument(message)
	}

	if message.Text == "" {
		return "", nil
	}

	chatID := message.Chat.ID
	userQuestion := message.Text
	messageID := message.MessageID
	userID := message.From.ID
	username := message.From.Username

	// Determine chat type
	chatType := message.Chat.Type
	isGroup := chatType == "group" || chatType == "supergroup"

	isTagged := false
	botUsername := th.Processor.GetBotUsername()

	// Check for explicit tagging in the message
	if isGroup && len(message.Entities) > 0 {
		for _, entity := range message.Entities {
			if entity.Type == "mention" {
				if entity.Offset+entity.Length > len(message.Text) {
					continue
				}
				mention := message.Text[entity.Offset : entity.Offset+entity.Length]
				if isTaggedMention(mention, botUsername) {
					isTagged = true
					userQuestion = removeMention(userQuestion, mention)
					break
				}
			}
		}
	}

	// In group chats, ignore any commands or messages unless explicitly tagged
	if isGroup && !isTagged {
		// Ignore messages not tagged
		return "", nil
	}

	// Handle commands
	if strings.HasPrefix(message.Text, "/") {
		// Check if the command is explicitly tagged in a group chat
		if isGroup && !isTagged {
			// Ignore commands in group chats if not tagged
			return "", nil
		}
		_, err := th.Processor.HandleCommand(message, userID, username)
		if err != nil {
			log.Printf("Error handling command: %v", err)
			return "", nil
		}
		return "", nil
	}

	// Process messages in private chats or tagged messages in group chats
	if !isGroup || isTagged {
		if err := th.Processor.ProcessMessage(chatID, userID, username, userQuestion, messageID); err != nil {
			log.Printf("Error processing message: %v", err)
			return "", nil
		}
	}

	return "", nil
}

// HandleDocument processes uploaded document files from users.
func (th *TelegramHandler) HandleDocument(message *types.TelegramMessage) (string, error) {
	document := message.Document
	if document == nil {
		return "", errors.New("no document found in the message")
	}

	// Determine chat type
	chatType := message.Chat.Type
	isGroup := chatType == "group" || chatType == "supergroup"

	// In group chats, ignore file uploads unless explicitly tagged
	if isGroup {
		if message.Text == "" || !strings.Contains(strings.ToLower(message.Text), "@"+strings.ToLower(th.Processor.GetBotUsername())) {
			// Ignore file uploads in group chats if not tagged
			return "", nil
		}
	}

	// Only accept text files
	if !strings.HasSuffix(strings.ToLower(document.FileName), ".txt") {
		errMsg := "❌ *Unsupported File Type*\n\nPlease upload a `.txt` file containing your source code."
		if err := th.Processor.SendMessage(message.Chat.ID, errMsg, message.MessageID); err != nil {
			log.Printf("Failed to send unsupported file type message: %v", err)
		}
		return "", nil
	}

	// Download the file from Telegram
	fileURL, err := th.getFileURL(document.FileID)
	if err != nil {
		log.Printf("Failed to get file URL: %v", err)
		errMsg := "❌ *File Retrieval Error*\n\nFailed to retrieve the uploaded file. Please try again."
		if err := th.Processor.SendMessage(message.Chat.ID, errMsg, message.MessageID); err != nil {
			log.Printf("Failed to send file retrieval error message: %v", err)
		}
		return "", err
	}

	fileContent, err := th.downloadFile(fileURL)
	if err != nil {
		log.Printf("Failed to download file: %v", err)
		errMsg := "❌ *File Download Error*\n\nFailed to download the uploaded file. Please ensure the file is accessible."
		if err := th.Processor.SendMessage(message.Chat.ID, errMsg, message.MessageID); err != nil {
			log.Printf("Failed to send file download error message: %v", err)
		}
		return "", err
	}

	// Store the file content associated with the user
	if err := th.Processor.StoreUserSourceCode(message.From.ID, fileContent); err != nil {
		log.Printf("Failed to store user source code: %v", err)
		errMsg := "❌ *File Processing Error*\n\nFailed to process the uploaded file. Please try again."
		if err := th.Processor.SendMessage(message.Chat.ID, errMsg, message.MessageID); err != nil {
			log.Printf("Failed to send storage error message: %v", err)
		}
		return "", err
	}

	// Calculate deletion time
	uploadedAt := time.Now()
	deletionTime := uploadedAt.Add(types.FileRetentionTime) // Accessing FileRetentionTime from types package

	// Send confirmation message with UTC and EDT upload and deletion times
	confirmationMsg := fmt.Sprintf(
		"✅ *File Uploaded Successfully*\n\nYour source code has been uploaded and will be stored until:\n\n"+
			"• *Upload Time:* UTC: %s | EDT: %s\n"+
			"• *Deletion Time:* UTC: %s | EDT: %s\n\n"+
			"Please save any work or prompts that may be useful in the future.",
		uploadedAt.UTC().Format(time.RFC1123),
		uploadedAt.In(time.FixedZone("EDT", -4*3600)).Format(time.RFC1123),
		deletionTime.UTC().Format(time.RFC1123),
		deletionTime.In(time.FixedZone("EDT", -4*3600)).Format(time.RFC1123),
	)
	if err := th.Processor.SendMessage(message.Chat.ID, confirmationMsg, message.MessageID); err != nil {
		log.Printf("Failed to send confirmation message: %v", err)
	}

	// Send analysis summary using the Processor's AnalyzeUserCode method
	summary, err := th.Processor.AnalyzeUserCode(message.From.ID)
	if err != nil {
		log.Printf("Failed to analyze user code: %v", err)
		// Optionally notify the user about the failure
		return "", nil
	}

	analysisMsg := fmt.Sprintf(
		"🔍 *Code Analysis Summary:*\n\n%s\n\nYou can reference your source code using `#source_code` in your questions.",
		summary,
	)
	if err := th.Processor.SendMessage(message.Chat.ID, analysisMsg, message.MessageID); err != nil {
		log.Printf("Failed to send code analysis summary message: %v", err)
	}

	return "", nil
}

// getFileURL retrieves the download URL for the given file ID using Telegram's getFile API.
func (th *TelegramHandler) getFileURL(fileID string) (string, error) {
	// Retrieve the Telegram token from the processor
	token := th.Processor.GetTelegramToken()
	if token == "" {
		return "", errors.New("telegram token not found")
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/getFile?file_id=%s", token, fileID)
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var fileResponse types.TelegramFileResponse
	if err := json.NewDecoder(resp.Body).Decode(&fileResponse); err != nil {
		return "", err
	}

	if !fileResponse.OK || fileResponse.Result.FilePath == "" {
		return "", errors.New("invalid file response from Telegram")
	}

	downloadURL := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", token, fileResponse.Result.FilePath)
	return downloadURL, nil
}

// downloadFile downloads the file content from the given URL.
func (th *TelegramHandler) downloadFile(url string) (string, error) {
	resp, err := th.client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", errors.New("failed to download file from Telegram")
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(bodyBytes), nil
}

// isTaggedMention checks if the mention is directed at the bot.
func isTaggedMention(mention, botUsername string) bool {
	return strings.ToLower(mention) == "@"+strings.ToLower(botUsername)
}

// removeMention removes the bot mention from the user's message.
func removeMention(text, mention string) string {
	return strings.TrimSpace(strings.Replace(text, mention, "", 1))
}

// FetchUpdates fetches updates from Telegram using long polling.
// It maintains the offset to ensure each update is processed only once.
func (th *TelegramHandler) FetchUpdates() ([]types.TelegramUpdate, error) {
	// Retrieve the Telegram token from the processor
	token := th.Processor.GetTelegramToken()
	if token == "" {
		return nil, errors.New("telegram token not found")
	}

	th.mutex.Lock()
	defer th.mutex.Unlock()

	url := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates?timeout=100&offset=%d", token, th.offset+1)

	resp, err := th.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get updates from Telegram: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("telegram getUpdates responded with status %s: %s", resp.Status, string(bodyBytes))
	}

	var updatesResponse types.TelegramUpdatesResponse
	if err := json.NewDecoder(resp.Body).Decode(&updatesResponse); err != nil {
		return nil, fmt.Errorf("failed to decode Telegram updates response: %v", err)
	}

	if !updatesResponse.OK {
		return nil, fmt.Errorf("telegram getUpdates returned not ok: %v", updatesResponse)
	}

	updates := updatesResponse.Result

	// Update the offset
	for _, update := range updates {
		if update.UpdateID > th.offset {
			th.offset = update.UpdateID
		}
	}

	return updates, nil
}

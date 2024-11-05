// internal/handlers/handlers.go

package handlers

import (
	"KernelSandersBot/internal/types"
)

// MessageProcessor defines the methods that the telegram package requires from the app package.
type MessageProcessor interface {
	ProcessMessage(chatID int64, userID int, username string, userQuestion string, messageID int) error
	HandleCommand(message *types.TelegramMessage, userID int, username string) (string, error)
	SendMessage(chatID int64, text string, replyToMessageID int) error
	GetBotUsername() string
	GetTelegramToken() string                           // Added to support file download
	StoreUserSourceCode(userID int, code string) error  // Added to store source code
	ListUserFiles(userID int) ([]types.UserFile, error) // Updated to use types.UserFile
	GetUserData(userID int) (string, error)             // Added to get user data
	HandleUpdate(update *types.TelegramUpdate)          // Added to handle incoming updates
	GetUserSourceCode(userID int) (string, bool)        // Added to retrieve user source code
	GetSummary(prompt string) (string, error)           // Added to generate summary of user source code
	AnalyzeUserCode(userID int) (string, error)         // **Added AnalyzeUserCode method**
}

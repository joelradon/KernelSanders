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

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

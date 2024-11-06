// internal/types/types.go

package types

import (
	"time"
)

// UserFile represents a user's uploaded file or shared repository with timestamps.
type UserFile struct {
	FileName        string
	UploadedAtUTC   time.Time
	UploadedAtEDT   time.Time
	DeletionTimeUTC time.Time
	DeletionTimeEDT time.Time
}

// UserResponse represents a user's web response with timestamps.
type UserResponse struct {
	ID              string
	CreatedAtUTC    time.Time
	CreatedAtEDT    time.Time
	DeletionTimeUTC time.Time
	DeletionTimeEDT time.Time
}

// TelegramUpdate represents an incoming update from Telegram.
type TelegramUpdate struct {
	UpdateID      int              `json:"update_id"`
	Message       *TelegramMessage `json:"message,omitempty"`
	EditedMessage *TelegramMessage `json:"edited_message,omitempty"`
	ChannelPost   *TelegramMessage `json:"channel_post,omitempty"`
}

// TelegramMessage represents a message in Telegram.
type TelegramMessage struct {
	MessageID      int               `json:"message_id"`
	From           TelegramUser      `json:"from"`
	Chat           TelegramChat      `json:"chat"`
	Date           int               `json:"date"`
	Text           string            `json:"text,omitempty"`
	Entities       []TelegramEntity  `json:"entities,omitempty"`
	ReplyToMessage *TelegramMessage  `json:"reply_to_message,omitempty"`
	Document       *TelegramDocument `json:"document,omitempty"` // Added to handle file uploads
}

// TelegramDocument represents a document (file) in Telegram.
type TelegramDocument struct {
	FileID   string `json:"file_id"`
	FileName string `json:"file_name"`
	FileSize int    `json:"file_size"`
	MimeType string `json:"mime_type,omitempty"`
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

// TelegramFileResponse represents the response from Telegram's getFile API.
type TelegramFileResponse struct {
	OK     bool             `json:"ok"`
	Result TelegramFileInfo `json:"result"`
}

// TelegramFileInfo contains information about the file from Telegram's getFile API.
type TelegramFileInfo struct {
	FileID   string `json:"file_id"`
	FileSize int    `json:"file_size"`
	FilePath string `json:"file_path"`
}

// TelegramUpdatesResponse represents the response from Telegram's getUpdates API.
type TelegramUpdatesResponse struct {
	OK     bool             `json:"ok"`
	Result []TelegramUpdate `json:"result"`
}

// Constants
const FileRetentionTime = 4 * time.Hour

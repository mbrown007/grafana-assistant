package chatops

import "context"

// Provider abstracts a messaging platform (Mattermost, Teams, etc.).
type Provider interface {
	// Start begins listening for inbound messages. Blocks until ctx is cancelled.
	Start(ctx context.Context, handler MessageHandler) error
	// Stop gracefully disconnects from the messaging platform.
	Stop() error
	// SendMessage posts a response to a channel, optionally in a thread.
	SendMessage(ctx context.Context, channelID, threadID, content string) error
	// BotUsername returns the bot's username for mention filtering.
	BotUsername() string
}

// MessageHandler is called for each inbound message the bridge should process.
type MessageHandler func(ctx context.Context, msg IncomingMessage)

// IncomingMessage represents a message received from a messaging platform.
type IncomingMessage struct {
	ChannelID  string // platform channel identifier
	ThreadID   string // root post/thread ID (empty = new thread)
	SenderID   string // platform user ID of the sender
	SenderName string // display name of the sender
	Text       string // message body with mention stripped
}

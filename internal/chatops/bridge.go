package chatops

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/brownster/grafana-assistant/internal/api"
	"github.com/brownster/grafana-assistant/internal/grafana"
)

const sessionIdleTTL = 1 * time.Hour

// ChatHandler is satisfied by *agent.Manager.
type ChatHandler interface {
	HandleChat(ctx context.Context, user *grafana.User, req api.ChatRequest, streamFn func(api.StreamChunk))
}

// Bridge connects a messaging Provider to the assistant's ChatHandler.
type Bridge struct {
	provider    Provider
	chatHandler ChatHandler
	botUser     *grafana.User
	sessions    *sessionMap
}

// NewBridge creates a ChatOps bridge that forwards messages from the provider
// to the assistant and posts responses back.
func NewBridge(provider Provider, chatHandler ChatHandler, botUser *grafana.User) *Bridge {
	return &Bridge{
		provider:    provider,
		chatHandler: chatHandler,
		botUser:     botUser,
		sessions:    newSessionMap(),
	}
}

// Start begins listening for messages from the provider. Blocks until ctx is
// cancelled or the provider returns an error.
func (b *Bridge) Start(ctx context.Context) error {
	slog.InfoContext(ctx, "chatops bridge starting",
		"event", "chatops_bridge",
		"bot_user", b.botUser.Login,
	)
	return b.provider.Start(ctx, b.handleMessage)
}

// Stop gracefully shuts down the provider.
func (b *Bridge) Stop() error {
	return b.provider.Stop()
}

func (b *Bridge) handleMessage(ctx context.Context, msg IncomingMessage) {
	if msg.Text == "" {
		return
	}

	threadKey := msg.ChannelID + ":" + msg.ThreadID
	sessionID := b.sessions.getOrCreate(threadKey)

	slog.InfoContext(ctx, "chatops message received",
		"event", "chatops_message",
		"channel", msg.ChannelID,
		"thread", msg.ThreadID,
		"sender", msg.SenderName,
		"session_id", sessionID,
		"message_len", len(msg.Text),
	)

	req := api.ChatRequest{
		Message:   msg.Text,
		SessionID: sessionID,
	}

	var mu sync.Mutex
	var chunks []api.StreamChunk

	streamFn := func(chunk api.StreamChunk) {
		mu.Lock()
		chunks = append(chunks, chunk)
		// Capture actual session_id from start event.
		if chunk.Type == "start" && chunk.SessionID != "" {
			b.sessions.updateSessionID(threadKey, chunk.SessionID)
		}
		mu.Unlock()
	}

	b.chatHandler.HandleChat(ctx, b.botUser, req, streamFn)

	mu.Lock()
	collected := chunks
	mu.Unlock()

	response := FormatResponse(collected)

	// Use the original message's thread ID for replies; if this is a new
	// conversation (no thread yet), some providers create a new thread.
	replyThread := msg.ThreadID

	if err := b.provider.SendMessage(ctx, msg.ChannelID, replyThread, response); err != nil {
		slog.ErrorContext(ctx, "chatops failed to send response",
			"event", "chatops_send_error",
			"channel", msg.ChannelID,
			"thread", replyThread,
			"error", err,
		)
	}
}

// sessionMap maps channel:thread keys to assistant session IDs with TTL.
type sessionMap struct {
	mu      sync.Mutex
	entries map[string]*sessionEntry
}

type sessionEntry struct {
	sessionID  string
	lastActive time.Time
}

func newSessionMap() *sessionMap {
	sm := &sessionMap{
		entries: make(map[string]*sessionEntry),
	}
	go sm.evictLoop()
	return sm
}

func (sm *sessionMap) getOrCreate(key string) string {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if e, ok := sm.entries[key]; ok {
		e.lastActive = time.Now()
		return e.sessionID
	}

	sid := uuid.New().String()
	sm.entries[key] = &sessionEntry{
		sessionID:  sid,
		lastActive: time.Now(),
	}
	return sid
}

func (sm *sessionMap) updateSessionID(key, sessionID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if e, ok := sm.entries[key]; ok {
		e.sessionID = sessionID
		e.lastActive = time.Now()
	}
}

func (sm *sessionMap) evictLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		sm.evictExpired()
	}
}

func (sm *sessionMap) evictExpired() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	cutoff := time.Now().Add(-sessionIdleTTL)
	for k, e := range sm.entries {
		if e.lastActive.Before(cutoff) {
			delete(sm.entries, k)
		}
	}
}

package chatops

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/marcusz/monitoring-assistant/internal/api"
	"github.com/marcusz/monitoring-assistant/internal/grafana"
)

// --- Mock provider ---------------------------------------------------------

type mockProvider struct {
	mu       sync.Mutex
	sent     []sentMessage
	startErr error
	handler  MessageHandler
}

type sentMessage struct {
	ChannelID string
	ThreadID  string
	Content   string
}

func (m *mockProvider) Start(ctx context.Context, handler MessageHandler) error {
	m.handler = handler
	if m.startErr != nil {
		return m.startErr
	}
	<-ctx.Done()
	return ctx.Err()
}

func (m *mockProvider) Stop() error { return nil }

func (m *mockProvider) SendMessage(_ context.Context, channelID, threadID, content string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, sentMessage{channelID, threadID, content})
	return nil
}

func (m *mockProvider) BotUsername() string { return "test-bot" }

func (m *mockProvider) getSent() []sentMessage {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]sentMessage, len(m.sent))
	copy(cp, m.sent)
	return cp
}

// --- Mock chat handler -----------------------------------------------------

type mockChatHandler struct {
	mu       sync.Mutex
	calls    []chatCall
	response []api.StreamChunk
}

type chatCall struct {
	UserLogin string
	Message   string
	SessionID string
}

func (m *mockChatHandler) HandleChat(_ context.Context, user *grafana.User, req api.ChatRequest, streamFn func(api.StreamChunk)) {
	m.mu.Lock()
	m.calls = append(m.calls, chatCall{
		UserLogin: user.Login,
		Message:   req.Message,
		SessionID: req.SessionID,
	})
	resp := m.response
	m.mu.Unlock()

	for _, c := range resp {
		streamFn(c)
	}
}

func (m *mockChatHandler) getCalls() []chatCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]chatCall, len(m.calls))
	copy(cp, m.calls)
	return cp
}

// --- Tests -----------------------------------------------------------------

func TestBridge_HandleMessage_CallsHandleChat(t *testing.T) {
	provider := &mockProvider{}
	handler := &mockChatHandler{
		response: []api.StreamChunk{
			{Type: "start", SessionID: "real-session-1"},
			{Type: "token", Message: "Hello!"},
			{Type: "complete"},
		},
	}
	botUser := &grafana.User{ID: 99, Login: "bot", Name: "Bot", OrgID: 1}
	bridge := NewBridge(provider, handler, botUser)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start bridge in background.
	go bridge.Start(ctx)
	// Wait for provider.Start to be called.
	time.Sleep(50 * time.Millisecond)

	// Simulate an inbound message.
	bridge.handleMessage(ctx, IncomingMessage{
		ChannelID:  "ch1",
		ThreadID:   "t1",
		SenderID:   "user1",
		SenderName: "Alice",
		Text:       "What is the CPU usage?",
	})

	calls := handler.getCalls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 HandleChat call, got %d", len(calls))
	}
	if calls[0].Message != "What is the CPU usage?" {
		t.Errorf("message = %q, want %q", calls[0].Message, "What is the CPU usage?")
	}
	if calls[0].UserLogin != "bot" {
		t.Errorf("user login = %q, want %q", calls[0].UserLogin, "bot")
	}

	sent := provider.getSent()
	if len(sent) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(sent))
	}
	if sent[0].ChannelID != "ch1" {
		t.Errorf("channel = %q, want %q", sent[0].ChannelID, "ch1")
	}
	if sent[0].ThreadID != "t1" {
		t.Errorf("thread = %q, want %q", sent[0].ThreadID, "t1")
	}
	if sent[0].Content != "Hello!" {
		t.Errorf("content = %q, want %q", sent[0].Content, "Hello!")
	}
}

func TestBridge_SessionMapping_SameThread(t *testing.T) {
	provider := &mockProvider{}
	handler := &mockChatHandler{
		response: []api.StreamChunk{
			{Type: "start", SessionID: "session-abc"},
			{Type: "token", Message: "OK"},
		},
	}
	botUser := &grafana.User{ID: 99, Login: "bot", Name: "Bot", OrgID: 1}
	bridge := NewBridge(provider, handler, botUser)

	ctx := context.Background()

	// First message creates session.
	bridge.handleMessage(ctx, IncomingMessage{
		ChannelID: "ch1", ThreadID: "t1", Text: "msg1",
	})

	// Second message in same thread should reuse session.
	bridge.handleMessage(ctx, IncomingMessage{
		ChannelID: "ch1", ThreadID: "t1", Text: "msg2",
	})

	calls := handler.getCalls()
	if len(calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(calls))
	}

	// After the first call, the session_id should be updated to "session-abc".
	// The second call should use "session-abc".
	if calls[1].SessionID != "session-abc" {
		t.Errorf("second call session = %q, want %q", calls[1].SessionID, "session-abc")
	}
}

func TestBridge_SessionMapping_DifferentThreads(t *testing.T) {
	provider := &mockProvider{}
	handler := &mockChatHandler{
		response: []api.StreamChunk{
			{Type: "token", Message: "OK"},
		},
	}
	botUser := &grafana.User{ID: 99, Login: "bot", Name: "Bot", OrgID: 1}
	bridge := NewBridge(provider, handler, botUser)

	ctx := context.Background()

	bridge.handleMessage(ctx, IncomingMessage{
		ChannelID: "ch1", ThreadID: "t1", Text: "msg1",
	})
	bridge.handleMessage(ctx, IncomingMessage{
		ChannelID: "ch1", ThreadID: "t2", Text: "msg2",
	})

	calls := handler.getCalls()
	if len(calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(calls))
	}
	if calls[0].SessionID == calls[1].SessionID {
		t.Error("different threads should have different session IDs")
	}
}

func TestBridge_EmptyMessage_Ignored(t *testing.T) {
	provider := &mockProvider{}
	handler := &mockChatHandler{}
	botUser := &grafana.User{ID: 99, Login: "bot", OrgID: 1}
	bridge := NewBridge(provider, handler, botUser)

	bridge.handleMessage(context.Background(), IncomingMessage{
		ChannelID: "ch1", ThreadID: "t1", Text: "",
	})

	if len(handler.getCalls()) != 0 {
		t.Error("empty message should not call HandleChat")
	}
}

func TestSessionMap_EvictExpired(t *testing.T) {
	sm := &sessionMap{
		entries: make(map[string]*sessionEntry),
	}

	// Add an entry that's already expired.
	sm.entries["old"] = &sessionEntry{
		sessionID:  "s1",
		lastActive: time.Now().Add(-2 * sessionIdleTTL),
	}
	// Add a fresh entry.
	sm.entries["fresh"] = &sessionEntry{
		sessionID:  "s2",
		lastActive: time.Now(),
	}

	sm.evictExpired()

	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, ok := sm.entries["old"]; ok {
		t.Error("expired entry should be evicted")
	}
	if _, ok := sm.entries["fresh"]; !ok {
		t.Error("fresh entry should remain")
	}
}

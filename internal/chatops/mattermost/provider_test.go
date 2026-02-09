package mattermost

import (
	"strings"
	"testing"

	"github.com/brownster/grafana-assistant/internal/chatops"
)

func TestParsePostedEvent_ValidMention(t *testing.T) {
	p := &Provider{
		cfg: Config{
			BotUserID:  "bot123",
			BotUsername: "assistant-bot",
		},
		channelSet: map[string]struct{}{},
	}

	event := wsEvent{
		Event: "posted",
		Data: map[string]any{
			"post":        `{"id":"p1","channel_id":"ch1","user_id":"user1","root_id":"","message":"@assistant-bot what is the CPU usage?"}`,
			"sender_name": "alice",
		},
	}

	msg, ok := p.parsePostedEvent(event)
	if !ok {
		t.Fatal("expected message to be parsed")
	}
	if msg.ChannelID != "ch1" {
		t.Errorf("ChannelID = %q, want %q", msg.ChannelID, "ch1")
	}
	if msg.SenderName != "alice" {
		t.Errorf("SenderName = %q, want %q", msg.SenderName, "alice")
	}
	if msg.SenderID != "user1" {
		t.Errorf("SenderID = %q, want %q", msg.SenderID, "user1")
	}
	// Thread ID should be the post's own ID (new root post).
	if msg.ThreadID != "p1" {
		t.Errorf("ThreadID = %q, want %q", msg.ThreadID, "p1")
	}
	if msg.Text != "what is the CPU usage?" {
		t.Errorf("Text = %q, want %q", msg.Text, "what is the CPU usage?")
	}
}

func TestParsePostedEvent_ThreadReply(t *testing.T) {
	p := &Provider{
		cfg: Config{
			BotUserID:  "bot123",
			BotUsername: "assistant-bot",
		},
		channelSet: map[string]struct{}{},
	}

	event := wsEvent{
		Event: "posted",
		Data: map[string]any{
			"post": `{"id":"p2","channel_id":"ch1","user_id":"user1","root_id":"p1","message":"@assistant-bot follow up question"}`,
		},
	}

	msg, ok := p.parsePostedEvent(event)
	if !ok {
		t.Fatal("expected message to be parsed")
	}
	// Thread ID should be the root_id for reply posts.
	if msg.ThreadID != "p1" {
		t.Errorf("ThreadID = %q, want %q (root_id)", msg.ThreadID, "p1")
	}
}

func TestParsePostedEvent_IgnoreOwnMessages(t *testing.T) {
	p := &Provider{
		cfg: Config{
			BotUserID:  "bot123",
			BotUsername: "assistant-bot",
		},
		channelSet: map[string]struct{}{},
	}

	event := wsEvent{
		Event: "posted",
		Data: map[string]any{
			"post": `{"id":"p1","channel_id":"ch1","user_id":"bot123","message":"@assistant-bot hello"}`,
		},
	}

	_, ok := p.parsePostedEvent(event)
	if ok {
		t.Error("should ignore own messages")
	}
}

func TestParsePostedEvent_NoMention(t *testing.T) {
	p := &Provider{
		cfg: Config{
			BotUserID:  "bot123",
			BotUsername: "assistant-bot",
		},
		channelSet: map[string]struct{}{},
	}

	event := wsEvent{
		Event: "posted",
		Data: map[string]any{
			"post": `{"id":"p1","channel_id":"ch1","user_id":"user1","message":"hello everyone"}`,
		},
	}

	_, ok := p.parsePostedEvent(event)
	if ok {
		t.Error("should ignore messages without bot mention")
	}
}

func TestParsePostedEvent_ChannelRestriction(t *testing.T) {
	p := &Provider{
		cfg: Config{
			BotUserID:  "bot123",
			BotUsername: "assistant-bot",
		},
		channelSet: map[string]struct{}{
			"allowed-ch": {},
		},
	}

	// Message in non-allowed channel.
	event := wsEvent{
		Event: "posted",
		Data: map[string]any{
			"post": `{"id":"p1","channel_id":"other-ch","user_id":"user1","message":"@assistant-bot hello"}`,
		},
	}

	_, ok := p.parsePostedEvent(event)
	if ok {
		t.Error("should ignore messages in restricted channels")
	}

	// Message in allowed channel.
	event.Data["post"] = `{"id":"p1","channel_id":"allowed-ch","user_id":"user1","message":"@assistant-bot hello"}`
	_, ok = p.parsePostedEvent(event)
	if !ok {
		t.Error("should accept messages in allowed channels")
	}
}

func TestParsePostedEvent_MentionOnly_Ignored(t *testing.T) {
	p := &Provider{
		cfg: Config{
			BotUserID:  "bot123",
			BotUsername: "assistant-bot",
		},
		channelSet: map[string]struct{}{},
	}

	event := wsEvent{
		Event: "posted",
		Data: map[string]any{
			"post": `{"id":"p1","channel_id":"ch1","user_id":"user1","message":"@assistant-bot"}`,
		},
	}

	_, ok := p.parsePostedEvent(event)
	if ok {
		t.Error("should ignore messages that are just the mention with no text")
	}
}

func TestStripMention(t *testing.T) {
	tests := []struct {
		message string
		mention string
		want    string
	}{
		{"@bot hello world", "@bot", " hello world"},
		{"Hey @bot how are you?", "@bot", "Hey  how are you?"},
		{"@BOT uppercase test", "@bot", " uppercase test"},
		{"no mention here", "@bot", "no mention here"},
	}

	for _, tt := range tests {
		got := stripMention(tt.message, tt.mention)
		if got != tt.want {
			t.Errorf("stripMention(%q, %q) = %q, want %q", tt.message, tt.mention, got, tt.want)
		}
	}
}

func TestBuildWSURL(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"http://localhost:8065", "ws://localhost:8065/api/v4/websocket"},
		{"https://mattermost.example.com", "wss://mattermost.example.com/api/v4/websocket"},
		{"http://mm:8065/subpath", "ws://mm:8065/api/v4/websocket"},
	}

	for _, tt := range tests {
		p := &Provider{cfg: Config{URL: tt.url}}
		got, err := p.buildWSURL()
		if err != nil {
			t.Errorf("buildWSURL(%q) error: %v", tt.url, err)
			continue
		}
		if got != tt.want {
			t.Errorf("buildWSURL(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}

func TestParsePostedEvent_InvalidPostJSON(t *testing.T) {
	p := &Provider{
		cfg:        Config{BotUserID: "bot123", BotUsername: "bot"},
		channelSet: map[string]struct{}{},
	}

	event := wsEvent{
		Event: "posted",
		Data: map[string]any{
			"post": `{invalid json`,
		},
	}
	_, ok := p.parsePostedEvent(event)
	if ok {
		t.Error("should return false for invalid JSON")
	}
}

func TestParsePostedEvent_MissingPostField(t *testing.T) {
	p := &Provider{
		cfg:        Config{BotUserID: "bot123", BotUsername: "bot"},
		channelSet: map[string]struct{}{},
	}

	event := wsEvent{
		Event: "posted",
		Data:  map[string]any{},
	}
	_, ok := p.parsePostedEvent(event)
	if ok {
		t.Error("should return false when post field is missing")
	}
}

func TestParsePostedEvent_CaseInsensitiveMention(t *testing.T) {
	p := &Provider{
		cfg: Config{
			BotUserID:  "bot123",
			BotUsername: "Assistant-Bot",
		},
		channelSet: map[string]struct{}{},
	}

	event := wsEvent{
		Event: "posted",
		Data: map[string]any{
			"post": `{"id":"p1","channel_id":"ch1","user_id":"user1","message":"@assistant-bot query something"}`,
		},
	}

	msg, ok := p.parsePostedEvent(event)
	if !ok {
		t.Fatal("should accept case-insensitive mentions")
	}
	if !strings.Contains(msg.Text, "query something") {
		t.Errorf("Text = %q, expected to contain %q", msg.Text, "query something")
	}
}

// Verify the IncomingMessage struct satisfies expected fields.
func TestIncomingMessage_Fields(t *testing.T) {
	msg := chatops.IncomingMessage{
		ChannelID:  "ch",
		ThreadID:   "th",
		SenderID:   "sid",
		SenderName: "sname",
		Text:       "text",
	}
	if msg.ChannelID != "ch" || msg.ThreadID != "th" || msg.SenderID != "sid" || msg.SenderName != "sname" || msg.Text != "text" {
		t.Error("unexpected field values")
	}
}

package mattermost

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/marcusz/monitoring-assistant/internal/chatops"
)

// Config holds Mattermost connection settings.
type Config struct {
	URL         string   // e.g. "http://localhost:18065"
	Token       string   // bot personal access token
	BotUserID   string   // resolved at startup via /api/v4/users/me
	BotUsername string   // resolved at startup
	TeamName    string   // team to operate in
	ChannelIDs  []string // restrict to these channels (empty = all)
}

// Provider implements chatops.Provider for Mattermost.
type Provider struct {
	cfg        Config
	client     *http.Client
	wsConn     *websocket.Conn
	wsMu       sync.Mutex
	channelSet map[string]struct{} // fast lookup for channel restriction
	cancel     context.CancelFunc
}

// New creates a Mattermost provider.
func New(cfg Config) *Provider {
	channelSet := make(map[string]struct{}, len(cfg.ChannelIDs))
	for _, id := range cfg.ChannelIDs {
		if id != "" {
			channelSet[id] = struct{}{}
		}
	}
	return &Provider{
		cfg:        cfg,
		client:     &http.Client{Timeout: 30 * time.Second},
		channelSet: channelSet,
	}
}

// BotUsername returns the bot's username for mention filtering.
func (p *Provider) BotUsername() string {
	return p.cfg.BotUsername
}

// Start connects to the Mattermost WebSocket and listens for posted events.
// Blocks until ctx is cancelled.
func (p *Provider) Start(ctx context.Context, handler chatops.MessageHandler) error {
	// Resolve bot identity.
	me, err := p.resolveMe(ctx)
	if err != nil {
		return fmt.Errorf("mattermost resolve bot identity: %w", err)
	}
	p.cfg.BotUserID = me.ID
	p.cfg.BotUsername = me.Username

	slog.InfoContext(ctx, "mattermost provider starting",
		"event", "mattermost_start",
		"bot_user_id", p.cfg.BotUserID,
		"bot_username", p.cfg.BotUsername,
		"team", p.cfg.TeamName,
		"channel_filter_count", len(p.channelSet),
	)

	ctx, cancel := context.WithCancel(ctx)
	p.cancel = cancel

	for {
		if err := p.connectAndListen(ctx, handler); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			slog.WarnContext(ctx, "mattermost websocket disconnected, reconnecting",
				"event", "mattermost_reconnect",
				"error", err,
			)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(3 * time.Second):
			}
		}
	}
}

// Stop closes the WebSocket connection.
func (p *Provider) Stop() error {
	if p.cancel != nil {
		p.cancel()
	}
	p.wsMu.Lock()
	defer p.wsMu.Unlock()
	if p.wsConn != nil {
		return p.wsConn.Close()
	}
	return nil
}

// SendMessage posts a message to a Mattermost channel.
func (p *Provider) SendMessage(ctx context.Context, channelID, threadID, content string) error {
	post := mmPost{
		ChannelID: channelID,
		Message:   content,
	}
	if threadID != "" {
		post.RootID = threadID
	}

	body, err := json.Marshal(post)
	if err != nil {
		return fmt.Errorf("marshal post: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.cfg.URL+"/api/v4/posts", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.cfg.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("send post: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("mattermost POST /api/v4/posts returned %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

func (p *Provider) connectAndListen(ctx context.Context, handler chatops.MessageHandler) error {
	wsURL, err := p.buildWSURL()
	if err != nil {
		return err
	}

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		return fmt.Errorf("websocket dial: %w", err)
	}

	p.wsMu.Lock()
	p.wsConn = conn
	p.wsMu.Unlock()

	defer func() {
		p.wsMu.Lock()
		p.wsConn = nil
		p.wsMu.Unlock()
		conn.Close()
	}()

	// Authenticate via WebSocket.
	authMsg := map[string]any{
		"seq":    1,
		"action": "authentication_challenge",
		"data":   map[string]any{"token": p.cfg.Token},
	}
	if err := conn.WriteJSON(authMsg); err != nil {
		return fmt.Errorf("websocket auth: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		var event wsEvent
		if err := conn.ReadJSON(&event); err != nil {
			return fmt.Errorf("websocket read: %w", err)
		}

		if event.Event != "posted" {
			continue
		}

		msg, ok := p.parsePostedEvent(event)
		if !ok {
			continue
		}

		go handler(ctx, msg)
	}
}

func (p *Provider) parsePostedEvent(event wsEvent) (chatops.IncomingMessage, bool) {
	postJSON, ok := event.Data["post"].(string)
	if !ok {
		return chatops.IncomingMessage{}, false
	}

	var post mmPost
	if err := json.Unmarshal([]byte(postJSON), &post); err != nil {
		return chatops.IncomingMessage{}, false
	}

	// Ignore own messages.
	if post.UserID == p.cfg.BotUserID {
		return chatops.IncomingMessage{}, false
	}

	// Channel restriction.
	if len(p.channelSet) > 0 {
		if _, allowed := p.channelSet[post.ChannelID]; !allowed {
			return chatops.IncomingMessage{}, false
		}
	}

	// Check for bot mention.
	mention := "@" + p.cfg.BotUsername
	if !strings.Contains(strings.ToLower(post.Message), strings.ToLower(mention)) {
		return chatops.IncomingMessage{}, false
	}

	// Strip the mention from the message text.
	text := stripMention(post.Message, mention)
	if strings.TrimSpace(text) == "" {
		return chatops.IncomingMessage{}, false
	}

	// Determine thread context.
	threadID := post.RootID
	if threadID == "" {
		// This is a new root post — use its ID as the thread anchor.
		threadID = post.ID
	}

	senderName, _ := event.Data["sender_name"].(string)
	if senderName == "" {
		senderName = post.UserID
	}

	return chatops.IncomingMessage{
		ChannelID:  post.ChannelID,
		ThreadID:   threadID,
		SenderID:   post.UserID,
		SenderName: senderName,
		Text:       strings.TrimSpace(text),
	}, true
}

func (p *Provider) resolveMe(ctx context.Context) (*mmUser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.cfg.URL+"/api/v4/users/me", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+p.cfg.Token)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("GET /api/v4/users/me returned %d: %s", resp.StatusCode, string(body))
	}

	var user mmUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("decode user: %w", err)
	}
	return &user, nil
}

func (p *Provider) buildWSURL() (string, error) {
	u, err := url.Parse(p.cfg.URL)
	if err != nil {
		return "", fmt.Errorf("parse URL: %w", err)
	}
	switch u.Scheme {
	case "https":
		u.Scheme = "wss"
	default:
		u.Scheme = "ws"
	}
	u.Path = "/api/v4/websocket"
	return u.String(), nil
}

func stripMention(message, mention string) string {
	lower := strings.ToLower(message)
	mentionLower := strings.ToLower(mention)
	idx := strings.Index(lower, mentionLower)
	if idx < 0 {
		return message
	}
	return message[:idx] + message[idx+len(mention):]
}

// Mattermost API types (minimal).

type mmUser struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

type mmPost struct {
	ID        string `json:"id,omitempty"`
	ChannelID string `json:"channel_id"`
	UserID    string `json:"user_id,omitempty"`
	RootID    string `json:"root_id,omitempty"`
	Message   string `json:"message"`
}

type wsEvent struct {
	Event string         `json:"event"`
	Data  map[string]any `json:"data"`
}

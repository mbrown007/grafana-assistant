package agent

import (
	"testing"

	openai "github.com/sashabaranov/go-openai"

	"github.com/marcusz/monitoring-assistant/internal/storage"
)

func TestMemory_BasicAddAndMessages(t *testing.T) {
	mem := NewMemory("You are a helper.", 0)
	mem.Add(openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: "Hello"})
	mem.Add(openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: "Hi there"})

	msgs := mem.Messages()
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages (system + 2), got %d", len(msgs))
	}
	if msgs[0].Role != openai.ChatMessageRoleSystem {
		t.Error("first message should be system")
	}
	if msgs[1].Content != "Hello" {
		t.Error("second message should be user's")
	}
}

func TestMemory_WindowTruncation(t *testing.T) {
	mem := NewMemory("sys", 3)

	for i := 0; i < 10; i++ {
		mem.Add(openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: "msg"})
	}

	msgs := mem.Messages()
	// system + last 3 = 4
	if len(msgs) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(msgs))
	}
	if msgs[0].Role != openai.ChatMessageRoleSystem {
		t.Error("first should be system")
	}
}

func TestMemory_DefaultWindow(t *testing.T) {
	mem := NewMemory("sys", 0)
	if mem.window != defaultWindowSize {
		t.Errorf("expected default window %d, got %d", defaultWindowSize, mem.window)
	}
}

func TestMemory_LoadHistory(t *testing.T) {
	mem := NewMemory("sys", 0)
	mem.LoadHistory([]storage.Message{
		{Role: "user", Content: "previous question"},
		{Role: "assistant", Content: "previous answer"},
	})

	msgs := mem.Messages()
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(msgs))
	}
	if msgs[1].Role != openai.ChatMessageRoleUser {
		t.Error("loaded user message should have correct role")
	}
	if msgs[2].Role != openai.ChatMessageRoleAssistant {
		t.Error("loaded assistant message should have correct role")
	}
}

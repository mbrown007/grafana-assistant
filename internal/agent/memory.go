package agent

import (
	openai "github.com/sashabaranov/go-openai"

	"github.com/brownster/grafana-assistant/internal/storage"
)

const defaultWindowSize = 20

// Memory maintains a sliding window of conversation messages.
type Memory struct {
	system   openai.ChatCompletionMessage
	messages []openai.ChatCompletionMessage
	window   int
}

// NewMemory creates a Memory with the given system prompt and window size.
// If window is <= 0, the default of 20 is used.
func NewMemory(systemPrompt string, window int) *Memory {
	if window <= 0 {
		window = defaultWindowSize
	}
	return &Memory{
		system: openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleSystem,
			Content: systemPrompt,
		},
		window: window,
	}
}

// Add appends a message to the memory.
func (m *Memory) Add(msg openai.ChatCompletionMessage) {
	m.messages = append(m.messages, msg)
}

// Messages returns the system message followed by the most recent messages
// within the window size.
func (m *Memory) Messages() []openai.ChatCompletionMessage {
	msgs := m.messages
	if len(msgs) > m.window {
		msgs = msgs[len(msgs)-m.window:]
	}
	result := make([]openai.ChatCompletionMessage, 0, 1+len(msgs))
	result = append(result, m.system)
	result = append(result, msgs...)
	return result
}

// LoadHistory converts stored messages to OpenAI format and prepends them.
func (m *Memory) LoadHistory(stored []storage.Message) {
	for _, msg := range stored {
		role := openai.ChatMessageRoleUser
		if msg.Role == "assistant" {
			role = openai.ChatMessageRoleAssistant
		}
		m.messages = append(m.messages, openai.ChatCompletionMessage{
			Role:    role,
			Content: msg.Content,
		})
	}
}

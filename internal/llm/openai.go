package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	openai "github.com/sashabaranov/go-openai"
)

// StreamChunk represents a chunk of streaming response.
type StreamChunk struct {
	Type      string         `json:"type"`
	Message   string         `json:"message,omitempty"`
	Tool      string         `json:"tool,omitempty"`
	ToolID    string         `json:"tool_id,omitempty"`
	Arguments map[string]any `json:"arguments,omitempty"`
	Result    any            `json:"result,omitempty"`
}

// Client wraps the OpenAI API for chat completions.
type Client struct {
	client *openai.Client
	model  string
}

// NewClient creates an OpenAI client.
// model defaults to "gpt-4o" if empty.
func NewClient(apiKey, model string) (*Client, error) {
	if apiKey == "" {
		return nil, errors.New("openai api key is required")
	}
	if model == "" {
		model = "gpt-4o"
	}
	return &Client{
		client: openai.NewClient(apiKey),
		model:  model,
	}, nil
}

// Chat performs a non-streaming chat completion.
func (c *Client) Chat(ctx context.Context, messages []openai.ChatCompletionMessage, tools []openai.Tool) (*openai.ChatCompletionMessage, error) {
	req := openai.ChatCompletionRequest{
		Model:    c.model,
		Messages: messages,
		Tools:    tools,
	}

	resp, err := c.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("openai chat: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, errors.New("no response from LLM")
	}

	return &resp.Choices[0].Message, nil
}

// StreamChat performs a streaming chat completion and returns a channel of chunks.
func (c *Client) StreamChat(ctx context.Context, messages []openai.ChatCompletionMessage, tools []openai.Tool) (<-chan StreamChunk, error) {
	req := openai.ChatCompletionRequest{
		Model:    c.model,
		Messages: messages,
		Tools:    tools,
		Stream:   true,
	}

	stream, err := c.client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("openai stream: %w", err)
	}

	chunks := make(chan StreamChunk, 100)

	go func() {
		defer close(chunks)
		defer stream.Close()

		chunks <- StreamChunk{Type: "start"}

		var fullContent string
		var toolCalls []openai.ToolCall

		for {
			response, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				chunks <- StreamChunk{
					Type:    "error",
					Message: fmt.Sprintf("stream error: %v", err),
				}
				return
			}

			if len(response.Choices) == 0 {
				continue
			}

			delta := response.Choices[0].Delta

			if delta.Content != "" {
				fullContent += delta.Content
				chunks <- StreamChunk{
					Type:    "token",
					Message: delta.Content,
				}
			}

			for _, tc := range delta.ToolCalls {
				if tc.Index == nil {
					continue
				}
				idx := *tc.Index
				for len(toolCalls) <= idx {
					toolCalls = append(toolCalls, openai.ToolCall{})
				}
				if tc.ID != "" {
					toolCalls[idx].ID = tc.ID
				}
				if tc.Type != "" {
					toolCalls[idx].Type = tc.Type
				}
				if tc.Function.Name != "" {
					toolCalls[idx].Function.Name = tc.Function.Name
				}
				if tc.Function.Arguments != "" {
					toolCalls[idx].Function.Arguments += tc.Function.Arguments
				}
			}
		}

		// Emit accumulated tool calls.
		for idx, tc := range toolCalls {
			if tc.Function.Name == "" {
				continue
			}
			var args map[string]any
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
				chunks <- StreamChunk{
					Type:    "error",
					Message: fmt.Sprintf("failed to parse tool arguments: %v", err),
				}
				continue
			}
			toolID := tc.ID
			if toolID == "" {
				toolID = fmt.Sprintf("tool-%d", idx)
			}
			chunks <- StreamChunk{
				Type:      "tool",
				Tool:      tc.Function.Name,
				ToolID:    toolID,
				Arguments: args,
			}
		}

		chunks <- StreamChunk{Type: "complete", Message: fullContent}
		chunks <- StreamChunk{Type: "done"}
	}()

	return chunks, nil
}

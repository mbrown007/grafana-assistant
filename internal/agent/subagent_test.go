package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	openai "github.com/sashabaranov/go-openai"

	"github.com/brownster/grafana-assistant/internal/api"
	"github.com/brownster/grafana-assistant/internal/llm"
	"github.com/brownster/grafana-assistant/internal/mcp"
)

type mockLLMResponse struct {
	message openai.ChatCompletionMessage
	usage   llm.ChatUsage
	err     error
}

type mockSubAgentLLM struct {
	mu        sync.Mutex
	responses []mockLLMResponse
	calls     int
	messages  [][]openai.ChatCompletionMessage
	tools     [][]openai.Tool
}

func (m *mockSubAgentLLM) ChatWithOptions(_ context.Context, messages []openai.ChatCompletionMessage, tools []openai.Tool, _ llm.ChatOptions) (*openai.ChatCompletionMessage, llm.ChatUsage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.calls++
	m.messages = append(m.messages, cloneMessages(messages))
	m.tools = append(m.tools, cloneTools(tools))

	idx := m.calls - 1
	if idx >= len(m.responses) {
		fallback := openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: "ok"}
		return &fallback, llm.ChatUsage{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2}, nil
	}
	resp := m.responses[idx]
	msg := resp.message
	if msg.Role == "" {
		msg.Role = openai.ChatMessageRoleAssistant
	}
	return &msg, resp.usage, resp.err
}

func (m *mockSubAgentLLM) Calls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

func (m *mockSubAgentLLM) RecordedMessages() [][]openai.ChatCompletionMessage {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([][]openai.ChatCompletionMessage, len(m.messages))
	for i := range m.messages {
		out[i] = cloneMessages(m.messages[i])
	}
	return out
}

func cloneMessages(in []openai.ChatCompletionMessage) []openai.ChatCompletionMessage {
	out := make([]openai.ChatCompletionMessage, len(in))
	copy(out, in)
	return out
}

func cloneTools(in []openai.Tool) []openai.Tool {
	out := make([]openai.Tool, len(in))
	copy(out, in)
	return out
}

type mockSubAgentMCPClient struct {
	tools    []mcp.Tool
	invokeFn func(name string, args map[string]any) (any, error)

	mu      sync.Mutex
	invoked []string
}

func (m *mockSubAgentMCPClient) Connect(context.Context) error { return nil }
func (m *mockSubAgentMCPClient) Health(context.Context) error  { return nil }
func (m *mockSubAgentMCPClient) DiscoverTools(context.Context) ([]mcp.Tool, error) {
	return m.tools, nil
}
func (m *mockSubAgentMCPClient) InvokeTool(_ context.Context, name string, args map[string]any) (any, error) {
	m.mu.Lock()
	m.invoked = append(m.invoked, name)
	m.mu.Unlock()
	if m.invokeFn == nil {
		return nil, errors.New("invoke function not configured")
	}
	return m.invokeFn(name, args)
}
func (m *mockSubAgentMCPClient) InvokedTools() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, len(m.invoked))
	copy(out, m.invoked)
	return out
}

func assistantToolCallMessage(toolName, argsJSON string) openai.ChatCompletionMessage {
	return openai.ChatCompletionMessage{
		Role: openai.ChatMessageRoleAssistant,
		ToolCalls: []openai.ToolCall{
			{
				ID:   "call_1",
				Type: openai.ToolTypeFunction,
				Function: openai.FunctionCall{
					Name:      toolName,
					Arguments: argsJSON,
				},
			},
		},
	}
}

func TestSubAgentExecute_ToolCallsAndResultCollection(t *testing.T) {
	toolName := "grafana__search_dashboards"
	llmClient := &mockSubAgentLLM{
		responses: []mockLLMResponse{
			{
				message: assistantToolCallMessage(toolName, `{"query":"payments"}`),
				usage: llm.ChatUsage{
					PromptTokens:     11,
					CompletionTokens: 3,
					TotalTokens:      14,
				},
			},
			{
				message: openai.ChatCompletionMessage{
					Role:    openai.ChatMessageRoleAssistant,
					Content: "Found dashboard 'Payments API' (uid=dash-1, folder=SRE).",
				},
				usage: llm.ChatUsage{
					PromptTokens:     9,
					CompletionTokens: 4,
					TotalTokens:      13,
				},
			},
		},
	}

	mcpClient := &mockSubAgentMCPClient{
		invokeFn: func(name string, args map[string]any) (any, error) {
			if name != toolName {
				return nil, fmt.Errorf("unexpected tool %q", name)
			}
			if args["query"] != "payments" {
				return nil, fmt.Errorf("unexpected query arg: %v", args["query"])
			}
			return map[string]any{"status": "ok", "uid": "dash-1"}, nil
		},
	}

	sa := &SubAgent{
		Name:          "dashboard",
		SystemPrompt:  dashboardSpecialistPrompt,
		Tools:         []mcp.Tool{{Name: toolName, Description: "Search dashboards"}},
		MaxIterations: 3,
	}

	chunks := make([]api.StreamChunk, 0)
	result := sa.executeWithClient(
		context.Background(),
		llmClient,
		map[string]mcp.Client{toolName: mcpClient},
		"find dashboard for payments",
		func(c api.StreamChunk) { chunks = append(chunks, c) },
	)

	if result.Error != "" {
		t.Fatalf("unexpected result error: %s", result.Error)
	}
	if !strings.Contains(result.Summary, "Found dashboard") {
		t.Fatalf("unexpected summary: %q", result.Summary)
	}
	if len(result.ToolsUsed) != 1 || result.ToolsUsed[0] != toolName {
		t.Fatalf("ToolsUsed = %v, want [%s]", result.ToolsUsed, toolName)
	}
	if result.TokensUsed != 27 {
		t.Fatalf("TokensUsed = %d, want %d", result.TokensUsed, 27)
	}
	if result.DurationSeconds < 0 {
		t.Fatalf("DurationSeconds = %f, want >= 0", result.DurationSeconds)
	}
	invoked := mcpClient.InvokedTools()
	if len(invoked) != 1 || invoked[0] != toolName {
		t.Fatalf("invoked tools = %v, want [%s]", invoked, toolName)
	}
	if llmClient.Calls() != 2 {
		t.Fatalf("expected 2 llm calls, got %d", llmClient.Calls())
	}

	hasPrefixedToolChunk := false
	for _, c := range chunks {
		if c.Type == "tool" && c.SubAgent == "dashboard" && strings.HasPrefix(c.Tool, "sub_agent:dashboard:") {
			hasPrefixedToolChunk = true
			break
		}
	}
	if !hasPrefixedToolChunk {
		t.Fatalf("expected sub-agent-prefixed tool chunk, got %v", chunks)
	}
}

func TestSubAgentExecute_IsolatedMemoryMessages(t *testing.T) {
	llmClient := &mockSubAgentLLM{
		responses: []mockLLMResponse{
			{
				message: openai.ChatCompletionMessage{
					Role:    openai.ChatMessageRoleAssistant,
					Content: "investigation complete",
				},
				usage: llm.ChatUsage{PromptTokens: 8, CompletionTokens: 5, TotalTokens: 13},
			},
		},
	}

	systemPrompt := "ISOLATED_SYSTEM_PROMPT"
	userMessage := "investigate checkout latency"
	sa := &SubAgent{
		Name:          "investigation",
		SystemPrompt:  systemPrompt,
		Tools:         nil,
		MaxIterations: 3,
	}
	result := sa.executeWithClient(context.Background(), llmClient, map[string]mcp.Client{}, userMessage, nil)
	if result.Error != "" {
		t.Fatalf("unexpected result error: %s", result.Error)
	}

	requests := llmClient.RecordedMessages()
	if len(requests) == 0 {
		t.Fatal("expected at least one LLM request")
	}
	first := requests[0]
	if len(first) != 2 {
		t.Fatalf("expected exactly 2 messages in first request, got %d", len(first))
	}
	if first[0].Role != openai.ChatMessageRoleSystem || first[0].Content != systemPrompt {
		t.Fatalf("unexpected system message: %+v", first[0])
	}
	if first[1].Role != openai.ChatMessageRoleUser || first[1].Content != userMessage {
		t.Fatalf("unexpected user message: %+v", first[1])
	}
}

func TestSubAgentExecute_MaxIterationsRespected(t *testing.T) {
	toolName := "grafana__query_prometheus"
	llmClient := &mockSubAgentLLM{
		responses: []mockLLMResponse{
			{message: assistantToolCallMessage(toolName, `{"query":"up"}`), usage: llm.ChatUsage{PromptTokens: 4, CompletionTokens: 2, TotalTokens: 6}},
			{message: assistantToolCallMessage(toolName, `{"query":"up"}`), usage: llm.ChatUsage{PromptTokens: 4, CompletionTokens: 2, TotalTokens: 6}},
			{message: assistantToolCallMessage(toolName, `{"query":"up"}`), usage: llm.ChatUsage{PromptTokens: 4, CompletionTokens: 2, TotalTokens: 6}},
		},
	}
	mcpClient := &mockSubAgentMCPClient{
		invokeFn: func(name string, args map[string]any) (any, error) {
			return map[string]any{"status": "ok", "data": "sample"}, nil
		},
	}

	sa := &SubAgent{
		Name:          "investigation",
		SystemPrompt:  "investigation prompt",
		Tools:         []mcp.Tool{{Name: toolName}},
		MaxIterations: 2,
	}
	result := sa.executeWithClient(context.Background(), llmClient, map[string]mcp.Client{toolName: mcpClient}, "investigate", nil)

	if !strings.Contains(result.Error, "reached max iterations") {
		t.Fatalf("expected max-iterations error, got %q", result.Error)
	}
	if !strings.Contains(result.Summary, "could not complete within 2 iterations") {
		t.Fatalf("unexpected summary: %q", result.Summary)
	}
	if len(mcpClient.InvokedTools()) != 2 {
		t.Fatalf("expected 2 tool invocations, got %d", len(mcpClient.InvokedTools()))
	}
	if llmClient.Calls() != 2 {
		t.Fatalf("expected 2 llm calls, got %d", llmClient.Calls())
	}
}

func TestSubAgentExecute_LLMErrorHandling(t *testing.T) {
	llmClient := &mockSubAgentLLM{
		responses: []mockLLMResponse{
			{err: errors.New("simulated llm failure")},
		},
	}

	sa := &SubAgent{
		Name:          "dashboard",
		SystemPrompt:  dashboardSpecialistPrompt,
		Tools:         []mcp.Tool{{Name: "grafana__search_dashboards"}},
		MaxIterations: 3,
	}
	result := sa.executeWithClient(context.Background(), llmClient, map[string]mcp.Client{}, "find dashboard", nil)
	if !strings.Contains(strings.ToLower(result.Error), "llm call failed") {
		t.Fatalf("expected llm failure error, got %q", result.Error)
	}
}

func TestShouldUseSubAgents_FeatureFlagDisabled(t *testing.T) {
	disabled := false
	mgr := NewManager(nil, nil, nil, nil, nil, ManagerConfig{SubAgentMode: &disabled})
	decision := CoordinatorDecision{UseDirect: false, SubAgents: []string{"dashboard"}}
	if mgr.shouldUseSubAgents(decision) {
		t.Fatal("expected shouldUseSubAgents=false when sub_agent_mode is disabled")
	}
}

func TestShouldUseSubAgents_FeatureFlagEnabledRespectsDecision(t *testing.T) {
	enabled := true
	mgr := NewManager(nil, nil, nil, nil, nil, ManagerConfig{SubAgentMode: &enabled})

	delegateDecision := mgr.coordinatorDecide(IntentResult{Label: IntentDashboardLookup, Confidence: 0.95}, "find dashboard for payments service")
	if !mgr.shouldUseSubAgents(delegateDecision) {
		t.Fatalf("expected delegated dashboard decision to use sub-agents, got %+v", delegateDecision)
	}

	directDecision := mgr.coordinatorDecide(IntentResult{Label: IntentHowToDocs, Confidence: 0.95}, "how do i configure alerts")
	if mgr.shouldUseSubAgents(directDecision) {
		t.Fatalf("expected direct docs decision to skip sub-agents, got %+v", directDecision)
	}
}

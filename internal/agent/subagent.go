package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"

	"github.com/brownster/grafana-assistant/internal/api"
	"github.com/brownster/grafana-assistant/internal/llm"
	"github.com/brownster/grafana-assistant/internal/mcp"
	"github.com/brownster/grafana-assistant/internal/metrics"
)

const defaultSubAgentIterations = 3

// SubAgentResult is the structured output of a sub-agent execution.
type SubAgentResult struct {
	Summary         string            // NL summary for coordinator synthesis.
	ToolsUsed       []string          // Tool names called by this sub-agent.
	TokensUsed      int               // Prompt + completion tokens consumed.
	DurationSeconds float64           // End-to-end execution duration.
	Artifacts       []any             // Any artifacts produced by the sub-agent.
	Metadata        map[string]string // Optional structured metadata.
	Error           string            // Non-empty when execution fails.
}

// SubAgent executes an isolated LLM call chain for a specific domain.
type SubAgent struct {
	Name          string
	SystemPrompt  string
	Tools         []mcp.Tool // Filtered tool set for this specialist.
	MaxIterations int        // Tool loop limit (typically 3-5).
}

type subAgentLLM interface {
	ChatWithOptions(ctx context.Context, messages []openai.ChatCompletionMessage, tools []openai.Tool, opts llm.ChatOptions) (*openai.ChatCompletionMessage, llm.ChatUsage, error)
}

// Execute runs the sub-agent with an isolated LLM conversation.
// streamFn emits tool call/result events for UI transparency.
func (sa *SubAgent) Execute(
	ctx context.Context,
	llmClient *llm.Client,
	mcpClients map[string]mcp.Client,
	userMessage string,
	streamFn func(api.StreamChunk),
) SubAgentResult {
	var client subAgentLLM
	if llmClient != nil {
		client = llmClient
	}
	return sa.executeWithClient(ctx, client, mcpClients, userMessage, streamFn)
}

func (sa *SubAgent) executeWithClient(
	ctx context.Context,
	llmClient subAgentLLM,
	mcpClients map[string]mcp.Client,
	userMessage string,
	streamFn func(api.StreamChunk),
) SubAgentResult {
	startedAt := time.Now()
	agentName := "unknown"
	if sa != nil {
		if candidate := strings.TrimSpace(sa.Name); candidate != "" {
			agentName = candidate
		}
	}
	metrics.SubAgentInvocationsTotal.WithLabelValues(agentName).Inc()

	result := SubAgentResult{
		Metadata: map[string]string{
			"agent_name": agentName,
		},
	}
	defer observeSubAgentExecutionMetrics(agentName, startedAt, &result)

	if sa == nil {
		result.Error = "sub-agent configuration is nil"
		return result
	}

	if agentName == "unknown" {
		agentName = "unnamed"
		result.Metadata["agent_name"] = agentName
	}

	if llmClient == nil {
		result.Error = fmt.Sprintf("sub-agent %q requires llm client", agentName)
		return result
	}

	systemPrompt := strings.TrimSpace(sa.SystemPrompt)
	if systemPrompt == "" {
		systemPrompt = "You are a focused specialist. Use tools when needed and return a concise evidence-based summary."
	}

	maxIterations := sa.MaxIterations
	if maxIterations <= 0 {
		maxIterations = defaultSubAgentIterations
	}
	result.Metadata["max_iterations"] = fmt.Sprintf("%d", maxIterations)

	mem := NewMemory(systemPrompt, 0)
	mem.Add(openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: strings.TrimSpace(userMessage),
	})

	openAITools := MCPToolsToOpenAI(sa.Tools)
	seenTools := make(map[string]struct{}, len(sa.Tools))

	for iteration := 0; iteration < maxIterations; iteration++ {
		messages := mem.Messages()
		resp, usage, err := llmClient.ChatWithOptions(ctx, messages, openAITools, llm.ChatOptions{})
		if err != nil {
			result.Error = fmt.Sprintf("sub-agent %q LLM call failed: %v", agentName, err)
			return result
		}

		promptTokens := usage.PromptTokens
		if promptTokens <= 0 {
			promptTokens = estimateMessagesPromptTokens(messages)
		}
		completionTokens := usage.CompletionTokens
		if completionTokens <= 0 {
			completionTokens = estimateTextTokens(resp.Content)
		}
		result.TokensUsed += promptTokens + completionTokens

		if len(resp.ToolCalls) == 0 {
			result.Summary = strings.TrimSpace(resp.Content)
			if result.Summary == "" {
				result.Summary = "No specialist summary was produced."
			}
			return result
		}

		mem.Add(openai.ChatCompletionMessage{
			Role:      openai.ChatMessageRoleAssistant,
			ToolCalls: resp.ToolCalls,
		})

		for _, tc := range resp.ToolCalls {
			toolName := strings.TrimSpace(tc.Function.Name)
			if toolName == "" {
				continue
			}

			var args map[string]any
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
				args = map[string]any{"raw": tc.Function.Arguments}
			}

			reason := deriveToolCallReason(toolName, args)
			prefixedToolName := prefixedSubAgentToolName(agentName, toolName)
			if streamFn != nil {
				streamFn(api.StreamChunk{
					Type:      "tool",
					Tool:      prefixedToolName,
					SubAgent:  agentName,
					Reason:    reason,
					ToolID:    tc.ID,
					Arguments: args,
				})
			}

			toolResult, toolErr := invokeSubAgentMCPTool(ctx, mcpClients, toolName, args)
			if toolErr != nil {
				toolResult = fmt.Sprintf("Error: %v", toolErr)
				if result.Error == "" {
					result.Error = fmt.Sprintf("sub-agent %q tool call failed: %v", agentName, toolErr)
				}
			} else {
				if _, ok := seenTools[toolName]; !ok {
					seenTools[toolName] = struct{}{}
					result.ToolsUsed = append(result.ToolsUsed, toolName)
				}
				appendSubAgentArtifacts(&result, toolResult)
			}

			if streamFn != nil {
				streamFn(api.StreamChunk{
					Type:     "tool",
					Tool:     prefixedToolName,
					SubAgent: agentName,
					Reason:   reason,
					ToolID:   tc.ID,
					Result:   toolResult,
				})
			}

			_, resultForModel := shapeToolResultOutputs(toolResult)
			mem.Add(openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Content:    resultForModel,
				ToolCallID: tc.ID,
			})
		}
	}

	if result.Summary == "" {
		if result.Error == "" {
			result.Error = fmt.Sprintf("sub-agent %q reached max iterations (%d)", agentName, maxIterations)
		}
		result.Summary = fmt.Sprintf("Specialist %q could not complete within %d iterations.", agentName, maxIterations)
	}
	return result
}

func observeSubAgentExecutionMetrics(agentName string, startedAt time.Time, result *SubAgentResult) {
	if result == nil {
		return
	}
	duration := time.Since(startedAt).Seconds()
	result.DurationSeconds = duration

	metrics.SubAgentDurationSeconds.WithLabelValues(agentName).Observe(duration)
	metrics.SubAgentTokensUsed.WithLabelValues(agentName).Observe(float64(result.TokensUsed))
	if strings.TrimSpace(result.Error) != "" {
		metrics.SubAgentErrorsTotal.WithLabelValues(agentName).Inc()
	}
}

func invokeSubAgentMCPTool(ctx context.Context, clients map[string]mcp.Client, toolName string, args map[string]any) (any, error) {
	client := clients[toolName]
	if client == nil {
		return nil, fmt.Errorf("no MCP client configured for tool %q", toolName)
	}
	return client.InvokeTool(ctx, toolName, args)
}

func prefixedSubAgentToolName(agentName, toolName string) string {
	name := strings.TrimSpace(agentName)
	if name == "" {
		name = "unknown"
	}
	return fmt.Sprintf("sub_agent:%s:%s", name, toolName)
}

func appendSubAgentArtifacts(result *SubAgentResult, toolResult any) {
	if result == nil || toolResult == nil {
		return
	}

	payload, ok := toolResult.(map[string]any)
	if !ok {
		return
	}
	rawArtifacts, ok := payload["artifacts"]
	if !ok || rawArtifacts == nil {
		return
	}
	if artifacts, ok := rawArtifacts.([]any); ok {
		result.Artifacts = append(result.Artifacts, artifacts...)
		return
	}
	result.Artifacts = append(result.Artifacts, rawArtifacts)
}

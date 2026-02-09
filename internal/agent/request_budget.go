package agent

import (
	"math"

	openai "github.com/sashabaranov/go-openai"
)

const (
	budgetReasonPromptTokens     = "prompt_tokens_exceeded"
	budgetReasonCompletionTokens = "completion_tokens_exceeded"
	budgetReasonToolCalls        = "tool_calls_exceeded"
	budgetReasonEstimatedCost    = "estimated_cost_exceeded"

	defaultPromptTokenCostPer1MUSD     = 0.80
	defaultCompletionTokenCostPer1MUSD = 3.20
)

type RequestBudget struct {
	MaxPromptTokens        int
	MaxCompletionTokens    int
	MaxToolIterations      int
	MaxToolCalls           int
	MaxEstimatedCostUSD    float64
	PromptCostPer1MUSD     float64
	CompletionCostPer1MUSD float64
}

func defaultRequestBudget() RequestBudget {
	return RequestBudget{
		MaxPromptTokens:        12000,
		MaxCompletionTokens:    4000,
		MaxToolIterations:      maxToolIterations,
		MaxToolCalls:           12,
		MaxEstimatedCostUSD:    0.10,
		PromptCostPer1MUSD:     defaultPromptTokenCostPer1MUSD,
		CompletionCostPer1MUSD: defaultCompletionTokenCostPer1MUSD,
	}
}

func normalizeRequestBudget(raw RequestBudget) RequestBudget {
	cfg := defaultRequestBudget()

	if raw.MaxPromptTokens > 0 {
		cfg.MaxPromptTokens = raw.MaxPromptTokens
	}
	if raw.MaxCompletionTokens > 0 {
		cfg.MaxCompletionTokens = raw.MaxCompletionTokens
	}
	if raw.MaxToolIterations > 0 {
		cfg.MaxToolIterations = raw.MaxToolIterations
	}
	if raw.MaxToolCalls > 0 {
		cfg.MaxToolCalls = raw.MaxToolCalls
	}
	if raw.MaxEstimatedCostUSD > 0 {
		cfg.MaxEstimatedCostUSD = raw.MaxEstimatedCostUSD
	}
	if raw.PromptCostPer1MUSD > 0 {
		cfg.PromptCostPer1MUSD = raw.PromptCostPer1MUSD
	}
	if raw.CompletionCostPer1MUSD > 0 {
		cfg.CompletionCostPer1MUSD = raw.CompletionCostPer1MUSD
	}

	return cfg
}

func estimateTextTokens(value string) int {
	if value == "" {
		return 0
	}
	return int(math.Ceil(float64(len(value)) / 4.0))
}

func estimateMessageTokens(msg openai.ChatCompletionMessage) int {
	total := 4
	total += estimateTextTokens(msg.Content)
	total += estimateTextTokens(msg.Role)
	total += estimateTextTokens(msg.Name)
	total += estimateTextTokens(msg.ToolCallID)
	for _, tc := range msg.ToolCalls {
		total += estimateTextTokens(tc.Function.Name)
		total += estimateTextTokens(tc.Function.Arguments)
		total += 4
	}
	return total
}

func estimateMessagesPromptTokens(messages []openai.ChatCompletionMessage) int {
	total := 2
	for _, msg := range messages {
		total += estimateMessageTokens(msg)
	}
	return total
}

func fitMessagesToPromptBudget(messages []openai.ChatCompletionMessage, maxPromptTokens int) ([]openai.ChatCompletionMessage, int, int) {
	if len(messages) == 0 {
		return messages, 0, 0
	}

	estimated := estimateMessagesPromptTokens(messages)
	if maxPromptTokens <= 0 || estimated <= maxPromptTokens {
		return messages, 0, estimated
	}

	system := messages[0]
	tail := messages[1:]

	for start := len(tail) - 1; start >= 0; start-- {
		candidate := make([]openai.ChatCompletionMessage, 0, 1+len(tail)-start)
		candidate = append(candidate, system)
		candidate = append(candidate, tail[start:]...)
		candidateEstimate := estimateMessagesPromptTokens(candidate)
		if candidateEstimate <= maxPromptTokens {
			return candidate, start, candidateEstimate
		}
	}

	systemOnly := []openai.ChatCompletionMessage{system}
	return systemOnly, len(tail), estimateMessagesPromptTokens(systemOnly)
}

func remainingCompletionTokens(cfg RequestBudget, used int) int {
	remaining := cfg.MaxCompletionTokens - used
	if remaining < 0 {
		return 0
	}
	return remaining
}

func estimateRequestCostUSD(promptTokens, completionTokens int, promptCostPer1MUSD, completionCostPer1MUSD float64) float64 {
	if promptTokens <= 0 && completionTokens <= 0 {
		return 0
	}
	promptCost := (float64(promptTokens) / 1_000_000.0) * promptCostPer1MUSD
	completionCost := (float64(completionTokens) / 1_000_000.0) * completionCostPer1MUSD
	return promptCost + completionCost
}

func budgetGuidanceMessage(reason string) string {
	switch reason {
	case budgetReasonPromptTokens:
		return "Budget guardrail triggered: prompt token budget reached. Do not call additional tools. Provide a concise best-effort answer based only on existing evidence and clearly state uncertainty."
	case budgetReasonCompletionTokens:
		return "Budget guardrail triggered: completion token budget reached. Do not call additional tools. Provide a concise final answer based on existing evidence."
	case budgetReasonToolCalls:
		return "Budget guardrail triggered: tool-call budget reached. Stop tool usage and summarize findings from the evidence already collected."
	case budgetReasonEstimatedCost:
		return "Budget guardrail triggered: estimated request cost reached configured limit. Stop tool usage and provide a concise final response from current evidence."
	default:
		return "Budget guardrail triggered. Provide a concise final response from existing evidence."
	}
}

func budgetUserMessage(reason string) string {
	switch reason {
	case budgetReasonPromptTokens:
		return "I hit the request budget for prompt size before I could continue. Narrow the scope (time range/dashboard/context) and retry."
	case budgetReasonCompletionTokens:
		return "I hit the request response budget and stopped additional reasoning steps. Narrow the request and retry for deeper analysis."
	case budgetReasonToolCalls:
		return "I hit the per-request tool-call budget and stopped additional tool executions."
	case budgetReasonEstimatedCost:
		return "I hit the configured per-request cost budget and stopped additional processing."
	default:
		return "I hit a request budget guardrail and stopped additional processing."
	}
}

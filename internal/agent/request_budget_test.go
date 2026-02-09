package agent

import (
	"testing"

	openai "github.com/sashabaranov/go-openai"
)

func TestNormalizeRequestBudgetDefaults(t *testing.T) {
	got := normalizeRequestBudget(RequestBudget{})
	want := defaultRequestBudget()
	if got != want {
		t.Fatalf("normalizeRequestBudget default mismatch: got=%+v want=%+v", got, want)
	}
}

func TestFitMessagesToPromptBudget(t *testing.T) {
	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: "system"},
		{Role: openai.ChatMessageRoleUser, Content: "old question"},
		{Role: openai.ChatMessageRoleAssistant, Content: "old answer"},
		{Role: openai.ChatMessageRoleUser, Content: "latest question"},
	}

	// Very small budget should retain system + newest suffix only.
	fitted, dropped, estimate := fitMessagesToPromptBudget(messages, 14)
	if dropped == 0 {
		t.Fatalf("expected dropped messages, got dropped=%d", dropped)
	}
	if len(fitted) >= len(messages) {
		t.Fatalf("expected fitted messages to be smaller than original, got len=%d", len(fitted))
	}
	if estimate <= 0 {
		t.Fatalf("expected positive estimate, got %d", estimate)
	}
	if fitted[0].Role != openai.ChatMessageRoleSystem {
		t.Fatalf("expected first message to be system, got role=%q", fitted[0].Role)
	}
}

func TestEstimateRequestCostUSD(t *testing.T) {
	cost := estimateRequestCostUSD(1000, 500, 0.80, 3.20)
	if cost <= 0 {
		t.Fatalf("expected positive cost, got %f", cost)
	}
}

func TestBudgetGuidanceMessage(t *testing.T) {
	msg := budgetGuidanceMessage(budgetReasonToolCalls)
	if msg == "" {
		t.Fatal("expected non-empty guidance message")
	}
}

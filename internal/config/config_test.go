package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalize_MCPToolLists(t *testing.T) {
	cfg := &Config{
		BasePath: "/assistant/",
		MCPServers: []MCPServer{
			{
				Type:          "grafana",
				ToolAllowlist: []string{" query_prometheus ", "", "query_prometheus", "search_dashboards"},
				ToolDenylist:  []string{" update_dashboard ", "update_dashboard", "  "},
			},
		},
	}

	if err := cfg.Normalize(); err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}

	if cfg.BasePath != "/assistant" {
		t.Fatalf("BasePath = %q, want %q", cfg.BasePath, "/assistant")
	}

	gotAllow := cfg.MCPServers[0].ToolAllowlist
	if len(gotAllow) != 2 || gotAllow[0] != "query_prometheus" || gotAllow[1] != "search_dashboards" {
		t.Fatalf("ToolAllowlist normalized incorrectly: %#v", gotAllow)
	}

	gotDeny := cfg.MCPServers[0].ToolDenylist
	if len(gotDeny) != 1 || gotDeny[0] != "update_dashboard" {
		t.Fatalf("ToolDenylist normalized incorrectly: %#v", gotDeny)
	}
}

func TestLoad_FeatureFlagEnvOverrides(t *testing.T) {
	path := writeTempConfigFile(t, "grafana_url: http://localhost:3000\n")
	t.Setenv("ASSISTANT_FEATURE_ROUTING_MODE", "false")
	t.Setenv("ASSISTANT_FEATURE_COMPOSITE_TOOL_MODE", "off")
	t.Setenv("ASSISTANT_FEATURE_JUDGE_GATE_MODE", "0")
	t.Setenv("ASSISTANT_FEATURE_EVIDENCE_REDACTION_MODE", "disabled")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.FeatureFlags.RoutingMode {
		t.Fatal("expected routing_mode=false from env override")
	}
	if cfg.FeatureFlags.CompositeToolMode {
		t.Fatal("expected composite_tool_mode=false from env override")
	}
	if cfg.FeatureFlags.JudgeGateMode {
		t.Fatal("expected judge_gate_mode=false from env override")
	}
	if cfg.FeatureFlags.EvidenceRedactionMode {
		t.Fatal("expected evidence_redaction_mode=false from env override")
	}
}

func TestLoad_InvalidFeatureFlagEnv(t *testing.T) {
	path := writeTempConfigFile(t, "grafana_url: http://localhost:3000\n")
	t.Setenv("ASSISTANT_FEATURE_ROUTING_MODE", "maybe")

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected invalid feature flag override error")
	}
	if !strings.Contains(err.Error(), "ASSISTANT_FEATURE_ROUTING_MODE") {
		t.Fatalf("expected env name in error, got %v", err)
	}
}

func TestParseBool(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		want    bool
		wantErr bool
	}{
		{name: "true", input: "true", want: true},
		{name: "one", input: "1", want: true},
		{name: "enabled", input: "enabled", want: true},
		{name: "false", input: "false", want: false},
		{name: "zero", input: "0", want: false},
		{name: "disabled", input: "disabled", want: false},
		{name: "invalid", input: "unknown", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseBool(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for input %q", tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for input %q: %v", tc.input, err)
			}
			if got != tc.want {
				t.Fatalf("parseBool(%q) = %t, want %t", tc.input, got, tc.want)
			}
		})
	}
}

func TestResolveModelProfile(t *testing.T) {
	cfg := &Config{
		OpenAIModel: "gpt-4o-mini-2025-01-01",
		ModelProfile: ModelProfileConfig{
			Default: "balanced",
			FamilyOverrides: map[string]string{
				"gpt-4o":      "balanced",
				"gpt-4o-mini": "compact",
			},
		},
	}

	if got := cfg.ResolvedModelProfile(); got != "compact" {
		t.Fatalf("ResolvedModelProfile() = %q, want %q", got, "compact")
	}
}

func TestResolveModelProfile_ExplicitNameWins(t *testing.T) {
	cfg := &Config{
		OpenAIModel: "gpt-4o-mini",
		ModelProfile: ModelProfileConfig{
			Name:    "strict",
			Default: "balanced",
			FamilyOverrides: map[string]string{
				"gpt-4o-mini": "compact",
			},
		},
	}

	if got := cfg.ResolvedModelProfile(); got != "strict" {
		t.Fatalf("ResolvedModelProfile() = %q, want %q", got, "strict")
	}
}

func TestLoad_ModelProfileEnvOverrides(t *testing.T) {
	path := writeTempConfigFile(t, "grafana_url: http://localhost:3000\n")
	t.Setenv("ASSISTANT_MODEL_PROFILE", "compact")
	t.Setenv("ASSISTANT_MODEL_PROFILE_DEFAULT", "balanced")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.ModelProfile.Name != "compact" {
		t.Fatalf("ModelProfile.Name = %q, want %q", cfg.ModelProfile.Name, "compact")
	}
	if cfg.ModelProfile.Default != "balanced" {
		t.Fatalf("ModelProfile.Default = %q, want %q", cfg.ModelProfile.Default, "balanced")
	}
}

func TestLoad_RequestBudgetDefaults(t *testing.T) {
	path := writeTempConfigFile(t, "grafana_url: http://localhost:3000\n")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.RequestBudget.MaxPromptTokens != 12000 {
		t.Fatalf("MaxPromptTokens = %d, want %d", cfg.RequestBudget.MaxPromptTokens, 12000)
	}
	if cfg.RequestBudget.MaxCompletionTokens != 4000 {
		t.Fatalf("MaxCompletionTokens = %d, want %d", cfg.RequestBudget.MaxCompletionTokens, 4000)
	}
	if cfg.RequestBudget.MaxToolIterations != 5 {
		t.Fatalf("MaxToolIterations = %d, want %d", cfg.RequestBudget.MaxToolIterations, 5)
	}
	if cfg.RequestBudget.MaxToolCalls != 12 {
		t.Fatalf("MaxToolCalls = %d, want %d", cfg.RequestBudget.MaxToolCalls, 12)
	}
	if cfg.RequestBudget.MaxEstimatedCostUSD != 0.10 {
		t.Fatalf("MaxEstimatedCostUSD = %f, want %f", cfg.RequestBudget.MaxEstimatedCostUSD, 0.10)
	}
	if cfg.RequestBudget.PromptCostPer1MUSD != 0.80 {
		t.Fatalf("PromptCostPer1MUSD = %f, want %f", cfg.RequestBudget.PromptCostPer1MUSD, 0.80)
	}
	if cfg.RequestBudget.CompletionCostPer1MUSD != 3.20 {
		t.Fatalf("CompletionCostPer1MUSD = %f, want %f", cfg.RequestBudget.CompletionCostPer1MUSD, 3.20)
	}
}

func TestLoad_RequestBudgetEnvOverrides(t *testing.T) {
	path := writeTempConfigFile(t, "grafana_url: http://localhost:3000\n")
	t.Setenv("ASSISTANT_REQUEST_BUDGET_MAX_PROMPT_TOKENS", "10000")
	t.Setenv("ASSISTANT_REQUEST_BUDGET_MAX_COMPLETION_TOKENS", "3000")
	t.Setenv("ASSISTANT_REQUEST_BUDGET_MAX_TOOL_ITERATIONS", "4")
	t.Setenv("ASSISTANT_REQUEST_BUDGET_MAX_TOOL_CALLS", "9")
	t.Setenv("ASSISTANT_REQUEST_BUDGET_MAX_ESTIMATED_COST_USD", "0.25")
	t.Setenv("ASSISTANT_REQUEST_BUDGET_PROMPT_COST_PER_1M_USD", "0.5")
	t.Setenv("ASSISTANT_REQUEST_BUDGET_COMPLETION_COST_PER_1M_USD", "1.5")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.RequestBudget.MaxPromptTokens != 10000 {
		t.Fatalf("MaxPromptTokens = %d, want %d", cfg.RequestBudget.MaxPromptTokens, 10000)
	}
	if cfg.RequestBudget.MaxCompletionTokens != 3000 {
		t.Fatalf("MaxCompletionTokens = %d, want %d", cfg.RequestBudget.MaxCompletionTokens, 3000)
	}
	if cfg.RequestBudget.MaxToolIterations != 4 {
		t.Fatalf("MaxToolIterations = %d, want %d", cfg.RequestBudget.MaxToolIterations, 4)
	}
	if cfg.RequestBudget.MaxToolCalls != 9 {
		t.Fatalf("MaxToolCalls = %d, want %d", cfg.RequestBudget.MaxToolCalls, 9)
	}
	if cfg.RequestBudget.MaxEstimatedCostUSD != 0.25 {
		t.Fatalf("MaxEstimatedCostUSD = %f, want %f", cfg.RequestBudget.MaxEstimatedCostUSD, 0.25)
	}
	if cfg.RequestBudget.PromptCostPer1MUSD != 0.5 {
		t.Fatalf("PromptCostPer1MUSD = %f, want %f", cfg.RequestBudget.PromptCostPer1MUSD, 0.5)
	}
	if cfg.RequestBudget.CompletionCostPer1MUSD != 1.5 {
		t.Fatalf("CompletionCostPer1MUSD = %f, want %f", cfg.RequestBudget.CompletionCostPer1MUSD, 1.5)
	}
}

func TestLoad_InvalidRequestBudgetEnv(t *testing.T) {
	path := writeTempConfigFile(t, "grafana_url: http://localhost:3000\n")
	t.Setenv("ASSISTANT_REQUEST_BUDGET_MAX_PROMPT_TOKENS", "bad")

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected invalid request budget env override error")
	}
	if !strings.Contains(err.Error(), "ASSISTANT_REQUEST_BUDGET_MAX_PROMPT_TOKENS") {
		t.Fatalf("expected env name in error, got %v", err)
	}
}

func TestLoad_InvalidRequestBudgetConfig(t *testing.T) {
	path := writeTempConfigFile(t, "grafana_url: http://localhost:3000\nrequest_budget:\n  max_tool_calls: 0\n")

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected request budget validation error")
	}
	if !strings.Contains(err.Error(), "request_budget.max_tool_calls") {
		t.Fatalf("expected request budget error, got %v", err)
	}
}

func TestLoad_ChatOpsYAMLConfig(t *testing.T) {
	yaml := `
grafana_url: http://localhost:3000
feature_flags:
  chatops_enabled: true
chatops:
  provider: mattermost
  bot_user_id: 99
  bot_user_name: assistant-bot
  bot_org_id: 1
  mattermost:
    url: http://localhost:18065
    token: secret-token
    team_name: monitoring
    channel_ids:
      - ch1
      - ch2
`
	path := writeTempConfigFile(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.FeatureFlags.ChatOpsEnabled {
		t.Fatal("expected ChatOpsEnabled=true")
	}
	if cfg.ChatOps.Provider != "mattermost" {
		t.Fatalf("Provider = %q, want %q", cfg.ChatOps.Provider, "mattermost")
	}
	if cfg.ChatOps.BotUserID != 99 {
		t.Fatalf("BotUserID = %d, want %d", cfg.ChatOps.BotUserID, 99)
	}
	if cfg.ChatOps.BotUserName != "assistant-bot" {
		t.Fatalf("BotUserName = %q, want %q", cfg.ChatOps.BotUserName, "assistant-bot")
	}
	if cfg.ChatOps.BotOrgID != 1 {
		t.Fatalf("BotOrgID = %d, want %d", cfg.ChatOps.BotOrgID, 1)
	}
	if cfg.ChatOps.Mattermost.URL != "http://localhost:18065" {
		t.Fatalf("Mattermost.URL = %q", cfg.ChatOps.Mattermost.URL)
	}
	if cfg.ChatOps.Mattermost.Token != "secret-token" {
		t.Fatalf("Mattermost.Token = %q", cfg.ChatOps.Mattermost.Token)
	}
	if cfg.ChatOps.Mattermost.TeamName != "monitoring" {
		t.Fatalf("Mattermost.TeamName = %q", cfg.ChatOps.Mattermost.TeamName)
	}
	if len(cfg.ChatOps.Mattermost.ChannelIDs) != 2 || cfg.ChatOps.Mattermost.ChannelIDs[0] != "ch1" {
		t.Fatalf("Mattermost.ChannelIDs = %v", cfg.ChatOps.Mattermost.ChannelIDs)
	}
}

func TestLoad_ChatOpsEnvOverrides(t *testing.T) {
	path := writeTempConfigFile(t, "grafana_url: http://localhost:3000\n")
	t.Setenv("ASSISTANT_FEATURE_CHATOPS_ENABLED", "true")
	t.Setenv("ASSISTANT_CHATOPS_PROVIDER", "mattermost")
	t.Setenv("ASSISTANT_CHATOPS_BOT_USER_NAME", "my-bot")
	t.Setenv("ASSISTANT_CHATOPS_BOT_USER_ID", "42")
	t.Setenv("ASSISTANT_CHATOPS_BOT_ORG_ID", "2")
	t.Setenv("ASSISTANT_CHATOPS_MATTERMOST_URL", "http://mm:8065")
	t.Setenv("ASSISTANT_CHATOPS_MATTERMOST_TOKEN", "env-token")
	t.Setenv("ASSISTANT_CHATOPS_MATTERMOST_TEAM", "ops")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.FeatureFlags.ChatOpsEnabled {
		t.Fatal("expected ChatOpsEnabled=true from env")
	}
	if cfg.ChatOps.Provider != "mattermost" {
		t.Fatalf("Provider = %q, want %q", cfg.ChatOps.Provider, "mattermost")
	}
	if cfg.ChatOps.BotUserName != "my-bot" {
		t.Fatalf("BotUserName = %q, want %q", cfg.ChatOps.BotUserName, "my-bot")
	}
	if cfg.ChatOps.BotUserID != 42 {
		t.Fatalf("BotUserID = %d, want %d", cfg.ChatOps.BotUserID, 42)
	}
	if cfg.ChatOps.BotOrgID != 2 {
		t.Fatalf("BotOrgID = %d, want %d", cfg.ChatOps.BotOrgID, 2)
	}
	if cfg.ChatOps.Mattermost.URL != "http://mm:8065" {
		t.Fatalf("Mattermost.URL = %q", cfg.ChatOps.Mattermost.URL)
	}
	if cfg.ChatOps.Mattermost.Token != "env-token" {
		t.Fatalf("Mattermost.Token = %q", cfg.ChatOps.Mattermost.Token)
	}
	if cfg.ChatOps.Mattermost.TeamName != "ops" {
		t.Fatalf("Mattermost.TeamName = %q", cfg.ChatOps.Mattermost.TeamName)
	}
}

func writeTempConfigFile(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write temp config: %v", err)
	}
	return path
}

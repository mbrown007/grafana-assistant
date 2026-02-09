package config

import (
	"bufio"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// MCPServer describes a single MCP server endpoint.
type MCPServer struct {
	URL           string            `yaml:"url"`
	Type          string            `yaml:"type"`      // e.g. "alertmanager", "grafana"
	Transport     string            `yaml:"transport"` // "sse" or "stdio"
	Command       string            `yaml:"command"`
	Args          []string          `yaml:"args"`
	Env           map[string]string `yaml:"env"`
	WorkDir       string            `yaml:"work_dir"`
	ToolAllowlist []string          `yaml:"tool_allowlist"`
	ToolDenylist  []string          `yaml:"tool_denylist"`
}

type Config struct {
	ListenAddr        string `yaml:"listen_addr"`
	GrafanaURL        string `yaml:"grafana_url"`
	GrafanaToken      string `yaml:"grafana_token"`
	DataRetentionDays int    `yaml:"data_retention_days"`
	ScratchpadTTLDays int    `yaml:"scratchpad_ttl_days"`
	ScratchpadFolder  string `yaml:"scratchpad_folder"`
	AuditLogPath      string `yaml:"audit_log_path"`
	BasePath          string `yaml:"base_path"`
	// Optional directory to record successful MCP tool responses as eval fixtures.
	EvalFixtureRecordDir string `yaml:"eval_fixture_record_dir"`
	// Dev/test only: bypass Grafana session auth for /api/chat and use synthetic user.
	EvalBypassAuth bool `yaml:"eval_bypass_auth"`

	// Database path for SQLite (default: data/assistant.db).
	DBPath string `yaml:"db_path"`

	// OpenAI configuration.
	OpenAIAPIKey string             `yaml:"openai_api_key"`
	OpenAIModel  string             `yaml:"openai_model"`
	ModelProfile ModelProfileConfig `yaml:"model_profile"`

	// MCP servers.
	MCPServers []MCPServer `yaml:"mcp_servers"`

	// Knowledge base (KB) path for domain context.
	KBPath string `yaml:"kb_path"`
	// KB context limits.
	KBMaxSections     int `yaml:"kb_max_sections"`
	KBMaxSectionChars int `yaml:"kb_max_section_chars"`

	// Hybrid KB: structured (token) and vector paths.
	KBStructuredPath   string            `yaml:"kb_structured_path"`    // default: "KB/runbooks"
	KBVectorPath       string            `yaml:"kb_vector_path"`        // default: "" (disabled)
	KBVectorDBPath     string            `yaml:"kb_vector_db_path"`     // default: "KB/.kb_vectors.db"
	KBEmbeddingModel   string            `yaml:"kb_embedding_model"`    // default: "text-embedding-3-small"
	KBVectorMaxResults int               `yaml:"kb_vector_max_results"` // default: 2
	KBDashboardMap     map[string]string `yaml:"kb_dashboard_map"`      // dashboard uid/name -> KB platform path

	// Metrics.
	MetricsEnabled bool `yaml:"metrics_enabled"`

	// Security settings.
	AllowedOrigin    string `yaml:"allowed_origin"`
	MaxMessageLength int    `yaml:"max_message_length"`
	MaxBodySize      int64  `yaml:"max_body_size"`
	RateLimitPerMin  int    `yaml:"rate_limit_per_minute"`
	RateLimitBurst   int    `yaml:"rate_limit_burst"`

	// Composite investigation routing.
	InvestigationToolTimeoutSeconds int `yaml:"investigation_tool_timeout_seconds"`

	// Runtime feature flags for controlled rollout.
	FeatureFlags FeatureFlags `yaml:"feature_flags"`

	// Per-request token/tool/cost budgets for the agent loop.
	RequestBudget RequestBudgetConfig `yaml:"request_budget"`

	// ChatOps bridge configuration.
	ChatOps ChatOpsConfig `yaml:"chatops"`
}

// FeatureFlags controls major assistant capabilities for rollout/rollback.
type FeatureFlags struct {
	RoutingMode           bool `yaml:"routing_mode"`
	CompositeToolMode     bool `yaml:"composite_tool_mode"`
	JudgeGateMode         bool `yaml:"judge_gate_mode"`
	EvidenceRedactionMode bool `yaml:"evidence_redaction_mode"`
	ChatOpsEnabled        bool `yaml:"chatops_enabled"`
}

// ModelProfileConfig controls prompt profile selection by model family.
type ModelProfileConfig struct {
	// Name explicitly selects a profile and bypasses family matching.
	Name string `yaml:"name"`
	// Default profile when no explicit name/family override matches.
	Default string `yaml:"default"`
	// FamilyOverrides maps model family prefixes to profile names.
	FamilyOverrides map[string]string `yaml:"family_overrides"`
}

// RequestBudgetConfig controls per-request token/tool/cost guardrails.
type RequestBudgetConfig struct {
	MaxPromptTokens        int     `yaml:"max_prompt_tokens"`
	MaxCompletionTokens    int     `yaml:"max_completion_tokens"`
	MaxToolIterations      int     `yaml:"max_tool_iterations"`
	MaxToolCalls           int     `yaml:"max_tool_calls"`
	MaxEstimatedCostUSD    float64 `yaml:"max_estimated_cost_usd"`
	PromptCostPer1MUSD     float64 `yaml:"prompt_cost_per_1m_usd"`
	CompletionCostPer1MUSD float64 `yaml:"completion_cost_per_1m_usd"`
}

// ChatOpsConfig configures the messaging bridge.
type ChatOpsConfig struct {
	Provider    string           `yaml:"provider"`      // "mattermost" (only option for now)
	BotUserID   int64            `yaml:"bot_user_id"`   // synthetic Grafana user ID for audit
	BotUserName string           `yaml:"bot_user_name"` // display name in audit log
	BotOrgID    int64            `yaml:"bot_org_id"`    // Grafana org for session ownership
	Mattermost  MattermostConfig `yaml:"mattermost"`
}

// MattermostConfig holds Mattermost-specific connection settings.
type MattermostConfig struct {
	URL        string   `yaml:"url"`
	Token      string   `yaml:"token"`
	TeamName   string   `yaml:"team_name"`
	ChannelIDs []string `yaml:"channel_ids"`
}

// loadDotEnv reads a .env file and sets any variables not already present
// in the environment. This is a best-effort operation; missing files are
// silently ignored.
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		val = strings.Trim(val, `"'`)
		// Don't override values already set in the real environment.
		if _, exists := os.LookupEnv(key); !exists {
			os.Setenv(key, val)
		}
	}
}

func Load(path string) (*Config, error) {
	// Load .env from the working directory (best-effort).
	loadDotEnv(".env")

	cfg := &Config{
		ListenAddr:        ":8080",
		DataRetentionDays: 30,
		DBPath:            "data/assistant.db",
		OpenAIModel:       "gpt-4o",
		ModelProfile: ModelProfileConfig{
			Default:         "balanced",
			FamilyOverrides: map[string]string{},
		},
		KBPath:                          "KB",
		KBMaxSections:                   2,
		KBMaxSectionChars:               2000,
		KBStructuredPath:                "KB/runbooks",
		KBVectorDBPath:                  "KB/.kb_vectors.db",
		KBEmbeddingModel:                "text-embedding-3-small",
		KBVectorMaxResults:              2,
		KBDashboardMap:                  map[string]string{},
		ScratchpadTTLDays:               7,
		ScratchpadFolder:                "Assistant Scratchpads",
		AuditLogPath:                    "/var/log/grafana-assistant/audit.log",
		MetricsEnabled:                  true,
		MaxMessageLength:                16000,
		MaxBodySize:                     65536,
		RateLimitPerMin:                 20,
		RateLimitBurst:                  5,
		InvestigationToolTimeoutSeconds: 8,
		FeatureFlags: FeatureFlags{
			RoutingMode:           true,
			CompositeToolMode:     true,
			JudgeGateMode:         true,
			EvidenceRedactionMode: true,
		},
		RequestBudget: RequestBudgetConfig{
			MaxPromptTokens:        12000,
			MaxCompletionTokens:    4000,
			MaxToolIterations:      5,
			MaxToolCalls:           12,
			MaxEstimatedCostUSD:    0.10,
			PromptCostPer1MUSD:     0.80,
			CompletionCostPer1MUSD: 3.20,
		},
	}

	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("reading config file: %w", err)
	}
	if err == nil {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parsing config file: %w", err)
		}
	}

	// Environment variable overrides
	if v := os.Getenv("ASSISTANT_LISTEN_ADDR"); v != "" {
		cfg.ListenAddr = v
	}
	if v := os.Getenv("ASSISTANT_GRAFANA_URL"); v != "" {
		cfg.GrafanaURL = v
	}
	if v := os.Getenv("ASSISTANT_GRAFANA_TOKEN"); v != "" {
		cfg.GrafanaToken = v
	}
	if v := os.Getenv("ASSISTANT_DATA_RETENTION_DAYS"); v != "" {
		days, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid ASSISTANT_DATA_RETENTION_DAYS: %w", err)
		}
		cfg.DataRetentionDays = days
	}
	if v := os.Getenv("ASSISTANT_SCRATCHPAD_TTL_DAYS"); v != "" {
		days, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid ASSISTANT_SCRATCHPAD_TTL_DAYS: %w", err)
		}
		cfg.ScratchpadTTLDays = days
	}
	if v := os.Getenv("ASSISTANT_SCRATCHPAD_FOLDER"); v != "" {
		cfg.ScratchpadFolder = v
	}
	if v := os.Getenv("ASSISTANT_DB_PATH"); v != "" {
		cfg.DBPath = v
	}
	if v := os.Getenv("ASSISTANT_AUDIT_LOG_PATH"); v != "" {
		cfg.AuditLogPath = v
	}
	if v := os.Getenv("ASSISTANT_BASE_PATH"); v != "" {
		cfg.BasePath = v
	}
	if v := os.Getenv("ASSISTANT_EVAL_FIXTURE_RECORD_DIR"); v != "" {
		cfg.EvalFixtureRecordDir = v
	}
	if v := os.Getenv("ASSISTANT_EVAL_BYPASS_AUTH"); v != "" {
		parsed, err := parseBool(v)
		if err != nil {
			return nil, fmt.Errorf("invalid ASSISTANT_EVAL_BYPASS_AUTH: %w", err)
		}
		cfg.EvalBypassAuth = parsed
	}
	if v := os.Getenv("ASSISTANT_OPENAI_API_KEY"); v != "" {
		cfg.OpenAIAPIKey = v
	}
	if v := os.Getenv("ASSISTANT_OPENAI_MODEL"); v != "" {
		cfg.OpenAIModel = v
	}
	if v := os.Getenv("ASSISTANT_MODEL_PROFILE"); v != "" {
		cfg.ModelProfile.Name = v
	}
	if v := os.Getenv("ASSISTANT_MODEL_PROFILE_DEFAULT"); v != "" {
		cfg.ModelProfile.Default = v
	}
	if v := os.Getenv("ASSISTANT_KB_PATH"); v != "" {
		cfg.KBPath = v
	}
	if v := os.Getenv("ASSISTANT_KB_MAX_SECTIONS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid ASSISTANT_KB_MAX_SECTIONS: %w", err)
		}
		cfg.KBMaxSections = n
	}
	if v := os.Getenv("ASSISTANT_KB_MAX_SECTION_CHARS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid ASSISTANT_KB_MAX_SECTION_CHARS: %w", err)
		}
		cfg.KBMaxSectionChars = n
	}
	if v := os.Getenv("ASSISTANT_KB_STRUCTURED_PATH"); v != "" {
		cfg.KBStructuredPath = v
	}
	if v := os.Getenv("ASSISTANT_KB_VECTOR_PATH"); v != "" {
		cfg.KBVectorPath = v
	}
	if v := os.Getenv("ASSISTANT_KB_VECTOR_DB_PATH"); v != "" {
		cfg.KBVectorDBPath = v
	}
	if v := os.Getenv("ASSISTANT_KB_EMBEDDING_MODEL"); v != "" {
		cfg.KBEmbeddingModel = v
	}
	if v := os.Getenv("ASSISTANT_KB_VECTOR_MAX_RESULTS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid ASSISTANT_KB_VECTOR_MAX_RESULTS: %w", err)
		}
		cfg.KBVectorMaxResults = n
	}
	if v := os.Getenv("ASSISTANT_INVESTIGATION_TOOL_TIMEOUT_SECONDS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid ASSISTANT_INVESTIGATION_TOOL_TIMEOUT_SECONDS: %w", err)
		}
		cfg.InvestigationToolTimeoutSeconds = n
	}
	if v := os.Getenv("ASSISTANT_REQUEST_BUDGET_MAX_PROMPT_TOKENS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid ASSISTANT_REQUEST_BUDGET_MAX_PROMPT_TOKENS: %w", err)
		}
		cfg.RequestBudget.MaxPromptTokens = n
	}
	if v := os.Getenv("ASSISTANT_REQUEST_BUDGET_MAX_COMPLETION_TOKENS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid ASSISTANT_REQUEST_BUDGET_MAX_COMPLETION_TOKENS: %w", err)
		}
		cfg.RequestBudget.MaxCompletionTokens = n
	}
	if v := os.Getenv("ASSISTANT_REQUEST_BUDGET_MAX_TOOL_ITERATIONS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid ASSISTANT_REQUEST_BUDGET_MAX_TOOL_ITERATIONS: %w", err)
		}
		cfg.RequestBudget.MaxToolIterations = n
	}
	if v := os.Getenv("ASSISTANT_REQUEST_BUDGET_MAX_TOOL_CALLS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid ASSISTANT_REQUEST_BUDGET_MAX_TOOL_CALLS: %w", err)
		}
		cfg.RequestBudget.MaxToolCalls = n
	}
	if v := os.Getenv("ASSISTANT_REQUEST_BUDGET_MAX_ESTIMATED_COST_USD"); v != "" {
		n, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid ASSISTANT_REQUEST_BUDGET_MAX_ESTIMATED_COST_USD: %w", err)
		}
		cfg.RequestBudget.MaxEstimatedCostUSD = n
	}
	if v := os.Getenv("ASSISTANT_REQUEST_BUDGET_PROMPT_COST_PER_1M_USD"); v != "" {
		n, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid ASSISTANT_REQUEST_BUDGET_PROMPT_COST_PER_1M_USD: %w", err)
		}
		cfg.RequestBudget.PromptCostPer1MUSD = n
	}
	if v := os.Getenv("ASSISTANT_REQUEST_BUDGET_COMPLETION_COST_PER_1M_USD"); v != "" {
		n, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid ASSISTANT_REQUEST_BUDGET_COMPLETION_COST_PER_1M_USD: %w", err)
		}
		cfg.RequestBudget.CompletionCostPer1MUSD = n
	}
	if err := applyBoolEnv(&cfg.FeatureFlags.RoutingMode, "ASSISTANT_FEATURE_ROUTING_MODE"); err != nil {
		return nil, err
	}
	if err := applyBoolEnv(&cfg.FeatureFlags.CompositeToolMode, "ASSISTANT_FEATURE_COMPOSITE_TOOL_MODE"); err != nil {
		return nil, err
	}
	if err := applyBoolEnv(&cfg.FeatureFlags.JudgeGateMode, "ASSISTANT_FEATURE_JUDGE_GATE_MODE"); err != nil {
		return nil, err
	}
	if err := applyBoolEnv(&cfg.FeatureFlags.EvidenceRedactionMode, "ASSISTANT_FEATURE_EVIDENCE_REDACTION_MODE"); err != nil {
		return nil, err
	}
	if err := applyBoolEnv(&cfg.FeatureFlags.ChatOpsEnabled, "ASSISTANT_FEATURE_CHATOPS_ENABLED"); err != nil {
		return nil, err
	}
	if v := os.Getenv("ASSISTANT_CHATOPS_PROVIDER"); v != "" {
		cfg.ChatOps.Provider = v
	}
	if v := os.Getenv("ASSISTANT_CHATOPS_BOT_USER_NAME"); v != "" {
		cfg.ChatOps.BotUserName = v
	}
	if v := os.Getenv("ASSISTANT_CHATOPS_BOT_USER_ID"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid ASSISTANT_CHATOPS_BOT_USER_ID: %w", err)
		}
		cfg.ChatOps.BotUserID = n
	}
	if v := os.Getenv("ASSISTANT_CHATOPS_BOT_ORG_ID"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid ASSISTANT_CHATOPS_BOT_ORG_ID: %w", err)
		}
		cfg.ChatOps.BotOrgID = n
	}
	if v := os.Getenv("ASSISTANT_CHATOPS_MATTERMOST_URL"); v != "" {
		cfg.ChatOps.Mattermost.URL = v
	}
	if v := os.Getenv("ASSISTANT_CHATOPS_MATTERMOST_TOKEN"); v != "" {
		cfg.ChatOps.Mattermost.Token = v
	}
	if v := os.Getenv("ASSISTANT_CHATOPS_MATTERMOST_TEAM"); v != "" {
		cfg.ChatOps.Mattermost.TeamName = v
	}

	if err := cfg.Normalize(); err != nil {
		return nil, fmt.Errorf("config normalization: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation: %w", err)
	}

	checkFilePermissions(path)
	checkFilePermissions(".env")

	return cfg, nil
}

func (c *Config) Normalize() error {
	base, err := normalizeBasePath(c.BasePath)
	if err != nil {
		return err
	}
	c.BasePath = base
	c.EvalFixtureRecordDir = strings.TrimSpace(c.EvalFixtureRecordDir)

	for i := range c.MCPServers {
		c.MCPServers[i].ToolAllowlist = normalizeStringList(c.MCPServers[i].ToolAllowlist)
		c.MCPServers[i].ToolDenylist = normalizeStringList(c.MCPServers[i].ToolDenylist)
	}
	c.ModelProfile = normalizeModelProfile(c.ModelProfile)
	return nil
}

func normalizeModelProfile(cfg ModelProfileConfig) ModelProfileConfig {
	cfg.Name = strings.ToLower(strings.TrimSpace(cfg.Name))
	cfg.Default = strings.ToLower(strings.TrimSpace(cfg.Default))
	if cfg.Default == "" {
		cfg.Default = "balanced"
	}
	if len(cfg.FamilyOverrides) == 0 {
		cfg.FamilyOverrides = map[string]string{}
		return cfg
	}

	normalized := make(map[string]string, len(cfg.FamilyOverrides))
	for rawFamily, rawProfile := range cfg.FamilyOverrides {
		family := strings.ToLower(strings.TrimSpace(rawFamily))
		profile := strings.ToLower(strings.TrimSpace(rawProfile))
		if family == "" || profile == "" {
			continue
		}
		normalized[family] = profile
	}
	cfg.FamilyOverrides = normalized
	return cfg
}

// ResolvedModelProfile returns the effective model prompt profile name.
func (c *Config) ResolvedModelProfile() string {
	return resolveModelProfile(c.OpenAIModel, c.ModelProfile)
}

func resolveModelProfile(model string, profileCfg ModelProfileConfig) string {
	normalizedModel := strings.ToLower(strings.TrimSpace(model))
	normalized := normalizeModelProfile(profileCfg)

	if normalized.Name != "" {
		return normalized.Name
	}

	selected := normalized.Default
	bestMatchLen := 0
	for family, profile := range normalized.FamilyOverrides {
		if !modelFamilyMatches(normalizedModel, family) {
			continue
		}
		if len(family) <= bestMatchLen {
			continue
		}
		bestMatchLen = len(family)
		selected = profile
	}

	if selected == "" {
		return "balanced"
	}
	return selected
}

func modelFamilyMatches(model, family string) bool {
	if model == "" || family == "" {
		return false
	}
	if model == family {
		return true
	}
	return strings.HasPrefix(model, family+"-") || strings.HasPrefix(model, family+"/") || strings.HasPrefix(model, family)
}

func normalizeStringList(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))

	for _, item := range items {
		value := strings.TrimSpace(item)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func applyBoolEnv(target *bool, envKey string) error {
	raw, ok := os.LookupEnv(envKey)
	if !ok || strings.TrimSpace(raw) == "" {
		return nil
	}
	parsed, err := parseBool(raw)
	if err != nil {
		return fmt.Errorf("invalid %s: %w", envKey, err)
	}
	*target = parsed
	return nil
}

func parseBool(raw string) (bool, error) {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "1", "true", "yes", "on", "enabled":
		return true, nil
	case "0", "false", "no", "off", "disabled":
		return false, nil
	default:
		return false, fmt.Errorf("unsupported boolean value %q", raw)
	}
}

func normalizeBasePath(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed == "/" {
		return "", nil
	}
	if !strings.HasPrefix(trimmed, "/") {
		trimmed = "/" + trimmed
	}
	trimmed = strings.TrimRight(trimmed, "/")
	if trimmed == "" || trimmed == "." {
		return "", nil
	}
	if strings.Contains(trimmed, " ") {
		return "", errors.New("base_path cannot contain spaces")
	}
	return trimmed, nil
}

func (c *Config) Validate() error {
	if c.GrafanaURL == "" {
		return errors.New("grafana_url is required")
	}
	if c.ListenAddr == "" {
		return errors.New("listen_addr is required")
	}
	if c.DataRetentionDays < 1 {
		return errors.New("data_retention_days must be >= 1")
	}
	if c.KBMaxSections < 1 {
		return errors.New("kb_max_sections must be >= 1")
	}
	if c.KBMaxSectionChars < 256 {
		return errors.New("kb_max_section_chars must be >= 256")
	}
	if c.ScratchpadTTLDays < 0 {
		return errors.New("scratchpad_ttl_days must be >= 0")
	}
	if c.InvestigationToolTimeoutSeconds < 1 {
		return errors.New("investigation_tool_timeout_seconds must be >= 1")
	}
	if c.RequestBudget.MaxPromptTokens < 256 {
		return errors.New("request_budget.max_prompt_tokens must be >= 256")
	}
	if c.RequestBudget.MaxCompletionTokens < 128 {
		return errors.New("request_budget.max_completion_tokens must be >= 128")
	}
	if c.RequestBudget.MaxToolIterations < 1 {
		return errors.New("request_budget.max_tool_iterations must be >= 1")
	}
	if c.RequestBudget.MaxToolCalls < 1 {
		return errors.New("request_budget.max_tool_calls must be >= 1")
	}
	if c.RequestBudget.MaxEstimatedCostUSD <= 0 {
		return errors.New("request_budget.max_estimated_cost_usd must be > 0")
	}
	if c.RequestBudget.PromptCostPer1MUSD <= 0 {
		return errors.New("request_budget.prompt_cost_per_1m_usd must be > 0")
	}
	if c.RequestBudget.CompletionCostPer1MUSD <= 0 {
		return errors.New("request_budget.completion_cost_per_1m_usd must be > 0")
	}
	return nil
}

// checkFilePermissions warns if a file is world-readable.
func checkFilePermissions(path string) {
	info, err := os.Stat(path)
	if err != nil {
		return // file doesn't exist or inaccessible — nothing to warn about
	}
	mode := info.Mode().Perm()
	if mode&0o077 != 0 {
		slog.Warn("file has overly permissive permissions",
			"path", path,
			"mode", fmt.Sprintf("%04o", mode),
			"recommendation", "chmod 600 "+path,
		)
	}
}

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
	URL       string            `yaml:"url"`
	Type      string            `yaml:"type"`      // e.g. "alertmanager", "grafana"
	Transport string            `yaml:"transport"` // "sse" or "stdio"
	Command   string            `yaml:"command"`
	Args      []string          `yaml:"args"`
	Env       map[string]string `yaml:"env"`
	WorkDir   string            `yaml:"work_dir"`
}

type Config struct {
	ListenAddr        string `yaml:"listen_addr"`
	GrafanaURL        string `yaml:"grafana_url"`
	GrafanaToken      string `yaml:"grafana_token"`
	DataRetentionDays int    `yaml:"data_retention_days"`
	ScratchpadTTLDays int    `yaml:"scratchpad_ttl_days"`
	ScratchpadFolder  string `yaml:"scratchpad_folder"`
	AuditLogPath      string `yaml:"audit_log_path"`

	// Database path for SQLite (default: data/assistant.db).
	DBPath string `yaml:"db_path"`

	// OpenAI configuration.
	OpenAIAPIKey string `yaml:"openai_api_key"`
	OpenAIModel  string `yaml:"openai_model"`

	// MCP servers.
	MCPServers []MCPServer `yaml:"mcp_servers"`

	// Knowledge base (KB) path for domain context.
	KBPath string `yaml:"kb_path"`
	// KB context limits.
	KBMaxSections     int `yaml:"kb_max_sections"`
	KBMaxSectionChars int `yaml:"kb_max_section_chars"`

	// Metrics.
	MetricsEnabled bool `yaml:"metrics_enabled"`

	// Security settings.
	AllowedOrigin    string `yaml:"allowed_origin"`
	MaxMessageLength int    `yaml:"max_message_length"`
	MaxBodySize      int64  `yaml:"max_body_size"`
	RateLimitPerMin  int    `yaml:"rate_limit_per_minute"`
	RateLimitBurst   int    `yaml:"rate_limit_burst"`
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
		KBPath:            "KB",
		KBMaxSections:     2,
		KBMaxSectionChars: 2000,
		ScratchpadTTLDays: 7,
		ScratchpadFolder:  "Assistant Scratchpads",
		AuditLogPath:      "/var/log/grafana-assistant/audit.log",
		MetricsEnabled:    true,
		MaxMessageLength:  16000,
		MaxBodySize:       65536,
		RateLimitPerMin:   20,
		RateLimitBurst:    5,
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
	if v := os.Getenv("ASSISTANT_OPENAI_API_KEY"); v != "" {
		cfg.OpenAIAPIKey = v
	}
	if v := os.Getenv("ASSISTANT_OPENAI_MODEL"); v != "" {
		cfg.OpenAIModel = v
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

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation: %w", err)
	}

	checkFilePermissions(path)
	checkFilePermissions(".env")

	return cfg, nil
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

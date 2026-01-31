package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

// MCPServer describes a single MCP server endpoint.
type MCPServer struct {
	URL  string `yaml:"url"`
	Type string `yaml:"type"` // e.g. "alertmanager", "grafana"
}

type Config struct {
	ListenAddr        string `yaml:"listen_addr"`
	GrafanaURL        string `yaml:"grafana_url"`
	GrafanaToken      string `yaml:"grafana_token"`
	DataRetentionDays int    `yaml:"data_retention_days"`

	// Database path for SQLite (default: data/assistant.db).
	DBPath string `yaml:"db_path"`

	// OpenAI configuration.
	OpenAIAPIKey string `yaml:"openai_api_key"`
	OpenAIModel  string `yaml:"openai_model"`

	// MCP servers.
	MCPServers []MCPServer `yaml:"mcp_servers"`
}

func Load(path string) (*Config, error) {
	cfg := &Config{
		ListenAddr:        ":8080",
		DataRetentionDays: 30,
		DBPath:            "data/assistant.db",
		OpenAIModel:       "gpt-4o",
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
	if v := os.Getenv("ASSISTANT_DB_PATH"); v != "" {
		cfg.DBPath = v
	}
	if v := os.Getenv("ASSISTANT_OPENAI_API_KEY"); v != "" {
		cfg.OpenAIAPIKey = v
	}
	if v := os.Getenv("ASSISTANT_OPENAI_MODEL"); v != "" {
		cfg.OpenAIModel = v
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation: %w", err)
	}

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
	return nil
}

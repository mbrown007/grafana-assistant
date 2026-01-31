package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/marcusz/monitoring-assistant/internal/config"
	"github.com/marcusz/monitoring-assistant/internal/proxy"
)

const indexHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Monitoring Assistant</title>
  <style>
    * { margin: 0; padding: 0; box-sizing: border-box; }
    body { display: flex; flex-direction: column; height: 100vh; font-family: system-ui, sans-serif; }
    header { padding: 8px 16px; background: #1a1a2e; color: #fff; font-size: 14px; }
    iframe { flex: 1; border: none; width: 100%; }
  </style>
</head>
<body>
  <header>Monitoring Assistant</header>
  <iframe src="/grafana/"></iframe>
</body>
</html>`

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	slog.Info("config loaded",
		"listen_addr", cfg.ListenAddr,
		"grafana_url", cfg.GrafanaURL,
		"data_retention_days", cfg.DataRetentionDays,
	)

	mux := http.NewServeMux()

	// Health endpoint
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Grafana reverse proxy
	grafanaHandler, err := proxy.GrafanaHandler(cfg.GrafanaURL)
	if err != nil {
		slog.Error("failed to create grafana proxy", "error", err)
		os.Exit(1)
	}
	mux.Handle("/grafana/", grafanaHandler)

	// Minimal test page (exact match on /)
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, indexHTML)
	})

	slog.Info("starting server", "addr", cfg.ListenAddr)
	if err := http.ListenAndServe(cfg.ListenAddr, mux); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}

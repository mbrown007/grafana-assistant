package main

import (
	"context"
	"embed"
	"encoding/json"
	"flag"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/marcusz/monitoring-assistant/internal/agent"
	"github.com/marcusz/monitoring-assistant/internal/api"
	"github.com/marcusz/monitoring-assistant/internal/auth"
	"github.com/marcusz/monitoring-assistant/internal/config"
	appcontext "github.com/marcusz/monitoring-assistant/internal/context"
	"github.com/marcusz/monitoring-assistant/internal/grafana"
	"github.com/marcusz/monitoring-assistant/internal/llm"
	"github.com/marcusz/monitoring-assistant/internal/logging"
	"github.com/marcusz/monitoring-assistant/internal/mcp"
	"github.com/marcusz/monitoring-assistant/internal/metrics"
	"github.com/marcusz/monitoring-assistant/internal/middleware"
	"github.com/marcusz/monitoring-assistant/internal/proxy"
	"github.com/marcusz/monitoring-assistant/internal/storage"
)

//go:embed static/*
var staticFiles embed.FS

func spaHandler() (http.Handler, error) {
	sub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return nil, err
	}
	fileServer := http.FileServer(http.FS(sub))
	indexFile, indexErr := sub.Open("index.html")
	indexExists := indexErr == nil
	if indexExists {
		indexFile.Close()
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}

		if !indexExists {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("<!doctype html><html><head><title>Monitoring Assistant</title></head><body style=\"font-family:sans-serif;background:#0b1020;color:#e2e8f0;padding:24px;\"><h1>Frontend assets not built</h1><p>Run <code>cd frontend && npm install && npm run build</code> to generate the static UI.</p></body></html>"))
			return
		}

		path := r.URL.Path
		if path == "/" {
			path = "/index.html"
		}

		f, err := sub.Open(strings.TrimPrefix(path, "/"))
		if err == nil {
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}

		r.URL.Path = "/index.html"
		fileServer.ServeHTTP(w, r)
	}), nil
}

func startRetentionPurger(ctx context.Context, store storage.Store, days int) {
	if days <= 0 {
		return
	}

	purge := func() {
		cutoff := time.Now().AddDate(0, 0, -days)
		purgeCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		deleted, err := store.PurgeOlderThan(purgeCtx, cutoff)
		if err != nil {
			slog.Warn("retention purge failed", "error", err)
			return
		}
		if deleted > 0 {
			slog.Info("retention purge completed", "deleted_sessions", deleted, "cutoff", cutoff.Format(time.RFC3339))
		}
	}

	purge()

	ticker := time.NewTicker(24 * time.Hour)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				purge()
			case <-ctx.Done():
				return
			}
		}
	}()
}

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	logger := slog.New(logging.NewContextHandler(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))
	slog.SetDefault(logger)

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	slog.Info("config loaded",
		"listen_addr", cfg.ListenAddr,
		"grafana_url", cfg.GrafanaURL,
		"db_path", cfg.DBPath,
		"data_retention_days", cfg.DataRetentionDays,
	)

	// Ensure database directory exists.
	if dir := filepath.Dir(cfg.DBPath); dir != "." {
		if err := os.MkdirAll(dir, 0750); err != nil {
			slog.Error("failed to create db directory", "error", err)
			os.Exit(1)
		}
	}

	// Initialize SQLite storage.
	store, err := storage.NewSQLite(cfg.DBPath)
	if err != nil {
		slog.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer store.Close()
	slog.Info("database connected", "path", cfg.DBPath)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	startRetentionPurger(ctx, store, cfg.DataRetentionDays)

	// Initialize Grafana client and session resolver.
	grafanaClient := grafana.NewClient(cfg.GrafanaURL, cfg.GrafanaToken)
	sessionResolver := auth.NewSessionResolver(grafanaClient)

	// Initialize dashboard context enricher with 5-minute cache TTL.
	enricher := appcontext.NewEnricher(grafanaClient, 5*time.Minute)

	// Initialize LLM client.
	llmClient, err := llm.NewClient(cfg.OpenAIAPIKey, cfg.OpenAIModel)
	if err != nil {
		slog.Warn("LLM client not configured, chat will be unavailable", "error", err)
	}

	// Initialize MCP clients: connect via SSE and discover tools.
	var mcpClients []*mcp.Client
	for _, srv := range cfg.MCPServers {
		c := mcp.NewClient(srv.URL, srv.Type)
		if err := c.Connect(context.Background()); err != nil {
			slog.Warn("failed to connect to MCP server (will skip)", "type", srv.Type, "url", srv.URL, "error", err)
			continue
		}
		mcpClients = append(mcpClients, c)
	}

	// Create agent manager.
	agentMgr := agent.NewManager(llmClient, mcpClients, enricher, store)
	if len(mcpClients) > 0 {
		if err := agentMgr.DiscoverTools(context.Background()); err != nil {
			slog.Warn("failed to discover MCP tools", "error", err)
		}
	}

	mux := http.NewServeMux()

	// Health endpoint
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Prometheus metrics endpoint.
	if cfg.MetricsEnabled {
		mux.Handle("GET /metrics", metrics.Handler())
		slog.Info("metrics endpoint registered")
	}

	// Dashboard context API
	mux.HandleFunc("GET /api/dashboard-context/{uid}", func(w http.ResponseWriter, r *http.Request) {
		uid := r.PathValue("uid")
		summary, err := enricher.GetDashboardSummary(r.Context(), uid)
		if err != nil {
			slog.Error("failed to get dashboard context", "uid", uid, "error", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(summary)
	})

	// Chat API (SSE streaming)
	if llmClient != nil {
		mux.HandleFunc("POST /api/chat", api.AuthenticatedChatHandlerWithLimit(
			sessionResolver,
			func(r *http.Request, user *grafana.User, req api.ChatRequest, streamFn func(api.StreamChunk)) {
				agentMgr.HandleChat(r.Context(), user, req, streamFn)
			},
			cfg.MaxMessageLength,
		))
		slog.Info("chat endpoint registered")
	} else {
		mux.HandleFunc("POST /api/chat", func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "chat not available: OpenAI API key not configured", http.StatusServiceUnavailable)
		})
	}

	// History API (user-scoped)
	mux.HandleFunc("GET /api/history", api.HistoryListHandler(store, sessionResolver))
	mux.HandleFunc("GET /api/history/{id}", api.HistoryDetailHandler(store, sessionResolver))
	mux.HandleFunc("DELETE /api/history/{id}", api.HistoryDeleteHandler(store, sessionResolver))
	mux.HandleFunc("GET /api/me", api.CurrentUserHandler(sessionResolver))

	// Grafana reverse proxy
	grafanaHandler, err := proxy.GrafanaHandler(cfg.GrafanaURL)
	if err != nil {
		slog.Error("failed to create grafana proxy", "error", err)
		os.Exit(1)
	}
	mux.Handle("/grafana/", grafanaHandler)

	// Serve embedded frontend (SPA fallback).
	spa, err := spaHandler()
	if err != nil {
		slog.Error("failed to load frontend assets", "error", err)
		os.Exit(1)
	}
	mux.Handle("/", spa)

	// Derive CORS allowed origin.
	allowedOrigin := cfg.AllowedOrigin
	if allowedOrigin == "" {
		allowedOrigin = middleware.DeriveAllowedOrigin(cfg.ListenAddr)
	}

	// Rate limiter.
	rateLimiter := middleware.NewRateLimiter(cfg.RateLimitPerMin, cfg.RateLimitBurst)

	// Middleware chain: RequestID → Metrics → SecurityHeaders → CORS → CSRFCheck → RateLimit → BodyLimit → mux
	var handler http.Handler = mux
	handler = middleware.BodyLimit(cfg.MaxBodySize, handler)
	handler = rateLimiter.Middleware(handler)
	handler = middleware.CSRFCheck(handler)
	handler = middleware.CORS(allowedOrigin, handler)
	handler = middleware.SecurityHeaders(handler)
	if cfg.MetricsEnabled {
		handler = middleware.Metrics(handler)
	}
	handler = middleware.RequestID(handler)

	// Graceful shutdown.
	srv := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second, // SSE needs long writes
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		slog.Info("starting server", "addr", cfg.ListenAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down server")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown error", "error", err)
	}

	slog.Info("server stopped")
}

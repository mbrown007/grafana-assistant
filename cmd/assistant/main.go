package main

import (
	"context"
	"encoding/json"
	"flag"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/marcusz/monitoring-assistant/frontend"
	"github.com/marcusz/monitoring-assistant/internal/agent"
	"github.com/marcusz/monitoring-assistant/internal/api"
	"github.com/marcusz/monitoring-assistant/internal/auth"
	"github.com/marcusz/monitoring-assistant/internal/config"
	appcontext "github.com/marcusz/monitoring-assistant/internal/context"
	"github.com/marcusz/monitoring-assistant/internal/dashboard"
	"github.com/marcusz/monitoring-assistant/internal/grafana"
	"github.com/marcusz/monitoring-assistant/internal/llm"
	"github.com/marcusz/monitoring-assistant/internal/logging"
	"github.com/marcusz/monitoring-assistant/internal/mcp"
	"github.com/marcusz/monitoring-assistant/internal/metrics"
	"github.com/marcusz/monitoring-assistant/internal/middleware"
	"github.com/marcusz/monitoring-assistant/internal/proxy"
	"github.com/marcusz/monitoring-assistant/internal/storage"
)

func spaHandler(basePath string) (http.Handler, error) {
	sub, err := fs.Sub(frontend.DistFS, "dist")
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

		if r.URL.Path == "/config.js" {
			w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
			w.WriteHeader(http.StatusOK)
			js := "window.__ASSISTANT_BASE_PATH__ = " + strconv.Quote(basePath) + ";\n"
			w.Write([]byte(js))
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

func startScratchpadPurger(ctx context.Context, client *grafana.Client, ttlDays int) {
	if ttlDays <= 0 {
		return
	}

	purge := func() {
		cutoff := time.Now().AddDate(0, 0, -ttlDays)
		purgeCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		hits, err := client.SearchDashboards(purgeCtx, []string{dashboard.ScratchpadTag})
		if err != nil {
			slog.Warn("scratchpad purge failed to list dashboards", "error", err)
			return
		}

		var deleted int
		for _, hit := range hits {
			lastUsed, ok := extractLastUsed(hit.Tags)
			if !ok {
				continue
			}
			if lastUsed.Before(cutoff) && hit.UID != "" {
				if err := client.DeleteDashboard(purgeCtx, hit.UID); err != nil {
					slog.Warn("scratchpad purge failed to delete dashboard", "uid", hit.UID, "error", err)
					continue
				}
				deleted++
			}
		}
		if deleted > 0 {
			slog.Info("scratchpad purge completed", "deleted_dashboards", deleted, "cutoff", cutoff.Format(time.RFC3339))
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

func extractLastUsed(tags []string) (time.Time, bool) {
	for _, tag := range tags {
		if strings.HasPrefix(tag, dashboard.LastUsedTagPrefix) {
			raw := strings.TrimPrefix(tag, dashboard.LastUsedTagPrefix)
			sec, err := strconv.ParseInt(raw, 10, 64)
			if err != nil {
				return time.Time{}, false
			}
			return time.Unix(sec, 0), true
		}
	}
	return time.Time{}, false
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
		"base_path", cfg.BasePath,
	)

	// Ensure database directory exists.
	if dir := filepath.Dir(cfg.DBPath); dir != "." {
		if err := os.MkdirAll(dir, 0750); err != nil {
			slog.Error("failed to create db directory", "error", err)
			os.Exit(1)
		}
	}

	// Initialize SQLite storage.
	store, err := storage.NewSQLiteWithAudit(cfg.DBPath, cfg.AuditLogPath)
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
	startScratchpadPurger(ctx, grafanaClient, cfg.ScratchpadTTLDays)

	// Initialize dashboard context enricher with 5-minute cache TTL.
	enricher := appcontext.NewEnricher(grafanaClient, 5*time.Minute)

	// Initialize LLM client.
	llmClient, err := llm.NewClient(cfg.OpenAIAPIKey, cfg.OpenAIModel)
	if err != nil {
		slog.Warn("LLM client not configured, chat will be unavailable", "error", err)
	}

	// Initialize MCP clients (SSE or stdio) and discover tools.
	var mcpClients []mcp.Client
	for _, srv := range cfg.MCPServers {
		transport := strings.ToLower(strings.TrimSpace(srv.Transport))
		if transport == "" {
			transport = "sse"
		}

		switch transport {
		case "sse":
			c := mcp.NewSSEClient(srv.URL, srv.Type)
			if err := c.Connect(context.Background()); err != nil {
				slog.Warn("failed to connect to MCP server (will skip)", "type", srv.Type, "url", srv.URL, "error", err)
				continue
			}
			mcpClients = append(mcpClients, c)
		case "stdio":
			c, err := mcp.NewStdioClient(mcp.StdioConfig{
				Command:    srv.Command,
				Args:       srv.Args,
				Env:        srv.Env,
				WorkDir:    srv.WorkDir,
				ServerType: srv.Type,
			})
			if err != nil {
				slog.Warn("failed to start stdio MCP server (will skip)", "type", srv.Type, "command", srv.Command, "error", err)
				continue
			}
			if err := c.Connect(context.Background()); err != nil {
				slog.Warn("failed to connect to stdio MCP server (will skip)", "type", srv.Type, "command", srv.Command, "error", err)
				continue
			}
			mcpClients = append(mcpClients, c)
		default:
			slog.Warn("unknown MCP transport (will skip)", "type", srv.Type, "transport", srv.Transport)
		}
	}

	// Create agent manager.
	scratchpadMgr := dashboard.NewManager(grafanaClient, cfg.ScratchpadFolder)
	agentMgr := agent.NewManager(llmClient, mcpClients, enricher, store, scratchpadMgr, agent.ManagerConfig{
		KBPath:             cfg.KBPath,
		KBMaxSections:      cfg.KBMaxSections,
		KBMaxSectionChars:  cfg.KBMaxSectionChars,
		KBStructuredPath:   cfg.KBStructuredPath,
		KBVectorPath:       cfg.KBVectorPath,
		KBVectorDBPath:     cfg.KBVectorDBPath,
		KBEmbeddingModel:   cfg.KBEmbeddingModel,
		KBVectorMaxResults: cfg.KBVectorMaxResults,
		KBDashboardMap:     cfg.KBDashboardMap,
		OpenAIAPIKey:       cfg.OpenAIAPIKey,
	})
	if len(mcpClients) > 0 {
		if err := agentMgr.DiscoverTools(context.Background()); err != nil {
			slog.Warn("failed to discover MCP tools", "error", err)
		}
	}

	appMux := http.NewServeMux()

	// Health endpoint
	appMux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Prometheus metrics endpoint.
	if cfg.MetricsEnabled {
		appMux.Handle("GET /metrics", metrics.Handler())
		slog.Info("metrics endpoint registered")
	}

	// Dashboard context API
	appMux.HandleFunc("GET /api/dashboard-context/{uid}", func(w http.ResponseWriter, r *http.Request) {
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
		appMux.HandleFunc("POST /api/chat", api.AuthenticatedChatHandlerWithLimit(
			sessionResolver,
			func(r *http.Request, user *grafana.User, req api.ChatRequest, streamFn func(api.StreamChunk)) {
				agentMgr.HandleChat(r.Context(), user, req, streamFn)
			},
			cfg.MaxMessageLength,
		))
		slog.Info("chat endpoint registered")
	} else {
		appMux.HandleFunc("POST /api/chat", func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "chat not available: OpenAI API key not configured", http.StatusServiceUnavailable)
		})
	}

	// History API (user-scoped)
	appMux.HandleFunc("GET /api/history", api.HistoryListHandler(store, sessionResolver))
	appMux.HandleFunc("GET /api/history/{id}", api.HistoryDetailHandler(store, sessionResolver))
	appMux.HandleFunc("DELETE /api/history/{id}", api.HistoryDeleteHandler(store, sessionResolver))
	appMux.HandleFunc("POST /api/feedback", api.FeedbackHandler(store, sessionResolver))
	appMux.HandleFunc("GET /api/me", api.CurrentUserHandler(sessionResolver))

	// Grafana reverse proxy
	grafanaHandler, err := proxy.GrafanaHandler(cfg.GrafanaURL)
	if err != nil {
		slog.Error("failed to create grafana proxy", "error", err)
		os.Exit(1)
	}
	appMux.Handle("/grafana/", grafanaHandler)

	// Serve embedded frontend (SPA fallback).
	spa, err := spaHandler(cfg.BasePath)
	if err != nil {
		slog.Error("failed to load frontend assets", "error", err)
		os.Exit(1)
	}
	appMux.Handle("/", spa)

	var mux http.Handler = appMux
	if cfg.BasePath != "" {
		rootMux := http.NewServeMux()
		basePrefix := cfg.BasePath
		rootMux.Handle(basePrefix+"/", http.StripPrefix(basePrefix, appMux))
		rootMux.HandleFunc(basePrefix, func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, basePrefix+"/", http.StatusPermanentRedirect)
		})
		mux = rootMux
	}

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

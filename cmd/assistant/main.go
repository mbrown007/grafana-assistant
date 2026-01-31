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

	"github.com/marcusz/monitoring-assistant/internal/auth"
	"github.com/marcusz/monitoring-assistant/internal/config"
	appcontext "github.com/marcusz/monitoring-assistant/internal/context"
	"github.com/marcusz/monitoring-assistant/internal/grafana"
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

	// Initialize Grafana client and session resolver.
	grafanaClient := grafana.NewClient(cfg.GrafanaURL, cfg.GrafanaToken)
	_ = auth.NewSessionResolver(grafanaClient)

	// Initialize dashboard context enricher with 5-minute cache TTL.
	enricher := appcontext.NewEnricher(grafanaClient, 5*time.Minute)

	mux := http.NewServeMux()

	// Health endpoint
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

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

	// Graceful shutdown.
	srv := &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: mux,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

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

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/mark3labs/mcp-go/server"
	"github.com/sabio/alertmanager-mcp-go/pkg/alertmanager"
	mcpserver "github.com/sabio/alertmanager-mcp-go/pkg/server"
)

var (
	transport = flag.String("transport", getEnv("MCP_TRANSPORT", "stdio"), "Transport mode: stdio or sse")
	host      = flag.String("host", getEnv("MCP_HOST", "127.0.0.1"), "Host to bind to (for SSE mode)")
	port      = flag.Int("port", getEnvInt("MCP_PORT", 8000), "Port to listen on (for SSE mode)")
)

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}

func setupEnvironment() (*alertmanager.Client, error) {
	url := os.Getenv("ALERTMANAGER_URL")
	if url == "" {
		return nil, fmt.Errorf("ALERTMANAGER_URL environment variable is not set")
	}

	username := os.Getenv("ALERTMANAGER_USERNAME")
	password := os.Getenv("ALERTMANAGER_PASSWORD")
	tenant := os.Getenv("ALERTMANAGER_TENANT")

	log.Println("Alertmanager configuration:")
	log.Printf("  Server URL: %s", url)

	if username != "" && password != "" {
		log.Println("  Authentication: Using basic auth")
	} else {
		log.Println("  Authentication: None (no credentials provided)")
	}

	if tenant != "" {
		log.Printf("  Static Tenant ID: %s", tenant)
	} else {
		log.Println("  Static Tenant ID: None")
	}

	log.Println("\nMulti-tenant Support:")
	log.Println("  - Send X-Scope-OrgId header with requests for multi-tenant setups")
	log.Println("  - Request header takes precedence over static ALERTMANAGER_TENANT config")

	return alertmanager.NewClient(url, username, password, tenant), nil
}

func runStdio(mcpServer *mcpserver.MCPServer) error {
	log.Println("Running server with stdio transport (default)")
	log.Println("This mode communicates through standard input/output")

	// Setup signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		log.Println("Shutting down server...")
		cancel()
	}()

	stdioServer := server.NewStdioServer(mcpServer.GetServer())
	return stdioServer.Listen(ctx, os.Stdin, os.Stdout)
}

func runSSE(mcpServer *mcpserver.MCPServer, addr string) error {
	log.Printf("Running server with SSE transport at %s", addr)
	log.Printf("SSE endpoint: http://%s/sse", addr)

	sseServer := server.NewSSEServer(mcpServer.GetServer(), "/sse")

	// Setup graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		log.Println("Shutting down server...")
		if err := sseServer.Shutdown(context.Background()); err != nil {
			log.Printf("Error during shutdown: %v", err)
		}
	}()

	log.Printf("Server listening on %s", addr)
	if err := sseServer.Start(addr); err != nil {
		return err
	}

	return nil
}

func main() {
	flag.Parse()

	log.Println("Starting Prometheus Alertmanager MCP Server...")

	// Setup alertmanager client
	client, err := setupEnvironment()
	if err != nil {
		log.Fatalf("Failed to setup environment: %v", err)
	}

	// Create MCP server
	mcpServer := mcpserver.NewMCPServer(client)
	mcpServer.RegisterTools()

	// Run with selected transport
	addr := fmt.Sprintf("%s:%d", *host, *port)

	switch *transport {
	case "stdio":
		if err := runStdio(mcpServer); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	case "sse":
		if err := runSSE(mcpServer, addr); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	default:
		log.Fatalf("Unknown transport mode: %s (must be stdio or sse)", *transport)
	}

	log.Println("Server stopped")
}

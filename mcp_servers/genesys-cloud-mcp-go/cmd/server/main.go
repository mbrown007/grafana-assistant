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
	"github.com/sabio/genesys-cloud-mcp-go/pkg/genesys"
	mcpserver "github.com/sabio/genesys-cloud-mcp-go/pkg/server"
)

var (
	transport = flag.String("transport", getEnv("MCP_TRANSPORT", "stdio"), "Transport mode: stdio or sse")
	host      = flag.String("host", getEnv("MCP_HOST", "127.0.0.1"), "Host to bind to (for SSE mode)")
	port      = flag.Int("port", getEnvInt("MCP_PORT", 8080), "Port to listen on (for SSE mode)")
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

func setupEnvironment() (*genesys.Client, error) {
	region := os.Getenv("GENESYSCLOUD_REGION")
	clientID := os.Getenv("GENESYSCLOUD_OAUTHCLIENT_ID")
	clientSecret := os.Getenv("GENESYSCLOUD_OAUTHCLIENT_SECRET")

	if region == "" || clientID == "" || clientSecret == "" {
		return nil, fmt.Errorf("missing required environment variables: GENESYSCLOUD_REGION, GENESYSCLOUD_OAUTHCLIENT_ID, GENESYSCLOUD_OAUTHCLIENT_SECRET")
	}

	log.Println("Genesys Cloud configuration:")
	log.Printf("  Region: %s", region)
	log.Printf("  Client ID: %s...", clientID[:min(8, len(clientID))])

	return genesys.NewClient(region, clientID, clientSecret)
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

	log.Println("Starting Genesys Cloud MCP Server...")

	// Setup Genesys client
	client, err := setupEnvironment()
	if err != nil {
		log.Fatalf("Failed to setup environment: %v", err)
	}

	log.Println("Successfully connected to Genesys Cloud")

	// Create MCP server
	mcpServer := mcpserver.NewMCPServer(client)
	mcpServer.RegisterTools()

	log.Printf("Registered %d MCP tools", 5)

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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

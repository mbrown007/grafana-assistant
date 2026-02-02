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

	mcpserver "github.com/marcusz/monitoring-assistant/mcp_servers/kb-mcp-go/pkg/server"
	kb "github.com/marcusz/monitoring-assistant/pkg/kb"
	"github.com/mark3labs/mcp-go/server"
)

var (
	transport = flag.String("transport", getEnv("MCP_TRANSPORT", "stdio"), "Transport mode: stdio or sse")
	host      = flag.String("host", getEnv("MCP_HOST", "0.0.0.0"), "Host to bind to (for SSE mode)")
	port      = flag.Int("port", getEnvInt("MCP_PORT", 8003), "Port to listen on (for SSE mode)")
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

func loadIndex() (*kb.Index, error) {
	root := getEnv("KB_PATH", "KB")
	indexPath := os.Getenv("KB_INDEX_PATH")
	return kb.LoadOrBuildWithPath(root, indexPath)
}

func runStdio(mcpServer *mcpserver.MCPServer) error {
	log.Println("Running server with stdio transport (default)")
	log.Println("This mode communicates through standard input/output")

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

	log.Println("Starting KB MCP Server...")

	index, err := loadIndex()
	if err != nil {
		log.Fatalf("Failed to load KB index: %v", err)
	}
	log.Printf("Loaded KB sections: %d", len(index.Sections))

	var opts []mcpserver.MCPOption

	// Set up vector search if configured.
	vectorDBPath := os.Getenv("KB_VECTOR_DB_PATH")
	apiKey := os.Getenv("ASSISTANT_OPENAI_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	embeddingModel := getEnv("KB_EMBEDDING_MODEL", "text-embedding-3-small")

	if vectorDBPath != "" && apiKey != "" {
		vi, err := kb.OpenVectorIndex(vectorDBPath)
		if err != nil {
			log.Printf("WARNING: failed to open vector index: %v (semantic search disabled)", err)
		} else {
			embedder := kb.NewEmbedder(apiKey, embeddingModel)
			opts = append(opts, mcpserver.WithVectorSearch(vi, embedder))
			log.Printf("Vector search enabled (sections=%d)", vi.Count())
		}
	}

	mcpServer := mcpserver.NewMCPServer(index, opts...)
	mcpServer.RegisterTools()

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
}

package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

func TestStdioClient(t *testing.T) {
	if os.Getenv("MCP_STDIO_HELPER") == "1" {
		runStdioHelper()
		return
	}

	cmd := os.Args[0]
	args := []string{"-test.run=TestStdioClient"}
	env := map[string]string{
		"MCP_STDIO_HELPER": "1",
	}

	client, err := NewStdioClient(StdioConfig{
		Command:    cmd,
		Args:       args,
		Env:        env,
		ServerType: "alertmanager",
	})
	if err != nil {
		t.Fatalf("NewStdioClient: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	tools, err := client.DiscoverTools(ctx)
	if err != nil {
		t.Fatalf("DiscoverTools: %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	if got, want := tools[0].Name, "alertmanager__list_alerts"; got != want {
		t.Fatalf("unexpected tool name: got %q want %q", got, want)
	}

	out, err := client.InvokeTool(ctx, "alertmanager__list_alerts", map[string]any{"severity": "critical"})
	if err != nil {
		t.Fatalf("InvokeTool: %v", err)
	}
	if got, ok := out.(string); !ok || !strings.Contains(got, "critical") {
		t.Fatalf("unexpected tool output: %#v", out)
	}
}

func runStdioHelper() {
	reader := bufio.NewReader(os.Stdin)
	writer := bufio.NewWriter(os.Stdout)
	defer writer.Flush()

	for i := 0; i < 2; i++ {
		msg, err := readFramedMessage(reader)
		if err != nil {
			if err == io.EOF {
				return
			}
			fmt.Fprintln(os.Stderr, "helper read error:", err)
			return
		}

		var req struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if err := json.Unmarshal(msg, &req); err != nil {
			fmt.Fprintln(os.Stderr, "helper json error:", err)
			return
		}

		switch req.Method {
		case "tools/list":
			resp := map[string]any{
				"jsonrpc": "2.0",
				"id":      json.RawMessage(req.ID),
				"result": map[string]any{
					"tools": []map[string]any{
						{
							"name":        "list_alerts",
							"description": "List alerts",
							"inputSchema": map[string]any{"type": "object"},
						},
					},
				},
			}
			writeFramedResponse(writer, resp)
		case "tools/call":
			var params struct {
				Name      string         `json:"name"`
				Arguments map[string]any `json:"arguments"`
			}
			_ = json.Unmarshal(req.Params, &params)
			resp := map[string]any{
				"jsonrpc": "2.0",
				"id":      json.RawMessage(req.ID),
				"result": map[string]any{
					"content": []map[string]any{
						{
							"type": "text",
							"text": fmt.Sprintf("ok: %v", params.Arguments["severity"]),
						},
					},
				},
			}
			writeFramedResponse(writer, resp)
		default:
			resp := map[string]any{
				"jsonrpc": "2.0",
				"id":      json.RawMessage(req.ID),
				"error": map[string]any{
					"code":    -32601,
					"message": "method not found",
				},
			}
			writeFramedResponse(writer, resp)
		}
	}
}

func writeFramedResponse(w *bufio.Writer, resp map[string]any) {
	b, _ := json.Marshal(resp)
	var buf bytes.Buffer
	buf.WriteString("Content-Length: ")
	buf.WriteString(fmt.Sprintf("%d", len(b)))
	buf.WriteString("\r\n\r\n")
	buf.Write(b)
	_, _ = w.Write(buf.Bytes())
	_ = w.Flush()
}

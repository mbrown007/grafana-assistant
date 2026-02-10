package mockserver

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestServerHandleLineLifecycle(t *testing.T) {
	dir := t.TempDir()
	toolDir := filepath.Join(dir, "grafana__query_prometheus")
	if err := os.MkdirAll(toolDir, 0o755); err != nil {
		t.Fatalf("mkdir fixtures: %v", err)
	}
	fixturePath := filepath.Join(toolDir, "case_01.json")
	fixtureJSON := `{
  "tool_name": "grafana__query_prometheus",
  "match": {
    "datasourceUid": "^prometheus$",
    "expr": "node_cpu"
  },
  "response": {
    "status": "success",
    "data": { "resultType": "matrix", "result": [{"metric":{"mode":"idle"}}] }
  }
}`
	if err := os.WriteFile(fixturePath, []byte(fixtureJSON), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	srv, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	initResp := callRPC(t, srv, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05"}}`)
	if initResp["error"] != nil {
		t.Fatalf("initialize returned error: %#v", initResp["error"])
	}

	listResp := callRPC(t, srv, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	result, ok := listResp["result"].(map[string]any)
	if !ok {
		t.Fatalf("tools/list missing result: %#v", listResp)
	}
	tools, ok := result["tools"].([]any)
	if !ok || len(tools) != 1 {
		t.Fatalf("unexpected tools payload: %#v", result["tools"])
	}
	tool, _ := tools[0].(map[string]any)
	if got, want := tool["name"], "query_prometheus"; got != want {
		t.Fatalf("unexpected listed tool name: got %v want %v", got, want)
	}

	callResp := callRPC(t, srv, `{
  "jsonrpc":"2.0",
  "id":3,
  "method":"tools/call",
  "params":{
    "name":"query_prometheus",
    "arguments":{"datasourceUid":"prometheus","expr":"rate(node_cpu_seconds_total[5m])"}
  }
}`)
	assertToolCallContentHasStatusSuccess(t, callResp)

	missResp := callRPC(t, srv, `{
  "jsonrpc":"2.0",
  "id":4,
  "method":"tools/call",
  "params":{
    "name":"query_prometheus",
    "arguments":{"datasourceUid":"prometheus","expr":"sum(rate(http_requests_total[5m]))"}
  }
}`)
	assertToolCallContentHasStatusSuccess(t, missResp)
	respData := extractToolCallData(t, missResp)
	if _, ok := respData["message"].(string); !ok {
		t.Fatalf("expected no-match message in fallback response: %#v", respData)
	}
}

func TestServerUsesToolsManifestWhenPresent(t *testing.T) {
	dir := t.TempDir()
	toolDir := filepath.Join(dir, "grafana__query_prometheus")
	if err := os.MkdirAll(toolDir, 0o755); err != nil {
		t.Fatalf("mkdir fixtures: %v", err)
	}
	if err := os.WriteFile(filepath.Join(toolDir, "case_01.json"), []byte(`{
  "tool_name": "grafana__query_prometheus",
  "match": {"expr":"node"},
  "response": {"status":"success"}
}`), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	manifest := `{
  "tools": [
    {
      "name": "query_prometheus",
      "description": "Manifest description",
      "inputSchema": {
        "type": "object",
        "properties": {"expr":{"type":"string"}},
        "required": ["expr"]
      }
    }
  ]
}`
	if err := os.WriteFile(filepath.Join(dir, toolsManifestFile), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write tools manifest: %v", err)
	}

	srv, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	listResp := callRPC(t, srv, `{"jsonrpc":"2.0","id":"tools","method":"tools/list"}`)
	result, _ := listResp["result"].(map[string]any)
	tools, _ := result["tools"].([]any)
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool from manifest, got %d", len(tools))
	}
	tool := tools[0].(map[string]any)
	if got, want := tool["description"], "Manifest description"; got != want {
		t.Fatalf("unexpected manifest tool description: got %v want %v", got, want)
	}
}

func TestServerFiltersByServerType(t *testing.T) {
	dir := t.TempDir()
	grafanaDir := filepath.Join(dir, "grafana__query_prometheus")
	if err := os.MkdirAll(grafanaDir, 0o755); err != nil {
		t.Fatalf("mkdir grafana fixture dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(grafanaDir, "cpu.json"), []byte(`{
  "tool_name": "grafana__query_prometheus",
  "match": {"expr":"cpu"},
  "response": {"status":"success"}
}`), 0o644); err != nil {
		t.Fatalf("write grafana fixture: %v", err)
	}

	alertDir := filepath.Join(dir, "alertmanager__list_alerts")
	if err := os.MkdirAll(alertDir, 0o755); err != nil {
		t.Fatalf("mkdir alertmanager fixture dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(alertDir, "alerts.json"), []byte(`{
  "tool_name": "alertmanager__list_alerts",
  "match": {"severity":"critical"},
  "response": {"status":"success"}
}`), 0o644); err != nil {
		t.Fatalf("write alertmanager fixture: %v", err)
	}

	srv, err := NewForServerType(dir, "grafana")
	if err != nil {
		t.Fatalf("NewForServerType: %v", err)
	}
	listResp := callRPC(t, srv, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	result, _ := listResp["result"].(map[string]any)
	tools, _ := result["tools"].([]any)
	if len(tools) != 1 {
		t.Fatalf("expected 1 filtered tool, got %d", len(tools))
	}
	tool := tools[0].(map[string]any)
	if got, want := tool["name"], "query_prometheus"; got != want {
		t.Fatalf("unexpected tool name: got %v want %v", got, want)
	}
}

func callRPC(t *testing.T, srv *Server, request string) map[string]any {
	t.Helper()
	raw, shouldReply, err := srv.handleLine([]byte(request))
	if err != nil {
		t.Fatalf("handleLine failed: %v", err)
	}
	if !shouldReply {
		t.Fatalf("expected reply for request: %s", request)
	}
	var envelope map[string]any
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("parse response: %v", err)
	}
	return envelope
}

func assertToolCallContentHasStatusSuccess(t *testing.T, envelope map[string]any) {
	t.Helper()
	if envelope["error"] != nil {
		t.Fatalf("tool call returned error: %#v", envelope["error"])
	}
	respData := extractToolCallData(t, envelope)
	if got := respData["status"]; got != "success" {
		t.Fatalf("unexpected tool response status: got %v", got)
	}
}

func extractToolCallData(t *testing.T, envelope map[string]any) map[string]any {
	t.Helper()
	result, ok := envelope["result"].(map[string]any)
	if !ok {
		t.Fatalf("missing result in response: %#v", envelope)
	}
	content, ok := result["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatalf("missing content in tool response: %#v", result)
	}
	item, ok := content[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected content item type: %#v", content[0])
	}
	data, ok := item["data"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected content data payload: %#v", item["data"])
	}
	return data
}

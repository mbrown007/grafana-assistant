module github.com/marcusz/monitoring-assistant/mcp_servers/kb-mcp-go

go 1.25.5

require (
	github.com/marcusz/monitoring-assistant v0.0.0
	github.com/mark3labs/mcp-go v0.7.0
)

replace github.com/marcusz/monitoring-assistant => ../..

require github.com/google/uuid v1.6.0 // indirect

# Testing Guide - Complete SM3 Stack

## Overview

This guide walks through testing the complete Grafana SM3 Chat Plugin with local MCP servers and the LGTM stack.

## Stack Components

### 1. LGTM Stack (Already Running)
- **Grafana**: http://localhost:3000 (admin/admin)
- **Prometheus**: http://localhost:9090
- **AlertManager**: http://localhost:9093
- **Loki**: Port 3100
- **Tempo**: Port 4317/4318
- **Pyroscope**: Port 4040

### 2. MCP Servers (Go)
- **Grafana MCP**: `/mcps/mcp-grafana` (Already Go ✅)
- **AlertManager MCP**: `/mcps/alertmanager-mcp-go` (New Go implementation ✅)
- **Genesys MCP**: `/mcps/genesys-cloud-mcp-go` (New Go implementation ✅)

### 3. Grafana Plugin
- **SM3 Chat Plugin**: `/grafana-sm3-chat-plugin` (Newly created ✅)

---

## Step 1: Start MCP Servers

### Terminal 1: Grafana MCP Server

```bash
cd /home/marc/Documents/github/sm3_agent/mcps/mcp-grafana

# Create .env file
cat > .env <<EOF
GRAFANA_URL=http://localhost:3000
GRAFANA_API_KEY=your-grafana-api-key
MCP_TRANSPORT=sse
MCP_HOST=0.0.0.0
MCP_PORT=8888
EOF

# Get Grafana API key
# 1. Go to http://localhost:3000
# 2. Login (admin/admin)
# 3. Settings → Service accounts → Add service account
# 4. Name: "MCP Server", Role: Admin
# 5. Add token, copy to .env file above

# Build and run
go build -o grafana-mcp ./cmd/mcp-grafana
./grafana-mcp
```

**Expected output:**
```
Starting Grafana MCP server (SSE mode) on 0.0.0.0:8888
Connected to Grafana at http://localhost:3000
Registered 25 MCP tools
```

### Terminal 2: AlertManager MCP Server

```bash
cd /home/marc/Documents/github/sm3_agent/mcps/alertmanager-mcp-go

# Create .env file
cat > .env <<EOF
ALERTMANAGER_URL=http://localhost:9093
MCP_TRANSPORT=sse
MCP_HOST=0.0.0.0
MCP_PORT=9300
EOF

# Build and run
make build
./bin/alertmanager-mcp-server
```

**Expected output:**
```
Starting AlertManager MCP server (SSE mode) on 0.0.0.0:9300
Connected to AlertManager at http://localhost:9093
Registered 8 MCP tools
```

### Terminal 3: Genesys MCP Server (Optional - requires credentials)

```bash
cd /home/marc/Documents/github/sm3_agent/mcps/genesys-cloud-mcp-go

# Create .env file (requires real Genesys Cloud credentials)
cat > .env <<EOF
GENESYSCLOUD_REGION=mypurecloud.com
GENESYSCLOUD_OAUTHCLIENT_ID=your-client-id
GENESYSCLOUD_OAUTHCLIENT_SECRET=your-client-secret
MCP_TRANSPORT=sse
MCP_HOST=0.0.0.0
MCP_PORT=9400
EOF

# Build and run
make build
./bin/genesys-mcp-server
```

**Note:** Skip this if you don't have Genesys Cloud credentials. The plugin will work with just Grafana and AlertManager MCPs.

---

## Step 2: Build and Install Grafana Plugin

### Terminal 4: Build Plugin

```bash
cd /home/marc/Documents/github/sm3_agent/grafana-sm3-chat-plugin

# Install frontend dependencies
npm install

# Build backend
make build-backend

# Build frontend
make build-frontend

# Check output
ls -la dist/
```

**Expected dist/ contents:**
```
gpx_sm3_chat_linux_amd64  # Backend binary
module.js                  # Frontend bundle
module.css
plugin.json
README.md
```

### Install Plugin to Grafana

```bash
# Create plugins directory if it doesn't exist
sudo mkdir -p /var/lib/grafana/plugins/sabio-sm3-chat-plugin

# Copy plugin files
sudo cp -r dist/* /var/lib/grafana/plugins/sabio-sm3-chat-plugin/

# Set ownership (if running Grafana as grafana user)
sudo chown -R grafana:grafana /var/lib/grafana/plugins/sabio-sm3-chat-plugin

# If using Docker Grafana from LGTM stack, copy to volume
# Find the volume path:
docker volume inspect docker-otel-lgtm_grafana-storage

# Copy to volume (adjust path based on above command)
# For Docker, you may need to mount the plugin directory
```

### Update Grafana Configuration

Edit `/etc/grafana/grafana.ini` or add environment variable:

```ini
[plugins]
allow_loading_unsigned_plugins = sabio-sm3-chat-plugin
```

**OR** if using Docker:

```bash
# Edit docker-compose.yml or run command
docker run -e GF_PLUGINS_ALLOW_LOADING_UNSIGNED_PLUGINS=sabio-sm3-chat-plugin ...
```

### Restart Grafana

```bash
# If systemd
sudo systemctl restart grafana-server

# If Docker (from LGTM stack)
cd /home/marc/Documents/docker-otel-lgtm
docker-compose restart grafana
```

---

## Step 3: Configure Plugin in Grafana UI

1. **Navigate to Plugin Settings**:
   - Go to http://localhost:3000
   - Login (admin/admin)
   - Settings (⚙️) → Plugins
   - Search for "SM3 Monitoring Agent"
   - Click on the plugin

2. **Configure Settings**:
   - Click **Configuration** tab
   - Enter the following JSON in the settings field:

```json
{
  "openai_api_key": "sk-your-openai-api-key-here",
  "grafana_mcp_url": "http://localhost:8888",
  "alertmanager_mcp_url": "http://localhost:9300",
  "genesys_mcp_url": "http://localhost:9400"
}
```

   - For security, use Grafana's secret management for the API key:
     - In the UI, there should be a separate field for secrets
     - Add `openai_api_key` as a secure field

3. **Save Configuration**

---

## Step 4: Create Test Dashboard

1. **Create Dashboard**:
   - Click **+ → Dashboard**
   - Click **Add visualization**

2. **Add SM3 Chat Panel**:
   - In the visualization picker, search for "SM3"
   - Select **SM3 Monitoring Agent**
   - You should see the chat interface

3. **Resize Panel**:
   - Recommended width: 400-500px (right sidebar layout)
   - Height: Full dashboard height

4. **Configure Panel Options**:
   - Toggle **Show Tool Calls**: ON (to see tool execution)

5. **Save Dashboard**:
   - Name: "SM3 Test Dashboard"
   - Folder: General

---

## Step 5: Test Chat Functionality

### Test 1: Basic Greeting

**User Message:**
```
Hello, what can you help me with?
```

**Expected Response:**
- Greeting message
- List of capabilities (Grafana, AlertManager, Genesys tools)
- No tool calls

**What to Check:**
✅ Message appears immediately
✅ Response streams in real-time (tokens appear progressively)
✅ No errors in browser console
✅ No errors in MCP server logs

### Test 2: Grafana Dashboard Query

**User Message:**
```
List all dashboards
```

**Expected Behavior:**
- Tool call appears: `search_dashboards`
- Arguments: `{}`
- Result: JSON array of dashboards
- LLM formats results into readable table or artifact

**What to Check:**
✅ Tool call section appears (if Show Tool Calls enabled)
✅ Tool executes successfully
✅ Dashboard list appears (may be empty if no dashboards exist)
✅ Artifact table renders if multiple dashboards found

### Test 3: Dashboard Context Injection

**User Message:**
```
What dashboard am I looking at?
```

**Expected Behavior:**
- Backend log shows dashboard context:
  ```
  [Dashboard Context]
  Name: SM3 Test Dashboard
  UID: <dashboard-uid>
  Folder: General
  Tags: []
  Time Range: <current-time-range>
  ```
- LLM responds with dashboard details

**What to Check:**
✅ Dashboard context appears in backend logs (plugin logs)
✅ Response mentions correct dashboard name and details
✅ Time range is correct

### Test 4: AlertManager Query

**User Message:**
```
Show me all active alerts
```

**Expected Behavior:**
- Tool call: `alertmanager__list_alerts` (note the prefix!)
- Result: Array of active alerts (may be empty)
- If alerts exist, formatted as artifact table

**What to Check:**
✅ Tool name has `alertmanager__` prefix
✅ AlertManager MCP server logs show request
✅ Alerts displayed (or "no active alerts" message)

### Test 5: Create Alert Silence (Advanced)

**User Message:**
```
Create a silence for all alerts matching alertname=TestAlert for 1 hour
```

**Expected Behavior:**
- Tool call: `alertmanager__create_silence`
- Arguments include matchers, start time, end time, creator
- Success response

**What to Check:**
✅ Silence created successfully
✅ Can verify in AlertManager UI: http://localhost:9093/#/silences

### Test 6: Prometheus Query via Grafana MCP

**User Message:**
```
Query Prometheus for up metric
```

**Expected Behavior:**
- Tool call: `query_prometheus`
- Arguments: `{ query: "up" }`
- Result: Prometheus metrics data
- LLM formats as table or chart artifact

**What to Check:**
✅ Query executes
✅ Metrics returned
✅ Artifact renders (if applicable)

---

## Step 6: Verify Streaming

### Real-Time Token Streaming Test

1. Ask a question that requires a long response:
   ```
   Explain how Grafana dashboards work and how to create effective visualizations
   ```

2. **Watch for:**
   - Tokens appearing word-by-word in real-time
   - Streaming indicator (blinking cursor or "typing" indicator)
   - Smooth UX without blocking

3. **Check Network Tab** (Browser DevTools):
   - Look for `/api/plugins/sabio-sm3-chat-plugin/resources/chat-stream` request
   - Headers should include `Content-Type: text/event-stream`
   - Response should be streaming

---

## Step 7: Verify Artifacts

### Test Chart Artifact

**User Message:**
```
Create a bar chart showing the number of dashboards by folder
```

**Expected Behavior:**
- LLM generates artifact JSON:
  ```artifact
  {
    "type": "chart",
    "title": "Dashboards by Folder",
    "chartType": "bar",
    "data": [
      {"name": "General", "count": 5},
      {"name": "Infrastructure", "count": 3}
    ]
  }
  ```
- Chart renders using Recharts

**What to Check:**
✅ Artifact code block appears in message
✅ Chart component renders
✅ Data displays correctly

### Test Table Artifact

**User Message:**
```
Show me a table of all AlertManager silences
```

**Expected Behavior:**
- Tool call: `alertmanager__list_silences`
- LLM wraps result in table artifact
- Table renders with columns: ID, Created By, Status, Start, End, Matchers

**What to Check:**
✅ Table artifact renders
✅ Columns display correctly
✅ Data populates

---

## Troubleshooting

### Plugin Not Appearing in Grafana

1. **Check plugin is in plugins directory:**
   ```bash
   ls -la /var/lib/grafana/plugins/sabio-sm3-chat-plugin/
   ```

2. **Check grafana.ini allows unsigned plugins:**
   ```bash
   grep allow_loading_unsigned_plugins /etc/grafana/grafana.ini
   ```

3. **Check Grafana logs:**
   ```bash
   tail -f /var/log/grafana/grafana.log | grep sm3
   # OR for Docker:
   docker logs <grafana-container> | grep sm3
   ```

### MCP Connection Failures

1. **Check MCP server is running:**
   ```bash
   curl http://localhost:8888/health  # Grafana MCP
   curl http://localhost:9300/health  # AlertManager MCP
   ```

2. **Check plugin backend logs:**
   - Look in Grafana logs for MCP connection errors
   - Check plugin settings are correct

3. **Verify URLs are accessible from Grafana:**
   - If Grafana is in Docker, MCPs must be accessible from container
   - Use `host.docker.internal` instead of `localhost` for Docker

### Streaming Not Working

1. **Check SSE headers in Network tab:**
   - Should see `Content-Type: text/event-stream`
   - Response should be chunked

2. **Check for proxy/firewall blocking SSE:**
   - Some proxies strip SSE connections
   - Test direct connection without proxy

3. **Check OpenAI API key:**
   ```bash
   # Test key directly
   curl https://api.openai.com/v1/models \
     -H "Authorization: Bearer $OPENAI_API_KEY"
   ```

### Tool Execution Errors

1. **Check tool name prefixes:**
   - Grafana tools: No prefix
   - AlertManager tools: `alertmanager__` prefix
   - Genesys tools: `genesys__` prefix

2. **Check MCP server logs for errors**

3. **Verify API credentials and permissions**

---

## Success Criteria

✅ All 3 MCP servers running and responding to health checks
✅ Grafana plugin installed and visible in Plugins page
✅ Chat panel renders in dashboard
✅ Messages send and receive successfully
✅ Streaming works (tokens appear in real-time)
✅ Tool calls execute and display results
✅ Dashboard context extracted and injected correctly
✅ Artifacts render (charts, tables, metrics)
✅ No errors in browser console or server logs

---

## Next Steps

Once basic testing is complete:

1. **Deploy to Production Grafana**:
   - Sign plugin: `npx @grafana/toolkit plugin:sign`
   - Package: `make package`
   - Install on production Grafana instances

2. **Configure for Multiple Customers**:
   - Use dynamic MCP container spawning (see `mcp_servers.json`)
   - Implement customer selection in plugin UI

3. **Add More Tools**:
   - Extend MCP servers with additional API integrations
   - Add custom tools for specific use cases

4. **Monitoring**:
   - Add metrics/logging to MCP servers
   - Monitor plugin performance
   - Track usage analytics

---

## Appendix: Useful Commands

### Check All Processes

```bash
# Check if MCP servers are running
ps aux | grep mcp

# Check ports
netstat -tuln | grep -E '(8888|9300|9400)'

# Check Grafana plugins
curl -u admin:admin http://localhost:3000/api/plugins
```

### Restart Everything

```bash
# Stop all
pkill -f mcp
docker-compose -f /home/marc/Documents/docker-otel-lgtm/docker-compose.yml restart grafana

# Start MCPs
# (Run commands from Step 1)

# Restart Grafana
sudo systemctl restart grafana-server
# OR
docker-compose -f /home/marc/Documents/docker-otel-lgtm/docker-compose.yml restart grafana
```

### View All Logs

```bash
# Terminal 1: Grafana MCP
tail -f /path/to/grafana-mcp.log

# Terminal 2: AlertManager MCP
tail -f /path/to/alertmanager-mcp.log

# Terminal 3: Plugin logs (Grafana)
tail -f /var/log/grafana/grafana.log

# Terminal 4: Browser console (DevTools)
```

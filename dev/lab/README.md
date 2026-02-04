# Lab Deployment

## Files to Copy

Copy these to `/home/mbrown/grafana-assistant/` on the lab server:

```
/home/mbrown/grafana-assistant/
├── assistant          # Binary (build with: go build -o assistant ./cmd/assistant)
├── config.yaml        # This config file
├── run.sh             # Runner script
├── .env               # Environment variables (copy from .env.example)
├── data/              # Created automatically
│   ├── assistant.db   # SQLite database
│   └── audit.log      # Audit log
└── KB/                # Knowledge base (optional)
```

## Setup

1. Build the binary:
   ```bash
   go build -o assistant ./cmd/assistant
   ```

2. Copy files to lab:
   ```bash
   scp assistant config.yaml run.sh .env.example mbrown@10.222.251.176:~/grafana-assistant/
   ```

3. On lab server:
   ```bash
   cd ~/grafana-assistant
   cp .env.example .env
   # Edit .env with your API keys
   chmod +x run.sh
   mkdir -p data
   ```

4. Update HAProxy (`/etc/haproxy/haproxy.cfg`):
   ```cfg
   backend assistant
     mode http
     http-request set-path %[path,regsub(^/assistant,)]
     http-request set-header X-Forwarded-Prefix /assistant
     http-request set-header X-Forwarded-Proto https
     server monitoring-rocky 127.0.0.1:5480 check
   ```

5. Reload HAProxy:
   ```bash
   sudo systemctl reload haproxy
   ```

## Usage

```bash
cd ~/grafana-assistant

# Run in foreground (Ctrl+C to stop)
./run.sh

# Or run in background
./run.sh start
./run.sh status
./run.sh logs
./run.sh stop
```

## Access

https://mon.sabio.cloud/assistant/

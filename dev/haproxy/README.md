# HAProxy Dev Setup

This directory contains a development environment that simulates a production HAProxy setup with:
- HAProxy on port 8443 (HTTPS)
- Grafana at root URL `/`
- Assistant at `/assistant`
- Prometheus at `/prometheus`
- Alertmanager at `/alertmanager`

## Quick Start

```bash
cd dev/haproxy
./setup.sh
```

Then start the assistant (from project root):

```bash
ASSISTANT_OPENAI_API_KEY=sk-xxx go run ./cmd/assistant -config dev/haproxy/config.assistant.yaml
```

Access via: https://localhost:8443 (accept self-signed cert warning)
- Grafana: https://localhost:8443/ (admin/admin)
- Assistant: https://localhost:8443/assistant/

## How It Works

The assistant now reads `X-Forwarded-*` headers from HAProxy:

1. **X-Forwarded-Prefix**: Used for frontend base path (`config.js`) and cookie paths
2. **X-Forwarded-Proto**: Used to set correct `SameSite` cookie attribute
   - HTTPS: `SameSite=None; Secure` (allows cross-origin/iframe)
   - HTTP: `SameSite=Lax` (more secure for direct access)

### HAProxy Config Required

Your production HAProxy needs these headers in the assistant backend:

```cfg
backend assistant
  mode http
  http-request set-path %[path,regsub(^/assistant,)]
  http-request set-header X-Forwarded-Prefix /assistant
  http-request set-header X-Forwarded-Proto https
  server monitoring-rocky 127.0.0.1:5480 check
```

### Assistant Config

The assistant runs with `base_path: ""` (empty) since HAProxy strips the `/assistant` prefix.
The `X-Forwarded-Prefix` header tells the assistant its external path for:
- Frontend API calls (`/assistant/api/chat`)
- Cookie paths (`Path=/assistant/`)
- Any redirects

## Testing Checklist

1. [ ] Access https://localhost:8443/ - Grafana login works
2. [ ] Access https://localhost:8443/assistant/ - Assistant loads
3. [ ] Login to Grafana, then access assistant - session carries over
4. [ ] Send a chat message - API calls work
5. [ ] Check browser Network tab - cookies have `Path=/assistant/`

## Debugging

Check HAProxy logs:
```bash
docker compose logs -f haproxy
```

Check browser Network tab for:
- Cookie headers on requests (should include `grafana_session`)
- Set-Cookie headers on responses (should have `Path=/assistant/; SameSite=None; Secure`)
- No CORS errors
- No 401/403 responses

Check assistant logs for auth failures.

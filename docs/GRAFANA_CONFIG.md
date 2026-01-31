# Grafana Configuration Requirements

The monitoring assistant embeds Grafana in an iframe via a reverse proxy. The following Grafana configuration changes are required.

## grafana.ini Changes

### Allow Embedding

Grafana blocks iframe embedding by default. Enable it:

```ini
[security]
allow_embedding = true
```

### Cookie SameSite Policy

When the wrapper and Grafana share the same origin (via the reverse proxy), `Lax` is sufficient:

```ini
[security]
cookie_samesite = lax
```

If for any reason they are on different origins, use:

```ini
[security]
cookie_samesite = none
cookie_secure = true
```

### Content Security Policy

If Grafana has CSP enabled, ensure the wrapper origin is allowed as a frame ancestor. The reverse proxy strips CSP headers from proxied responses, but if Grafana sends CSP via `<meta>` tags, adjust:

```ini
[security]
content_security_policy = false
```

Or configure `content_security_policy_template` to include the wrapper origin.

## Service Account Token

The assistant uses a Grafana service account token to access the Grafana API (fetching dashboard JSON, user info, etc.).

### Create a Service Account

1. Go to **Administration > Service Accounts** in Grafana
2. Click **Add service account**
3. Name it (e.g., `monitoring-assistant`)
4. Set the role to **Viewer** (minimum required; Editor if write operations are needed later)
5. Click **Create**

### Generate a Token

1. On the service account page, click **Add service account token**
2. Set an expiration (or no expiration for development)
3. Copy the token (starts with `glsa_...`)
4. Add it to your `config.yaml`:

```yaml
grafana_token: "glsa_your_token_here"
```

Or set the environment variable:

```bash
export ASSISTANT_GRAFANA_TOKEN="glsa_your_token_here"
```

## Verification

After making these changes:

1. Restart Grafana: `sudo systemctl restart grafana-server`
2. Start the assistant: `make run`
3. Open `http://localhost:8080` in your browser
4. Grafana should load inside the iframe with your existing session preserved
5. Check the browser console for any CSP or cookie errors

#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "=== HAProxy Dev Setup ==="

# Create certs directory
mkdir -p certs

# Generate self-signed certificate for dev
if [ ! -f certs/dev.pem ]; then
    echo "Generating self-signed certificate..."
    openssl req -x509 -newkey rsa:2048 -keyout certs/dev.key -out certs/dev.crt \
        -days 365 -nodes -subj "/CN=localhost" \
        -addext "subjectAltName=DNS:localhost,DNS:maas.local,IP:127.0.0.1"

    # HAProxy needs combined cert+key in single PEM file
    cat certs/dev.crt certs/dev.key > certs/dev.pem
    chmod 600 certs/dev.pem
    echo "Certificate created: certs/dev.pem"
else
    echo "Certificate already exists: certs/dev.pem"
fi

echo ""
echo "=== Starting services ==="
docker compose up -d

echo ""
echo "=== Service URLs ==="
echo "HAProxy (HTTPS):     https://localhost:8443"
echo "  - Grafana:         https://localhost:8443/"
echo "  - Assistant:       https://localhost:8443/assistant/"
echo "  - Prometheus:      https://localhost:8443/prometheus/"
echo "  - Alertmanager:    https://localhost:8443/alertmanager/"
echo ""
echo "Direct access (for debugging):"
echo "  - Grafana:         http://localhost:13000"
echo "  - Prometheus:      http://localhost:19090"
echo "  - Alertmanager:    http://localhost:19093"
echo ""
echo "=== Next steps ==="
echo "1. Start the assistant on port 5470 with base_path=/assistant:"
echo "   ASSISTANT_BASE_PATH=/assistant ASSISTANT_LISTEN_ADDR=:5470 go run ./cmd/assistant"
echo ""
echo "2. Access Grafana at https://localhost:8443 (accept the self-signed cert)"
echo "   Login: admin / admin"
echo ""
echo "3. Test the assistant iframe at https://localhost:8443/assistant/"
echo ""
echo "=== Logs ==="
echo "docker compose logs -f haproxy"

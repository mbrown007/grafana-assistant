# Build stage
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o alertmanager-mcp-server ./cmd/server

# Runtime stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/alertmanager-mcp-server .

# Expose default port (for SSE/HTTP modes)
EXPOSE 8000

# Run as non-root user
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser && \
    chown -R appuser:appuser /app

USER appuser

# Default to stdio transport (can be overridden)
ENV MCP_TRANSPORT=stdio
ENV MCP_HOST=0.0.0.0
ENV MCP_PORT=8000

# Health check (only works for HTTP/SSE transports)
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD [ -n "$MCP_TRANSPORT" ] && [ "$MCP_TRANSPORT" != "stdio" ] && wget --no-verbose --tries=1 --spider http://localhost:${MCP_PORT}/health || exit 0

ENTRYPOINT ["./alertmanager-mcp-server"]

# Multi-stage build for Genesys Cloud MCP Server
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o genesys-mcp-server ./cmd/server

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

# Create non-root user
RUN addgroup -g 1000 mcp && \
    adduser -D -u 1000 -G mcp mcp

WORKDIR /home/mcp

# Copy binary from builder
COPY --from=builder /app/genesys-mcp-server .

# Set ownership
RUN chown -R mcp:mcp /home/mcp

# Switch to non-root user
USER mcp

# Expose SSE port (if using SSE transport)
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD if [ "$MCP_TRANSPORT" = "sse" ]; then wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1; else exit 0; fi

# Run the server
ENTRYPOINT ["./genesys-mcp-server"]

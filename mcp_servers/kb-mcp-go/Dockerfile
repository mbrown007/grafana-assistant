FROM golang:1.23-alpine AS build
WORKDIR /src
COPY . .
RUN go build -o /out/kb-mcp-server ./cmd/server

FROM alpine:3.20
WORKDIR /app
COPY --from=build /out/kb-mcp-server /app/kb-mcp-server

# Default to stdio transport (can be overridden)
ENV MCP_TRANSPORT=stdio
ENV KB_PATH=/app/KB

ENTRYPOINT ["/app/kb-mcp-server"]

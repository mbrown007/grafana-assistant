package server

import (
	"github.com/mark3labs/mcp-go/mcp"
	"golang.org/x/time/rate"
)

const (
	defaultRateLimitRPS   = 5.0
	defaultRateLimitBurst = 10
)

var (
	DefaultRateLimitRPS   = getEnvFloat("MCP_RATE_LIMIT_RPS", defaultRateLimitRPS)
	DefaultRateLimitBurst = getEnvInt("MCP_RATE_LIMIT_BURST", defaultRateLimitBurst)
)

func newRateLimiter() *rate.Limiter {
	if DefaultRateLimitRPS <= 0 || DefaultRateLimitBurst <= 0 {
		return nil
	}
	return rate.NewLimiter(rate.Limit(DefaultRateLimitRPS), DefaultRateLimitBurst)
}

func (s *MCPServer) enforceRateLimit() *mcp.CallToolResult {
	if s.limiter == nil {
		return nil
	}
	if !s.limiter.Allow() {
		return mcp.NewToolResultError("rate limit exceeded")
	}
	return nil
}

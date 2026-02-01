package server

import (
	"os"
	"strconv"

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

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}

func getEnvFloat(key string, defaultVal float64) float64 {
	if val := os.Getenv(key); val != "" {
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f
		}
	}
	return defaultVal
}

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

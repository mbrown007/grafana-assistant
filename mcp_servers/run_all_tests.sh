#!/bin/bash

# SM3 Test Runner
# Runs all tests for Grafana plugin and MCP servers

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "========================================="
echo "     SM3 Test Runner"
echo "========================================="
echo ""

# Store root directory
ROOT_DIR=$(pwd)
FAILED=0

# Function to run tests for a component
run_tests() {
    local name=$1
    local dir=$2

    echo -e "${YELLOW}Testing: $name${NC}"
    echo "Directory: $dir"

    if [ ! -d "$dir" ]; then
        echo -e "${RED}✗ Directory not found: $dir${NC}"
        FAILED=$((FAILED + 1))
        return
    fi

    cd "$dir"

    if go test ./... -v -cover; then
        echo -e "${GREEN}✓ $name tests passed${NC}"
    else
        echo -e "${RED}✗ $name tests failed${NC}"
        FAILED=$((FAILED + 1))
    fi

    echo ""
    cd "$ROOT_DIR"
}

# Run tests for each component
echo "=== Running Tests ==="
echo ""

run_tests "Grafana SM3 Chat Plugin" "grafana-sm3-chat-plugin"
run_tests "AlertManager MCP Server" "mcps/alertmanager-mcp-go"
run_tests "Genesys Cloud MCP Server" "mcps/genesys-cloud-mcp-go"

# Summary
echo "========================================="
echo "     Test Summary"
echo "========================================="

if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}✓ All tests passed!${NC}"
    exit 0
else
    echo -e "${RED}✗ $FAILED test suite(s) failed${NC}"
    exit 1
fi

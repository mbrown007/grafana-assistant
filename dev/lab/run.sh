#!/bin/bash
# Monitoring Assistant - Lab Runner
# Usage:
#   ./run.sh          - Start in foreground (Ctrl+C to stop)
#   ./run.sh start    - Start in background
#   ./run.sh stop     - Stop background process
#   ./run.sh status   - Check if running
#   ./run.sh logs     - Tail the log file

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

BINARY="./assistant"
CONFIG="./config.yaml"
ENV_FILE="./.env"
PID_FILE="./assistant.pid"
LOG_FILE="./assistant.log"

# Load environment variables from .env if it exists
load_env() {
    if [ -f "$ENV_FILE" ]; then
        echo "Loading environment from $ENV_FILE"
        set -a
        source "$ENV_FILE"
        set +a
    fi
}

check_binary() {
    if [ ! -f "$BINARY" ]; then
        echo "Error: Binary not found at $BINARY"
        echo "Copy the assistant binary to this directory first."
        exit 1
    fi
    if [ ! -x "$BINARY" ]; then
        chmod +x "$BINARY"
    fi
}

check_config() {
    if [ ! -f "$CONFIG" ]; then
        echo "Error: Config not found at $CONFIG"
        exit 1
    fi
}

get_pid() {
    if [ -f "$PID_FILE" ]; then
        cat "$PID_FILE"
    fi
}

is_running() {
    local pid=$(get_pid)
    if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
        return 0
    fi
    return 1
}

start_foreground() {
    check_binary
    check_config
    load_env

    echo "========================================"
    echo "  Monitoring Assistant - Lab"
    echo "========================================"
    echo "Config:  $CONFIG"
    echo "Listen:  :5480"
    echo "Grafana: http://127.0.0.1:3000"
    echo ""
    echo "Access via: https://mon.sabio.cloud/assistant/"
    echo ""
    echo "Press Ctrl+C to stop"
    echo "========================================"
    echo ""

    exec "$BINARY" -config "$CONFIG"
}

start_background() {
    check_binary
    check_config
    load_env

    if is_running; then
        echo "Already running (PID: $(get_pid))"
        exit 1
    fi

    echo "Starting in background..."
    nohup "$BINARY" -config "$CONFIG" > "$LOG_FILE" 2>&1 &
    echo $! > "$PID_FILE"
    sleep 1

    if is_running; then
        echo "Started (PID: $(get_pid))"
        echo "Logs: $LOG_FILE"
    else
        echo "Failed to start. Check $LOG_FILE for errors."
        rm -f "$PID_FILE"
        exit 1
    fi
}

stop_process() {
    if ! is_running; then
        echo "Not running"
        rm -f "$PID_FILE"
        exit 0
    fi

    local pid=$(get_pid)
    echo "Stopping (PID: $pid)..."
    kill "$pid"

    # Wait for graceful shutdown
    for i in {1..10}; do
        if ! is_running; then
            echo "Stopped"
            rm -f "$PID_FILE"
            exit 0
        fi
        sleep 1
    done

    # Force kill if still running
    echo "Force killing..."
    kill -9 "$pid" 2>/dev/null || true
    rm -f "$PID_FILE"
    echo "Stopped"
}

show_status() {
    if is_running; then
        echo "Running (PID: $(get_pid))"
        echo "Uptime: $(ps -o etime= -p $(get_pid) 2>/dev/null | tr -d ' ')"
    else
        echo "Not running"
        rm -f "$PID_FILE" 2>/dev/null
    fi
}

show_logs() {
    if [ -f "$LOG_FILE" ]; then
        tail -f "$LOG_FILE"
    else
        echo "No log file found at $LOG_FILE"
        exit 1
    fi
}

case "${1:-}" in
    start)
        start_background
        ;;
    stop)
        stop_process
        ;;
    status)
        show_status
        ;;
    logs)
        show_logs
        ;;
    restart)
        stop_process
        sleep 1
        start_background
        ;;
    "")
        start_foreground
        ;;
    *)
        echo "Usage: $0 [start|stop|status|logs|restart]"
        echo ""
        echo "  (no args)  Run in foreground (Ctrl+C to stop)"
        echo "  start      Start in background"
        echo "  stop       Stop background process"
        echo "  status     Check if running"
        echo "  logs       Tail the log file"
        echo "  restart    Stop and start"
        exit 1
        ;;
esac

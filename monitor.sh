#!/bin/bash

# Lavender Messenger Server Health Monitor
# This script checks if the server is running and restarts it if necessary
# Intended to be run via cron (e.g., */30 * * * * /home/ferz/LavenderMessenger/monitor.sh)

SERVER_DIR="/home/ferz/LavenderMessenger"
LOG_FILE="$SERVER_DIR/logs.txt"
PID_FILE="$SERVER_DIR/server.pid"
PORT=50051

cd "$SERVER_DIR" || exit 1

# Log function with timestamp
log_message() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] [MONITOR] $1" >> "$LOG_FILE"
}

# Check if server is running on port 50051
check_server() {
    # Check if any process is listening on port 50051
    if lsof -ti:$PORT > /dev/null 2>&1; then
        return 0  # Server is running
    fi
    return 1  # Server is not running
}

log_message "Health check started"

# Check if process exists
if check_server; then
    log_message "Server is running on port $PORT - OK"
    exit 0
fi

log_message "Server not detected on port $PORT. Restarting..."

# Kill any lingering processes (just in case)
lsof -ti:$PORT | xargs kill -9 2>/dev/null
pkill -f lavender-server 2>/dev/null

# Wait a moment for cleanup
sleep 2

# Start the server
if [ -f ./lavender-server ]; then
    nohup ./lavender-server >> "$LOG_FILE" 2>&1 &
    NEW_PID=$!
    echo "$NEW_PID" > "$PID_FILE"

    # Wait and verify
    sleep 3

    if check_server; then
        log_message "Server restarted successfully (PID: $NEW_PID)"
    else
        log_message "ERROR: Failed to restart server!"
    fi
else
    log_message "ERROR: lavender-server binary not found!"
fi

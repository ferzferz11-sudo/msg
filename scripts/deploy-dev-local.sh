#!/bin/bash
# deploy-dev-local.sh — Cross-compile + deploy dev server from local machine to remote
# Использование: ./scripts/deploy-dev-local.sh
# Требования: SSH alias "lava" в ~/.ssh/config

set -e

SERVER="lava"
REMOTE_DIR="/root/LavenderMessenger/run"
BINARY="lavender-server-dev"

echo "=== 1. Cross-compile for Linux ==="
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o /tmp/$BINARY .
echo "✓ Built /tmp/$BINARY ($(du -h /tmp/$BINARY | cut -f1))"

echo "=== 2. Upload to server ==="
scp /tmp/$BINARY ${SERVER}:/tmp/$BINARY
echo "✓ Uploaded"

echo "=== 3. Stop dev server ==="
ssh $SERVER "systemctl stop lavender-server-dev" 2>/dev/null || true
sleep 1

echo "=== 4. Replace binary ==="
ssh $SERVER "cp /tmp/$BINARY ${REMOTE_DIR}/${BINARY} && chmod +x ${REMOTE_DIR}/${BINARY}"
echo "✓ Binary replaced"

echo "=== 5. Start dev server ==="
ssh $SERVER "systemctl start lavender-server-dev"
sleep 2

echo "=== 6. Restart log monitor ==="
ssh $SERVER "systemctl restart log-monitor-dev" 2>/dev/null || true
sleep 1

echo "=== 7. Verify ==="
ssh $SERVER "systemctl is-active lavender-server-dev" | grep -q "active" && {
    echo "✓ Dev server running"
} || {
    echo "✗ Dev server FAILED"
    ssh $SERVER "journalctl -u lavender-server-dev --no-pager -n 10"
    exit 1
}

echo ""
echo "Dev server: gRPC :50052, HTTP :8083"
echo "Logs: http://13.140.25.249/server-logs-dev"

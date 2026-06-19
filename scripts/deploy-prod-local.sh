#!/bin/bash
# deploy-prod-local.sh — Cross-compile + deploy prod server from local machine to remote
# Использование: ./scripts/deploy-prod-local.sh
# Требования: SSH alias "lava" в ~/.ssh/config

set -e

SERVER="lava"
REMOTE_DIR="/root/LavenderMessenger/run"
BINARY="lavender-server"

echo "=== 1. Cross-compile for Linux ==="
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o /tmp/$BINARY .
echo "✓ Built /tmp/$BINARY ($(du -h /tmp/$BINARY | cut -f1))"

echo "=== 2. Backup current binary ==="
ssh $SERVER "cp ${REMOTE_DIR}/${BINARY} ${REMOTE_DIR}/${BINARY}-old" 2>/dev/null || true
echo "✓ Backup saved"

echo "=== 3. Upload to server ==="
scp /tmp/$BINARY ${SERVER}:/tmp/$BINARY
echo "✓ Uploaded"

echo "=== 4. Stop prod server ==="
ssh $SERVER "systemctl stop lavender-server" 2>/dev/null || true
sleep 1

echo "=== 5. Replace binary ==="
ssh $SERVER "cp /tmp/$BINARY ${REMOTE_DIR}/${BINARY} && chmod +x ${REMOTE_DIR}/${BINARY}"
echo "✓ Binary replaced"

echo "=== 6. Start prod server ==="
ssh $SERVER "systemctl start lavender-server"
sleep 3

echo "=== 7. Verify ==="
ssh $SERVER "systemctl is-active lavender-server" | grep -q "active" && {
    echo "✓ Prod server running"
} || {
    echo "✗ Prod server FAILED — rolling back..."
    ssh $SERVER "cp ${REMOTE_DIR}/${BINARY}-old ${REMOTE_DIR}/${BINARY} && systemctl start lavender-server"
    exit 1
}

echo ""
echo "Prod server: gRPC :50051, HTTP :8082"

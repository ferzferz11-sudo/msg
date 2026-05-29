#!/bin/bash
# deploy_server.sh — Build & deploy Lavender Messenger server (run from repo root)
set -euo pipefail

REMOTE_DIR="/root/LavenderMessenger/run"
SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"

echo "🔨 Building server..."
cd "$SCRIPT_DIR"
go build -o lavender-server .

echo "📦 Deploying to ${REMOTE_DIR}..."
cp -f lavender-server "$REMOTE_DIR/"
cp -f server *.go *.proto go.mod go.sum "$REMOTE_DIR/" 2>/dev/null || true
cp -f scripts/*.sh "$REMOTE_DIR/../scripts/" 2>/dev/null || true

echo "🔄 Restarting server..."
systemctl restart lavender-server
sleep 2

if systemctl is-active lavender-server >/dev/null 2>&1; then
    echo "✅ Server running"
else
    echo "❌ Server failed!"
    journalctl -u lavender-server --no-pager -n 20
    exit 1
fi

echo "🚀 Deploy complete!"

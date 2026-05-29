#!/bin/bash
# deploy_server.sh — Build & deploy Lavender Messenger server to 13.140.25.249
# Run from server repo root: ./scripts/deploy_server.sh

set -euo pipefail

REMOTE_HOST="13.140.25.249"
REMOTE_USER="root"
REMOTE_DIR="/root/LavenderMessenger/run"
LOCAL_DIR="$(cd "$(dirname "$0")/.." && pwd)"

echo "🔨 Building server..."
cd "$LOCAL_DIR"
go build -o lavender-server .
go vet ./... 2>&1 || true

echo "📦 Uploading to ${REMOTE_HOST}..."
rsync -avz --delete \
    --exclude '.git' \
    --exclude '.env' \
    --exclude '.idea' \
    --exclude 'client' \
    --exclude 'scripts' \
    --exclude '*.apk' \
    --exclude 'index.html' \
    --exclude 'changelog.txt' \
    --exclude 'version.txt' \
    -e "ssh -o StrictHostKeyChecking=no" \
    "$LOCAL_DIR/" "${REMOTE_USER}@${REMOTE_HOST}:${REMOTE_DIR}/"

echo "🔄 Restarting server on ${REMOTE_HOST}..."
ssh -o StrictHostKeyChecking=no "${REMOTE_USER}@${REMOTE_HOST}" "
    systemctl restart lavender-server
    sleep 2
    systemctl is-active lavender-server && echo '✅ Server running' || echo '❌ Server failed'
"

echo "🚀 Deploy complete!"

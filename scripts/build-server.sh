#!/bin/bash
# build-server.sh — собрать сервер и доставить в run/
set -eu

REPO_DIR="/root/msg"
RUN_DIR="/root/LavenderMessenger/run"

echo "🔨 Building server..."
cd "$REPO_DIR"
go clean -cache
go build -o lavender-server .

echo "📦 Copying to ${RUN_DIR}..."
cp -f lavender-server "$RUN_DIR/"
cp -f .env "$RUN_DIR/"
# Firebase ключ — берём тот что указан в .env, если нет то новый
if grep -q FIREBASE_CREDENTIALS_PATH "$RUN_DIR/.env" 2>/dev/null; then
    FIREBASE_KEY=$(grep FIREBASE_CREDENTIALS_PATH "$REPO_DIR/.env" | cut -d= -f2)
    cp -f "$REPO_DIR/$(basename "$FIREBASE_KEY")" "$RUN_DIR/" 2>/dev/null || true
fi
# Копируем остальные необходимые файлы
cp -f .env.example "$RUN_DIR/" 2>/dev/null || true
cp -f config.yaml "$RUN_DIR/" 2>/dev/null || true
cp -f monitor.sh "$RUN_DIR/" 2>/dev/null || true

echo "🔄 Restarting server..."
systemctl restart lavender-server
sleep 3

if systemctl is-active lavender-server >/dev/null 2>&1; then
    echo "✅ Server running"
    journalctl -u lavender-server --no-pager -n 5
else
    echo "❌ Server failed!"
    journalctl -u lavender-server --no-pager -n 20
    exit 1
fi

echo "🚀 Done!"

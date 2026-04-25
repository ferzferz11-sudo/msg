#!/bin/bash

# Lavender Messenger Server Deployment Script (Run on Local Mac)
# This script builds for Linux, uploads, and restarts the server.

REMOTE_USER="ferz"
REMOTE_HOST="159.195.38.145"
REMOTE_PORT="31703"
REMOTE_KEY="$HOME/.ssh/ferzz@x-cart.com"
REMOTE_DIR="/home/ferz/LavenderMessenger"

echo "🚀 Starting server deployment to $REMOTE_HOST..."

# 1. Сборка бинарного файла для Linux (Кросс-компиляция)
echo "🔨 Building Lavender Server for Linux (amd64)..."
GOOS=linux GOARCH=amd64 go build -o lavender-server .
if [ $? -ne 0 ]; then
    echo "❌ Local build failed! Check your Go code."
    exit 1
fi

# 2. Синхронизация кода и бинарного файла
echo "📦 Syncing files to server..."
rsync -avz -e "ssh -p $REMOTE_PORT -i $REMOTE_KEY" \
    --exclude '.git' \
    --exclude '.env' \
    --exclude '.idea' \
    --exclude 'client' \
    ./ $REMOTE_USER@$REMOTE_HOST:$REMOTE_DIR/

# 3. Перезапуск сервера
echo "🔄 Restarting remote server..."
# Используем -f для ssh, чтобы он ушел в бэкграунд сразу после выполнения команды
# И полностью перенаправляем потоки внутри удаленной команды
ssh -p $REMOTE_PORT -i $REMOTE_KEY $REMOTE_USER@$REMOTE_HOST \
    "cd $REMOTE_DIR && chmod +x lavender-server && \
    (pkill lavender-server || true) && \
    nohup ./lavender-server > logs.txt 2>&1 < /dev/null &"

# Удаляем локальный линукс-бинарник
rm lavender-server

echo "✅ Server deployment successful!"
echo "📝 Server is running in background. You can check logs on the server: tail -f $REMOTE_DIR/logs.txt"

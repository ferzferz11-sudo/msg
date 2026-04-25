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

# 2. Создание серверного скрипта запуска (Server-side restart script)
cat << 'EOF' > start.sh
#!/bin/bash
# Server-side script to safely restart the Lavender Server
cd "$(dirname "$0")"
# Kill existing process
pkill lavender-server || true
# Start server in background with log redirection
nohup ./lavender-server > logs.txt 2>&1 &
echo "Server started in background."
EOF
chmod +x start.sh

# 3. Синхронизация кода, бинарного файла и скрипта запуска
echo "📦 Syncing files to server..."
rsync -avz -e "ssh -p $REMOTE_PORT -i $REMOTE_KEY" \
    --exclude '.git' \
    --exclude '.env' \
    --exclude '.idea' \
    --exclude 'client' \
    ./ $REMOTE_USER@$REMOTE_HOST:$REMOTE_DIR/

# 4. Удаленный запуск
echo "🔄 Triggering remote restart..."
# Выполняем скрипт и сразу отключаемся
ssh -p $REMOTE_PORT -i $REMOTE_KEY $REMOTE_USER@$REMOTE_HOST \
    "bash $REMOTE_DIR/start.sh"

# Удаляем временные локальные файлы
rm lavender-server
rm start.sh

echo "✅ Deployment completed!"
echo "📝 Control returned to local console."
echo "📝 You can monitor logs on server: tail -f $REMOTE_DIR/logs.txt"

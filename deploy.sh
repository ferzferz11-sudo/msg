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
# МЫ ИЗМЕНИЛИ ИМЯ ВЫХОДНОГО ФАЙЛА НА lavender-server-new
GOOS=linux GOARCH=amd64 go build -o lavender-server-new .
if [ $? -ne 0 ]; then
    echo "❌ Local build failed! Check your Go code."
    exit 1
fi

# 2. Создание БЕЗОПАСНОГО серверного скрипта запуска
cat << 'EOF' > start.sh
#!/bin/bash
cd "$(dirname "$0")"

echo "🔄 Starting health check for the new version..."

# 1. Жестко убиваем процесс, занимающий порт 50051
echo "Killing process on port 50051..."
lsof -ti:50051 | xargs kill -9 2>/dev/null
pkill lavender-server || true

# 2. Запускаем новую версию в фоне на временном логе
nohup ./lavender-server-new > logs_new.txt 2>&1 &
NEW_PID=$!

# Даем серверу 3 секунды на инициализацию
sleep 3

# 3. Проверяем, жив ли процесс
if ps -p $NEW_PID > /dev/null; then
    echo "✅ New version started successfully!"

    # Ротируем логи
    cat logs_new.txt >> logs.txt

    # Подменяем рабочий бинарник
    mv lavender-server-new lavender-server

    # Перезапускаем его уже под правильным именем
    lsof -ti:50051 | xargs kill -9 2>/dev/null
    nohup ./lavender-server >> logs.txt 2>&1 &

    echo "🚀 Server updated and running in background."
    rm -f logs_new.txt
else
    echo "❌ Error: New version failed to start!"
    echo "📝 Checking why it failed (last 20 lines of logs_new.txt):"
    tail -n 10 logs_new.txt

    # Роллбэк: если новый сервер упал, возвращаем старую версию
    if [ -f lavender-server ]; then
        echo "🔄 Rolling back to the previous working version..."
        nohup ./lavender-server >> logs.txt 2>&1 &
    fi

    rm -f lavender-server-new
    exit 1
fi
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
ssh -p $REMOTE_PORT -i $REMOTE_KEY $REMOTE_USER@$REMOTE_HOST \
    "bash $REMOTE_DIR/start.sh"

# Удаляем временные локальные файлы на Mac
rm lavender-server-new
rm start.sh

echo "✅ Deployment completed!"
echo "📝 Control returned to local console."
echo "📝 You can monitor logs on server: tail -f $REMOTE_DIR/logs.txt"

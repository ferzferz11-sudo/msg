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

    # Очищаем старые логи, оставляем только последние 30 строк
    if [ -f logs.txt ]; then
        tail -n 10000 logs.txt > logs.txt.tmp && mv logs.txt.tmp logs.txt
    fi

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

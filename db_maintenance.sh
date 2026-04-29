#!/bin/bash

# Lavender Messenger Database Maintenance Script (Run on Local Mac)
# This script deploys and executes a cleanup script on the remote server.

REMOTE_USER="ferz"
REMOTE_HOST="159.195.38.145"
REMOTE_PORT="31703"
REMOTE_KEY="$HOME/.ssh/ferzz@x-cart.com"
REMOTE_DIR="/home/ferz/LavenderMessenger"

echo "🚀 Starting database maintenance on $REMOTE_HOST..."

# Use a Here-doc to execute commands remotely
ssh -p $REMOTE_PORT -i $REMOTE_KEY $REMOTE_USER@$REMOTE_HOST << 'REMOTE_EXEC'
cd /home/ferz/LavenderMessenger

# Load database URL from .env if it exists
if [ -f .env ]; then
    export $(grep -v '^#' .env | xargs)
fi

if [ -z "$DATABASE_URL" ]; then
    echo "❌ Error: DATABASE_URL not found in environment or .env file."
    exit 1
fi

export LC_ALL=C
export LANG=C

echo "🔍 Running database maintenance tasks..."

# Use psql to run maintenance queries
psql "$DATABASE_URL" << 'SQL'
\set ON_ERROR_STOP on

-- 1. Удаление сообщений со сбитой расшифровкой (DECRYPTION_FAILED).
-- Сюда мы вручную или пакетным апдейтом помещаем битые ID.
\echo 'Удаление сообщений со сбитой расшифровкой (DECRYPTION_FAILED)...'
DELETE FROM messages
WHERE encrypted_text = 'DECRYPTION_FAILED'::bytea;

-- 2. Очистка остаточных сильно поврежденных записей (меньше 4 байт или NULL).
-- Помогает держать базу в консистентном состоянии, если бэкенд запишет пустую строку.
\echo 'Очистка пустых и поврежденных записей...'
DELETE FROM messages
WHERE (octet_length(encrypted_text) < 4 OR encrypted_text IS NULL);

-- 3. Быстрый анализ базы (НЕ блокирует таблицу)
-- Мы убрали тяжелый VACUUM, оставив только актуализацию статистики для планировщика БД.
\echo 'Актуализация статистики базы данных...'
ANALYZE messages;
SQL

if [ $? -eq 0 ]; then
    echo "✅ Database maintenance completed successfully!"
else
    echo "❌ Database maintenance failed!"
    exit 1
fi
REMOTE_EXEC

echo "✅ All tasks completed!"

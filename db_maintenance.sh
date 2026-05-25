#!/bin/bash

REMOTE_USER="ferz"
REMOTE_HOST="159.195.38.145"
REMOTE_PORT="31703"
REMOTE_KEY="$HOME/.ssh/ferzz@x-cart.com"

echo "🚀 Starting database maintenance V7 (WIPE OLD CALLS) on $REMOTE_HOST..."

ssh -p $REMOTE_PORT -i $REMOTE_KEY $REMOTE_USER@$REMOTE_HOST << 'REMOTE_EXEC'
export LC_ALL=C
export LANG=C
export LANGUAGE=C

cd /home/ferz/LavenderMessenger
if [ -f .env ]; then export $(grep -v '^#' .env | xargs); fi

if [ -z "$DATABASE_URL" ]; then
    echo "❌ Error: DATABASE_URL not found."
    exit 1
fi

psql "$DATABASE_URL" << 'SQL'
\set ON_ERROR_STOP on

-- 1. Удаление ВСЕХ старых сообщений о звонках (по иконкам и тексту)
\echo 'Полная очистка сообщений о звонках в чатах...'
DELETE FROM messages
WHERE encrypted_text LIKE '%📹%'
   OR encrypted_text LIKE '%📞%'
   OR encrypted_text LIKE '%Видеозвонок%'
   OR username = 'SYSTEM';

-- 2. Очистка самой таблицы звонков для чистого старта
\echo 'Очистка истории системных звонков...'
DELETE FROM calls;

-- 3. Синхронизация времени для всех остальных обычных сообщений
\echo 'Финальная коррекция времени для обычных сообщений...'
UPDATE messages SET created_at = created_at - INTERVAL '3 hours'
WHERE created_at > NOW() + INTERVAL '1 minute';

SQL
REMOTE_EXEC

echo "✅ ALL OLD CALLS WIPED! Please clear local cache in the app settings (Additional Settings -> Clear local cache)."

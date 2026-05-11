#!/bin/bash

# Скрипт для проверки конкретного сообщения и его реакций в базе данных Lavender Messenger

# Загружаем переменные окружения из .env файла
if [ -f .env ]; then
  source .env
fi

if [ -z "$DATABASE_URL" ]; then
  echo "Ошибка: DATABASE_URL не установлена. Пожалуйста, убедитесь, что она есть в .env файле."
  exit 1
fi

MESSAGE_ID=$1

if [ -z "$MESSAGE_ID" ]; then
  echo "Использование: ./check_message.sh <message_id>"
  echo "Пример: ./check_message.sh c0856390-8246-46f2-b9eb-bd4c5d9ed7fa"
  exit 1
fi

echo "======================================================"
echo "Данные для сообщения ID: $MESSAGE_ID"
echo "======================================================"

# Выполняем SQL запрос через psql
psql "$DATABASE_URL" -c "
SELECT
    m.message_id,
    m.username as sender,
    m.room_id,
    m.created_at,
    substring(encode(m.encrypted_text, 'hex') from 1 for 20) || '...' as encrypted_preview,
    COALESCE(
        json_agg(
            json_build_object('user', r.username, 'emoji', r.emoji)
        ) FILTER (WHERE r.id IS NOT NULL),
        '[]'
    ) as reactions
FROM messages m
LEFT JOIN reactions r ON m.message_id = r.message_id
WHERE m.message_id = '$MESSAGE_ID'
GROUP BY
    m.id, m.message_id, m.username, m.room_id, m.created_at, m.encrypted_text;
"

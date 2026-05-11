#!/bin/bash

# Скрипт для получения последних 10 сообщений с их реакциями из базы данных Lavender Messenger

# Загружаем переменные окружения из .env файла
if [ -f .env ]; then
  source .env
fi

if [ -z "$DATABASE_URL" ]; then
  echo "Ошибка: DATABASE_URL не установлена. Пожалуйста, убедитесь, что она есть в .env файле."
  exit 1
fi

echo "======================================================"
echo "Последние 10 сообщений в базе данных Lavender Messenger"
echo "======================================================"

# Выполняем SQL запрос через psql
# Для расшифровки текста требуется ключ, поэтому выводим зашифрованные данные как hex
# В реальной системе текст зашифрован, но мы можем проверить связи и реакции

psql "$DATABASE_URL" -c "
WITH last_msgs AS (
    SELECT
        m.id,
        m.message_id,
        m.username,
        m.room_id,
        m.created_at,
        substring(encode(m.encrypted_text, 'hex') from 1 for 20) || '...' as encrypted_preview
    FROM messages m
    ORDER BY m.created_at DESC
    LIMIT 10
)
SELECT
    lm.message_id,
    lm.username as sender,
    lm.room_id,
    lm.created_at,
    COALESCE(
        json_agg(
            json_build_object('user', r.username, 'emoji', r.emoji)
        ) FILTER (WHERE r.id IS NOT NULL),
        '[]'
    ) as reactions
FROM last_msgs lm
LEFT JOIN reactions r ON lm.message_id = r.message_id
GROUP BY
    lm.id, lm.message_id, lm.username, lm.room_id, lm.created_at, lm.encrypted_preview
ORDER BY
    lm.created_at DESC;
"

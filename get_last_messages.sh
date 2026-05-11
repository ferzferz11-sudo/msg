#!/bin/bash

# Скрипт для получения последних 10 сообщений с самыми свежими реакциями

# Загружаем переменные окружения из .env файла
if [ -f .env ]; then
  source .env
fi

if [ -z "$DATABASE_URL" ]; then
  echo "Ошибка: DATABASE_URL не установлена. Пожалуйста, убедитесь, что она есть в .env файле."
  exit 1
fi

echo "======================================================================="
echo "Последние 10 сообщений с самыми свежими реакциями"
echo "======================================================================="

psql "$DATABASE_URL" -c "
WITH latest_reactions AS (
    SELECT
        message_id,
        MAX(id) as last_reaction_id
    FROM reactions
    GROUP BY message_id
),
ranked_messages AS (
    SELECT
        m.id,
        m.message_id,
        m.username,
        m.room_id,
        m.created_at,
        lr.last_reaction_id,
        substring(encode(m.encrypted_text, 'hex') from 1 for 20) || '...' as encrypted_preview
    FROM messages m
    LEFT JOIN latest_reactions lr ON m.message_id = lr.message_id
    ORDER BY lr.last_reaction_id DESC NULLS LAST, m.created_at DESC
    LIMIT 10
)
SELECT
    rm.message_id,
    rm.username as sender,
    rm.room_id,
    rm.created_at,
    COALESCE(
        json_agg(
            json_build_object('user', r.username, 'emoji', r.emoji)
        ) FILTER (WHERE r.id IS NOT NULL),
        '[]'
    ) as reactions
FROM ranked_messages rm
LEFT JOIN reactions r ON rm.message_id = r.message_id
GROUP BY
    rm.id, rm.message_id, rm.username, rm.room_id, rm.created_at, rm.encrypted_preview, rm.last_reaction_id
ORDER BY
    rm.last_reaction_id DESC NULLS LAST, rm.created_at DESC;
"

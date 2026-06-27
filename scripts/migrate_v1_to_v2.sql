-- Migration: v1 messages → v2 messages_v2
-- Run BEFORE deploying v2-only server
-- Safe: INSERT ... ON CONFLICT DO NOTHING (idempotent)

-- 1. Migrate v1 messages to messages_v2
INSERT INTO messages_v2 (id, room_id, sender_id, content_type, text, media_url, media_urls, duration, reply_to_id, reply_preview, edited, is_read, is_e2ee, e2ee_payload, reactions, created_at)
SELECT
    m.message_id,
    m.room_id,
    COALESCE(
        (SELECT u.id::text FROM users u WHERE u.username = m.username LIMIT 1),
        m.user_id::text,
        '00000000-0000-0000-0000-000000000000'
    )::uuid AS sender_id,
    CASE
        WHEN m.image_url != '' AND m.image_url IS NOT NULL THEN 'image'
        WHEN m.voice_url IS NOT NULL AND m.voice_url != '' THEN 'voice'
        ELSE 'text'
    END AS content_type,
    COALESCE(m.encrypted_text::text, '') AS text,
    COALESCE(m.image_url, '') AS media_url,
    COALESCE(m.image_urls, '[]')::jsonb AS media_urls,
    COALESCE(m.duration, 0) AS duration,
    m.replied_to_message_id AS reply_to_id,
    LEFT(COALESCE(m.replied_to_text, ''), 100) AS reply_preview,
    COALESCE(m.edited, FALSE) AS edited,
    COALESCE(m.is_read, FALSE) AS is_read,
    COALESCE(m.is_e2ee, FALSE) AS is_e2ee,
    NULL AS e2ee_payload,
    '{}'::jsonb AS reactions,
    m.created_at
FROM messages m
WHERE NOT EXISTS (
    SELECT 1 FROM messages_v2 mv WHERE mv.id = m.message_id
);

-- 2. Migrate reactions from v1 reactions table → messages_v2.reactions JSONB
UPDATE messages_v2 mv
SET reactions = (
    SELECT COALESCE(jsonb_object_agg(r.username, r.emoji), '{}'::jsonb)
    FROM reactions r
    WHERE r.message_id = mv.id
)
WHERE EXISTS (
    SELECT 1 FROM reactions r WHERE r.message_id = mv.id
)
AND mv.reactions = '{}'::jsonb;

-- 3. Verify migration counts
SELECT 'v1 messages' AS source, COUNT(*) AS count FROM messages
UNION ALL
SELECT 'v2 messages_v2' AS source, COUNT(*) AS count FROM messages_v2
UNION ALL
SELECT 'v1 reactions rows' AS source, COUNT(*) AS count FROM reactions;

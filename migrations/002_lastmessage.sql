-- Phase 2: Last message columns in chats table
-- Run: psql -U postgres -d chat_db_dev -f migrations/002_lastmessage.sql

-- 1. Add missing columns
DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='chats' AND column_name='last_message_username') THEN
        ALTER TABLE chats ADD COLUMN last_message_username VARCHAR(255) DEFAULT '';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='chats' AND column_name='last_message_has_image') THEN
        ALTER TABLE chats ADD COLUMN last_message_has_image BOOLEAN DEFAULT FALSE;
    END IF;
END $$;

-- 2. Backfill from messages for chats that have messages
UPDATE chats c SET
    last_message_text = COALESCE(
        (SELECT LEFT(decrypt(m.encrypted_text), 500) FROM messages m WHERE m.room_id = c.id ORDER BY m.created_at DESC LIMIT 1),
        ''
    ),
    last_message_time = (
        SELECT m.created_at FROM messages m WHERE m.room_id = c.id ORDER BY m.created_at DESC LIMIT 1
    ),
    last_message_username = COALESCE(
        (SELECT m.username FROM messages m WHERE m.room_id = c.id ORDER BY m.created_at DESC LIMIT 1),
        ''
    ),
    last_message_has_image = COALESCE(
        (SELECT (COALESCE(m.image_url, '') != '' OR COALESCE(m.image_urls, '[]') != '[]')
         FROM messages m WHERE m.room_id = c.id ORDER BY m.created_at DESC LIMIT 1),
        FALSE
    )
WHERE EXISTS (SELECT 1 FROM messages m WHERE m.room_id = c.id)
  AND c.type NOT IN ('owl', 'hermes');

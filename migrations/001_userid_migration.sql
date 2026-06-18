-- Phase 1: Add UUID columns to tables that still use username as PK
-- Run: psql -U postgres -d chat_db_dev -f migrations/001_userid_migration.sql
-- Safe: all operations are IF NOT EXISTS / additive only

-- ============================================
-- 1. REACTIONS — add user_id UUID column
-- ============================================
DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='reactions' AND column_name='user_id') THEN
        ALTER TABLE reactions ADD COLUMN user_id UUID;
        UPDATE reactions r SET user_id = (SELECT id FROM users u WHERE u.username = r.username);
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_reactions_user_id ON reactions(user_id);

-- ============================================
-- 2. CONTACTS — add user_id and contact_user_id UUID columns
-- ============================================
DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='contacts' AND column_name='user_id') THEN
        ALTER TABLE contacts ADD COLUMN user_id UUID;
        UPDATE contacts c SET user_id = (SELECT id FROM users u WHERE u.username = c.username);
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='contacts' AND column_name='contact_user_id') THEN
        ALTER TABLE contacts ADD COLUMN contact_user_id UUID;
        UPDATE contacts c SET contact_user_id = (SELECT id FROM users u WHERE u.username = c.contact_username);
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_contacts_user_id ON contacts(user_id);
CREATE INDEX IF NOT EXISTS idx_contacts_contact_user_id ON contacts(contact_user_id);

-- ============================================
-- 3. USER_TOKENS — add user_id UUID column
-- ============================================
DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='user_tokens' AND column_name='user_id') THEN
        ALTER TABLE user_tokens ADD COLUMN user_id UUID;
        UPDATE user_tokens ut SET user_id = (SELECT id FROM users u WHERE u.username = ut.username);
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_user_tokens_user_id ON user_tokens(user_id);

-- ============================================
-- 4. USER_THEMES — add user_id UUID column
-- ============================================
DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='user_themes' AND column_name='user_id') THEN
        ALTER TABLE user_themes ADD COLUMN user_id UUID;
        UPDATE user_themes ut SET user_id = (SELECT id FROM users u WHERE u.username = ut.username);
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_user_themes_user_id ON user_themes(user_id);

-- ============================================
-- 5. CHATS — add participant_ids UUID[] column
-- ============================================
DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='chats' AND column_name='participant_ids') THEN
        ALTER TABLE chats ADD COLUMN participant_ids UUID[];
        UPDATE chats SET participant_ids = (
            SELECT array_agg(u.id ORDER BY u.username)
            FROM users u
            WHERE u.username = ANY(
                SELECT json_array_elements_text(chats.participants::json)
            )
        );
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_chats_participant_ids ON chats USING GIN(participant_ids);

-- ============================================
-- 6. MUTED_CHATS — add user_id index (column already exists)
-- ============================================
DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='muted_chats' AND column_name='user_id') THEN
        -- Fill any NULL user_ids
        UPDATE muted_chats mc SET user_id = (SELECT id FROM users u WHERE u.username = mc.username) WHERE mc.user_id IS NULL;
        CREATE INDEX IF NOT EXISTS idx_muted_chats_user_id ON muted_chats(user_id);
    END IF;
END $$;

-- ============================================
-- 7. DRAFT_MESSAGES — fill user_id (column already exists)
-- ============================================
UPDATE draft_messages dm SET user_id = (SELECT id FROM users u WHERE u.username = dm.username) WHERE dm.user_id IS NULL;

-- ============================================
-- 8. MESSAGES — fill user_id (column already exists)
-- ============================================
UPDATE messages m SET user_id = (SELECT id FROM users u WHERE u.username = m.username) WHERE m.user_id IS NULL;
CREATE INDEX IF NOT EXISTS idx_messages_user_id ON messages(user_id);

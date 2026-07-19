package main

import "database/sql"

var coreMigrations = []string{
	// --- Auth ---
	`CREATE TABLE IF NOT EXISTS user_devices (device_id VARCHAR(255) PRIMARY KEY, user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE, device_name VARCHAR(255), client_version VARCHAR(50), last_seen_at TIMESTAMP NOT NULL DEFAULT NOW(), ip_address VARCHAR(255))`,
	`CREATE TABLE IF NOT EXISTS password_reset_tokens (token VARCHAR(255) PRIMARY KEY, user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE, expires_at TIMESTAMP NOT NULL, created_at TIMESTAMP NOT NULL DEFAULT NOW())`,

	// --- Users ---
	`CREATE TABLE IF NOT EXISTS users (id UUID PRIMARY KEY DEFAULT gen_random_uuid(), username VARCHAR(255) UNIQUE NOT NULL, password_hash VARCHAR(255) NOT NULL, created_at TIMESTAMP NOT NULL DEFAULT NOW())`,
	`DO $$ BEGIN
		IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='users' AND column_name='avatar_url') THEN ALTER TABLE users ADD COLUMN avatar_url VARCHAR(512); END IF;
		IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='users' AND column_name='full_avatar_url') THEN ALTER TABLE users ADD COLUMN full_avatar_url VARCHAR(512); END IF;
		IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='users' AND column_name='bio') THEN ALTER TABLE users ADD COLUMN bio TEXT; END IF;
		IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='users' AND column_name='status') THEN ALTER TABLE users ADD COLUMN status VARCHAR(255); END IF;
		IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='users' AND column_name='chat_list_version') THEN ALTER TABLE users ADD COLUMN chat_list_version BIGINT DEFAULT 0; END IF;
		IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='users' AND column_name='is_super_admin') THEN ALTER TABLE users ADD COLUMN is_super_admin BOOLEAN DEFAULT FALSE; END IF;
		IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='users' AND column_name='email') THEN ALTER TABLE users ADD COLUMN email VARCHAR(255); END IF;
		IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='users' AND column_name='last_seen_at') THEN ALTER TABLE users ADD COLUMN last_seen_at TIMESTAMP; END IF;
		IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='users' AND column_name='last_client_version') THEN ALTER TABLE users ADD COLUMN last_client_version VARCHAR(50); END IF;
	END $$;`,

	// --- Messages v2 ---
	`CREATE TABLE IF NOT EXISTS messages_v2 (
		id              VARCHAR(255) PRIMARY KEY,
		room_id         VARCHAR(255) NOT NULL,
		sender_id       UUID NOT NULL REFERENCES users(id),
		content_type    VARCHAR(20) NOT NULL,
		text            TEXT DEFAULT '',
		media_url       VARCHAR(512) DEFAULT '',
		media_urls      JSONB DEFAULT '[]',
		duration        INT DEFAULT 0,
		reply_to_id     VARCHAR(255) DEFAULT NULL,
		reply_preview   TEXT DEFAULT '',
		edited          BOOLEAN DEFAULT FALSE,
		is_read         BOOLEAN DEFAULT FALSE,
		is_e2ee         BOOLEAN DEFAULT FALSE,
		e2ee_payload    BYTEA DEFAULT NULL,
		reactions       JSONB DEFAULT '{}',
		created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	`CREATE INDEX IF NOT EXISTS idx_messages_v2_room_cursor ON messages_v2(room_id, created_at DESC, id DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_messages_v2_reply_to ON messages_v2(reply_to_id) WHERE reply_to_id IS NOT NULL`,
	`CREATE INDEX IF NOT EXISTS idx_messages_v2_sender ON messages_v2(sender_id)`,
	`DO $$ BEGIN
		IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='messages_v2' AND column_name='reply_preview') THEN ALTER TABLE messages_v2 ADD COLUMN reply_preview TEXT DEFAULT ''; END IF;
	END $$;`,
	`DO $$ BEGIN
		IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='messages_v2' AND column_name='mentions') THEN ALTER TABLE messages_v2 ADD COLUMN mentions TEXT DEFAULT '[]'; END IF;
	END $$;`,
	`DO $$ BEGIN
		IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='messages_v2' AND column_name='reply_sender_id') THEN ALTER TABLE messages_v2 ADD COLUMN reply_sender_id VARCHAR(255) DEFAULT ''; END IF;
	END $$;`,

	// --- Chats ---
	`CREATE TABLE IF NOT EXISTS chats (id VARCHAR(255) PRIMARY KEY, name VARCHAR(255) NOT NULL, type VARCHAR(50) NOT NULL, participants TEXT NOT NULL, creator_username VARCHAR(255), created_at TIMESTAMP NOT NULL DEFAULT NOW(), avatar_url TEXT DEFAULT '', full_avatar_url TEXT DEFAULT '')`,
	`DO $$ BEGIN
		IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='chats' AND column_name='creator_id') THEN ALTER TABLE chats ADD COLUMN creator_id UUID; END IF;
		IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='chats' AND column_name='allow_members_to_add') THEN ALTER TABLE chats ADD COLUMN allow_members_to_add BOOLEAN DEFAULT FALSE; END IF;
		IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='chats' AND column_name='is_secret') THEN ALTER TABLE chats ADD COLUMN is_secret BOOLEAN DEFAULT FALSE; END IF;
		IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='chats' AND column_name='public_key_a') THEN ALTER TABLE chats ADD COLUMN public_key_a TEXT DEFAULT ''; END IF;
		IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='chats' AND column_name='public_key_b') THEN ALTER TABLE chats ADD COLUMN public_key_b TEXT DEFAULT ''; END IF;
		IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='chats' AND column_name='e2ee_ready') THEN ALTER TABLE chats ADD COLUMN e2ee_ready BOOLEAN DEFAULT FALSE; END IF;
	END $$;`,

	// --- Contacts ---
	`CREATE TABLE IF NOT EXISTS contacts (id SERIAL PRIMARY KEY, username VARCHAR(255) NOT NULL, contact_username VARCHAR(255) NOT NULL, user_id UUID, contact_user_id UUID, created_at TIMESTAMP NOT NULL DEFAULT NOW(), UNIQUE(username, contact_username))`,

	// --- User Tokens (Push) ---
	`CREATE TABLE IF NOT EXISTS user_tokens (username VARCHAR(255) PRIMARY KEY, fcm_token TEXT, push_enabled BOOLEAN DEFAULT TRUE, updated_at TIMESTAMP, user_id UUID)`,

	// --- User Themes ---
	`CREATE TABLE IF NOT EXISTS user_themes (username VARCHAR(255) NOT NULL, theme_id VARCHAR(255) NOT NULL, name VARCHAR(255), primary_color VARCHAR(255), on_primary_color VARCHAR(255), surface_color VARCHAR(255), on_surface_color VARCHAR(255), background_color VARCHAR(255), text_primary_color VARCHAR(255), text_secondary_color VARCHAR(255), is_dark BOOLEAN, chat_background_image_url TEXT, chat_list_background_image_url TEXT, bottom_panel_color VARCHAR(255), on_bottom_panel_color VARCHAR(255), surface_container VARCHAR(255), outgoing_bubble_color VARCHAR(255), incoming_bubble_color VARCHAR(255), user_id UUID, UNIQUE(username, theme_id))`,

	// --- User Chat Metadata ---
	`CREATE TABLE IF NOT EXISTS user_chat_metadata (username VARCHAR(255) NOT NULL, room_id VARCHAR(255) NOT NULL, last_read_at TIMESTAMP, pinned BOOLEAN DEFAULT FALSE, UNIQUE(username, room_id))`,

	// --- Muted Chats ---
	`CREATE TABLE IF NOT EXISTS muted_chats (username VARCHAR(255) NOT NULL, room_id VARCHAR(255) NOT NULL, muted BOOLEAN DEFAULT TRUE, UNIQUE(username, room_id))`,

	// --- Draft Messages ---
	`CREATE TABLE IF NOT EXISTS draft_messages (username VARCHAR(255) NOT NULL, room_id VARCHAR(255) NOT NULL, draft_text TEXT, replied_to_message_id VARCHAR(255), replied_to_user VARCHAR(255), replied_to_text TEXT, updated_at TIMESTAMP DEFAULT NOW(), user_id UUID, UNIQUE(username, room_id))`,

	// --- Servers ---
	`CREATE TABLE IF NOT EXISTS servers (id VARCHAR(255) PRIMARY KEY DEFAULT gen_random_uuid()::text, name VARCHAR(255) NOT NULL, host VARCHAR(255) NOT NULL, port INTEGER NOT NULL, is_default BOOLEAN DEFAULT FALSE, is_protected BOOLEAN DEFAULT FALSE, created_at TIMESTAMP NOT NULL DEFAULT NOW())`,

	// --- Calls ---
	`CREATE TABLE IF NOT EXISTS calls (id VARCHAR(255) PRIMARY KEY DEFAULT gen_random_uuid()::text, caller_id UUID NOT NULL, receiver_id UUID NOT NULL, type VARCHAR(50) NOT NULL, room_id VARCHAR(255), status VARCHAR(50) DEFAULT 'pending', started_at TIMESTAMP, ended_at TIMESTAMP, created_at TIMESTAMP NOT NULL DEFAULT NOW())`,

	// --- Secret Chat Keys ---
	`CREATE TABLE IF NOT EXISTS secret_chat_keys (chat_id VARCHAR(255) NOT NULL, user_id UUID NOT NULL, public_key TEXT NOT NULL, created_at TIMESTAMP NOT NULL DEFAULT NOW(), PRIMARY KEY (chat_id, user_id))`,

	// --- Free OpenRouter Models ---
	`CREATE TABLE IF NOT EXISTS free_openrouter_models (id SERIAL PRIMARY KEY, model_id VARCHAR(255) UNIQUE NOT NULL, display_name VARCHAR(255) NOT NULL, is_active BOOLEAN DEFAULT TRUE, sort_order INTEGER DEFAULT 0)`,

	// --- Pinned Messages ---
	`CREATE TABLE IF NOT EXISTS pinned_messages (id SERIAL PRIMARY KEY, message_id VARCHAR(255) UNIQUE NOT NULL, room_id VARCHAR(255) NOT NULL, username VARCHAR(255) NOT NULL, pinned_by VARCHAR(255) NOT NULL, pinned_at TIMESTAMP NOT NULL DEFAULT NOW())`,

	// --- Favorites ---
	`CREATE TABLE IF NOT EXISTS favorites (id SERIAL PRIMARY KEY, user_id UUID NOT NULL, message_id VARCHAR(255) NOT NULL, created_at TIMESTAMP NOT NULL DEFAULT NOW(), UNIQUE(user_id, message_id))`,

	// --- Chat List v2 ---
	`CREATE TABLE IF NOT EXISTS chat_list_v2 (id SERIAL PRIMARY KEY, user_id UUID NOT NULL, chat_id VARCHAR(255) NOT NULL, unread_count INTEGER DEFAULT 0, last_message_preview TEXT, last_message_at TIMESTAMP, is_pinned BOOLEAN DEFAULT FALSE, is_muted BOOLEAN DEFAULT FALSE, UNIQUE(user_id, chat_id))`,

	// --- Company System ---
	`CREATE TABLE IF NOT EXISTS companies (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		name VARCHAR(255) NOT NULL,
		owner_id UUID NOT NULL REFERENCES users(id),
		avatar_url TEXT DEFAULT '',
		created_at TIMESTAMP NOT NULL DEFAULT NOW()
	)`,
	`CREATE TABLE IF NOT EXISTS company_positions (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
		title VARCHAR(255) NOT NULL,
		level INT NOT NULL DEFAULT 0,
		chat_access VARCHAR(50) DEFAULT 'member',
		created_at TIMESTAMP NOT NULL DEFAULT NOW(),
		UNIQUE(company_id, title)
	)`,
	`CREATE TABLE IF NOT EXISTS company_members (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
		user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		position_id UUID NOT NULL REFERENCES company_positions(id),
		joined_at TIMESTAMP NOT NULL DEFAULT NOW(),
		UNIQUE(company_id, user_id)
	)`,
	`DO $$ BEGIN
		IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='users' AND column_name='primary_company_id') THEN ALTER TABLE users ADD COLUMN primary_company_id UUID REFERENCES companies(id); END IF;
	END $$;`,
	`CREATE TABLE IF NOT EXISTS company_chats (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		chat_id VARCHAR(255) NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
		company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
		access_level VARCHAR(50) DEFAULT 'member',
		min_position_level INT DEFAULT 0,
		created_at TIMESTAMP NOT NULL DEFAULT NOW(),
		UNIQUE(chat_id)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_company_members_user ON company_members(user_id)`,
	`CREATE INDEX IF NOT EXISTS idx_company_members_company ON company_members(company_id)`,
	`CREATE INDEX IF NOT EXISTS idx_company_chats_chat ON company_chats(chat_id)`,
	`CREATE INDEX IF NOT EXISTS idx_company_chats_company ON company_chats(company_id)`,
	`CREATE INDEX IF NOT EXISTS idx_company_positions_company ON company_positions(company_id)`,

	// --- Sticker System ---
	`CREATE TABLE IF NOT EXISTS sticker_packs (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		title VARCHAR(255) NOT NULL,
		name VARCHAR(255) NOT NULL,
		creator_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		cover_sticker_id VARCHAR(255) DEFAULT '',
		status VARCHAR(20) DEFAULT 'draft',
		rejection_reason TEXT DEFAULT '',
		is_featured BOOLEAN DEFAULT FALSE,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	`CREATE INDEX IF NOT EXISTS idx_sticker_packs_creator ON sticker_packs(creator_user_id)`,
	`CREATE INDEX IF NOT EXISTS idx_sticker_packs_status ON sticker_packs(status)`,
	`CREATE TABLE IF NOT EXISTS stickers (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		pack_id UUID NOT NULL REFERENCES sticker_packs(id) ON DELETE CASCADE,
		lottie_url TEXT NOT NULL,
		thumbnail_url TEXT DEFAULT '',
		emoji VARCHAR(20) DEFAULT '',
		width INT DEFAULT 512,
		height INT DEFAULT 512,
		sort_order INT DEFAULT 0,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	`CREATE INDEX IF NOT EXISTS idx_stickers_pack ON stickers(pack_id)`,

	// --- Company Settings ---
	`CREATE TABLE IF NOT EXISTS company_settings (
		company_id UUID PRIMARY KEY REFERENCES companies(id) ON DELETE CASCADE,
		invite_only BOOLEAN DEFAULT FALSE,
		default_position_id UUID,
		allow_member_invite BOOLEAN DEFAULT FALSE,
		chat_access VARCHAR(20) DEFAULT 'member',
		require_approval BOOLEAN DEFAULT FALSE,
		updated_at TIMESTAMPTZ DEFAULT NOW()
	)`,

	// --- Company Invite Codes ---
	`CREATE TABLE IF NOT EXISTS company_invite_codes (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
		code VARCHAR(12) NOT NULL UNIQUE,
		created_by UUID NOT NULL REFERENCES users(id),
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		expires_at TIMESTAMPTZ,
		max_uses INT DEFAULT 1,
		use_count INT DEFAULT 0,
		is_active BOOLEAN DEFAULT TRUE
	)`,
	`CREATE INDEX IF NOT EXISTS idx_invite_codes_code ON company_invite_codes(code)`,
	`CREATE INDEX IF NOT EXISTS idx_invite_codes_company ON company_invite_codes(company_id)`,

	// --- Fix invalid UTF-8 in existing messages ---
	`DO $$ BEGIN
		UPDATE messages_v2 SET text = convert_to(convert_from(text::bytea, 'UTF8'), 'UTF8') WHERE text IS NOT NULL AND NOT octet_length(text) = octet_length(convert_to(text, 'UTF8'));
		UPDATE messages_v2 SET reply_preview = convert_to(convert_from(reply_preview::bytea, 'UTF8'), 'UTF8') WHERE reply_preview IS NOT NULL AND NOT octet_length(reply_preview) = octet_length(convert_to(reply_preview, 'UTF8'));
		UPDATE messages_v2 SET media_url = convert_to(convert_from(media_url::bytea, 'UTF8'), 'UTF8') WHERE media_url IS NOT NULL AND NOT octet_length(media_url) = octet_length(convert_to(media_url, 'UTF8'));
	EXCEPTION WHEN OTHERS THEN
		RAISE NOTICE 'UTF8 cleanup migration skipped: %', SQLERRM;
	END $$;`,
}

func runCoreMigrations(db *sql.DB) error {
	for _, q := range coreMigrations {
		if _, err := db.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

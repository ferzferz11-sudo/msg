// Lavender Messenger - A secure messaging application
// Author: Pavel Davydov (ferz)

package main

import (
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "github.com/lib/pq"
)

type DB struct {
	*sql.DB
}

type MessageRow struct {
	MessageID, Username                                      string
	Encrypted                                                []byte
	CreatedAt                                                time.Time
	RepliedToMessageID, RepliedToUser, RepliedToText, RoomID string
	IsRead                                                   bool
	AvatarURL, ImageURL, ImageURLs                           string
	Edited                                                   bool
	VoiceURL                                                 string
	Duration                                                 int32
	IsE2EE                                                   bool
}

func ConnectDB() (*DB, error) {
	dbUrl := os.Getenv("DATABASE_URL")
	if dbUrl == "" {
		return nil, fmt.Errorf("DATABASE_URL is not set")
	}
	db, err := sql.Open("postgres", dbUrl)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(50)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(time.Hour)
	db.SetConnMaxIdleTime(5 * time.Minute)
	if err = db.Ping(); err != nil {
		db.Close()
		return nil, err
	}

	// ======= CORE MIGRATIONS: users, messages, chats, auth, chat list =======

	queries := []string{
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

		// --- Messages ---
		`CREATE TABLE IF NOT EXISTS messages (id SERIAL PRIMARY KEY, message_id VARCHAR(255) UNIQUE, username VARCHAR(255) NOT NULL, encrypted_text BYTEA NOT NULL, created_at TIMESTAMP NOT NULL)`,
		`DO $$ BEGIN
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='messages' AND column_name='room_id') THEN ALTER TABLE messages ADD COLUMN room_id VARCHAR(255) DEFAULT ''; END IF;
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='messages' AND column_name='is_read') THEN ALTER TABLE messages ADD COLUMN is_read BOOLEAN DEFAULT FALSE; END IF;
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='messages' AND column_name='image_url') THEN ALTER TABLE messages ADD COLUMN image_url VARCHAR(512) DEFAULT ''; END IF;
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='messages' AND column_name='image_urls') THEN ALTER TABLE messages ADD COLUMN image_urls TEXT DEFAULT '[]'; END IF;
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='messages' AND column_name='edited') THEN ALTER TABLE messages ADD COLUMN edited BOOLEAN DEFAULT FALSE; END IF;
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='messages' AND column_name='voice_url') THEN ALTER TABLE messages ADD COLUMN voice_url VARCHAR(512); END IF;
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='messages' AND column_name='duration') THEN ALTER TABLE messages ADD COLUMN duration INTEGER DEFAULT 0; END IF;
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='messages' AND column_name='replied_to_message_id') THEN ALTER TABLE messages ADD COLUMN replied_to_message_id VARCHAR(255); END IF;
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='messages' AND column_name='replied_to_user') THEN ALTER TABLE messages ADD COLUMN replied_to_user VARCHAR(255); END IF;
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='messages' AND column_name='replied_to_text') THEN ALTER TABLE messages ADD COLUMN replied_to_text TEXT; END IF;
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='messages' AND column_name='user_id') THEN ALTER TABLE messages ADD COLUMN user_id UUID; END IF;
		END $$;`,
		`CREATE INDEX IF NOT EXISTS idx_messages_room ON messages(room_id, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_messages_user_id ON messages(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_messages_username_time ON messages(username, created_at)`,

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

		// --- Reactions ---
		`CREATE TABLE IF NOT EXISTS reactions (message_id VARCHAR(255) NOT NULL, username VARCHAR(255) NOT NULL, emoji VARCHAR(255) NOT NULL, user_id UUID, created_at TIMESTAMP NOT NULL DEFAULT NOW(), UNIQUE(message_id, username))`,

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
	}

	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			db.Close()
			return nil, fmt.Errorf("migration failed: %w\nQuery: %s", err, q)
		}
	}

	// Bootstrap: set ferz as super_admin (one-time)
	{
		res, _ := db.Exec(`UPDATE users SET is_super_admin = TRUE WHERE username = 'ferz' AND (is_super_admin IS FALSE OR is_super_admin IS NULL)`)
		if res != nil {
			if n, _ := res.RowsAffected(); n > 0 {
				logger.Info("Bootstrap: set ferz as super_admin (one-time)")
			}
		}
	}

	// Hermes Orchestrator migrations
	runHermesMigrations(db)

	// ChatList v2 migrations
	MigrateChatListV2(db)
	MigratePinnedMessages(db)

	return &DB{db}, nil
}

func (db *DB) Close() error { return db.DB.Close() }

// Query — прокси к sql.DB.Query
func (db *DB) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return db.DB.Query(query, args...)
}

// QueryRow — прокси к sql.DB.QueryRow
func (db *DB) QueryRow(query string, args ...interface{}) *sql.Row {
	return db.DB.QueryRow(query, args...)
}

// Exec — прокси к sql.DB.Exec
func (db *DB) Exec(query string, args ...interface{}) (sql.Result, error) {
	return db.DB.Exec(query, args...)
}

type freeModel struct {
	ID          int    `json:"id"`
	ModelID     string `json:"model_id"`
	DisplayName string `json:"display_name"`
	IsActive    bool   `json:"is_active"`
	SortOrder   int    `json:"sort_order"`
}

func (db *DB) GetFreeModels() ([]freeModel, error) {
	rows, err := db.Query("SELECT id, model_id, display_name, is_active, sort_order FROM free_openrouter_models WHERE is_active = TRUE ORDER BY sort_order, display_name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var models []freeModel
	for rows.Next() {
		var m freeModel
		if err := rows.Scan(&m.ID, &m.ModelID, &m.DisplayName, &m.IsActive, &m.SortOrder); err == nil {
			models = append(models, m)
		}
	}
	return models, nil
}

func (db *DB) AddFreeModel(modelID, displayName string, sortOrder int) error {
	_, err := db.Exec(
		"INSERT INTO free_openrouter_models (model_id, display_name, sort_order) VALUES ($1, $2, $3) ON CONFLICT (model_id) DO UPDATE SET display_name=$2, sort_order=$3, is_active=TRUE",
		modelID, displayName, sortOrder,
	)
	return err
}

func (db *DB) RemoveFreeModel(modelID string) error {
	_, err := db.Exec("UPDATE free_openrouter_models SET is_active = FALSE WHERE model_id = $1", modelID)
	return err
}

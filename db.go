// Lavender Messenger - A secure messaging application
// Author: Pavel Davydov (ferz)

package main

import (
	"LavenderMessenger/gen"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

type DB struct {
	*sql.DB
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
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(15)
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

		// --- Chats ---
		`CREATE TABLE IF NOT EXISTS chats (id VARCHAR(255) PRIMARY KEY, name VARCHAR(255) NOT NULL, type VARCHAR(50) NOT NULL, participants TEXT NOT NULL, creator_username VARCHAR(255), created_at TIMESTAMP NOT NULL DEFAULT NOW(), avatar_url TEXT DEFAULT '', full_avatar_url TEXT DEFAULT '')`,
		`DO $$ BEGIN
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='chats' AND column_name='creator_id') THEN ALTER TABLE chats ADD COLUMN creator_id UUID; END IF;
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='chats' AND column_name='allow_members_to_add') THEN ALTER TABLE chats ADD COLUMN allow_members_to_add BOOLEAN DEFAULT FALSE; END IF;
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='chats' AND column_name='is_secret') THEN ALTER TABLE chats ADD COLUMN is_secret BOOLEAN DEFAULT FALSE; END IF;
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='chats' AND column_name='public_key_a') THEN ALTER TABLE chats ADD COLUMN public_key_a TEXT DEFAULT ''; END IF;
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='chats' AND column_name='public_key_b') THEN ALTER TABLE chats ADD COLUMN public_key_b TEXT DEFAULT ''; END IF;
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='chats' AND column_name='e2ee_ready') THEN ALTER TABLE chats ADD COLUMN e2ee_ready BOOLEAN DEFAULT FALSE; END IF;
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='chats' AND column_name='last_message_text') THEN ALTER TABLE chats ADD COLUMN last_message_text TEXT DEFAULT ''; END IF;
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='chats' AND column_name='last_message_time') THEN ALTER TABLE chats ADD COLUMN last_message_time TIMESTAMP; END IF;
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='chats' AND column_name='last_message_username') THEN ALTER TABLE chats ADD COLUMN last_message_username VARCHAR(255) DEFAULT ''; END IF;
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='chats' AND column_name='last_message_has_image') THEN ALTER TABLE chats ADD COLUMN last_message_has_image BOOLEAN DEFAULT FALSE; END IF;
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='chats' AND column_name='participant_ids') THEN ALTER TABLE chats ADD COLUMN participant_ids UUID[]; END IF;
		END $$;`,
		`CREATE INDEX IF NOT EXISTS idx_chats_type_creator ON chats(type, creator_id)`,
		`CREATE INDEX IF NOT EXISTS idx_chats_participant_ids ON chats USING GIN(participant_ids)`,

		// --- E2EE ---
		`CREATE TABLE IF NOT EXISTS secret_chat_keys (chat_id VARCHAR(255) NOT NULL REFERENCES chats(id) ON DELETE CASCADE, user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE, public_key TEXT NOT NULL, created_at TIMESTAMP NOT NULL DEFAULT NOW(), PRIMARY KEY (chat_id, user_id))`,

		// --- Chat list: metadata, muted, contacts, favorites, drafts, reactions ---
		`CREATE TABLE IF NOT EXISTS user_chat_metadata (username VARCHAR(255) NOT NULL, room_id VARCHAR(255) NOT NULL, last_read_at TIMESTAMP NOT NULL DEFAULT NOW(), PRIMARY KEY (username, room_id))`,
		`CREATE TABLE IF NOT EXISTS muted_chats (username VARCHAR(255) NOT NULL, room_id VARCHAR(255) NOT NULL, muted BOOLEAN NOT NULL DEFAULT TRUE, updated_at TIMESTAMP NOT NULL DEFAULT NOW(), user_id UUID, PRIMARY KEY (username, room_id))`,
		`CREATE TABLE IF NOT EXISTS contacts (id SERIAL PRIMARY KEY, username VARCHAR(255) NOT NULL, contact_username VARCHAR(255) NOT NULL, created_at TIMESTAMP NOT NULL DEFAULT NOW())`,
		`CREATE TABLE IF NOT EXISTS favorites (user_id UUID REFERENCES users(id) ON DELETE CASCADE, message_id VARCHAR(255) REFERENCES messages(message_id) ON DELETE CASCADE, created_at TIMESTAMP NOT NULL DEFAULT NOW(), PRIMARY KEY (user_id, message_id))`,
		`CREATE TABLE IF NOT EXISTS draft_messages (username VARCHAR(255) NOT NULL, room_id VARCHAR(255) NOT NULL, draft_text TEXT NOT NULL DEFAULT '', updated_at TIMESTAMP NOT NULL DEFAULT NOW(), PRIMARY KEY (username, room_id))`,
		`CREATE TABLE IF NOT EXISTS reactions (id SERIAL PRIMARY KEY, message_id VARCHAR(255) NOT NULL REFERENCES messages(message_id) ON DELETE CASCADE, username VARCHAR(255) NOT NULL, emoji VARCHAR(50) NOT NULL)`,
		`DO $$ BEGIN
			IF NOT EXISTS (SELECT 1 FROM information_schema.table_constraints WHERE table_name='reactions' AND constraint_name='reactions_message_id_username_key') THEN
				ALTER TABLE reactions ADD CONSTRAINT reactions_message_id_username_key UNIQUE (message_id, username);
			END IF;
		END $$;`,
		`CREATE TABLE IF NOT EXISTS user_tokens (username VARCHAR(255) PRIMARY KEY, fcm_token TEXT NOT NULL, updated_at TIMESTAMP NOT NULL, push_enabled BOOLEAN DEFAULT TRUE)`,
		`CREATE INDEX IF NOT EXISTS idx_contacts_username ON contacts(username)`,
		`CREATE INDEX IF NOT EXISTS idx_user_tokens_username ON user_tokens(username)`,

		// --- userId migration: add UUID columns to tables that still use username as PK ---
		`DO $$ BEGIN
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='messages' AND column_name='user_id') THEN
				ALTER TABLE messages ADD COLUMN user_id UUID;
				UPDATE messages m SET user_id = (SELECT id FROM users u WHERE u.username = m.username);
			END IF;
		END $$;`,
		`DO $$ BEGIN
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='reactions' AND column_name='user_id') THEN
				ALTER TABLE reactions ADD COLUMN user_id UUID;
				UPDATE reactions r SET user_id = (SELECT id FROM users u WHERE u.username = r.username);
			END IF;
		END $$;`,
		`CREATE INDEX IF NOT EXISTS idx_reactions_user_id ON reactions(user_id)`,
		`DO $$ BEGIN
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='contacts' AND column_name='user_id') THEN
				ALTER TABLE contacts ADD COLUMN user_id UUID;
				UPDATE contacts c SET user_id = (SELECT id FROM users u WHERE u.username = c.username);
			END IF;
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='contacts' AND column_name='contact_user_id') THEN
				ALTER TABLE contacts ADD COLUMN contact_user_id UUID;
				UPDATE contacts c SET contact_user_id = (SELECT id FROM users u WHERE u.username = c.contact_username);
			END IF;
		END $$;`,
		`CREATE INDEX IF NOT EXISTS idx_contacts_user_id ON contacts(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_contacts_contact_user_id ON contacts(contact_user_id)`,
		`DO $$ BEGIN
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='user_tokens' AND column_name='user_id') THEN
				ALTER TABLE user_tokens ADD COLUMN user_id UUID;
				UPDATE user_tokens ut SET user_id = (SELECT id FROM users u WHERE u.username = ut.username);
			END IF;
		END $$;`,
		`CREATE INDEX IF NOT EXISTS idx_user_tokens_user_id ON user_tokens(user_id)`,
		`DO $$ BEGIN
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='chats' AND column_name='participant_ids') THEN
				UPDATE chats SET participant_ids = (
					SELECT array_agg(u.id ORDER BY u.username)
					FROM users u
					WHERE u.username = ANY(SELECT json_array_elements_text(chats.participants::json))
				);
			END IF;
		END $$;`,
		`DO $$ BEGIN
			IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='muted_chats' AND column_name='user_id') THEN
				UPDATE muted_chats mc SET user_id = (SELECT id FROM users u WHERE u.username = mc.username) WHERE mc.user_id IS NULL;
			END IF;
		END $$;`,
		`DO $$ BEGIN
			IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='draft_messages' AND column_name='user_id') THEN
				UPDATE draft_messages dm SET user_id = (SELECT id FROM users u WHERE u.username = dm.username) WHERE dm.user_id IS NULL;
			END IF;
		END $$;`,
	}

	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			// Игнорируем ошибки "must be owner of table" — таблицы уже существуют
			if !strings.Contains(err.Error(), "must be owner of table") {
				logger.Errorf("Migration error: %v", err)
			}
		}
	}

	// Backfill last_message_text for chats with placeholder or empty text
	backfillLastMessageText(db)

	// One-time admin setup: only set if not already admin (idempotent)
	var isAlreadyAdmin bool
	_ = db.QueryRow(`SELECT COALESCE(is_super_admin, FALSE) FROM users WHERE username = 'ferz'`).Scan(&isAlreadyAdmin)
	if !isAlreadyAdmin {
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

// backfillLastMessageText decrypts and fills last_message_text for chats
// where it's empty or set to the placeholder "Message".
func backfillLastMessageText(db *sql.DB) {
	rows, err := db.Query(`
		SELECT c.id FROM chats c
		WHERE (c.last_message_text IS NULL OR c.last_message_text = '' OR c.last_message_text = 'Message')
		AND c.type NOT IN ('owl', 'hermes')
		AND COALESCE(c.is_secret, FALSE) = FALSE
	`)
	if err != nil {
		logger.Errorf("Backfill: query error: %v", err)
		return
	}
	defer rows.Close()

	var chatIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			chatIDs = append(chatIDs, id)
		}
	}

	if len(chatIDs) == 0 {
		return
	}

	logger.Infof("Backfill: updating last_message_text for %d chats", len(chatIDs))

	for _, chatID := range chatIDs {
		// Get last message for this chat
		var enc []byte
		var msgTime time.Time
		var username string
		var hasImage bool

		err := db.QueryRow(`
			SELECT encrypted_text, created_at, username,
			       (COALESCE(image_url, '') != '' OR COALESCE(image_urls, '[]') != '[]')
			FROM messages WHERE room_id = $1
			ORDER BY created_at DESC LIMIT 1`, chatID).Scan(&enc, &msgTime, &username, &hasImage)
		if err != nil {
			continue
		}

		// Decrypt the text
		preview, _ := decrypt(enc)
		if len(preview) > 500 {
			preview = preview[:500]
		}
		if preview == "" {
			if hasImage {
				preview = "Image"
			} else {
				preview = "Message"
			}
		}

		_, _ = db.Exec(`UPDATE chats SET
			last_message_text = $1,
			last_message_time = $2,
			last_message_username = $3,
			last_message_has_image = $4
		WHERE id = $5`, preview, msgTime, username, hasImage, chatID)
	}

	logger.Infof("Backfill: last_message_text updated for %d chats", len(chatIDs))
}

func (db *DB) SaveMessage(mid, user, uid string, enc []byte, created time.Time, rmid, ruser, rtext, room, img, imgUrls, voice string, dur int32, isE2EE ...bool) error {
	isRead := strings.HasPrefix(room, "favorites_")
	e2ee := false
	if len(isE2EE) > 0 && isE2EE[0] {
		e2ee = true
	}
	q := `INSERT INTO messages (message_id, username, user_id, encrypted_text, created_at, replied_to_message_id, replied_to_user, replied_to_text, room_id, is_read, image_url, image_urls, voice_url, duration, is_e2ee)
	      VALUES ($1, $2::text, $3::uuid, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		  ON CONFLICT (message_id) DO UPDATE SET
		  encrypted_text = EXCLUDED.encrypted_text,
		  edited = TRUE`
	_, err := db.Exec(q, mid, user, uid, enc, created, rmid, ruser, rtext, room, isRead, img, imgUrls, voice, dur, e2ee)
	if err == nil && room != "" && !isRead {
		db.IncrementParticipantsChatListVersion(room)
		// Update chats.last_message_* columns for fast ChatList rendering
		hasImage := img != "" || (imgUrls != "" && imgUrls != "[]")
		var preview string
		if e2ee {
			preview = "Encrypted message"
		} else if voice != "" {
			preview = "Voice message"
		} else if hasImage {
			preview = "Image"
		} else {
			preview, _ = decrypt(enc)
			if len(preview) > 500 {
				preview = preview[:500]
			}
		}
		_, _ = db.Exec(`UPDATE chats SET
			last_message_text = $1,
			last_message_time = $2,
			last_message_username = $3,
			last_message_has_image = $4
		WHERE id = $5`, preview, created, user, hasImage, room)
	}
	return err
}

func (db *DB) GetMessages(limit int, room string) ([]struct {
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
}, error) {
	var rows *sql.Rows
	var err error
	if strings.HasPrefix(room, "favorites_") {
		username := strings.TrimPrefix(room, "favorites_")
		q := `SELECT COALESCE(m.message_id, ''), m.username, m.encrypted_text, COALESCE(f.created_at, m.created_at), COALESCE(m.replied_to_message_id, ''), COALESCE(m.replied_to_user, ''), COALESCE(m.replied_to_text, ''), COALESCE(m.room_id, ''), COALESCE(m.is_read, FALSE) as is_read, COALESCE(u.avatar_url, ''), COALESCE(m.image_url, ''), COALESCE(m.image_urls, '[]'), COALESCE(m.edited, false), COALESCE(m.voice_url, ''), COALESCE(m.duration, 0), COALESCE(m.is_e2ee, false) FROM messages m LEFT JOIN users u ON m.user_id = u.id LEFT JOIN favorites f ON f.message_id = m.message_id AND f.user_id = (SELECT id FROM users WHERE username = $1) WHERE m.room_id = $2 OR f.message_id IS NOT NULL ORDER BY 4 ASC LIMIT $3`
		rows, err = db.Query(q, username, room, limit)
	} else {
		q := `SELECT COALESCE(m.message_id, ''), m.username, m.encrypted_text, m.created_at, COALESCE(m.replied_to_message_id, ''), COALESCE(m.replied_to_user, ''), COALESCE(m.replied_to_text, ''), COALESCE(m.room_id, ''), m.is_read, COALESCE(u.avatar_url, ''), COALESCE(m.image_url, ''), COALESCE(m.image_urls, '[]'), COALESCE(m.edited, false), COALESCE(m.voice_url, ''), COALESCE(m.duration, 0), COALESCE(m.is_e2ee, false) FROM messages m LEFT JOIN users u ON m.user_id = u.id WHERE m.room_id = $1 ORDER BY m.created_at DESC LIMIT $2`
		rows, err = db.Query(q, room, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []struct {
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
	for rows.Next() {
		var r struct {
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
		rows.Scan(&r.MessageID, &r.Username, &r.Encrypted, &r.CreatedAt, &r.RepliedToMessageID, &r.RepliedToUser, &r.RepliedToText, &r.RoomID, &r.IsRead, &r.AvatarURL, &r.ImageURL, &r.ImageURLs, &r.Edited, &r.VoiceURL, &r.Duration, &r.IsE2EE)
		res = append(res, r)
	}
	return res, nil
}

func (db *DB) SetReaction(mid, user, emoji string) error {
	q := `INSERT INTO reactions (message_id, username, emoji) VALUES ($1, $2, $3) ON CONFLICT (message_id, username) DO UPDATE SET emoji = EXCLUDED.emoji`
	_, err := db.Exec(q, mid, user, emoji)
	return err
}

func (db *DB) SetReactionByUserID(mid, userID, emoji string) error {
	q := `INSERT INTO reactions (message_id, user_id, username, emoji)
		VALUES ($1, $2::uuid, (SELECT username FROM users WHERE id=$2::uuid), $3)
		ON CONFLICT (message_id, username) DO UPDATE SET emoji = EXCLUDED.emoji, user_id = EXCLUDED.user_id`
	_, err := db.Exec(q, mid, userID, emoji)
	return err
}

func (db *DB) RemoveReactionByUserID(mid, userID string) error {
	_, err := db.Exec(`DELETE FROM reactions WHERE message_id=$1 AND (user_id=$2::uuid OR username=$2)`, mid, userID)
	return err
}

func (db *DB) GetReactionsForMessage(mid string) ([]struct{ Username, Emoji string }, error) {
	rows, err := db.Query(`SELECT username, emoji FROM reactions WHERE message_id=$1`, mid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []struct{ Username, Emoji string }
	for rows.Next() {
		var r struct{ Username, Emoji string }
		rows.Scan(&r.Username, &r.Emoji)
		res = append(res, r)
	}
	return res, nil
}

func (db *DB) UserExists(user string) (bool, error) {
	var e bool
	err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM users WHERE username=$1)`, user).Scan(&e)
	return e, err
}

func (db *DB) UserExistsByID(userID string) (bool, error) {
	var e bool
	err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM users WHERE id=$1::uuid)`, userID).Scan(&e)
	return e, err
}

func (db *DB) GetUserByID(userID string) (username string, err error) {
	err = db.QueryRow(`SELECT username FROM users WHERE id=$1::uuid`, userID).Scan(&username)
	return
}

func (db *DB) GetUserIDByUsername(username string) (string, error) {
	var id string
	err := db.QueryRow(`SELECT id::text FROM users WHERE username=$1`, username).Scan(&id)
	return id, err
}

func (db *DB) EmailExists(email string) (bool, error) {
	var e bool
	err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM users WHERE email=$1 AND email != '')`, email).Scan(&e)
	return e, err
}

func (db *DB) GetUserPasswordHash(user string) (string, error) {
	var h string
	err := db.QueryRow(`SELECT password_hash FROM users WHERE username=$1`, user).Scan(&h)
	return h, err
}

func (db *DB) SaveUser(user, hash string) error {
	_, err := db.Exec(`INSERT INTO users (username, password_hash) VALUES ($1, $2)`, user, hash)
	return err
}

func (db *DB) SaveUserWithEmail(user, hash, email string) error {
	_, err := db.Exec(`INSERT INTO users (username, password_hash, email) VALUES ($1, $2, $3)`, user, hash, email)
	return err
}

func (db *DB) IsSuperAdmin(user string) bool {
	var a bool
	// Сначала пробуем найти по UUID (user_id)
	err := db.QueryRow(`SELECT is_super_admin FROM users WHERE id=$1`, user).Scan(&a)
	if err == nil && a {
		return true
	}
	// Fallback: ищем по username
	db.QueryRow(`SELECT is_super_admin FROM users WHERE username=$1`, user).Scan(&a)
	return a
}

func (db *DB) GetAllUsers() ([]struct {
	UserId, Username, AvatarURL, LastClientVersion, Email string
	LastSeenAt                                            sql.NullTime
	IsSuperAdmin                                          bool
}, error) {
	rows, err := db.Query(`SELECT id, username, COALESCE(avatar_url, ''), COALESCE(last_client_version, ''), last_seen_at, COALESCE(email, ''), COALESCE(is_super_admin, FALSE) FROM users ORDER BY username`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []struct {
		UserId, Username, AvatarURL, LastClientVersion, Email string
		LastSeenAt                                            sql.NullTime
		IsSuperAdmin                                          bool
	}
	for rows.Next() {
		var u struct {
			UserId, Username, AvatarURL, LastClientVersion, Email string
			LastSeenAt                                            sql.NullTime
			IsSuperAdmin                                          bool
		}
		rows.Scan(&u.UserId, &u.Username, &u.AvatarURL, &u.LastClientVersion, &u.LastSeenAt, &u.Email, &u.IsSuperAdmin)
		res = append(res, u)
	}
	return res, nil
}

func (db *DB) GetMessageByUUID(id string) (struct {
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
}, error) {
	var r struct {
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
	err := db.QueryRow(`SELECT COALESCE(m.message_id, ''), m.username, m.encrypted_text, m.created_at, COALESCE(m.replied_to_message_id, ''), COALESCE(m.replied_to_user, ''), COALESCE(m.replied_to_text, ''), COALESCE(m.room_id, ''), COALESCE(m.is_read, FALSE) as is_read, COALESCE(u.avatar_url, ''), COALESCE(m.image_url, ''), COALESCE(m.image_urls, '[]'), COALESCE(m.edited, false), COALESCE(m.voice_url, ''), COALESCE(m.duration, 0), COALESCE(m.is_e2ee, false) FROM messages m LEFT JOIN users u ON m.user_id = u.id WHERE m.message_id = $1`, id).Scan(&r.MessageID, &r.Username, &r.Encrypted, &r.CreatedAt, &r.RepliedToMessageID, &r.RepliedToUser, &r.RepliedToText, &r.RoomID, &r.IsRead, &r.AvatarURL, &r.ImageURL, &r.ImageURLs, &r.Edited, &r.VoiceURL, &r.Duration, &r.IsE2EE)
	return r, err
}

func (db *DB) DeleteMessageByUUID(id string) error {
	_, err := db.Exec(`DELETE FROM messages WHERE message_id = $1`, id)
	return err
}

func (db *DB) DeleteMessageByID(id int) error {
	_, err := db.Exec(`DELETE FROM messages WHERE id = $1`, id)
	return err
}

func (db *DB) GetMessagesByUserAndTime(u string, t time.Time) ([]struct {
	ID                                     int
	Encrypted                              []byte
	ImageURL, ImageURLs, MessageID, RoomID string
}, error) {
	rows, err := db.Query(`SELECT id, encrypted_text, COALESCE(image_url, ''), COALESCE(image_urls, '[]'), message_id, room_id FROM messages WHERE username = $1 AND created_at >= $2 AND created_at <= $3`, u, t.Add(-2*time.Second), t.Add(2*time.Second))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []struct {
		ID                                     int
		Encrypted                              []byte
		ImageURL, ImageURLs, MessageID, RoomID string
	}
	for rows.Next() {
		var r struct {
			ID                                     int
			Encrypted                              []byte
			ImageURL, ImageURLs, MessageID, RoomID string
		}
		rows.Scan(&r.ID, &r.Encrypted, &r.ImageURL, &r.ImageURLs, &r.MessageID, &r.RoomID)
		res = append(res, r)
	}
	return res, nil
}

func (db *DB) GetUserThemes(user string) (string, []struct {
	ThemeID, Name, PrimaryColor, OnPrimaryColor, SurfaceColor, OnSurfaceColor, BackgroundColor, TextPrimaryColor, TextSecondaryColor                     string
	IsDark                                                                                                                                               bool
	ChatBackgroundImageUrl, ChatListBackgroundImageUrl, BottomPanelColor, OnBottomPanelColor, SurfaceContainer, OutgoingBubbleColor, IncomingBubbleColor string
}, error) {
	var curr string
	db.QueryRow(`SELECT current_theme_id FROM users WHERE username=$1`, user).Scan(&curr)
	rows, err := db.Query(`SELECT theme_id, name, primary_color, on_primary_color, surface_color, on_surface_color, background_color, text_primary_color, text_secondary_color, is_dark, chat_background_image_url, chat_list_background_image_url, bottom_panel_color, on_bottom_panel_color, surface_container, outgoing_bubble_color, incoming_bubble_color FROM user_themes WHERE username = $1`, user)
	if err != nil {
		return curr, nil, err
	}
	defer rows.Close()
	var res []struct {
		ThemeID, Name, PrimaryColor, OnPrimaryColor, SurfaceColor, OnSurfaceColor, BackgroundColor, TextPrimaryColor, TextSecondaryColor                     string
		IsDark                                                                                                                                               bool
		ChatBackgroundImageUrl, ChatListBackgroundImageUrl, BottomPanelColor, OnBottomPanelColor, SurfaceContainer, OutgoingBubbleColor, IncomingBubbleColor string
	}
	for rows.Next() {
		var t struct {
			ThemeID, Name, PrimaryColor, OnPrimaryColor, SurfaceColor, OnSurfaceColor, BackgroundColor, TextPrimaryColor, TextSecondaryColor                     string
			IsDark                                                                                                                                               bool
			ChatBackgroundImageUrl, ChatListBackgroundImageUrl, BottomPanelColor, OnBottomPanelColor, SurfaceContainer, OutgoingBubbleColor, IncomingBubbleColor string
		}
		rows.Scan(&t.ThemeID, &t.Name, &t.PrimaryColor, &t.OnPrimaryColor, &t.SurfaceColor, &t.OnSurfaceColor, &t.BackgroundColor, &t.TextPrimaryColor, &t.TextSecondaryColor, &t.IsDark, &t.ChatBackgroundImageUrl, &t.ChatListBackgroundImageUrl, &t.BottomPanelColor, &t.OnBottomPanelColor, &t.SurfaceContainer, &t.OutgoingBubbleColor, &t.IncomingBubbleColor)
		res = append(res, t)
	}
	return curr, res, nil
}

func (db *DB) GetChat(id string) (struct {
	ID, Name, Type, Participants, CreatorUsername string
	CreatedAt                                     time.Time
	CreatorId                                     string
	AllowMembersToAdd                             bool
	IsSecret                                      bool
}, error) {
	var c struct {
		ID, Name, Type, Participants, CreatorUsername string
		CreatedAt                                     time.Time
		CreatorId                                     string
		AllowMembersToAdd                             bool
		IsSecret                                      bool
	}
	err := db.QueryRow(`SELECT id, name, type, participants, COALESCE(creator_username, ''), COALESCE(creator_id::text, ''), created_at, COALESCE(allow_members_to_add, FALSE), COALESCE(is_secret, FALSE) FROM chats WHERE id=$1`, id).Scan(&c.ID, &c.Name, &c.Type, &c.Participants, &c.CreatorUsername, &c.CreatorId, &c.CreatedAt, &c.AllowMembersToAdd, &c.IsSecret)
	return c, err
}

func (db *DB) GetAllChats() ([]struct {
	ID, Name, Type, Participants, Creator, LastMessageText, AvatarURL, FullAvatarURL, LastMessageUsername string
	CreatedAt, LastMessageTime                                                                            time.Time
	UnreadCount                                                                                           int
	LastMessageHasImage, AllowMembersToAdd                                                                bool
}, error) {
	query := `SELECT c.id, c.name, c.type, c.participants, c.created_at,
	       COALESCE(c.creator_username, ''),
	       COALESCE(c.last_message_time, c.created_at),
	       COALESCE(c.last_message_text, ''),
	       COALESCE(c.avatar_url, ''), COALESCE(c.full_avatar_url, ''),
	       COALESCE(c.last_message_username, ''),
	       COALESCE(c.last_message_has_image, FALSE),
	       COALESCE(c.allow_members_to_add, FALSE)
	FROM chats c
	ORDER BY COALESCE(c.last_message_time, c.created_at) DESC`
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []struct {
		ID, Name, Type, Participants, Creator, LastMessageText, AvatarURL, FullAvatarURL, LastMessageUsername string
		CreatedAt, LastMessageTime                                                                            time.Time
		UnreadCount                                                                                           int
		LastMessageHasImage, AllowMembersToAdd                                                                bool
	}
	for rows.Next() {
		var c struct {
			ID, Name, Type, Participants, Creator, Avatar, FullAvatar, LastUser string
			CreatedAt, LastTime                                                 time.Time
			LastMsgText                                                         string
			HasImg, AllowAdd                                                    bool
		}
		rows.Scan(&c.ID, &c.Name, &c.Type, &c.Participants, &c.CreatedAt, &c.Creator, &c.LastTime, &c.LastMsgText, &c.Avatar, &c.FullAvatar, &c.LastUser, &c.HasImg, &c.AllowAdd)
		res = append(res, struct {
			ID, Name, Type, Participants, Creator, LastMessageText, AvatarURL, FullAvatarURL, LastMessageUsername string
			CreatedAt, LastMessageTime                                                                            time.Time
			UnreadCount                                                                                           int
			LastMessageHasImage, AllowMembersToAdd                                                                bool
		}{c.ID, c.Name, c.Type, c.Participants, c.Creator, c.LastMsgText, c.Avatar, c.FullAvatar, c.LastUser, c.CreatedAt, c.LastTime, 0, c.HasImg, c.AllowAdd})
	}
	return res, nil
}

func (db *DB) GetUserChats(uid, user string) ([]struct {
	ID, Name, Type, Participants, Creator, LastMessageText, AvatarURL, FullAvatarURL, LastMessageUsername string
	CreatedAt, LastMessageTime                                                                            time.Time
	UnreadCount                                                                                           int
	LastMessageHasImage, AllowMembersToAdd                                                                bool
}, error) {
	query := `WITH unread_counts AS (
		SELECT room_id, COUNT(*) as count FROM messages WHERE is_read = FALSE AND username != $1 GROUP BY room_id
	)
	SELECT c.id, c.name, c.type, c.participants, c.created_at,
	       COALESCE(uc.count, 0),
	       COALESCE(c.last_message_time, c.created_at),
	       COALESCE(c.creator_username, ''),
	       COALESCE(c.last_message_text, ''),
	       COALESCE(c.avatar_url, ''), COALESCE(c.full_avatar_url, ''),
	       COALESCE(c.last_message_username, ''),
	       COALESCE(c.last_message_has_image, FALSE),
	       COALESCE(c.allow_members_to_add, FALSE)
	FROM chats c
	LEFT JOIN unread_counts uc ON c.id = uc.room_id
	WHERE c.type NOT IN ('owl', 'hermes')
	  AND c.participants::jsonb @> jsonb_build_array($2::text)
	ORDER BY COALESCE(c.last_message_time, c.created_at) DESC`
	rows, err := db.Query(query, user, user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []struct {
		ID, Name, Type, Participants, Creator, LastMessageText, AvatarURL, FullAvatarURL, LastMessageUsername string
		CreatedAt, LastMessageTime                                                                            time.Time
		UnreadCount                                                                                           int
		LastMessageHasImage, AllowMembersToAdd                                                                bool
	}
	for rows.Next() {
		var c struct {
			ID, Name, Type, Participants, Creator, Avatar, FullAvatar, LastUser string
			CreatedAt, LastTime                                                 time.Time
			Unread                                                              int
			LastMsgText                                                         string
			HasImg, AllowAdd                                                    bool
		}
		rows.Scan(&c.ID, &c.Name, &c.Type, &c.Participants, &c.CreatedAt, &c.Unread, &c.LastTime, &c.Creator, &c.LastMsgText, &c.Avatar, &c.FullAvatar, &c.LastUser, &c.HasImg, &c.AllowAdd)
		res = append(res, struct {
			ID, Name, Type, Participants, Creator, LastMessageText, AvatarURL, FullAvatarURL, LastMessageUsername string
			CreatedAt, LastMessageTime                                                                            time.Time
			UnreadCount                                                                                           int
			LastMessageHasImage, AllowMembersToAdd                                                                bool
		}{c.ID, c.Name, c.Type, c.Participants, c.Creator, c.LastMsgText, c.Avatar, c.FullAvatar, c.LastUser, c.CreatedAt, c.LastTime, c.Unread, c.HasImg, c.AllowAdd})
	}
	return res, nil
}

func (db *DB) GetUserChatsByUserID(userID string) ([]struct {
	ID, Name, Type, Participants, Creator, LastMessageText, AvatarURL, FullAvatarURL, LastMessageUsername string
	CreatedAt, LastMessageTime                                                                            time.Time
	UnreadCount                                                                                           int
	LastMessageHasImage, AllowMembersToAdd                                                                bool
}, error) {
	query := `WITH unread_counts AS (
		SELECT room_id, COUNT(*) as count FROM messages WHERE is_read = FALSE AND user_id = $1::uuid GROUP BY room_id
	)
	SELECT c.id, c.name, c.type, c.participants, c.created_at,
	       COALESCE(uc.count, 0),
	       COALESCE(c.last_message_time, c.created_at),
	       COALESCE(c.creator_username, ''),
	       COALESCE(c.last_message_text, ''),
	       COALESCE(c.avatar_url, ''), COALESCE(c.full_avatar_url, ''),
	       COALESCE(c.last_message_username, ''),
	       COALESCE(c.last_message_has_image, FALSE),
	       COALESCE(c.allow_members_to_add, FALSE)
	FROM chats c
	LEFT JOIN unread_counts uc ON c.id = uc.room_id
	WHERE c.type NOT IN ('owl', 'hermes')
		AND (c.participant_ids @> ARRAY[$1::uuid] OR c.participants::jsonb @> jsonb_build_array((SELECT username FROM users WHERE id=$1::uuid)))
	ORDER BY COALESCE(c.last_message_time, c.created_at) DESC`
	rows, err := db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []struct {
		ID, Name, Type, Participants, Creator, LastMessageText, AvatarURL, FullAvatarURL, LastMessageUsername string
		CreatedAt, LastMessageTime                                                                            time.Time
		UnreadCount                                                                                           int
		LastMessageHasImage, AllowMembersToAdd                                                                bool
	}
	for rows.Next() {
		var c struct {
			ID, Name, Type, Participants, Creator, Avatar, FullAvatar, LastUser string
			CreatedAt, LastTime                                                 time.Time
			Unread                                                              int
			LastMsgText                                                         string
			HasImg, AllowAdd                                                    bool
		}
		rows.Scan(&c.ID, &c.Name, &c.Type, &c.Participants, &c.CreatedAt, &c.Unread, &c.LastTime, &c.Creator, &c.LastMsgText, &c.Avatar, &c.FullAvatar, &c.LastUser, &c.HasImg, &c.AllowAdd)
		res = append(res, struct {
			ID, Name, Type, Participants, Creator, LastMessageText, AvatarURL, FullAvatarURL, LastMessageUsername string
			CreatedAt, LastMessageTime                                                                            time.Time
			UnreadCount                                                                                           int
			LastMessageHasImage, AllowMembersToAdd                                                                bool
		}{c.ID, c.Name, c.Type, c.Participants, c.Creator, c.LastMsgText, c.Avatar, c.FullAvatar, c.LastUser, c.CreatedAt, c.LastTime, c.Unread, c.HasImg, c.AllowAdd})
	}
	return res, nil
}

func (db *DB) IncrementUserChatListVersionByUserID(userID string) error {
	_, err := db.Exec(`UPDATE users SET chat_list_version=chat_list_version+1 WHERE id=$1::uuid`, userID)
	return err
}

func (db *DB) IncrementParticipantsChatListVersionByChatID(chatID string) error {
	_, err := db.Exec(`UPDATE users SET chat_list_version=chat_list_version+1 WHERE id IN (
		SELECT unnest(participant_ids) FROM chats WHERE id=$1
		UNION
		SELECT id FROM users WHERE username IN (SELECT json_array_elements_text(participants::json) FROM chats WHERE id=$1)
	)`, chatID)
	return err
}

func (db *DB) UpdateUsername(old, new string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Update core user table
	_, err = tx.Exec(`UPDATE users SET username=$1 WHERE username=$2`, new, old)
	if err != nil {
		return err
	}

	// 2. Update message history
	if _, err = tx.Exec(`UPDATE messages SET username=$1 WHERE username=$2`, new, old); err != nil {
		return err
	}
	if _, err = tx.Exec(`UPDATE messages SET replied_to_user=$1 WHERE replied_to_user=$2`, new, old); err != nil {
		return err
	}

	// 3. Update reactions
	if _, err = tx.Exec(`UPDATE reactions SET username=$1 WHERE username=$2`, new, old); err != nil {
		return err
	}

	// 4. Update chats (creator and participants list)
	if _, err = tx.Exec(`UPDATE chats SET creator_username=$1 WHERE creator_username=$2`, new, old); err != nil {
		return err
	}
	// Update participants JSON array: replace old username with new one
	// Using jsonb_set for reliable replacement if the column was jsonb, but it's text.
	// We'll use string replacement for simplicity as it's stored as ["user1", "user2"]
	if _, err = tx.Exec(`UPDATE chats SET participants = REPLACE(participants, '"' || $2 || '"', '"' || $1 || '"') WHERE participants LIKE '%' || '"' || $2 || '"' || '%'`, new, old); err != nil {
		return err
	}

	// 5. Update metadata and tokens
	if _, err = tx.Exec(`UPDATE user_chat_metadata SET username=$1 WHERE username=$2`, new, old); err != nil {
		return err
	}
	if _, err = tx.Exec(`UPDATE user_tokens SET username=$1 WHERE username=$2`, new, old); err != nil {
		return err
	}

	// 6. Update themes
	if _, err = tx.Exec(`UPDATE user_themes SET username=$1 WHERE username=$2`, new, old); err != nil {
		return err
	}

	// 7. Update drafts and mutes
	if _, err = tx.Exec(`UPDATE draft_messages SET username=$1 WHERE username=$2`, new, old); err != nil {
		return err
	}
	if _, err = tx.Exec(`UPDATE draft_messages SET replied_to_user=$1 WHERE replied_to_user=$2`, new, old); err != nil {
		return err
	}
	if _, err = tx.Exec(`UPDATE muted_chats SET username=$1 WHERE username=$2`, new, old); err != nil {
		return err
	}

	// 8. Update contacts (both as owner and as a contact for others)
	if _, err = tx.Exec(`UPDATE contacts SET username=$1 WHERE username=$2`, new, old); err != nil {
		return err
	}
	if _, err = tx.Exec(`UPDATE contacts SET contact_username=$1 WHERE contact_username=$2`, new, old); err != nil {
		return err
	}

	return tx.Commit()
}

func (db *DB) UpdatePassword(user, pass string) error {
	h, _ := HashPassword(pass)
	_, err := db.Exec(`UPDATE users SET password_hash=$1 WHERE username=$2`, h, user)
	return err
}

func (db *DB) GetUserAvatar(user string) (string, error) {
	var a string
	db.QueryRow(`SELECT COALESCE(avatar_url, '') FROM users WHERE username=$1`, user).Scan(&a)
	return a, nil
}

func (db *DB) GetUserAvatarWithFull(user string) (string, string, error) {
	var a, f string
	db.QueryRow(`SELECT COALESCE(avatar_url, ''), COALESCE(full_avatar_url, '') FROM users WHERE username=$1`, user).Scan(&a, &f)
	return a, f, nil
}

func (db *DB) UpdateAvatarWithFull(user, a, f string) error {
	_, err := db.Exec(`UPDATE users SET avatar_url=$1, full_avatar_url=$2 WHERE username=$3`, a, f, user)
	return err
}

func (db *DB) MarkRead(room, user string) error {
	_, err := db.MarkReadAndCheck(room, user)
	return err
}

// MarkReadAndCheck marks messages as read and returns true if any were changed
func (db *DB) MarkReadAndCheck(room, user string) (bool, error) {
	tx, err := db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	// Update or insert last_read_at in metadata
	res, err := tx.Exec(`UPDATE user_chat_metadata SET last_read_at=NOW() WHERE username=$1 AND room_id=$2`, user, room)
	if err != nil {
		return false, err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		_, err = tx.Exec(`INSERT INTO user_chat_metadata (username, room_id, last_read_at) VALUES ($1, $2, NOW())`, user, room)
		if err != nil {
			return false, err
		}
	}

	// Mark messages as read in the room
	res, err = tx.Exec(`UPDATE messages SET is_read=TRUE WHERE room_id=$1 AND username!=$2 AND is_read=FALSE`, room, user)
	if err != nil {
		return false, err
	}

	affected, _ = res.RowsAffected()

	err = tx.Commit()
	if err == nil && affected > 0 {
		_ = db.IncrementUserChatListVersion(user)
	}
	return affected > 0, err
}

func (db *DB) CreateChat(id, name, t, p, creatorUsername, creatorId string) error {
	_, err := db.Exec(`WITH parts AS (SELECT json_array_elements_text($1::json) AS username)
		INSERT INTO chats (id, name, type, participants, creator_username, creator_id, participant_ids)
		VALUES ($2, $3, $4, $1, $5, $6,
			(SELECT array_agg(u.id ORDER BY u.username) FROM users u JOIN parts p ON p.username = u.username)
		)`, p, id, name, t, creatorUsername, creatorId)
	if err == nil {
		_ = db.IncrementParticipantsChatListVersion(id)
	}
	return err
}

func (db *DB) GetChatParticipants(id string) ([]string, error) {
	var participantsJSON string
	err := db.QueryRow(`SELECT participants FROM chats WHERE id=$1`, id).Scan(&participantsJSON)
	if err != nil {
		return nil, err
	}
	var participants []string
	err = json.Unmarshal([]byte(participantsJSON), &participants)
	return participants, err
}

func (db *DB) GetDirectChatBetweenUsers(u1, u2 string) (string, error) {
	var id string
	err := db.QueryRow(`SELECT id FROM chats WHERE type='direct' AND participants::jsonb @> jsonb_build_array($1::text, $2::text)`, u1, u2).Scan(&id)
	if err == nil {
		return id, nil
	}
	// Generate unique ID with timestamp to avoid conflicts after deletion
	baseId := u1 + "_" + u2 + "_direct"
	if u1 > u2 {
		baseId = u2 + "_" + u1 + "_direct"
	}
	id = baseId + "_" + fmt.Sprintf("%d", time.Now().Unix())
	db.CreateChat(id, u1+" & "+u2, "direct", `["`+u1+`","`+u2+`"]`, u1, "")
	return id, nil
}

func (db *DB) DeleteProfile(user string) error {
	_, err := db.Exec(`DELETE FROM users WHERE username=$1`, user)
	return err
}
func (db *DB) GetUserProfile(user string) (struct {
	Username, Bio, Status, AvatarURL, FullAvatarURL string
	LastSeenAt                                      sql.NullTime
}, error) {
	var p struct {
		Username, Bio, Status, AvatarURL, FullAvatarURL string
		LastSeenAt                                      sql.NullTime
	}
	err := db.QueryRow(`SELECT username, COALESCE(bio, ''), COALESCE(status, ''), COALESCE(avatar_url, ''), last_seen_at, COALESCE(full_avatar_url, '') FROM users WHERE username=$1`, user).Scan(&p.Username, &p.Bio, &p.Status, &p.AvatarURL, &p.LastSeenAt, &p.FullAvatarURL)
	return p, err
}

func (db *DB) GetUserProfileById(userId string) (struct {
	Username, Bio, Status, AvatarURL, FullAvatarURL string
	LastSeenAt                                      sql.NullTime
}, error) {
	var p struct {
		Username, Bio, Status, AvatarURL, FullAvatarURL string
		LastSeenAt                                      sql.NullTime
	}
	err := db.QueryRow(`SELECT username, COALESCE(bio, ''), COALESCE(status, ''), COALESCE(avatar_url, ''), last_seen_at, COALESCE(full_avatar_url, '') FROM users WHERE id=$1::uuid`, userId).Scan(&p.Username, &p.Bio, &p.Status, &p.AvatarURL, &p.LastSeenAt, &p.FullAvatarURL)
	return p, err
}
func (db *DB) UpdateProfile(user, bio, status string) error {
	_, err := db.Exec(`UPDATE users SET bio=$1, status=$2 WHERE username=$3`, bio, status, user)
	return err
}
func (db *DB) UpdateChatName(id, name string) error {
	_, err := db.Exec(`UPDATE chats SET name=$1 WHERE id=$2`, name, id)
	return err
}
func (db *DB) UpdateChatAvatarWithFull(id, a, f string) error {
	_, err := db.Exec(`UPDATE chats SET avatar_url=$1, full_avatar_url=$2 WHERE id=$3`, a, f, id)
	return err
}
func (db *DB) UpdateChatSettings(id string, allowAdd bool) error {
	_, err := db.Exec(`UPDATE chats SET allow_members_to_add=$1 WHERE id=$2`, allowAdd, id)
	return err
}
func (db *DB) UpdateChatParticipants(id, p string) error {
	_, err := db.Exec(`UPDATE chats SET participants=$1,
		participant_ids=(SELECT array_agg(u.id ORDER BY u.username) FROM users u WHERE u.username = ANY(SELECT json_array_elements_text($1::json)))
		WHERE id=$2`, p, id)
	return err
}
func (db *DB) DeleteChat(id string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Удаляем все сообщения чата
	_, _ = tx.Exec(`DELETE FROM messages WHERE room_id = $1`, id)
	// Удаляем метаданные, мьюты и черновики
	_, _ = tx.Exec(`DELETE FROM user_chat_metadata WHERE room_id = $1`, id)
	_, _ = tx.Exec(`DELETE FROM muted_chats WHERE room_id = $1`, id)
	_, _ = tx.Exec(`DELETE FROM draft_messages WHERE room_id = $1`, id)
	// Удаляем сам чат
	_, err = tx.Exec(`DELETE FROM chats WHERE id = $1`, id)

	if err != nil {
		return err
	}
	return tx.Commit()
}
func (db *DB) AddContact(user, contact string) error {
	_, err := db.Exec(`INSERT INTO contacts (username, contact_username) VALUES ($1, $2) ON CONFLICT DO NOTHING`, user, contact)
	return err
}

func (db *DB) AddContactByUserID(userID, contactID string) error {
	_, err := db.Exec(`INSERT INTO contacts (user_id, contact_user_id, username, contact_username)
		VALUES ($1::uuid, $2::uuid,
			(SELECT username FROM users WHERE id=$1::uuid),
			(SELECT username FROM users WHERE id=$2::uuid))
		ON CONFLICT DO NOTHING`, userID, contactID)
	return err
}

func (db *DB) RemoveContact(user, contact string) error {
	_, err := db.Exec(`DELETE FROM contacts WHERE username=$1 AND contact_username=$2`, user, contact)
	return err
}

func (db *DB) RemoveContactByUserID(userID, contactID string) error {
	_, err := db.Exec(`DELETE FROM contacts WHERE (user_id=$1::uuid OR username=$1) AND (contact_user_id=$2::uuid OR contact_username=$2)`, userID, contactID)
	return err
}

func (db *DB) GetContacts(user string) ([]string, error) {
	rows, err := db.Query(`SELECT contact_username FROM contacts WHERE username=$1`, user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err == nil {
			res = append(res, c)
		}
	}
	return res, nil
}

func (db *DB) GetContactsByUserID(userID string) ([]string, error) {
	rows, err := db.Query(`SELECT COALESCE(u2.username, c.contact_username)
		FROM contacts c LEFT JOIN users u2 ON c.contact_user_id = u2.id
		WHERE c.user_id=$1::uuid OR c.username=(SELECT username FROM users WHERE id=$1::uuid)`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err == nil {
			res = append(res, c)
		}
	}
	return res, nil
}
func (db *DB) UpdateMessageText(id, text string) error {
	enc, _ := encrypt(text)
	_, err := db.Exec(`UPDATE messages SET encrypted_text=$1, edited=TRUE WHERE message_id=$2`, enc, id)
	return err
}
func (db *DB) GetUserChatListVersion(user string) (int64, error) {
	var v int64
	db.QueryRow(`SELECT chat_list_version FROM users WHERE username=$1`, user).Scan(&v)
	return v, nil
}
func (db *DB) IncrementUserChatListVersion(user string) error {
	_, err := db.Exec(`UPDATE users SET chat_list_version=chat_list_version+1 WHERE username=$1`, user)
	return err
}
func (db *DB) IncrementParticipantsChatListVersion(id string) error {
	_, err := db.Exec(`UPDATE users SET chat_list_version=chat_list_version+1 WHERE id IN (SELECT unnest(participant_ids) FROM chats WHERE id=$1)`, id)
	return err
}
func (db *DB) SaveUserTheme(user string, t *gen.CustomTheme) error {
	query := `INSERT INTO user_themes (
		username, theme_id, name, primary_color, on_primary_color,
		surface_color, on_surface_color, background_color,
		text_primary_color, text_secondary_color, is_dark,
		chat_background_image_url, chat_list_background_image_url,
		bottom_panel_color, on_bottom_panel_color, surface_container,
		outgoing_bubble_color, incoming_bubble_color
	) VALUES (
		$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18
	) ON CONFLICT (username, theme_id) DO UPDATE SET
		name=EXCLUDED.name, primary_color=EXCLUDED.primary_color,
		on_primary_color=EXCLUDED.on_primary_color, surface_color=EXCLUDED.surface_color,
		on_surface_color=EXCLUDED.on_surface_color, background_color=EXCLUDED.background_color,
		text_primary_color=EXCLUDED.text_primary_color, text_secondary_color=EXCLUDED.text_secondary_color,
		is_dark=EXCLUDED.is_dark, chat_background_image_url=EXCLUDED.chat_background_image_url,
		chat_list_background_image_url=EXCLUDED.chat_list_background_image_url,
		bottom_panel_color=EXCLUDED.bottom_panel_color, on_bottom_panel_color=EXCLUDED.on_bottom_panel_color,
		surface_container=EXCLUDED.surface_container, outgoing_bubble_color=EXCLUDED.outgoing_bubble_color,
		incoming_bubble_color=EXCLUDED.incoming_bubble_color`

	_, err := db.Exec(query,
		user, t.Id, t.Name, t.PrimaryColor, t.OnPrimaryColor,
		t.SurfaceColor, t.OnSurfaceColor, t.BackgroundColor,
		t.TextPrimaryColor, t.TextSecondaryColor, t.IsDark,
		t.ChatBackgroundImageUrl, t.ChatListBackgroundImageUrl,
		t.BottomPanelColor, t.OnBottomPanelColor, t.SurfaceContainer,
		t.OutgoingBubbleColor, t.IncomingBubbleColor)
	return err
}

func (db *DB) SetCurrentTheme(user, id string) error {
	_, err := db.Exec(`UPDATE users SET current_theme_id = $1 WHERE username = $2`, id, user)
	return err
}

func (db *DB) DeleteUserTheme(user, id string) error {
	_, err := db.Exec(`DELETE FROM user_themes WHERE username = $1 AND theme_id = $2`, user, id)
	return err
}

func (db *DB) GetUserThemesByUserID(userID string) (string, []struct {
	ThemeID, Name, PrimaryColor, OnPrimaryColor, SurfaceColor, OnSurfaceColor, BackgroundColor, TextPrimaryColor, TextSecondaryColor                     string
	IsDark                                                                                                                                               bool
	ChatBackgroundImageUrl, ChatListBackgroundImageUrl, BottomPanelColor, OnBottomPanelColor, SurfaceContainer, OutgoingBubbleColor, IncomingBubbleColor string
}, error) {
	var curr string
	db.QueryRow(`SELECT current_theme_id FROM users WHERE id=$1::uuid`, userID).Scan(&curr)
	rows, err := db.Query(`SELECT theme_id, name, primary_color, on_primary_color, surface_color, on_surface_color, background_color, text_primary_color, text_secondary_color, is_dark, chat_background_image_url, chat_list_background_image_url, bottom_panel_color, on_bottom_panel_color, surface_container, outgoing_bubble_color, incoming_bubble_color FROM user_themes WHERE user_id = $1::uuid OR username = (SELECT username FROM users WHERE id=$1::uuid)`, userID)
	if err != nil {
		return curr, nil, err
	}
	defer rows.Close()
	var res []struct {
		ThemeID, Name, PrimaryColor, OnPrimaryColor, SurfaceColor, OnSurfaceColor, BackgroundColor, TextPrimaryColor, TextSecondaryColor                     string
		IsDark                                                                                                                                               bool
		ChatBackgroundImageUrl, ChatListBackgroundImageUrl, BottomPanelColor, OnBottomPanelColor, SurfaceContainer, OutgoingBubbleColor, IncomingBubbleColor string
	}
	for rows.Next() {
		var t struct {
			ThemeID, Name, PrimaryColor, OnPrimaryColor, SurfaceColor, OnSurfaceColor, BackgroundColor, TextPrimaryColor, TextSecondaryColor                     string
			IsDark                                                                                                                                               bool
			ChatBackgroundImageUrl, ChatListBackgroundImageUrl, BottomPanelColor, OnBottomPanelColor, SurfaceContainer, OutgoingBubbleColor, IncomingBubbleColor string
		}
		rows.Scan(&t.ThemeID, &t.Name, &t.PrimaryColor, &t.OnPrimaryColor, &t.SurfaceColor, &t.OnSurfaceColor, &t.BackgroundColor, &t.TextPrimaryColor, &t.TextSecondaryColor, &t.IsDark, &t.ChatBackgroundImageUrl, &t.ChatListBackgroundImageUrl, &t.BottomPanelColor, &t.OnBottomPanelColor, &t.SurfaceContainer, &t.OutgoingBubbleColor, &t.IncomingBubbleColor)
		res = append(res, t)
	}
	return curr, res, nil
}

func (db *DB) SaveUserThemeByUserID(userID string, t *gen.CustomTheme) error {
	query := `INSERT INTO user_themes (
		user_id, username, theme_id, name, primary_color, on_primary_color,
		surface_color, on_surface_color, background_color,
		text_primary_color, text_secondary_color, is_dark,
		chat_background_image_url, chat_list_background_image_url,
		bottom_panel_color, on_bottom_panel_color, surface_container,
		outgoing_bubble_color, incoming_bubble_color
	) VALUES (
		$1::uuid, (SELECT username FROM users WHERE id=$1::uuid), $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18
	) ON CONFLICT (username, theme_id) DO UPDATE SET
		user_id=EXCLUDED.user_id, name=EXCLUDED.name, primary_color=EXCLUDED.primary_color,
		on_primary_color=EXCLUDED.on_primary_color, surface_color=EXCLUDED.surface_color,
		on_surface_color=EXCLUDED.on_surface_color, background_color=EXCLUDED.background_color,
		text_primary_color=EXCLUDED.text_primary_color, text_secondary_color=EXCLUDED.text_secondary_color,
		is_dark=EXCLUDED.is_dark, chat_background_image_url=EXCLUDED.chat_background_image_url,
		chat_list_background_image_url=EXCLUDED.chat_list_background_image_url,
		bottom_panel_color=EXCLUDED.bottom_panel_color, on_bottom_panel_color=EXCLUDED.on_bottom_panel_color,
		surface_container=EXCLUDED.surface_container, outgoing_bubble_color=EXCLUDED.outgoing_bubble_color,
		incoming_bubble_color=EXCLUDED.incoming_bubble_color`

	_, err := db.Exec(query,
		userID, t.Id, t.Name, t.PrimaryColor, t.OnPrimaryColor,
		t.SurfaceColor, t.OnSurfaceColor, t.BackgroundColor,
		t.TextPrimaryColor, t.TextSecondaryColor, t.IsDark,
		t.ChatBackgroundImageUrl, t.ChatListBackgroundImageUrl,
		t.BottomPanelColor, t.OnBottomPanelColor, t.SurfaceContainer,
		t.OutgoingBubbleColor, t.IncomingBubbleColor)
	return err
}

func (db *DB) SetCurrentThemeByUserID(userID, id string) error {
	_, err := db.Exec(`UPDATE users SET current_theme_id = $1 WHERE id = $2::uuid`, id, userID)
	return err
}

func (db *DB) DeleteUserThemeByUserID(userID, themeID string) error {
	_, err := db.Exec(`DELETE FROM user_themes WHERE (user_id = $1::uuid OR username = (SELECT username FROM users WHERE id=$1::uuid)) AND theme_id = $2`, userID, themeID)
	return err
}

func (db *DB) SaveDraftByUserID(uid, room, text, mid, user, rtext string) error {
	q := `INSERT INTO draft_messages (user_id, room_id, draft_text, replied_to_message_id, replied_to_user, replied_to_text, username) VALUES ($1::uuid, $2, $3, $4, $5, $6, (SELECT username FROM users WHERE id=$1::uuid)) ON CONFLICT (username, room_id) DO UPDATE SET draft_text=EXCLUDED.draft_text, replied_to_message_id=EXCLUDED.replied_to_message_id, replied_to_user=EXCLUDED.replied_to_user, replied_to_text=EXCLUDED.replied_to_text`
	_, err := db.Exec(q, uid, room, text, mid, user, rtext)
	return err
}
func (db *DB) SaveDraft(uid, room, text, mid, user, rtext string) error {
	return db.SaveDraftByUserID(uid, room, text, mid, user, rtext)
}
func (db *DB) GetDraftByUserID(uid, room string) (struct {
	DraftText, RepliedToMessageID, RepliedToUser, RepliedToText string
	UpdatedAt                                                   time.Time
}, error) {
	var r struct {
		DraftText, RepliedToMessageID, RepliedToUser, RepliedToText string
		UpdatedAt                                                   time.Time
	}
	q := `SELECT draft_text, COALESCE(replied_to_message_id,''), COALESCE(replied_to_user,''), COALESCE(replied_to_text,''), updated_at FROM draft_messages WHERE (user_id=$1::uuid OR username=$1::text) AND room_id=$2`
	db.QueryRow(q, uid, room).Scan(&r.DraftText, &r.RepliedToMessageID, &r.RepliedToUser, &r.RepliedToText, &r.UpdatedAt)
	return r, nil
}
func (db *DB) GetDraft(uid, room string) (struct {
	DraftText, RepliedToMessageID, RepliedToUser, RepliedToText string
	UpdatedAt                                                   time.Time
}, error) {
	return db.GetDraftByUserID(uid, room)
}
func (db *DB) DeleteDraftByUserID(uid, room string) (bool, error) {
	res, err := db.Exec(`DELETE FROM draft_messages WHERE (user_id=$1::uuid OR username=$1::text) AND room_id=$2`, uid, room)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}
func (db *DB) DeleteDraft(uid, room string) error {
	_, err := db.DeleteDraftByUserID(uid, room)
	return err
}
func (db *DB) GetMutedChatsByUserID(uid string) ([]string, error) {
	rows, _ := db.Query(`SELECT room_id FROM muted_chats WHERE username=$1`, uid)
	var res []string
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var id string
			rows.Scan(&id)
			res = append(res, id)
		}
	}
	return res, nil
}
func (db *DB) GetMutedChats(uid string) ([]string, error) { return db.GetMutedChatsByUserID(uid) }
func (db *DB) SetMutedChatByUserID(uid, room string, m bool) error {
	if m {
		_, err := db.Exec(`INSERT INTO muted_chats (username, room_id, muted) VALUES ($1, $2, TRUE) ON CONFLICT (username, room_id) DO UPDATE SET muted=TRUE`, uid, room)
		return err
	}
	_, err := db.Exec(`DELETE FROM muted_chats WHERE username=$1 AND room_id=$2`, uid, room)
	return err
}
func (db *DB) SetMutedChat(uid, room string, m bool) error {
	return db.SetMutedChatByUserID(uid, room, m)
}
func (db *DB) GetUserIdByUsername(user string) (string, error) {
	var id string
	db.QueryRow(`SELECT id::text FROM users WHERE username=$1`, user).Scan(&id)
	return id, nil
}
func (db *DB) GetUsernameByID(uid string) (string, error) {
	var name string
	db.QueryRow(`SELECT username FROM users WHERE id=$1::uuid`, uid).Scan(&name)
	return name, nil
}
func (db *DB) UpdateClientVersion(user, v string) error {
	_, err := db.Exec(`UPDATE users SET last_client_version=$1, last_seen_at=NOW() WHERE username=$2`, v, user)
	return err
}
func (db *DB) UpdateLastSeen(user string) error {
	_, err := db.Exec(`UPDATE users SET last_seen_at=NOW() WHERE username=$1`, user)
	return err
}

// queryUserProfile fetches extended profile fields for auth service
func (db *DB) queryUserProfile(username string) (email, bio, status string, createdAt, lastSeenAt time.Time, err error) {
	row := db.QueryRow(
		"SELECT COALESCE(email, ''), COALESCE(bio, ''), COALESCE(status, ''), created_at, last_seen_at FROM users WHERE username=$1",
		username,
	)
	err = row.Scan(&email, &bio, &status, &createdAt, &lastSeenAt)
	return
}
func (db *DB) GetUserPushStatus(user string) bool {
	var e bool
	db.QueryRow(`SELECT push_enabled FROM user_tokens WHERE username=$1`, user).Scan(&e)
	return e
}
func (db *DB) GetUserToken(user string) (string, error) {
	var t string
	db.QueryRow(`SELECT fcm_token FROM user_tokens WHERE username=$1`, user).Scan(&t)
	return t, nil
}

func (db *DB) GetUserTokenByUserID(uid string) (string, error) {
	var t string
	err := db.QueryRow(`SELECT fcm_token FROM user_tokens ut JOIN users u ON ut.username = u.username WHERE u.id = $1::uuid`, uid).Scan(&t)
	return t, err
}
func (db *DB) SaveUserToken(user, token string, e bool) error {
	_, err := db.Exec(`INSERT INTO user_tokens (username, fcm_token, push_enabled, updated_at) VALUES ($1, $2, $3, NOW()) ON CONFLICT (username) DO UPDATE SET fcm_token=EXCLUDED.fcm_token, updated_at=NOW()`, user, token, e)
	return err
}

func (db *DB) SaveUserTokenByUserID(userID, token string, e bool) error {
	_, err := db.Exec(`INSERT INTO user_tokens (user_id, username, fcm_token, push_enabled, updated_at)
		VALUES ($1::uuid, (SELECT username FROM users WHERE id=$1::uuid), $2, $3, NOW())
		ON CONFLICT (username) DO UPDATE SET user_id=EXCLUDED.user_id, fcm_token=EXCLUDED.fcm_token, updated_at=NOW()`, userID, token, e)
	return err
}

func (db *DB) GetUserPushStatusByUserID(userID string) bool {
	var e bool
	db.QueryRow(`SELECT push_enabled FROM user_tokens WHERE user_id=$1::uuid OR username=(SELECT username FROM users WHERE id=$1::uuid)`, userID).Scan(&e)
	return e
}

func (db *DB) SetUserPushStatusByUserID(userID string, enabled bool) error {
	_, err := db.Exec(`UPDATE user_tokens SET push_enabled=$1 WHERE user_id=$2::uuid OR username=(SELECT username FROM users WHERE id=$2::uuid)`, enabled, userID)
	return err
}
func (db *DB) GetMessageImageURL(mid string) (string, error) {
	var u string
	db.QueryRow(`SELECT COALESCE(image_url,'') FROM messages WHERE message_id=$1`, mid).Scan(&u)
	return u, nil
}
func (db *DB) GetChatMessagesImageURLs(room string) ([]string, error) {
	rows, _ := db.Query(`SELECT image_url FROM messages WHERE room_id=$1 AND image_url!=''`, room)
	var res []string
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var u string
			rows.Scan(&u)
			res = append(res, u)
		}
	}
	return res, nil
}
func (db *DB) GetFavorites(uid string) ([]struct {
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
}, error) {
	// Try to resolve username from UUID if uid looks like a UUID
	username := uid
	if len(uid) > 20 {
		_ = db.QueryRow(`SELECT username FROM users WHERE id = $1::uuid OR username = $1`, uid).Scan(&username)
	}

	q := `SELECT COALESCE(m.message_id, ''), m.username, m.encrypted_text, COALESCE(f.created_at, m.created_at), COALESCE(m.replied_to_message_id, ''), COALESCE(m.replied_to_user, ''), COALESCE(m.replied_to_text, ''), COALESCE(m.room_id, ''), COALESCE(m.is_read, FALSE) as is_read, COALESCE(u.avatar_url, ''), COALESCE(m.image_url, ''), COALESCE(m.image_urls, '[]'), COALESCE(m.edited, false), COALESCE(m.voice_url, ''), COALESCE(m.duration, 0), COALESCE(m.is_e2ee, false)
	      FROM messages m
	      LEFT JOIN users u ON m.user_id = u.id
	      LEFT JOIN favorites f ON f.message_id = m.message_id AND f.user_id = (SELECT id FROM users WHERE username = $1::text)
	      WHERE m.room_id = 'favorites_' || $1::text OR (f.message_id IS NOT NULL AND f.user_id = (SELECT id FROM users WHERE username = $1::text))
	      ORDER BY 4 ASC`
	rows, err := db.Query(q, username)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []struct {
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
	for rows.Next() {
		var r struct {
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
		rows.Scan(&r.MessageID, &r.Username, &r.Encrypted, &r.CreatedAt, &r.RepliedToMessageID, &r.RepliedToUser, &r.RepliedToText, &r.RoomID, &r.IsRead, &r.AvatarURL, &r.ImageURL, &r.ImageURLs, &r.Edited, &r.VoiceURL, &r.Duration, &r.IsE2EE)
		res = append(res, r)
	}
	return res, nil
}
func (db *DB) AddFavorite(uid, mid string) error {
	// Если передан не UUID (а имя пользователя), сначала получим его ID
	query := `INSERT INTO favorites (user_id, message_id)
	          VALUES (CASE WHEN $1 ~ '^[0-9a-fA-F-]{36}$' THEN $1::uuid ELSE (SELECT id FROM users WHERE username=$1::text) END, $2)
	          ON CONFLICT DO NOTHING`
	_, err := db.Exec(query, uid, mid)
	return err
}

func (db *DB) RemoveFavorite(uid, mid string) error {
	query := `DELETE FROM favorites
	          WHERE user_id = (CASE WHEN $1 ~ '^[0-9a-fA-F-]{36}$' THEN $1::uuid ELSE (SELECT id FROM users WHERE username=$1::text) END)
	          AND message_id = $2`
	_, err := db.Exec(query, uid, mid)
	return err
}
func (db *DB) CleanupEmptyMessages() (int64, error) {
	q := `DELETE FROM messages WHERE encrypted_text = 'DECRYPTION_FAILED'::bytea OR encrypted_text = 'CORRUPTED_FIX'::bytea`
	res, err := db.Exec(q)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
func (db *DB) GetChatMessages(room string) ([]struct {
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
}, error) {
	return db.GetMessages(100, room)
}
func (db *DB) GetFavoritesMessages(uid string) ([]struct {
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
}, error) {
	return db.GetFavorites(uid)
}
func (db *DB) GetUserAvatarWithFullURL(user string) (string, string, error) {
	return db.GetUserAvatarWithFull(user)
}
func (db *DB) GetUserPassword(user string) (string, error) { return db.GetUserPasswordHash(user) }
func (db *DB) GetUserId(user string) (string, error)       { return db.GetUserIdByUsername(user) }

func (db *DB) AddUserDevice(userID, deviceID, deviceName, clientVersion, ipAddress string) error {
	_, err := db.Exec(`
		INSERT INTO user_devices (user_id, device_id, device_name, client_version, ip_address, last_seen_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
		ON CONFLICT (user_id, device_id) DO UPDATE SET
			device_name = EXCLUDED.device_name,
			client_version = EXCLUDED.client_version,
			ip_address = EXCLUDED.ip_address,
			is_active = TRUE,
			last_seen_at = NOW()
	`, userID, deviceID, deviceName, clientVersion, ipAddress)
	return err
}

func (db *DB) GetUserDevices(userId string) ([]struct {
	DeviceID, DeviceName, ClientVersion, IPAddress string
	LastSeenAt                                     time.Time
}, error) {
	rows, err := db.Query(`SELECT device_id, COALESCE(device_name, ''), COALESCE(client_version, ''), COALESCE(ip_address, ''), last_seen_at FROM user_devices WHERE user_id = $1::uuid ORDER BY last_seen_at DESC`, userId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []struct {
		DeviceID, DeviceName, ClientVersion, IPAddress string
		LastSeenAt                                     time.Time
	}
	for rows.Next() {
		var d struct {
			DeviceID, DeviceName, ClientVersion, IPAddress string
			LastSeenAt                                     time.Time
		}
		rows.Scan(&d.DeviceID, &d.DeviceName, &d.ClientVersion, &d.IPAddress, &d.LastSeenAt)
		res = append(res, d)
	}
	return res, nil
}

func (db *DB) DeleteUserDevice(deviceID, userId string) error {
	_, err := db.Exec(`DELETE FROM user_devices WHERE device_id = $1 AND user_id = $2::uuid`, deviceID, userId)
	return err
}

func (db *DB) DeleteOtherDevices(userId, exceptDeviceId string) error {
	_, err := db.Exec(`DELETE FROM user_devices WHERE user_id = $1::uuid AND device_id != $2`, userId, exceptDeviceId)
	return err
}

func (db *DB) GetUserIdByEmail(email string) (string, error) {
	var id string
	err := db.QueryRow(`SELECT id::text FROM users WHERE email=$1`, email).Scan(&id)
	return id, err
}

func (db *DB) CreatePasswordResetToken(token, userId string, expiresAt time.Time) error {
	_, err := db.Exec(`INSERT INTO password_reset_tokens (token, user_id, expires_at) VALUES ($1, $2::uuid, $3)`, token, userId, expiresAt)
	return err
}

func (db *DB) ValidatePasswordResetToken(token string) (string, error) {
	var userId string
	var expiresAt time.Time
	err := db.QueryRow(`SELECT user_id::text, expires_at FROM password_reset_tokens WHERE token=$1`, token).Scan(&userId, &expiresAt)
	if err != nil {
		return "", err
	}
	if time.Now().After(expiresAt) {
		return "", fmt.Errorf("token expired")
	}
	return userId, nil
}

func (db *DB) DeletePasswordResetToken(token string) error {
	_, err := db.Exec(`DELETE FROM password_reset_tokens WHERE token=$1`, token)
	return err
}

func (db *DB) CreateCall(callerID, receiverID, callType, roomID string) (string, error) {
	var id string
	err := db.QueryRow(`INSERT INTO calls (caller_id, receiver_id, type, room_id, status) VALUES (
		CASE WHEN $1 ~ '^[0-9a-fA-F-]{36}$' THEN $1::uuid ELSE (SELECT id FROM users WHERE username=$1::text) END,
		CASE WHEN $2 ~ '^[0-9a-fA-F-]{36}$' THEN $2::uuid ELSE (SELECT id FROM users WHERE username=$2::text) END,
		$3, $4, 'pending') RETURNING id`, callerID, receiverID, callType, roomID).Scan(&id)
	return id, err
}

func (db *DB) UpdateCallStatus(callID, status string) error {
	var query string
	if status == "active" {
		query = `UPDATE calls SET status = $1, started_at = NOW() WHERE id = $2::uuid`
	} else if status == "completed" || status == "rejected" || status == "missed" || status == "busy" {
		query = `UPDATE calls SET status = $1, ended_at = NOW() WHERE id = $2::uuid`
	} else {
		query = `UPDATE calls SET status = $1 WHERE id = $2::uuid`
	}
	_, err := db.Exec(query, status, callID)
	return err
}

func (db *DB) GetCallDuration(callID string) (int, error) {
	var duration float64
	err := db.QueryRow(`SELECT EXTRACT(EPOCH FROM (ended_at - started_at)) FROM calls WHERE id = $1::uuid AND started_at IS NOT NULL AND ended_at IS NOT NULL`, callID).Scan(&duration)
	return int(duration), err
}

// GetActiveCallsByUser returns all active/pending calls where user is caller or receiver
func (db *DB) GetActiveCallsByUser(userID string) ([]struct {
	CallID    string
	CallerID  string
	ReceiverID string
	RoomID    string
}, error) {
	rows, err := db.Query(`SELECT id, caller_id::text, receiver_id::text, COALESCE(room_id, '') FROM calls WHERE (caller_id = $1::uuid OR receiver_id = $1::uuid) AND status IN ('pending', 'active')`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var calls []struct {
		CallID    string
		CallerID  string
		ReceiverID string
		RoomID    string
	}
	for rows.Next() {
		var c struct {
			CallID    string
			CallerID  string
			ReceiverID string
			RoomID    string
		}
		if err := rows.Scan(&c.CallID, &c.CallerID, &c.ReceiverID, &c.RoomID); err == nil {
			calls = append(calls, c)
		}
	}
	return calls, nil
}

// ======= Secret Chat Methods =======

func (db *DB) CreateSecretChat(id, name, creator string, participants []string) error {
	participantsJSON, _ := json.Marshal(participants)
	_, err := db.Exec(`INSERT INTO chats (id, name, type, participants, creator_username, is_secret) VALUES ($1, $2, 'secret', $3, $4, TRUE)`, id, name, string(participantsJSON), creator)
	if err == nil {
		_ = db.IncrementParticipantsChatListVersion(id)
	}
	return err
}

func (db *DB) GetSecretChat(chatID string) (struct {
	ID           string
	Name         string
	Type         string
	Participants string
	Creator      string
	IsSecret     bool
	PublicKeyA   string
	PublicKeyB   string
	E2EEReady    bool
	CreatedAt    time.Time
}, error) {
	var c struct {
		ID           string
		Name         string
		Type         string
		Participants string
		Creator      string
		IsSecret     bool
		PublicKeyA   string
		PublicKeyB   string
		E2EEReady    bool
		CreatedAt    time.Time
	}
	err := db.QueryRow(`SELECT id, name, type, participants, COALESCE(creator_username, ''), COALESCE(is_secret, FALSE), COALESCE(public_key_a, ''), COALESCE(public_key_b, ''), COALESCE(e2ee_ready, FALSE), created_at FROM chats WHERE id=$1`, chatID).Scan(&c.ID, &c.Name, &c.Type, &c.Participants, &c.Creator, &c.IsSecret, &c.PublicKeyA, &c.PublicKeyB, &c.E2EEReady, &c.CreatedAt)
	return c, err
}

func (db *DB) StoreSecretChatKey(chatID, userID, publicKey string) error {
	_, err := db.Exec(`INSERT INTO secret_chat_keys (chat_id, user_id, public_key) VALUES ($1, $2::uuid, $3) ON CONFLICT (chat_id, user_id) DO UPDATE SET public_key = EXCLUDED.public_key, created_at = NOW()`, chatID, userID, publicKey)
	return err
}

func (db *DB) GetSecretChatKey(chatID, userID string) (string, error) {
	var publicKey string
	err := db.QueryRow(`SELECT public_key FROM secret_chat_keys WHERE chat_id=$1 AND user_id=$2::uuid`, chatID, userID).Scan(&publicKey)
	return publicKey, err
}

func (db *DB) GetAllSecretChatKeys(chatID string) (map[string]string, error) {
	rows, err := db.Query(`SELECT user_id::text, public_key FROM secret_chat_keys WHERE chat_id=$1`, chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	keys := make(map[string]string)
	for rows.Next() {
		var userID, publicKey string
		if err := rows.Scan(&userID, &publicKey); err == nil {
			keys[userID] = publicKey
		}
	}
	return keys, nil
}

func (db *DB) SetSecretChatE2EEReady(chatID string, ready bool) error {
	_, err := db.Exec(`UPDATE chats SET e2ee_ready=$1 WHERE id=$2`, ready, chatID)
	return err
}

// ======= Server Methods =======

func (db *DB) CreateServer(name, host string, port int, isDefault bool) (string, error) {
	var id string
	err := db.QueryRow(`INSERT INTO servers (name, host, port, is_default) VALUES ($1, $2, $3, $4) RETURNING id`, name, host, port, isDefault).Scan(&id)
	return id, err
}

func (db *DB) GetAllServers() ([]struct {
	ID          string
	Name        string
	Host        string
	Port        int
	IsDefault   bool
	IsProtected bool
	CreatedAt   time.Time
}, error) {
	rows, err := db.Query(`SELECT id, name, host, port, is_default, COALESCE(is_protected, FALSE), created_at FROM servers ORDER BY is_default DESC, created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []struct {
		ID          string
		Name        string
		Host        string
		Port        int
		IsDefault   bool
		IsProtected bool
		CreatedAt   time.Time
	}
	for rows.Next() {
		var s struct {
			ID          string
			Name        string
			Host        string
			Port        int
			IsDefault   bool
			IsProtected bool
			CreatedAt   time.Time
		}
		rows.Scan(&s.ID, &s.Name, &s.Host, &s.Port, &s.IsDefault, &s.IsProtected, &s.CreatedAt)
		res = append(res, s)
	}
	return res, nil
}

func (db *DB) GetDefaultServer() (struct {
	ID   string
	Name string
	Host string
	Port int
}, error) {
	var s struct {
		ID   string
		Name string
		Host string
		Port int
	}
	err := db.QueryRow(`SELECT id, name, host, port FROM servers WHERE is_default = TRUE LIMIT 1`).Scan(&s.ID, &s.Name, &s.Host, &s.Port)
	return s, err
}

func (db *DB) UpdateServer(id, name, host string, port int) error {
	_, err := db.Exec(`UPDATE servers SET name=$1, host=$2, port=$3 WHERE id=$4`, name, host, port, id)
	return err
}

func (db *DB) SetDefaultServer(id string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE servers SET is_default = FALSE`); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE servers SET is_default = TRUE WHERE id = $1`, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (db *DB) DeleteServer(id string) error {
	var isDefault, isProtected bool
	err := db.QueryRow(`SELECT is_default, COALESCE(is_protected, FALSE) FROM servers WHERE id = $1`, id).Scan(&isDefault, &isProtected)
	if err != nil {
		return err
	}
	if isDefault {
		return fmt.Errorf("cannot delete default server")
	}
	if isProtected {
		return fmt.Errorf("cannot delete protected server")
	}
	_, err = db.Exec(`DELETE FROM servers WHERE id = $1`, id)
	return err
}

// ======= Free OpenRouter Models =======

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

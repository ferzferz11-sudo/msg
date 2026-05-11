// Lavender Messenger - A secure messaging application
// Author: Pavel Davydov (ferz)

package main

import (
	"LavenderMessenger/gen"
	"database/sql"
	"fmt"
	"log"
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
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)
	if err = db.Ping(); err != nil {
		db.Close()
		return nil, err
	}

	queries := []string{
		`CREATE TABLE IF NOT EXISTS messages (id SERIAL PRIMARY KEY, message_id VARCHAR(255) UNIQUE, username VARCHAR(255) NOT NULL, encrypted_text BYTEA NOT NULL, created_at TIMESTAMP NOT NULL)`,
		`DO $$ BEGIN
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='messages' AND column_name='replied_to_message_id') THEN ALTER TABLE messages ADD COLUMN replied_to_message_id VARCHAR(255); END IF;
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='messages' AND column_name='replied_to_user') THEN ALTER TABLE messages ADD COLUMN replied_to_user VARCHAR(255); END IF;
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='messages' AND column_name='replied_to_text') THEN ALTER TABLE messages ADD COLUMN replied_to_text TEXT; END IF;
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='messages' AND column_name='room_id') THEN ALTER TABLE messages ADD COLUMN room_id VARCHAR(255) DEFAULT ''; END IF;
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='messages' AND column_name='is_read') THEN ALTER TABLE messages ADD COLUMN is_read BOOLEAN DEFAULT FALSE; END IF;
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='messages' AND column_name='image_url') THEN ALTER TABLE messages ADD COLUMN image_url VARCHAR(512) DEFAULT ''; END IF;
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='messages' AND column_name='edited') THEN ALTER TABLE messages ADD COLUMN edited BOOLEAN DEFAULT FALSE; END IF;
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='messages' AND column_name='voice_url') THEN ALTER TABLE messages ADD COLUMN voice_url VARCHAR(512); END IF;
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='messages' AND column_name='duration') THEN ALTER TABLE messages ADD COLUMN duration INTEGER DEFAULT 0; END IF;
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='messages' AND column_name='user_id') THEN ALTER TABLE messages ADD COLUMN user_id UUID; END IF;
		END $$;`,
		`CREATE TABLE IF NOT EXISTS users (id UUID PRIMARY KEY DEFAULT gen_random_uuid(), username VARCHAR(255) UNIQUE NOT NULL, password_hash VARCHAR(255) NOT NULL, created_at TIMESTAMP NOT NULL DEFAULT NOW())`,
		`DO $$ BEGIN
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='users' AND column_name='avatar_url') THEN ALTER TABLE users ADD COLUMN avatar_url VARCHAR(512); END IF;
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='users' AND column_name='full_avatar_url') THEN ALTER TABLE users ADD COLUMN full_avatar_url VARCHAR(512); END IF;
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='users' AND column_name='bio') THEN ALTER TABLE users ADD COLUMN bio TEXT; END IF;
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='users' AND column_name='status') THEN ALTER TABLE users ADD COLUMN status VARCHAR(255); END IF;
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='users' AND column_name='chat_list_version') THEN ALTER TABLE users ADD COLUMN chat_list_version BIGINT DEFAULT 0; END IF;
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='users' AND column_name='current_theme_id') THEN ALTER TABLE users ADD COLUMN current_theme_id VARCHAR(255) DEFAULT 'dark'; END IF;
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='users' AND column_name='is_super_admin') THEN ALTER TABLE users ADD COLUMN is_super_admin BOOLEAN DEFAULT FALSE; END IF;
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='users' AND column_name='last_client_version') THEN ALTER TABLE users ADD COLUMN last_client_version VARCHAR(50); END IF;
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='users' AND column_name='last_seen_at') THEN ALTER TABLE users ADD COLUMN last_seen_at TIMESTAMP; END IF;
		END $$;`,
		`CREATE TABLE IF NOT EXISTS chats (id VARCHAR(255) PRIMARY KEY, name VARCHAR(255) NOT NULL, type VARCHAR(50) NOT NULL, participants TEXT NOT NULL, creator_username VARCHAR(255), created_at TIMESTAMP NOT NULL DEFAULT NOW(), avatar_url TEXT DEFAULT '', full_avatar_url TEXT DEFAULT '')`,
		`CREATE TABLE IF NOT EXISTS reactions (id SERIAL PRIMARY KEY, message_id VARCHAR(255) NOT NULL REFERENCES messages(message_id) ON DELETE CASCADE, username VARCHAR(255) NOT NULL, emoji VARCHAR(50) NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS user_chat_metadata (username VARCHAR(255) NOT NULL, room_id VARCHAR(255) NOT NULL, last_read_at TIMESTAMP NOT NULL DEFAULT NOW(), PRIMARY KEY (username, room_id))`,
		`CREATE TABLE IF NOT EXISTS user_tokens (username VARCHAR(255) PRIMARY KEY, fcm_token TEXT NOT NULL, updated_at TIMESTAMP NOT NULL, push_enabled BOOLEAN DEFAULT TRUE)`,
		`CREATE TABLE IF NOT EXISTS contacts (id SERIAL PRIMARY KEY, username VARCHAR(255) NOT NULL, contact_username VARCHAR(255) NOT NULL, created_at TIMESTAMP NOT NULL DEFAULT NOW())`,
		`CREATE TABLE IF NOT EXISTS user_themes (id UUID PRIMARY KEY DEFAULT gen_random_uuid(), username VARCHAR(255) NOT NULL REFERENCES users(username) ON DELETE CASCADE, theme_id VARCHAR(255) NOT NULL, name VARCHAR(255) NOT NULL, primary_color VARCHAR(10), on_primary_color VARCHAR(10), surface_color VARCHAR(10), on_surface_color VARCHAR(10), background_color VARCHAR(10), text_primary_color VARCHAR(10), text_secondary_color VARCHAR(10), is_dark BOOLEAN DEFAULT FALSE, chat_background_image_url VARCHAR(512), chat_list_background_image_url VARCHAR(512), bottom_panel_color VARCHAR(10), on_bottom_panel_color VARCHAR(10), surface_container VARCHAR(10), outgoing_bubble_color VARCHAR(10), incoming_bubble_color VARCHAR(10), UNIQUE(username, theme_id))`,
		`CREATE TABLE IF NOT EXISTS draft_messages (username VARCHAR(255) NOT NULL, room_id VARCHAR(255) NOT NULL, draft_text TEXT NOT NULL DEFAULT '', updated_at TIMESTAMP NOT NULL DEFAULT NOW(), PRIMARY KEY (username, room_id))`,
		`DO $$ BEGIN
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='draft_messages' AND column_name='replied_to_message_id') THEN ALTER TABLE draft_messages ADD COLUMN replied_to_message_id VARCHAR(255); END IF;
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='draft_messages' AND column_name='replied_to_user') THEN ALTER TABLE draft_messages ADD COLUMN replied_to_user VARCHAR(255); END IF;
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='draft_messages' AND column_name='replied_to_text') THEN ALTER TABLE draft_messages ADD COLUMN replied_to_text TEXT; END IF;
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='draft_messages' AND column_name='user_id') THEN ALTER TABLE draft_messages ADD COLUMN user_id UUID; END IF;
		END $$;`,
		`CREATE TABLE IF NOT EXISTS muted_chats (username VARCHAR(255) NOT NULL, room_id VARCHAR(255) NOT NULL, muted BOOLEAN NOT NULL DEFAULT TRUE, updated_at TIMESTAMP NOT NULL DEFAULT NOW(), user_id UUID, PRIMARY KEY (username, room_id))`,
		`CREATE TABLE IF NOT EXISTS favorites (user_id UUID REFERENCES users(id) ON DELETE CASCADE, message_id VARCHAR(255) REFERENCES messages(message_id) ON DELETE CASCADE, created_at TIMESTAMP NOT NULL DEFAULT NOW(), PRIMARY KEY (user_id, message_id))`,
	}

	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			log.Printf("Migration error: %v", err)
		}
	}
	db.Exec(`UPDATE users SET is_super_admin = TRUE WHERE username = 'ferz'`)
	return &DB{db}, nil
}

func (db *DB) Close() error { return db.DB.Close() }

func (db *DB) SaveMessage(mid, user string, enc []byte, created time.Time, rmid, ruser, rtext, room, img, voice string, dur int32) error {
	isRead := strings.HasPrefix(room, "favorites_")
	q := `INSERT INTO messages (message_id, username, user_id, encrypted_text, created_at, replied_to_message_id, replied_to_user, replied_to_text, room_id, is_read, image_url, voice_url, duration) VALUES ($1, $2::text, (SELECT id FROM users WHERE username=$2::text), $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`
	_, err := db.Exec(q, mid, user, enc, created, rmid, ruser, rtext, room, isRead, img, voice, dur)
	if err == nil && room != "" {
		db.IncrementParticipantsChatListVersion(room)
	}
	return err
}

func (db *DB) GetMessages(limit int, room string) ([]struct {
	MessageID, Username                                      string
	Encrypted                                                []byte
	CreatedAt                                                time.Time
	RepliedToMessageID, RepliedToUser, RepliedToText, RoomID string
	IsRead                                                   bool
	AvatarURL, ImageURL                                      string
	Edited                                                   bool
	VoiceURL                                                 string
	Duration                                                 int32
}, error) {
	var rows *sql.Rows
	var err error
	if strings.HasPrefix(room, "favorites_") {
		username := strings.TrimPrefix(room, "favorites_")
		q := `SELECT COALESCE(m.message_id, ''), m.username, m.encrypted_text, COALESCE(f.created_at, m.created_at), COALESCE(m.replied_to_message_id, ''), COALESCE(m.replied_to_user, ''), COALESCE(m.replied_to_text, ''), COALESCE(m.room_id, ''), TRUE as is_read, COALESCE(u.avatar_url, ''), COALESCE(m.image_url, ''), COALESCE(m.edited, false), COALESCE(m.voice_url, ''), COALESCE(m.duration, 0) FROM messages m LEFT JOIN users u ON m.username = u.username LEFT JOIN favorites f ON f.message_id = m.message_id AND f.user_id = (SELECT id FROM users WHERE username = $1) WHERE m.room_id = $2 OR f.message_id IS NOT NULL ORDER BY 4 ASC LIMIT $3`
		rows, err = db.Query(q, username, room, limit)
	} else {
		q := `SELECT COALESCE(m.message_id, ''), m.username, m.encrypted_text, m.created_at, COALESCE(m.replied_to_message_id, ''), COALESCE(m.replied_to_user, ''), COALESCE(m.replied_to_text, ''), COALESCE(m.room_id, ''), m.is_read, COALESCE(u.avatar_url, ''), COALESCE(m.image_url, ''), COALESCE(m.edited, false), COALESCE(m.voice_url, ''), COALESCE(m.duration, 0) FROM messages m LEFT JOIN users u ON m.username = u.username WHERE m.room_id = $1 ORDER BY m.created_at DESC LIMIT $2`
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
		AvatarURL, ImageURL                                      string
		Edited                                                   bool
		VoiceURL                                                 string
		Duration                                                 int32
	}
	for rows.Next() {
		var r struct {
			MessageID, Username                                      string
			Encrypted                                                []byte
			CreatedAt                                                time.Time
			RepliedToMessageID, RepliedToUser, RepliedToText, RoomID string
			IsRead                                                   bool
			AvatarURL, ImageURL                                      string
			Edited                                                   bool
			VoiceURL                                                 string
			Duration                                                 int32
		}
		rows.Scan(&r.MessageID, &r.Username, &r.Encrypted, &r.CreatedAt, &r.RepliedToMessageID, &r.RepliedToUser, &r.RepliedToText, &r.RoomID, &r.IsRead, &r.AvatarURL, &r.ImageURL, &r.Edited, &r.VoiceURL, &r.Duration)
		res = append(res, r)
	}
	return res, nil
}

func (db *DB) SetReaction(mid, user, emoji string) error {
	q := `INSERT INTO reactions (message_id, username, emoji) VALUES ($1, $2, $3) ON CONFLICT (message_id, username) DO UPDATE SET emoji = EXCLUDED.emoji`
	_, err := db.Exec(q, mid, user, emoji)
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

func (db *DB) GetUserPasswordHash(user string) (string, error) {
	var h string
	err := db.QueryRow(`SELECT password_hash FROM users WHERE username=$1`, user).Scan(&h)
	return h, err
}

func (db *DB) SaveUser(user, hash string) error {
	_, err := db.Exec(`INSERT INTO users (username, password_hash) VALUES ($1, $2)`, user, hash)
	return err
}

func (db *DB) IsSuperAdmin(user string) bool {
	var a bool
	db.QueryRow(`SELECT is_super_admin FROM users WHERE username=$1`, user).Scan(&a)
	return a
}

func (db *DB) GetAllUsers() ([]struct {
	Username, AvatarURL, LastClientVersion string
	LastSeenAt                             sql.NullTime
}, error) {
	rows, err := db.Query(`SELECT username, COALESCE(avatar_url, ''), COALESCE(last_client_version, ''), last_seen_at FROM users ORDER BY username`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []struct {
		Username, AvatarURL, LastClientVersion string
		LastSeenAt                             sql.NullTime
	}
	for rows.Next() {
		var u struct {
			Username, AvatarURL, LastClientVersion string
			LastSeenAt                             sql.NullTime
		}
		rows.Scan(&u.Username, &u.AvatarURL, &u.LastClientVersion, &u.LastSeenAt)
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
	AvatarURL, ImageURL                                      string
	Edited                                                   bool
	VoiceURL                                                 string
	Duration                                                 int32
}, error) {
	var r struct {
		MessageID, Username                                      string
		Encrypted                                                []byte
		CreatedAt                                                time.Time
		RepliedToMessageID, RepliedToUser, RepliedToText, RoomID string
		IsRead                                                   bool
		AvatarURL, ImageURL                                      string
		Edited                                                   bool
		VoiceURL                                                 string
		Duration                                                 int32
	}
	err := db.QueryRow(`SELECT COALESCE(m.message_id, ''), m.username, m.encrypted_text, m.created_at, COALESCE(m.replied_to_message_id, ''), COALESCE(m.replied_to_user, ''), COALESCE(m.replied_to_text, ''), COALESCE(m.room_id, ''), TRUE as is_read, COALESCE(u.avatar_url, ''), COALESCE(m.image_url, ''), COALESCE(m.edited, false), COALESCE(m.voice_url, ''), COALESCE(m.duration, 0) FROM messages m LEFT JOIN users u ON m.username = u.username WHERE m.message_id = $1`, id).Scan(&r.MessageID, &r.Username, &r.Encrypted, &r.CreatedAt, &r.RepliedToMessageID, &r.RepliedToUser, &r.RepliedToText, &r.RoomID, &r.IsRead, &r.AvatarURL, &r.ImageURL, &r.Edited, &r.VoiceURL, &r.Duration)
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
	ID                          int
	Encrypted                   []byte
	ImageURL, MessageID, RoomID string
}, error) {
	rows, err := db.Query(`SELECT id, encrypted_text, COALESCE(image_url, ''), message_id, room_id FROM messages WHERE username = $1 AND created_at >= $2 AND created_at <= $3`, u, t.Add(-2*time.Second), t.Add(2*time.Second))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []struct {
		ID                          int
		Encrypted                   []byte
		ImageURL, MessageID, RoomID string
	}
	for rows.Next() {
		var r struct {
			ID                          int
			Encrypted                   []byte
			ImageURL, MessageID, RoomID string
		}
		rows.Scan(&r.ID, &r.Encrypted, &r.ImageURL, &r.MessageID, &r.RoomID)
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
}, error) {
	var c struct {
		ID, Name, Type, Participants, CreatorUsername string
		CreatedAt                                     time.Time
	}
	err := db.QueryRow(`SELECT id, name, type, participants, COALESCE(creator_username, ''), created_at FROM chats WHERE id=$1`, id).Scan(&c.ID, &c.Name, &c.Type, &c.Participants, &c.CreatorUsername, &c.CreatedAt)
	return c, err
}

func (db *DB) GetAllChats() ([]struct {
	ID, Name, Type, Participants, Creator, LastMessageText, AvatarURL, LastMessageUsername string
	CreatedAt, LastMessageTime                                                             time.Time
	UnreadCount                                                                            int
}, error) {
	query := `WITH last_messages AS (SELECT DISTINCT ON (room_id) room_id, created_at, encrypted_text, username FROM messages ORDER BY room_id, created_at DESC) SELECT c.id, c.name, c.type, c.participants, c.created_at, COALESCE(c.creator_username, ''), COALESCE(lm.created_at, c.created_at), COALESCE(lm.encrypted_text, ''::bytea), COALESCE(c.avatar_url, ''), COALESCE(lm.username, '') FROM chats c LEFT JOIN last_messages lm ON c.id = lm.room_id ORDER BY 7 DESC`
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []struct {
		ID, Name, Type, Participants, Creator, LastMessageText, AvatarURL, LastMessageUsername string
		CreatedAt, LastMessageTime                                                             time.Time
		UnreadCount                                                                            int
	}
	for rows.Next() {
		var c struct {
			ID, Name, Type, Participants, Creator, Avatar, LastUser string
			CreatedAt, LastTime                                     time.Time
			Enc                                                     []byte
		}
		rows.Scan(&c.ID, &c.Name, &c.Type, &c.Participants, &c.CreatedAt, &c.Creator, &c.LastTime, &c.Enc, &c.Avatar, &c.LastUser)
		txt, _ := decrypt(c.Enc)
		res = append(res, struct {
			ID, Name, Type, Participants, Creator, LastMessageText, AvatarURL, LastMessageUsername string
			CreatedAt, LastMessageTime                                                             time.Time
			UnreadCount                                                                            int
		}{c.ID, c.Name, c.Type, c.Participants, c.Creator, txt, c.Avatar, c.LastUser, c.CreatedAt, c.LastTime, 0})
	}
	return res, nil
}

func (db *DB) GetUserChats(uid, user string) ([]struct {
	ID, Name, Type, Participants, Creator, LastMessageText, AvatarURL, LastMessageUsername string
	CreatedAt, LastMessageTime                                                             time.Time
	UnreadCount                                                                            int
}, error) {
	query := `WITH last_messages AS (SELECT DISTINCT ON (room_id) room_id, created_at, encrypted_text, username FROM messages ORDER BY room_id, created_at DESC), unread_counts AS (SELECT room_id, COUNT(*) as count FROM messages WHERE is_read = FALSE AND username != $1 GROUP BY room_id) SELECT c.id, c.name, c.type, c.participants, c.created_at, COALESCE(uc.count, 0), COALESCE(lm.created_at, c.created_at), COALESCE(c.creator_username, ''), COALESCE(lm.encrypted_text, ''::bytea), COALESCE(c.avatar_url, ''), COALESCE(lm.username, '') FROM chats c LEFT JOIN last_messages lm ON c.id = lm.room_id LEFT JOIN unread_counts uc ON c.id = uc.room_id WHERE c.participants::jsonb @> jsonb_build_array($2::text) ORDER BY 7 DESC`
	rows, err := db.Query(query, user, user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []struct {
		ID, Name, Type, Participants, Creator, LastMessageText, AvatarURL, LastMessageUsername string
		CreatedAt, LastMessageTime                                                             time.Time
		UnreadCount                                                                            int
	}
	for rows.Next() {
		var c struct {
			ID, Name, Type, Participants, Creator, Avatar, LastUser string
			CreatedAt, LastTime                                     time.Time
			Enc                                                     []byte
			Unread                                                  int
		}
		rows.Scan(&c.ID, &c.Name, &c.Type, &c.Participants, &c.CreatedAt, &c.Unread, &c.LastTime, &c.Creator, &c.Enc, &c.Avatar, &c.LastUser)
		txt, _ := decrypt(c.Enc)
		res = append(res, struct {
			ID, Name, Type, Participants, Creator, LastMessageText, AvatarURL, LastMessageUsername string
			CreatedAt, LastMessageTime                                                             time.Time
			UnreadCount                                                                            int
		}{c.ID, c.Name, c.Type, c.Participants, c.Creator, txt, c.Avatar, c.LastUser, c.CreatedAt, c.LastTime, c.Unread})
	}
	return res, nil
}

func (db *DB) UpdateUsername(old, new string) error {
	tx, _ := db.Begin()
	defer tx.Rollback()
	tx.Exec(`UPDATE users SET username=$1 WHERE username=$2`, new, old)
	tx.Exec(`UPDATE messages SET username=$1 WHERE username=$2`, new, old)
	tx.Exec(`UPDATE messages SET replied_to_user=$1 WHERE replied_to_user=$2`, new, old)
	tx.Exec(`UPDATE chats SET creator_username=$1 WHERE creator_username=$2`, new, old)
	tx.Exec(`UPDATE user_tokens SET username=$1 WHERE username=$2`, new, old)
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

	// Update last_read_at in metadata to clear unread count in chat list
	tx.Exec(`INSERT INTO user_chat_metadata (username, room_id, last_read_at, user_id)
	          VALUES ($1::text, $2, NOW(), (SELECT id FROM users WHERE username=$1::text))
	          ON CONFLICT (username, room_id) DO UPDATE SET last_read_at=NOW()`, user, room)

	// Mark messages as read in the room
	res, err := tx.Exec(`UPDATE messages SET is_read=TRUE WHERE room_id=$1 AND username!=$2 AND is_read=FALSE`, room, user)
	if err != nil {
		return false, err
	}

	affected, _ := res.RowsAffected()

	err = tx.Commit()
	if err == nil && affected > 0 {
		_ = db.IncrementUserChatListVersion(user)
	}
	return affected > 0, err
}

func (db *DB) CreateChat(id, name, t, p, c string) error {
	_, err := db.Exec(`INSERT INTO chats (id, name, type, participants, creator_username) VALUES ($1, $2, $3, $4, $5)`, id, name, t, p, c)
	if err == nil {
		_ = db.IncrementParticipantsChatListVersion(id)
	}
	return err
}

func (db *DB) GetDirectChatBetweenUsers(u1, u2 string) (string, error) {
	var id string
	err := db.QueryRow(`SELECT id FROM chats WHERE type='direct' AND participants::jsonb @> jsonb_build_array($1::text, $2::text)`, u1, u2).Scan(&id)
	if err == nil {
		return id, nil
	}
	id = u1 + "_" + u2 + "_direct"
	if u1 > u2 {
		id = u2 + "_" + u1 + "_direct"
	}
	db.CreateChat(id, u1+" & "+u2, "direct", `["`+u1+`","`+u2+`"]`, u1)
	return id, nil
}

func (db *DB) DeleteProfile(user string) error {
	_, err := db.Exec(`DELETE FROM users WHERE username=$1`, user)
	return err
}
func (db *DB) GetUserProfile(user string) (struct{ Username, Bio, Status, AvatarURL string }, error) {
	var p struct{ Username, Bio, Status, AvatarURL string }
	err := db.QueryRow(`SELECT username, COALESCE(bio, ''), COALESCE(status, ''), COALESCE(avatar_url, '') FROM users WHERE username=$1`, user).Scan(&p.Username, &p.Bio, &p.Status, &p.AvatarURL)
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
func (db *DB) UpdateChatParticipants(id, p string) error {
	_, err := db.Exec(`UPDATE chats SET participants=$1 WHERE id=$2`, p, id)
	return err
}
func (db *DB) DeleteChat(id string) error {
	_, err := db.Exec(`DELETE FROM chats WHERE id=$1`, id)
	return err
}
func (db *DB) AddContact(user, contact string) error {
	_, err := db.Exec(`INSERT INTO contacts (username, contact_username) VALUES ($1, $2) ON CONFLICT DO NOTHING`, user, contact)
	return err
}
func (db *DB) RemoveContact(user, contact string) error {
	_, err := db.Exec(`DELETE FROM contacts WHERE username=$1 AND contact_username=$2`, user, contact)
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
	_, err := db.Exec(`UPDATE users SET chat_list_version=chat_list_version+1 WHERE username IN (SELECT json_array_elements_text(participants::json) FROM chats WHERE id=$1)`, id)
	return err
}
func (db *DB) SaveUserTheme(user string, t *gen.CustomTheme) error { return nil }
func (db *DB) SetCurrentTheme(user, id string) error               { return nil }
func (db *DB) DeleteUserTheme(user, id string) error               { return nil }
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
func (db *DB) SaveUserToken(user, token string, e bool) error {
	_, err := db.Exec(`INSERT INTO user_tokens (username, fcm_token, push_enabled, updated_at) VALUES ($1, $2, $3, NOW()) ON CONFLICT (username) DO UPDATE SET fcm_token=EXCLUDED.fcm_token, updated_at=NOW()`, user, token, e)
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
	AvatarURL, ImageURL                                      string
	Edited                                                   bool
	VoiceURL                                                 string
	Duration                                                 int32
}, error) {
	// Try to resolve username from UUID if uid looks like a UUID
	username := uid
	if len(uid) > 20 {
		_ = db.QueryRow(`SELECT username FROM users WHERE id = $1::uuid OR username = $1`, uid).Scan(&username)
	}

	q := `SELECT COALESCE(m.message_id, ''), m.username, m.encrypted_text, COALESCE(f.created_at, m.created_at), COALESCE(m.replied_to_message_id, ''), COALESCE(m.replied_to_user, ''), COALESCE(m.replied_to_text, ''), COALESCE(m.room_id, ''), TRUE as is_read, COALESCE(u.avatar_url, ''), COALESCE(m.image_url, ''), COALESCE(m.edited, false), COALESCE(m.voice_url, ''), COALESCE(m.duration, 0)
	      FROM messages m
	      LEFT JOIN users u ON m.username = u.username
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
		AvatarURL, ImageURL                                      string
		Edited                                                   bool
		VoiceURL                                                 string
		Duration                                                 int32
	}
	for rows.Next() {
		var r struct {
			MessageID, Username                                      string
			Encrypted                                                []byte
			CreatedAt                                                time.Time
			RepliedToMessageID, RepliedToUser, RepliedToText, RoomID string
			IsRead                                                   bool
			AvatarURL, ImageURL                                      string
			Edited                                                   bool
			VoiceURL                                                 string
			Duration                                                 int32
		}
		rows.Scan(&r.MessageID, &r.Username, &r.Encrypted, &r.CreatedAt, &r.RepliedToMessageID, &r.RepliedToUser, &r.RepliedToText, &r.RoomID, &r.IsRead, &r.AvatarURL, &r.ImageURL, &r.Edited, &r.VoiceURL, &r.Duration)
		res = append(res, r)
	}
	return res, nil
}
func (db *DB) AddFavorite(uid, mid string) error    { return nil }
func (db *DB) RemoveFavorite(uid, mid string) error { return nil }
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
	AvatarURL, ImageURL                                      string
	Edited                                                   bool
	VoiceURL                                                 string
	Duration                                                 int32
}, error) {
	return db.GetMessages(100, room)
}
func (db *DB) GetFavoritesMessages(uid string) ([]struct {
	MessageID, Username                                      string
	Encrypted                                                []byte
	CreatedAt                                                time.Time
	RepliedToMessageID, RepliedToUser, RepliedToText, RoomID string
	IsRead                                                   bool
	AvatarURL, ImageURL                                      string
	Edited                                                   bool
	VoiceURL                                                 string
	Duration                                                 int32
}, error) {
	return db.GetFavorites(uid)
}
func (db *DB) GetUserAvatarWithFullURL(user string) (string, string, error) {
	return db.GetUserAvatarWithFull(user)
}
func (db *DB) GetUserPassword(user string) (string, error) { return db.GetUserPasswordHash(user) }
func (db *DB) GetUserId(user string) (string, error)       { return db.GetUserIdByUsername(user) }

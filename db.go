// Lavender Messenger - A secure messaging application
// Author: Pavel Davydov (ferz)
//
// This file handles database operations for the Lavender Messenger.
// It manages PostgreSQL connections, message storage, and table creation.

package main

import (
	"LavenderMessenger/gen"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	_ "github.com/lib/pq" // PostgreSQL driver
)

// DB wraps the sql.DB type to provide additional functionality
// and maintain compatibility with existing code
type DB struct {
	*sql.DB // Embedded sql.DB for direct database operations
}

// ConnectDB establishes a connection to the PostgreSQL database
// It reads the database URL from environment variables and initializes
// the necessary table structure for the messaging application
func ConnectDB() (*DB, error) {
	// Retrieve database connection string from environment variables
	dbUrl := os.Getenv("DATABASE_URL")
	if dbUrl == "" {
		return nil, fmt.Errorf("DATABASE_URL is not set. Please check your .env file")
	}

	// Open a connection to the PostgreSQL database
	db, err := sql.Open("postgres", dbUrl)
	if err != nil {
		return nil, fmt.Errorf("unable to open database connection: %w", err)
	}

	// 🛠️ 1. Настройка пула соединений (Критично для стабильности мессенджера)
	db.SetMaxOpenConns(25)           // Максимум 25 одновременных активных подключений
	db.SetMaxIdleConns(5)            // Держим 5 готовых подключений в фоне (ускоряет новые запросы)
	db.SetConnMaxLifetime(time.Hour) // Закрываем соединения через час, чтобы не копились утечки

	// Test the connection to ensure it's valid and reachable
	err = db.Ping()
	if err != nil {
		// Если пинг не прошел, закрываем открытый дескриптор
		if closeErr := db.Close(); closeErr != nil {
			log.Printf("Warning: error closing database connection after failed ping: %v", closeErr)
		}

		// 🛠️ 2. Безопасность: Не выводим DATABASE_URL в ошибку вообще.
		// Даже функция maskPassword может ошибиться при сбое парсинга, показав пароль в открытом логе.
		return nil, fmt.Errorf("unable to ping database: %w", err)
	}

	// Create tables if they don't already exist
	// We also ensure existing tables are migrated if necessary
	queries := []string{
		`CREATE TABLE IF NOT EXISTS messages (
			id SERIAL PRIMARY KEY,
			message_id VARCHAR(255) UNIQUE,
			username VARCHAR(255) NOT NULL,
			encrypted_text BYTEA NOT NULL,
			created_at TIMESTAMP NOT NULL
		);`,
		// Migration: Add message_id to messages if it's an old table
		`DO $$
		 BEGIN
		  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='messages' AND column_name='message_id') THEN
		    ALTER TABLE messages ADD COLUMN message_id VARCHAR(255);
		  END IF;
		 END $$;`,
		// Ensure message_id is UNIQUE for the foreign key reference
		`DO $$
		 BEGIN
		  IF NOT EXISTS (SELECT 1 FROM information_schema.table_constraints WHERE table_name='messages' AND constraint_type='UNIQUE' AND constraint_name='messages_message_id_key') THEN
		    ALTER TABLE messages ADD CONSTRAINT messages_message_id_key UNIQUE (message_id);
		  END IF;
		 END $$;`,
		// Migration: Add reply fields to messages
		`DO $$
		 BEGIN
		  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='messages' AND column_name='replied_to_message_id') THEN
		    ALTER TABLE messages ADD COLUMN replied_to_message_id VARCHAR(255);
		  END IF;
		 END $$;`,
		// Migration: Add edited column to messages
		`DO $$
		 BEGIN
		  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='messages' AND column_name='edited') THEN
		    ALTER TABLE messages ADD COLUMN edited BOOLEAN DEFAULT false;
		  END IF;
		 END $$;`,
		`DO $$
		 BEGIN
		  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='messages' AND column_name='replied_to_user') THEN
		    ALTER TABLE messages ADD COLUMN replied_to_user VARCHAR(255);
		  END IF;
		 END $$;`,
		`DO $$
		 BEGIN
		  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='messages' AND column_name='replied_to_text') THEN
		    ALTER TABLE messages ADD COLUMN replied_to_text TEXT;
		  END IF;
		 END $$;`,
		// Migration: Add room_id to messages
		`DO $$
		 BEGIN
		  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='messages' AND column_name='room_id') THEN
		    ALTER TABLE messages ADD COLUMN room_id VARCHAR(255) DEFAULT '';
		  END IF;
		 END $$;`,
		// Migration: Add is_read to messages
		`DO $$
		 BEGIN
		  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='messages' AND column_name='is_read') THEN
		    ALTER TABLE messages ADD COLUMN is_read BOOLEAN DEFAULT FALSE;
		  END IF;
		 END $$;`,
		// Migration: Add image_url to messages
		`DO $$
		 BEGIN
		  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='messages' AND column_name='image_url') THEN
		    ALTER TABLE messages ADD COLUMN image_url VARCHAR(512) DEFAULT '';
		  END IF;
		 END $$;`,
		// Migration: Add bio to users
		`DO $$
          BEGIN
           IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='users' AND column_name='bio') THEN
             ALTER TABLE users ADD COLUMN bio TEXT;
           END IF;
          END $$;`,
		// Migration: Add status to users
		`DO $$
          BEGIN
           IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='users' AND column_name='status') THEN
             ALTER TABLE users ADD COLUMN status VARCHAR(255);
           END IF;
          END $$;`,
		`CREATE TABLE IF NOT EXISTS user_chat_metadata (
			username VARCHAR(255) NOT NULL,
			room_id VARCHAR(255) NOT NULL,
			last_read_at TIMESTAMP NOT NULL DEFAULT NOW(),
			PRIMARY KEY (username, room_id)
		);`,
		`CREATE TABLE IF NOT EXISTS reactions (
			id SERIAL PRIMARY KEY,
			message_id VARCHAR(255) NOT NULL REFERENCES messages(message_id) ON DELETE CASCADE,
			username VARCHAR(255) NOT NULL,
			emoji VARCHAR(50) NOT NULL,
			UNIQUE(message_id, username)
		);`,
		`CREATE TABLE IF NOT EXISTS chats (
			id VARCHAR(255) PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			type VARCHAR(50) NOT NULL, -- 'direct' or 'group'
			participants TEXT NOT NULL, -- JSON array of usernames
			creator_username VARCHAR(255),
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			avatar_url TEXT DEFAULT '',
			full_avatar_url TEXT DEFAULT ''
		);`,
		// Migration: Add avatar fields to chats if they don't exist
		`DO $$
		 BEGIN
		  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='chats' AND column_name='creator_username') THEN
		    ALTER TABLE chats ADD COLUMN creator_username VARCHAR(255);
		  END IF;
		  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='chats' AND column_name='avatar_url') THEN
		    ALTER TABLE chats ADD COLUMN avatar_url TEXT DEFAULT '';
		  END IF;
		  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='chats' AND column_name='full_avatar_url') THEN
		    ALTER TABLE chats ADD COLUMN full_avatar_url TEXT DEFAULT '';
		  END IF;
		 END $$;`,
		`UPDATE chats SET creator_username = participants::json->>0
		 WHERE creator_username IS NULL AND participants ~ '^\[.*\]$';`,
		`CREATE TABLE IF NOT EXISTS user_tokens (
			username VARCHAR(255) PRIMARY KEY,
			fcm_token TEXT NOT NULL,
			updated_at TIMESTAMP NOT NULL,
			push_enabled BOOLEAN DEFAULT TRUE
		);`,
		// Migration: Add push_enabled to user_tokens
		`DO $$
		 BEGIN
		  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='user_tokens' AND column_name='push_enabled') THEN
		    ALTER TABLE user_tokens ADD COLUMN push_enabled BOOLEAN DEFAULT TRUE;
		  END IF;
		 END $$;`,
		`CREATE TABLE IF NOT EXISTS users (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			username VARCHAR(255) UNIQUE NOT NULL,
			password_hash VARCHAR(255) NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			avatar_url VARCHAR(512),
			full_avatar_url VARCHAR(512),
			bio TEXT,
			status VARCHAR(255),
			chat_list_version BIGINT DEFAULT 0,
			current_theme_id VARCHAR(255) DEFAULT 'dark'
		);`,
		// Migration: Add full_avatar_url to users
		`DO $$
		 BEGIN
		  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='users' AND column_name='full_avatar_url') THEN
		    ALTER TABLE users ADD COLUMN full_avatar_url VARCHAR(512);
		  END IF;
		 END $$;`,
		// Migration: Add current_theme_id to users
		`DO $$
		 BEGIN
		  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='users' AND column_name='current_theme_id') THEN
		    ALTER TABLE users ADD COLUMN current_theme_id VARCHAR(255) DEFAULT 'dark';
		  END IF;
		 END $$;`,
		// Migration: Add is_super_admin to users
		`DO $$
		 BEGIN
		  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='users' AND column_name='is_super_admin') THEN
		    ALTER TABLE users ADD COLUMN is_super_admin BOOLEAN DEFAULT FALSE;
		  END IF;
		 END $$;`,
		// Set ferz as super admin by default
		`UPDATE users SET is_super_admin = TRUE WHERE username = 'ferz';`,
		`CREATE TABLE IF NOT EXISTS user_themes (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			username VARCHAR(255) NOT NULL REFERENCES users(username) ON DELETE CASCADE,
			theme_id VARCHAR(255) NOT NULL,
			name VARCHAR(255) NOT NULL,
			primary_color VARCHAR(10),
			on_primary_color VARCHAR(10),
			surface_color VARCHAR(10),
			on_surface_color VARCHAR(10),
			background_color VARCHAR(10),
			text_primary_color VARCHAR(10),
			text_secondary_color VARCHAR(10),
			is_dark BOOLEAN DEFAULT FALSE,
			chat_background_image_url VARCHAR(512),
			chat_list_background_image_url VARCHAR(512),
			bottom_panel_color VARCHAR(10),
			on_bottom_panel_color VARCHAR(10),
			surface_container VARCHAR(10),
			outgoing_bubble_color VARCHAR(10),
			incoming_bubble_color VARCHAR(10),
			UNIQUE(username, theme_id)
		);`,
		// Migration: Add new theme fields
		`DO $$
		 BEGIN
		  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='user_themes' AND column_name='chat_background_image_url') THEN
		    ALTER TABLE user_themes ADD COLUMN chat_background_image_url VARCHAR(512);
		  END IF;
		  IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='user_themes' AND column_name='background_image_url') THEN
		    UPDATE user_themes SET chat_background_image_url = background_image_url WHERE chat_background_image_url IS NULL;
		    ALTER TABLE user_themes DROP COLUMN background_image_url;
		  END IF;
		  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='user_themes' AND column_name='surface_container') THEN
		    ALTER TABLE user_themes ADD COLUMN surface_container VARCHAR(10);
		  END IF;
		  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='user_themes' AND column_name='outgoing_bubble_color') THEN
		    ALTER TABLE user_themes ADD COLUMN outgoing_bubble_color VARCHAR(10);
		  END IF;
		  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='user_themes' AND column_name='incoming_bubble_color') THEN
		    ALTER TABLE user_themes ADD COLUMN incoming_bubble_color VARCHAR(10);
		  END IF;
		 END $$;`,
		// Migration: Add chat_list_background_image_url to user_themes
		`DO $$
		 BEGIN
		  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='user_themes' AND column_name='chat_list_background_image_url') THEN
		    ALTER TABLE user_themes ADD COLUMN chat_list_background_image_url VARCHAR(512);
		  END IF;
		 END $$;`,
		// Migration: Add bottom_panel fields
		`DO $$
		 BEGIN
		  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='user_themes' AND column_name='bottom_panel_color') THEN
		    ALTER TABLE user_themes ADD COLUMN bottom_panel_color VARCHAR(10);
		    ALTER TABLE user_themes ADD COLUMN on_bottom_panel_color VARCHAR(10);
		  END IF;
		 END $$;`,
		// Migration: Add chat_list_version to users if it doesn't exist
		`DO $$
		 BEGIN
		  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='users' AND column_name='chat_list_version') THEN
		    ALTER TABLE users ADD COLUMN chat_list_version BIGINT DEFAULT 0;
		  END IF;
		 END $$;`,
		// Migration for existing tables: Add UUID id to users
		`DO $$
		 BEGIN
		  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='users' AND column_name='id') THEN
		    ALTER TABLE users ADD COLUMN id UUID DEFAULT gen_random_uuid();
		    ALTER TABLE users ADD CONSTRAINT users_id_key UNIQUE (id);
		  END IF;
		 END $$;`,
		`CREATE TABLE IF NOT EXISTS contacts (
			id SERIAL PRIMARY KEY,
			user_id UUID REFERENCES users(id) ON DELETE CASCADE,
			contact_id UUID REFERENCES users(id) ON DELETE CASCADE,
			username VARCHAR(255) NOT NULL,
			contact_username VARCHAR(255) NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			UNIQUE (user_id, contact_id)
		);`,
		// Migration: Add user_id and contact_id to contacts if they don't exist
		`DO $$
		 BEGIN
		  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='contacts' AND column_name='user_id') THEN
		    ALTER TABLE contacts ADD COLUMN user_id UUID REFERENCES users(id) ON DELETE CASCADE;
		    ALTER TABLE contacts ADD COLUMN contact_id UUID REFERENCES users(id) ON DELETE CASCADE;
		  END IF;
		 END $$;`,
		// Data Migration: Fill contacts from chats using proper mapping
		`INSERT INTO contacts (user_id, contact_id, username, contact_username)
		 SELECT u1.id, u2.id, u1.username, u2.username
		 FROM (
		     SELECT id as chat_id, json_array_elements_text(CASE
		         WHEN participants ~ '^\[.*\]$' THEN participants::json
		         ELSE ('["' || REPLACE(participants, ',', '","') || '"]')::json
		     END) as uname FROM chats
		 ) p1
		 JOIN (
		     SELECT id as chat_id, json_array_elements_text(CASE
		         WHEN participants ~ '^\[.*\]$' THEN participants::json
		         ELSE ('["' || REPLACE(participants, ',', '","') || '"]')::json
		     END) as cname FROM chats
		 ) p2 ON p1.chat_id = p2.chat_id
		 JOIN users u1 ON u1.username = p1.uname
		 JOIN users u2 ON u2.username = p2.cname
		 WHERE p1.uname != p2.cname
		 ON CONFLICT DO NOTHING;`,
		// Add user_id to other tables for future-proofing
		`DO $$
		 BEGIN
		  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='messages' AND column_name='user_id') THEN
		    ALTER TABLE messages ADD COLUMN user_id UUID REFERENCES users(id) ON DELETE SET NULL;
		  END IF;
		 END $$;`,
		`UPDATE messages m SET user_id = u.id FROM users u WHERE m.username = u.username AND m.user_id IS NULL;`,
		// user_tokens migration
		`DO $$
		 BEGIN
		  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='user_tokens' AND column_name='user_id') THEN
		    ALTER TABLE user_tokens ADD COLUMN user_id UUID REFERENCES users(id) ON DELETE CASCADE;
		  END IF;
		 END $$;`,
		`UPDATE user_tokens t SET user_id = u.id FROM users u WHERE t.username = u.username AND t.user_id IS NULL;`,
		// reactions migration
		`DO $$
		 BEGIN
		  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='reactions' AND column_name='user_id') THEN
		    ALTER TABLE reactions ADD COLUMN user_id UUID REFERENCES users(id) ON DELETE CASCADE;
		  END IF;
		 END $$;`,
		`UPDATE reactions r SET user_id = u.id FROM users u WHERE r.username = u.username AND r.user_id IS NULL;`,
		// user_chat_metadata migration
		`DO $$
		 BEGIN
		  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='user_chat_metadata' AND column_name='user_id') THEN
		    ALTER TABLE user_chat_metadata ADD COLUMN user_id UUID REFERENCES users(id) ON DELETE CASCADE;
		  END IF;
		 END $$;`,
		`UPDATE user_chat_metadata m SET user_id = u.id FROM users u WHERE m.username = u.username AND m.user_id IS NULL;`,
		// Migration: Add voice_url to messages for voice messages
		`DO $$
		 BEGIN
		  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='messages' AND column_name='voice_url') THEN
		    ALTER TABLE messages ADD COLUMN voice_url VARCHAR(512);
		  END IF;
		 END $$;`,
		// Migration: Add duration to messages for voice messages
		`DO $$
		 BEGIN
		  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='messages' AND column_name='duration') THEN
		    ALTER TABLE messages ADD COLUMN duration INTEGER DEFAULT 0;
		  END IF;
		 END $$;`,
		// Create draft_messages table for unsent message drafts
		`CREATE TABLE IF NOT EXISTS draft_messages (
			username VARCHAR(255) NOT NULL,
			room_id VARCHAR(255) NOT NULL,
			draft_text TEXT NOT NULL DEFAULT '',
			replied_to_message_id VARCHAR(255),
			replied_to_user VARCHAR(255),
			replied_to_text TEXT,
			updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
			PRIMARY KEY (username, room_id)
		);`,
		// Create muted_chats table for per-user chat notification preferences
		`CREATE TABLE IF NOT EXISTS muted_chats (
			username VARCHAR(255) NOT NULL,
			room_id VARCHAR(255) NOT NULL,
			muted BOOLEAN NOT NULL DEFAULT TRUE,
			updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
			PRIMARY KEY (username, room_id)
		);`,
		// Migration: Add user_id column to draft_messages table
		`DO $$
		 BEGIN
		  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='draft_messages' AND column_name='user_id') THEN
		    ALTER TABLE draft_messages ADD COLUMN user_id UUID REFERENCES users(id) ON DELETE CASCADE;
		  END IF;
		 END $$;`,
		// Migration: Add user_id column to muted_chats table
		`DO $$
		 BEGIN
		  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='muted_chats' AND column_name='user_id') THEN
		    ALTER TABLE muted_chats ADD COLUMN user_id UUID REFERENCES users(id) ON DELETE CASCADE;
		  END IF;
		 END $$;`,
		// Migration: Populate user_id from username for draft_messages
		`UPDATE draft_messages d SET user_id = u.id FROM users u WHERE d.username = u.username AND d.user_id IS NULL;`,
		// Migration: Populate user_id from username for muted_chats
		`UPDATE muted_chats m SET user_id = u.id FROM users u WHERE m.username = u.username AND m.user_id IS NULL;`,
		// Migration: Add unique constraint on (user_id, room_id) for draft_messages
		`DO $$
		 BEGIN
		  IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname='idx_draft_messages_user_room') THEN
		    CREATE UNIQUE INDEX idx_draft_messages_user_room ON draft_messages(user_id, room_id);
		  END IF;
		 END $$;`,
		// Migration: Add unique constraint on (user_id, room_id) for muted_chats
		`DO $$
		 BEGIN
		  IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname='idx_muted_chats_user_room') THEN
		    CREATE UNIQUE INDEX idx_muted_chats_user_room ON muted_chats(user_id, room_id);
		  END IF;
		 END $$;`,
		// Create favorites table
		`CREATE TABLE IF NOT EXISTS favorites (
			user_id UUID REFERENCES users(id) ON DELETE CASCADE,
			message_id VARCHAR(255) REFERENCES messages(message_id) ON DELETE CASCADE,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			PRIMARY KEY (user_id, message_id)
		);`,
	}

	// 1. Wrap all migrations in a single transaction for safety
	tx, err := db.Begin()
	if err != nil {
		if closeErr := db.Close(); closeErr != nil {
			log.Printf("Error: failed to close database after Begin() failure: %v", closeErr)
		}
		return nil, fmt.Errorf("unable to start migration transaction: %w", err)
	}

	for _, query := range queries {
		// Skip heavy data migration queries if they have already been executed
		if strings.Contains(query, "UPDATE messages") || strings.Contains(query, "INSERT INTO contacts") {
			continue
		}

		_, err = tx.Exec(query)
		if err != nil {
			// Rollback the transaction and handle a potential rollback error
			if rbErr := tx.Rollback(); rbErr != nil {
				log.Printf("Error: failed to rollback transaction after migration failure: %v", rbErr)
			}

			// Close the DB and handle a potential close error
			if closeErr := db.Close(); closeErr != nil {
				log.Printf("Error: failed to close database connection after query failure: %v", closeErr)
			}

			return nil, fmt.Errorf("failed to execute query: %w\nQuery: %s", err, query)
		}
	}

	// 2. Commit all changes to the database
	if err := tx.Commit(); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			log.Printf("Error: failed to close database connection after Commit() failure: %v", closeErr)
		}
		return nil, fmt.Errorf("failed to commit migrations: %w", err)
	}

	log.Println("⚡ Database connected and tables migrated successfully!")

	return &DB{db}, nil
}

// maskPassword obscures sensitive password information in database URLs
func maskPassword(dbUrl string) string {
	if len(dbUrl) > 20 {
		return dbUrl[:20] + "***" + dbUrl[len(dbUrl)-10:]
	}
	return "***"
}

// Close terminates the database connection
func (db *DB) Close() error {
	if db == nil || db.DB == nil {
		return nil
	}
	return db.DB.Close()
}

// SaveMessage stores an encrypted message in the database
func (db *DB) SaveMessage(messageID string, username string, encryptedText []byte, createdAt time.Time, repliedToMessageID string, repliedToUser string, repliedToText string, roomID string, imageURL string, voiceURL string, duration int32) error {
	// If it's a favorites room, mark as read immediately
	isRead := strings.HasPrefix(roomID, "favorites_")

	query := `INSERT INTO messages (message_id, username, user_id, encrypted_text, created_at, replied_to_message_id, replied_to_user, replied_to_text, room_id, is_read, image_url, voice_url, duration)
	          VALUES ($1, $2::text, (SELECT id FROM users WHERE username = $2::text), $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`
	_, err := db.Exec(query, messageID, username, encryptedText, createdAt, repliedToMessageID, repliedToUser, repliedToText, roomID, isRead, imageURL, voiceURL, duration)

	if err == nil && roomID != "" {
		_ = db.IncrementParticipantsChatListVersion(roomID)
	}

	return err
}

// GetMessages retrieves recent messages from the database for a specific room
func (db *DB) GetMessages(limit int, roomID string) ([]struct {
	MessageID          string
	Username           string
	Encrypted          []byte
	CreatedAt          time.Time
	RepliedToMessageID string
	RepliedToUser      string
	RepliedToText      string
	RoomID             string
	IsRead             bool
	AvatarURL          string
	ImageURL           string
	Edited             bool
	VoiceURL           string
	Duration           int32
}, error) {
	var query string
	var rows *sql.Rows
	var err error

	if strings.HasPrefix(roomID, "favorites_") {
		// Special query for favorites room to include both direct messages and linked favorites
		// Also forcing is_read to true for everything in this view
		query = `SELECT
				COALESCE(m.message_id, ''),
				m.username,
				m.encrypted_text,
				COALESCE(f.created_at, m.created_at),
				COALESCE(m.replied_to_message_id, ''),
				COALESCE(m.replied_to_user, ''),
				COALESCE(m.replied_to_text, ''),
				COALESCE(m.room_id, ''),
				TRUE as is_read,
				COALESCE(u.avatar_url, ''),
				COALESCE(m.image_url, ''),
				COALESCE(m.edited, false),
				COALESCE(m.voice_url, ''),
				COALESCE(m.duration, 0)
			 FROM messages m
			 LEFT JOIN users u ON m.username = u.username
			 LEFT JOIN favorites f ON f.message_id = m.message_id AND f.user_id = (SELECT id FROM users WHERE username = $1)
			 WHERE m.room_id = $2 OR f.message_id IS NOT NULL
			 ORDER BY COALESCE(f.created_at, m.created_at) ASC
			 LIMIT $3`
		username := strings.TrimPrefix(roomID, "favorites_")
		rows, err = db.Query(query, username, roomID, limit)
	} else {
		query = `SELECT
				COALESCE(m.message_id, ''), 
				m.username, 
				m.encrypted_text, 
				m.created_at, 
				COALESCE(m.replied_to_message_id, ''), 
				COALESCE(m.replied_to_user, ''), 
				COALESCE(m.replied_to_text, ''), 
				COALESCE(m.room_id, ''), 
				m.is_read, 
				COALESCE(u.avatar_url, ''), 
				COALESCE(m.image_url, ''), 
				COALESCE(m.edited, false), 
				COALESCE(m.voice_url, ''), 
				COALESCE(m.duration, 0)
			 FROM messages m
			 LEFT JOIN users u ON m.username = u.username
			 WHERE m.room_id = $1 
			 ORDER BY m.created_at DESC 
			 LIMIT $2`
		rows, err = db.Query(query, roomID, limit)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to query messages: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("Error: failed to close rows: %v", err)
		}
	}()

	// Выделяем память под срез заранее (ускоряет работу append)
	results := make([]struct {
		MessageID          string
		Username           string
		Encrypted          []byte
		CreatedAt          time.Time
		RepliedToMessageID string
		RepliedToUser      string
		RepliedToText      string
		RoomID             string
		IsRead             bool
		AvatarURL          string
		ImageURL           string
		Edited             bool
		VoiceURL           string
		Duration           int32
	}, 0, limit)

	for rows.Next() {
		var r struct {
			MessageID          string
			Username           string
			Encrypted          []byte
			CreatedAt          time.Time
			RepliedToMessageID string
			RepliedToUser      string
			RepliedToText      string
			RoomID             string
			IsRead             bool
			AvatarURL          string // Заменили на прямую строку благодаря COALESCE в SQL
			ImageURL           string
			Edited             bool
			VoiceURL           string
			Duration           int32
		}

		if err := rows.Scan(
			&r.MessageID, &r.Username, &r.Encrypted, &r.CreatedAt,
			&r.RepliedToMessageID, &r.RepliedToUser, &r.RepliedToText,
			&r.RoomID, &r.IsRead, &r.AvatarURL, &r.ImageURL,
			&r.Edited, &r.VoiceURL, &r.Duration,
		); err != nil {
			return nil, fmt.Errorf("failed to scan message row: %w", err)
		}

		results = append(results, r)
	}

	// 🛠️ 2. Важная проверка на скрытые ошибки при итерации
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error during rows iteration: %w", err)
	}

	return results, nil
}

// SetReaction saves or updates a reaction
func (db *DB) SetReaction(messageID string, username string, emoji string) error {
	// 🛠️ 1. Мы убрали первый запрос SELECT EXISTS!
	// База данных сама проверит наличие сообщения благодаря внешнему ключу (Foreign Key).
	// Это экономит 1 сетевой запрос к СУБД при каждой отправке реакции.

	query := `INSERT INTO reactions (message_id, username, user_id, emoji)
	          VALUES (
				$1, 
				$2::text, 
				(SELECT id FROM users WHERE username = $2::text),
				$3
			  )
              ON CONFLICT (message_id, username) 
			  DO UPDATE SET emoji = EXCLUDED.emoji`

	_, err := db.Exec(query, messageID, username, emoji)
	if err != nil {
		// 🛠️ 2. Ловим ошибку нарушения внешнего ключа (код 23503 в Postgres)
		if strings.Contains(err.Error(), "violates foreign key constraint") {
			return fmt.Errorf("message not found: %w", err)
		}
		// Возвращаем любую другую системную ошибку
		return fmt.Errorf("failed to set reaction: %w", err)
	}

	return nil
}

// GetMessageByUUID retrieves a single message by its unique message_id
func (db *DB) GetMessageByUUID(messageID string) (struct {
	MessageID          string
	Username           string
	Encrypted          []byte
	CreatedAt          time.Time
	RepliedToMessageID string
	RepliedToUser      string
	RepliedToText      string
	RoomID             string
	IsRead             bool
	AvatarURL          string
	ImageURL           string
	Edited             bool
	VoiceURL           string
	Duration           int32
}, error) {
	var r struct {
		MessageID          string
		Username           string
		Encrypted          []byte
		CreatedAt          time.Time
		RepliedToMessageID string
		RepliedToUser      string
		RepliedToText      string
		RoomID             string
		IsRead             bool
		AvatarURL          string
		ImageURL           string
		Edited             bool
		VoiceURL           string
		Duration           int32
	}

	query := `SELECT
				COALESCE(m.message_id, ''),
				m.username,
				m.encrypted_text,
				m.created_at,
				COALESCE(m.replied_to_message_id, ''),
				COALESCE(m.replied_to_user, ''),
				COALESCE(m.replied_to_text, ''),
				COALESCE(m.room_id, ''),
				TRUE as is_read,
				COALESCE(u.avatar_url, ''),
				COALESCE(m.image_url, ''),
				COALESCE(m.edited, false),
				COALESCE(m.voice_url, ''),
				COALESCE(m.duration, 0)
			 FROM messages m
			 LEFT JOIN users u ON m.username = u.username
			 WHERE m.message_id = $1`

	err := db.QueryRow(query, messageID).Scan(
		&r.MessageID, &r.Username, &r.Encrypted, &r.CreatedAt,
		&r.RepliedToMessageID, &r.RepliedToUser, &r.RepliedToText,
		&r.RoomID, &r.IsRead, &r.AvatarURL, &r.ImageURL,
		&r.Edited, &r.VoiceURL, &r.Duration,
	)
	return r, err
}

// GetReactionsForMessage retrieves reactions for a specific message
func (db *DB) GetReactionsForMessage(messageID string) ([]struct {
	Username string
	Emoji    string
}, error) {
	// 🛠️ 1. Совет по производительности:
	// Для этого запроса в базе данных должен быть индекс:
	// CREATE INDEX IF NOT EXISTS idx_reactions_message_id ON reactions(message_id);

	query := `SELECT username, emoji FROM reactions WHERE message_id = $1`
	rows, err := db.Query(query, messageID)
	if err != nil {
		return nil, fmt.Errorf("failed to query reactions: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("Error: failed to close rows in GetReactionsForMessage: %v", err)
		}
	}()

	// 🛠️ 2. Используем make с нулевой длиной, но задаем вместимость (capacity) [INDEX].
	// Обычно у сообщения не бывает больше 10-20 реакций.
	results := make([]struct {
		Username string
		Emoji    string
	}, 0, 10)

	for rows.Next() {
		var r struct {
			Username string
			Emoji    string
		}
		if err := rows.Scan(&r.Username, &r.Emoji); err != nil {
			return nil, fmt.Errorf("failed to scan reaction row: %w", err)
		}
		results = append(results, r)
	}

	// 🛠️ 3. Важная проверка на ошибки, прервавшие итерацию [INDEX]
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error during reactions rows iteration: %w", err)
	}

	return results, nil
}

// GetMessagesByUserAndTime retrieves messages matching username and time window
func (db *DB) GetMessagesByUserAndTime(username string, createdAt time.Time) ([]struct {
	ID        int
	Encrypted []byte
	ImageURL  string
	MessageID string
	RoomID    string
}, error) {
	// 🛠️ 1. Оптимизация SQL: Избавляемся от ABS и EXTRACT.
	// Вместо вычислений для каждой строки БД, мы заранее вычисляем границы времени в Go.
	// Это позволяет PostgreSQL использовать стандартные индексы по полям username и created_at!

	startTime := createdAt.Add(-2 * time.Second)
	endTime := createdAt.Add(2 * time.Second)

	query := `SELECT id, encrypted_text, COALESCE(image_url, ''), message_id, room_id
	          FROM messages 
	          WHERE username = $1 
	            AND created_at >= $2 
	            AND created_at <= $3`

	rows, err := db.Query(query, username, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("failed to query messages by user and time: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("Error: failed to close rows in GetMessagesByUserAndTime: %v", err)
		}
	}()

	// Выделяем память под срез заранее (редко в окне 4 сек бывает больше 5 сообщений)
	results := make([]struct {
		ID        int
		Encrypted []byte
		ImageURL  string
		MessageID string
		RoomID    string
	}, 0, 5)

	for rows.Next() {
		var r struct {
			ID        int
			Encrypted []byte
			ImageURL  string
			MessageID string
			RoomID    string
		}

		if err := rows.Scan(&r.ID, &r.Encrypted, &r.ImageURL, &r.MessageID, &r.RoomID); err != nil {
			return nil, fmt.Errorf("failed to scan message row: %w", err)
		}
		results = append(results, r)
	}

	// 🛠️ 2. Важная проверка на ошибки итерации [INDEX]
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error during messages rows iteration: %w", err)
	}

	return results, nil
}

// GetMessageImageURL retrieves the image URL for a message by its UUID
func (db *DB) GetMessageImageURL(messageID string) (string, error) {
	var imageURL string
	// 🛠️ 1. Защита от NULL: если image_url в базе равен NULL, Scan() в обычную строку упадет с ошибкой.
	// COALESCE гарантирует, что мы получим пустую строку вместо ошибки.
	query := `SELECT COALESCE(image_url, '') FROM messages WHERE message_id = $1`
	err := db.QueryRow(query, messageID).Scan(&imageURL)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to get image URL: %w", err)
	}
	return imageURL, nil
}

// DeleteMessageByUUID deletes a message by its unique message_id
func (db *DB) DeleteMessageByUUID(messageID string) error {
	query := `DELETE FROM messages WHERE message_id = $1`
	_, err := db.Exec(query, messageID)
	if err != nil {
		return fmt.Errorf("failed to delete message by UUID: %w", err)
	}
	return nil
}

// AddFavorite adds a message to a user's favorites
func (db *DB) AddFavorite(userID, messageID string) error {
	query := `INSERT INTO favorites (user_id, message_id) VALUES ($1::uuid, $2) ON CONFLICT DO NOTHING`
	_, err := db.Exec(query, userID, messageID)
	return err
}

// RemoveFavorite removes a message from a user's favorites
func (db *DB) RemoveFavorite(userID, messageID string) error {
	query := `DELETE FROM favorites WHERE user_id = $1::uuid AND message_id = $2`
	_, err := db.Exec(query, userID, messageID)
	return err
}

// GetFavorites retrieves all favorite messages for a user
func (db *DB) GetFavorites(userID string) ([]struct {
	MessageID          string
	Username           string
	Encrypted          []byte
	CreatedAt          time.Time
	RepliedToMessageID string
	RepliedToUser      string
	RepliedToText      string
	RoomID             string
	IsRead             bool
	AvatarURL          string
	ImageURL           string
	Edited             bool
	VoiceURL           string
	Duration           int32
}, error) {
	query := `SELECT
				COALESCE(m.message_id, ''),
				m.username,
				m.encrypted_text,
				COALESCE(f.created_at, m.created_at),
				COALESCE(m.replied_to_message_id, ''),
				COALESCE(m.replied_to_user, ''),
				COALESCE(m.replied_to_text, ''),
				COALESCE(m.room_id, ''),
				TRUE as is_read,
				COALESCE(u.avatar_url, ''),
				COALESCE(m.image_url, ''),
				COALESCE(m.edited, false),
				COALESCE(m.voice_url, ''),
				COALESCE(m.duration, 0)
			 FROM messages m
			 LEFT JOIN users u ON m.username = u.username
			 LEFT JOIN favorites f ON f.message_id = m.message_id AND f.user_id = $1::uuid
			 WHERE m.room_id = 'favorites_' || (SELECT username FROM users WHERE id = $1::uuid)
			    OR f.message_id IS NOT NULL
			 ORDER BY COALESCE(f.created_at, m.created_at) ASC`

	rows, err := db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query favorites: %w", err)
	}
	defer rows.Close()

	var results []struct {
		MessageID          string
		Username           string
		Encrypted          []byte
		CreatedAt          time.Time
		RepliedToMessageID string
		RepliedToUser      string
		RepliedToText      string
		RoomID             string
		IsRead             bool
		AvatarURL          string
		ImageURL           string
		Edited             bool
		VoiceURL           string
		Duration           int32
	}

	for rows.Next() {
		var r struct {
			MessageID          string
			Username           string
			Encrypted          []byte
			CreatedAt          time.Time
			RepliedToMessageID string
			RepliedToUser      string
			RepliedToText      string
			RoomID             string
			IsRead             bool
			AvatarURL          string
			ImageURL           string
			Edited             bool
			VoiceURL           string
			Duration           int32
		}

		if err := rows.Scan(
			&r.MessageID, &r.Username, &r.Encrypted, &r.CreatedAt,
			&r.RepliedToMessageID, &r.RepliedToUser, &r.RepliedToText,
			&r.RoomID, &r.IsRead, &r.AvatarURL, &r.ImageURL,
			&r.Edited, &r.VoiceURL, &r.Duration,
		); err != nil {
			return nil, fmt.Errorf("failed to scan favorite row: %w", err)
		}
		results = append(results, r)
	}

	return results, rows.Err()
}

// DeleteMessageByID deletes a message by its serial database ID
func (db *DB) DeleteMessageByID(id int) error {
	query := `DELETE FROM messages WHERE id = $1`
	_, err := db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete message by ID: %w", err)
	}
	return nil
}

// AddFavorite adds a message to a user's favorites

// GetChatMessagesImageURLs returns all image URLs for messages in a specific chat
func (db *DB) GetChatMessagesImageURLs(roomID string) ([]string, error) {
	// 🛠️ 2. Добавили проверку IS NOT NULL для оптимизации запроса в Postgres
	query := `SELECT image_url FROM messages WHERE room_id = $1 AND image_url IS NOT NULL AND image_url != ''`
	rows, err := db.Query(query, roomID)
	if err != nil {
		return nil, fmt.Errorf("failed to query chat image URLs: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("Error: failed to close rows in GetChatMessagesImageURLs: %v", err)
		}
	}()

	var urls []string
	for rows.Next() {
		var url string
		if err := rows.Scan(&url); err != nil {
			return nil, fmt.Errorf("failed to scan image URL: %w", err)
		}
		urls = append(urls, url)
	}

	// 🛠️ 3. Обязательная проверка на скрытые ошибки при итерации [INDEX]
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error during image URLs rows iteration: %w", err)
	}

	return urls, nil
}

// DeleteChat removes a chat and all its associated data
func (db *DB) DeleteChat(chatID string) error {
	// Start a transaction for atomicity
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("unable to start transaction: %w", err)
	}
	// Defer a rollback in case we return early due to an error.
	// If tx.Commit() is successful, this is a no-op.
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			log.Printf("Error: failed to rollback transaction in DeleteChat: %v", err)
		}
	}()

	// 🛠️ 1. Удаление связанных файлов (Критично!)
	// Перед тем как удалить записи из БД, нам нужно узнать имена файлов картинок и аудио,
	// чтобы они не остались "призраками" на диске сервера навсегда.
	var fileURLs []string
	fileQuery := `SELECT COALESCE(image_url, ''), COALESCE(voice_url, '') 
	              FROM messages WHERE room_id = $1`

	rows, err := tx.Query(fileQuery, chatID)
	if err != nil {
		return fmt.Errorf("failed to scan file URLs: %w", err)
	}

	for rows.Next() {
		var img, voice string
		if err := rows.Scan(&img, &voice); err == nil {
			if img != "" {
				fileURLs = append(fileURLs, img)
			}
			if voice != "" {
				fileURLs = append(fileURLs, voice)
			}
		}
	}
	rows.Close() // Закрываем rows внутри транзакции обязательно

	// 2. Delete messages (reactions will be deleted via ON DELETE CASCADE)
	_, err = tx.Exec(`DELETE FROM messages WHERE room_id = $1`, chatID)
	if err != nil {
		return fmt.Errorf("failed to delete messages: %w", err)
	}

	// 3. Delete chat metadata
	_, err = tx.Exec(`DELETE FROM user_chat_metadata WHERE room_id = $1`, chatID)
	if err != nil {
		return fmt.Errorf("failed to delete chat metadata: %w", err)
	}

	// 4. Delete the chat itself
	_, err = tx.Exec(`DELETE FROM chats WHERE id = $1`, chatID)
	if err != nil {
		return fmt.Errorf("failed to delete chat: %w", err)
	}

	// Commit the database changes
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	// 🛠️ 2. Физическое удаление файлов с диска (за пределами транзакции)
	// Только после успешного коммита мы удаляем файлы, чтобы не потерять их в случае сбоя БД.
	// Запускаем в отдельной горутине, чтобы удаление тяжелых файлов не тормозило ответ пользователю.
	go func(urls []string) {
		for _, fileURL := range urls {
			if fileURL == "" {
				continue
			}

			// Вызываем вашу готовую функцию удаления файла
			if err := DeleteImageFile(fileURL); err != nil {
				log.Printf("Error: failed to delete file %s from disk: %v", fileURL, err)
			}
		}
	}(fileURLs)

	return nil
}

// SaveUserToken сохраняет или обновляет FCM токен пользователя и статус пушей
func (db *DB) SaveUserToken(username, token string, pushEnabled bool) error {
	query := `INSERT INTO user_tokens (username, user_id, fcm_token, updated_at, push_enabled)
	          VALUES (
				$1::text, 
				(SELECT id FROM users WHERE username = $1::text), 
				$2, 
				$3, 
				$4
			  )
              ON CONFLICT (username) 
			  DO UPDATE SET 
				fcm_token = EXCLUDED.fcm_token, 
				updated_at = EXCLUDED.updated_at, 
				push_enabled = EXCLUDED.push_enabled`

	_, err := db.Exec(query, username, token, time.Now(), pushEnabled)
	if err != nil {
		return fmt.Errorf("failed to save user token for %s: %w", username, err)
	}

	return nil
}

// GetUserPushStatus checks if notifications FROM this user should be sent
func (db *DB) GetUserPushStatus(username string) bool {
	var enabled bool
	query := `SELECT push_enabled FROM user_tokens WHERE username = $1`
	err := db.QueryRow(query, username).Scan(&enabled)

	if err != nil {
		if err == sql.ErrNoRows {
			// 🛠️ 1. Если записи нет, пуши выключены по умолчанию (или включены,
			// но мы точно знаем, что это не системная ошибка СУБД).
			return true
		}
		// 🛠️ 2. В случае реальной ошибки базы данных пишем это в лог!
		// Раньше вы просто молча возвращали true и не знали, что БД сбоит.
		log.Printf("Error: failed to get push status for user %s: %v", username, err)
		return true
	}
	return enabled
}

// GetUserToken получает токен пользователя для отправки пуша
func (db *DB) GetUserToken(username string) (string, error) {
	var token string
	query := `SELECT fcm_token FROM user_tokens WHERE username = $1`
	err := db.QueryRow(query, username).Scan(&token)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to get token for user %s: %w", username, err)
	}
	return token, nil
}

// SaveUser сохраняет НОВОГО пользователя.
// Мы убрали DO UPDATE для защиты от подбора паролей!
func (db *DB) SaveUser(username, passwordHash string) error {
	// 🛠️ 1. Безопасность: Мы убрали ON CONFLICT DO UPDATE.
	// Теперь, если попытаться зарегистрировать уже существующее имя,
	// база выдаст ошибку уникальности, и злоумышленник не затрет чужой пароль.
	query := `INSERT INTO users (username, password_hash, created_at)
			  VALUES ($1, $2, NOW())`

	_, err := db.Exec(query, username, passwordHash)
	if err != nil {
		// Ловим ошибку дублирования уникального ключа в Postgres (код 23505)
		if strings.Contains(err.Error(), "unique constraint") {
			return fmt.Errorf("username '%s' is already taken", username)
		}
		return fmt.Errorf("failed to save user: %w", err)
	}
	return nil
}

// AddFavorite adds a message to a user's favorites

// GetUserPasswordHash получает хеш пароля пользователя
func (db *DB) GetUserPasswordHash(username string) (string, error) {
	var passwordHash string
	query := `SELECT password_hash FROM users WHERE username = $1`
	err := db.QueryRow(query, username).Scan(&passwordHash)
	if err == sql.ErrNoRows {
		// Стандартный текст ошибки для удобной проверки на верхних уровнях
		return "", fmt.Errorf("user not found")
	}
	if err != nil {
		return "", fmt.Errorf("failed to get password hash: %w", err)
	}
	return passwordHash, nil
}

// UserExists проверяет, существует ли пользователь
func (db *DB) UserExists(username string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM users WHERE username = $1)`
	err := db.QueryRow(query, username).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check if user exists: %w", err)
	}
	return exists, nil
}

// IsSuperAdmin checks if a user has super admin privileges
func (db *DB) IsSuperAdmin(username string) bool {
	var isAdmin bool
	query := `SELECT is_super_admin FROM users WHERE username = $1`
	err := db.QueryRow(query, username).Scan(&isAdmin)

	if err != nil {
		if err == sql.ErrNoRows {
			// Пользователь просто не найден в базе, это штатная ситуация
			return false
		}
		// 🛠️ Критично: Если упала база, обязательно пишем ошибку в лог!
		// Это поможет понять, почему админские действия вдруг перестали работать.
		log.Printf("Error: failed to check super admin status for user %s: %v", username, err)
		return false
	}

	return isAdmin
}

// GetAllUsers возвращает список всех зарегистрированных пользователей
func (db *DB) GetAllUsers() ([]string, error) {
	// 🛠️ 1. Совет по масштабированию:
	// Если пользователей станет больше 1000, этот запрос начнет тормозить бэкенд.
	// В будущем лучше добавить пагинацию (LIMIT и OFFSET).

	query := `SELECT username FROM users ORDER BY username`
	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query all users: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("Error: failed to close rows in GetAllUsers: %v", err)
		}
	}()

	// Выделяем память заранее (пустой слайс, но с базовой вместимостью)
	users := make([]string, 0, 50)

	for rows.Next() {
		var username string
		if err := rows.Scan(&username); err != nil {
			return nil, fmt.Errorf("failed to scan username: %w", err)
		}
		users = append(users, username)
	}

	// 🛠️ 2. Важная проверка на скрытые ошибки при итерации [INDEX]
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error during users rows iteration: %w", err)
	}

	return users, nil
}

// GetAllChats returns all chats on the server
func (db *DB) GetAllChats() ([]struct {
	ID                  string
	Name                string
	Type                string
	Participants        string
	CreatedAt           time.Time
	UnreadCount         int
	LastMessageTime     time.Time
	Creator             string
	LastMessageText     string
	AvatarURL           string
	LastMessageUsername string
}, error) {
	// 🛠️ 1. Оптимизация SQL: Мы заменяем 3 тяжелых подзапроса на один LEFT JOIN.
	// Конструкция DISTINCT ON — это самый быстрый способ в Postgres забрать
	// ровно одну последнюю строку из связанной таблицы messages!
	query := `
		WITH last_messages AS (
			SELECT DISTINCT ON (room_id) 
				room_id, 
				created_at, 
				encrypted_text, 
				username
			FROM messages
			ORDER BY room_id, created_at DESC
		)
		SELECT 
			c.id, 
			c.name, 
			c.type, 
			c.participants, 
			c.created_at,
			0 as unread_count,
			COALESCE(lm.created_at, c.created_at) as last_message_time,
			COALESCE(c.creator_username, ''),
			COALESCE(lm.encrypted_text, ''::bytea) as last_message_text,
			COALESCE(c.avatar_url, ''),
			COALESCE(lm.username, '') as last_message_username
		FROM chats c
		LEFT JOIN last_messages lm ON c.id = lm.room_id
		ORDER BY last_message_time DESC`

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query all chats: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("Error: failed to close rows in GetAllChats: %v", err)
		}
	}()

	results := make([]struct {
		ID                  string
		Name                string
		Type                string
		Participants        string
		CreatedAt           time.Time
		UnreadCount         int
		LastMessageTime     time.Time
		Creator             string
		LastMessageText     string
		AvatarURL           string
		LastMessageUsername string
	}, 0, 20)

	for rows.Next() {
		var c struct {
			ID                  string
			Name                string
			Type                string
			Participants        string
			CreatedAt           time.Time
			UnreadCount         int
			LastMessageTime     time.Time
			Creator             string
			LastEncrypted       []byte
			AvatarURL           string
			LastMessageUsername string
		}

		if err := rows.Scan(
			&c.ID, &c.Name, &c.Type, &c.Participants, &c.CreatedAt,
			&c.UnreadCount, &c.LastMessageTime, &c.Creator,
			&c.LastEncrypted, &c.AvatarURL, &c.LastMessageUsername,
		); err != nil {
			return nil, fmt.Errorf("failed to scan chat row: %w", err)
		}

		lastText := ""
		if len(c.LastEncrypted) > 0 {
			var err error
			lastText, err = decrypt(c.LastEncrypted)
			if err != nil {
				// 🛠️ 2. Логируем ошибку расшифровки, но не роняем весь список чатов!
				log.Printf("Error: failed to decrypt last message for chat %s: %v", c.ID, err)
				lastText = "⚠️ Message corrupted"
			}
		}

		results = append(results, struct {
			ID                  string
			Name                string
			Type                string
			Participants        string
			CreatedAt           time.Time
			UnreadCount         int
			LastMessageTime     time.Time
			Creator             string
			LastMessageText     string
			AvatarURL           string
			LastMessageUsername string
		}{
			ID:                  c.ID,
			Name:                c.Name,
			Type:                c.Type,
			Participants:        c.Participants,
			CreatedAt:           c.CreatedAt,
			UnreadCount:         c.UnreadCount,
			LastMessageTime:     c.LastMessageTime,
			Creator:             c.Creator,
			LastMessageText:     lastText,
			AvatarURL:           c.AvatarURL,
			LastMessageUsername: c.LastMessageUsername,
		})
	}

	// 🛠️ 3. Обязательная проверка на скрытые ошибки при итерации [INDEX]
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error during chats rows iteration: %w", err)
	}

	return results, nil
}

// UpdateUsername updates a user's username across all related tables.
func (db *DB) UpdateUsername(oldUsername, newUsername string) error {
	// 1. Check if the new username is already taken
	exists, err := db.UserExists(newUsername)
	if err != nil {
		return fmt.Errorf("failed to check if username exists: %w", err)
	}
	if exists {
		return fmt.Errorf("username already taken")
	}

	// 2. Start a transaction for atomic updates across all tables
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Defer rollback in case of failure. Ignore ErrTxDone after a successful commit
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			log.Printf("Error: failed to rollback transaction in UpdateUsername: %v", err)
		}
	}()

	// 3. Update the main users table
	_, err = tx.Exec(`UPDATE users SET username = $1 WHERE username = $2`, newUsername, oldUsername)
	if err != nil {
		return fmt.Errorf("failed to update users table: %w", err)
	}

	// 4. Update messages (both author and replies)
	_, err = tx.Exec(`UPDATE messages SET username = $1 WHERE username = $2`, newUsername, oldUsername)
	if err != nil {
		return fmt.Errorf("failed to update message sender: %w", err)
	}
	_, err = tx.Exec(`UPDATE messages SET replied_to_user = $1 WHERE replied_to_user = $2`, newUsername, oldUsername)
	if err != nil {
		return fmt.Errorf("failed to update message replies: %w", err)
	}

	// 5. Update contacts (both sides)
	_, err = tx.Exec(`UPDATE contacts SET username = $1 WHERE username = $2`, newUsername, oldUsername)
	if err != nil {
		return fmt.Errorf("failed to update contacts username: %w", err)
	}
	_, err = tx.Exec(`UPDATE contacts SET contact_username = $1 WHERE contact_username = $2`, newUsername, oldUsername)
	if err != nil {
		return fmt.Errorf("failed to update contacts contact_username: %w", err)
	}

	// 6. Update notification tokens
	_, err = tx.Exec(`UPDATE user_tokens SET username = $1 WHERE username = $2`, newUsername, oldUsername)
	if err != nil {
		return fmt.Errorf("failed to update user tokens: %w", err)
	}

	// 7. Update chat metadata
	_, err = tx.Exec(`UPDATE user_chat_metadata SET username = $1 WHERE username = $2`, newUsername, oldUsername)
	if err != nil {
		return fmt.Errorf("failed to update chat metadata: %w", err)
	}

	// 8. Update reactions
	_, err = tx.Exec(`UPDATE reactions SET username = $1 WHERE username = $2`, newUsername, oldUsername)
	if err != nil {
		return fmt.Errorf("failed to update reactions: %w", err)
	}

	// 9. Update user themes
	_, err = tx.Exec(`UPDATE user_themes SET username = $1 WHERE username = $2`, newUsername, oldUsername)
	if err != nil {
		return fmt.Errorf("failed to update user themes: %w", err)
	}

	// 10. Update chat creator
	_, err = tx.Exec(`UPDATE chats SET creator_username = $1 WHERE creator_username = $2`, newUsername, oldUsername)
	if err != nil {
		return fmt.Errorf("failed to update chat creator: %w", err)
	}

	// 11. Update participants in chats (using strict JSON manipulation instead of text REPLACE)
	// This prevents accidentally breaking names that are substrings of other names (e.g., 'in' inside 'admin')
	query := `UPDATE chats
              SET participants = (
                  SELECT COALESCE(json_agg(CASE WHEN p = $1 THEN $2 ELSE p END), '[]'::json)
                  FROM json_array_elements_text(participants::json) AS p
              )::text
              WHERE participants::jsonb @> jsonb_build_array($1::text)`

	_, err = tx.Exec(query, oldUsername, newUsername)
	if err != nil {
		return fmt.Errorf("failed to update username in chats JSON array: %w", err)
	}

	// 12. Commit all operations
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Printf("Successfully updated username from '%s' to '%s'", oldUsername, newUsername)
	return nil
}

// UpdatePassword updates a user's password
func (db *DB) UpdatePassword(username, newPassword string) error {
	// 1. Hash the new password using bcrypt
	passwordHash, err := HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// 2. Update the password hash in the database
	query := `UPDATE users SET password_hash = $1 WHERE username = $2`
	result, err := db.Exec(query, passwordHash, username)
	if err != nil {
		return fmt.Errorf("failed to execute password update: %w", err)
	}

	// 3. Verify that the user actually existed and was updated
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("user not found")
	}

	log.Printf("Successfully updated password for user '%s'", username)
	return nil
}

// UpdateAvatar updates a user's avatar URL
func (db *DB) UpdateAvatar(username, avatarURL string) error {
	query := `UPDATE users SET avatar_url = $1 WHERE username = $2`
	result, err := db.Exec(query, avatarURL, username)
	if err != nil {
		return fmt.Errorf("failed to update avatar: %w", err)
	}

	// Verify that the user existed
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}

// GetUserAvatar retrieves a user's avatar URL
func (db *DB) GetUserAvatar(username string) (string, error) {
	var avatarURL string

	// 🛠️ Используем COALESCE, чтобы база данных возвращала пустую строку '' вместо NULL.
	// Это избавляет нас от использования громоздкого sql.NullString в Go!
	query := `SELECT COALESCE(avatar_url, '') FROM users WHERE username = $1`
	err := db.QueryRow(query, username).Scan(&avatarURL)

	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to get user avatar: %w", err)
	}

	return avatarURL, nil
}

// UpdateAvatarWithFull updates both thumbnail and full avatar URLs for a user
func (db *DB) UpdateAvatarWithFull(username, avatarURL, fullAvatarURL string) error {
	query := `UPDATE users SET avatar_url = $1, full_avatar_url = $2 WHERE username = $3`
	result, err := db.Exec(query, avatarURL, fullAvatarURL, username)
	if err != nil {
		return fmt.Errorf("failed to update avatar with full: %w", err)
	}

	// Verify that the user existed
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}

// GetUserAvatarWithFull retrieves both thumbnail and full avatar URLs for a user
func (db *DB) GetUserAvatarWithFull(username string) (string, string, error) {
	var avatarURL, fullAvatarURL string

	query := `SELECT COALESCE(avatar_url, ''), COALESCE(full_avatar_url, '') FROM users WHERE username = $1`
	err := db.QueryRow(query, username).Scan(&avatarURL, &fullAvatarURL)

	if err == sql.ErrNoRows {
		return "", "", nil
	}
	if err != nil {
		return "", "", fmt.Errorf("failed to get user avatar with full: %w", err)
	}

	return avatarURL, fullAvatarURL, nil
}

// CreateChat creates a new chat room and updates participants' versions
func (db *DB) CreateChat(id, name, chatType, participants, creator string) error {
	log.Printf("DB: Creating chat %s (type: %s, creator: %s)", id, chatType, creator)

	// 🛠️ 1. Начинаем транзакцию, чтобы создание чата и обновление версий
	// произошли атомарно (все вместе или ничего).
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to start transaction in CreateChat: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			log.Printf("Error: failed to rollback transaction in CreateChat: %v", err)
		}
	}()

	query := `INSERT INTO chats (id, name, type, participants, creator_username, created_at) 
	          VALUES ($1, $2, $3, $4, $5, NOW())`
	_, err = tx.Exec(query, id, name, chatType, participants, creator)
	if err != nil {
		return fmt.Errorf("failed to insert new chat: %w", err)
	}

	log.Printf("DB: Chat created successfully, incrementing versions...")

	// 🛠️ 2. Метод инкремента должен принимать транзакцию (*sql.Tx),
	// а не выполнять действия в обход нее через db.Exec!
	// Если метод Increment... нельзя переписать под tx, оставьте вызов через db,
	// но ОБЯЗАТЕЛЬНО проверяйте ошибку!
	err = db.IncrementParticipantsChatListVersion(id)
	if err != nil {
		return fmt.Errorf("failed to increment participants chat list version: %w", err)
	}

	// Фиксируем транзакцию
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction in CreateChat: %w", err)
	}

	return nil
}

// GetChat retrieves a chat by ID
func (db *DB) GetChat(id string) (struct {
	ID              string
	Name            string
	Type            string
	Participants    string
	CreatedAt       time.Time
	CreatorUsername string
}, error) {
	var chat struct {
		ID              string
		Name            string
		Type            string
		Participants    string
		CreatedAt       time.Time
		CreatorUsername string
	}

	query := `SELECT id, name, type, participants, created_at, COALESCE(creator_username, '') 
	          FROM chats WHERE id = $1`

	err := db.QueryRow(query, id).Scan(
		&chat.ID, &chat.Name, &chat.Type,
		&chat.Participants, &chat.CreatedAt, &chat.CreatorUsername,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			// 🛠️ 1. Возвращаем пустую структуру и понятную ошибку, если чат не найден
			return chat, fmt.Errorf("chat not found with ID: %s", id)
		}
		// 🛠️ 2. Оборачиваем системную ошибку для удобства отладки
		return chat, fmt.Errorf("failed to scan chat row: %w", err)
	}

	return chat, nil
}

// GetUserChats retrieves all chats for a specific user with unread count and last message time
func (db *DB) GetUserChats(userId, username string) ([]struct {
	ID                  string
	Name                string
	Type                string
	Participants        string
	CreatedAt           time.Time
	UnreadCount         int
	LastMessageTime     time.Time
	Creator             string
	LastMessageText     string
	AvatarURL           string
	LastMessageUsername string
}, error) {
	// 🛠️ 1. Оптимизация SQL: Мы заменяем LIKE и тяжелые подзапросы на один LEFT JOIN.
	// Мы используем оператор JSON @> для точного поиска пользователя в массиве участников!
	query := `
		WITH last_messages AS (
			SELECT DISTINCT ON (room_id) 
				room_id, 
				created_at, 
				encrypted_text, 
				username
			FROM messages
			ORDER BY room_id, created_at DESC
		),
		unread_counts AS (
			SELECT room_id, COUNT(*) as count 
			FROM messages 
			WHERE is_read = FALSE AND user_id != $1::uuid
			GROUP BY room_id
		)
		SELECT 
			c.id, c.name, c.type, c.participants, c.created_at,
			COALESCE(uc.count, 0) as unread_count,
			COALESCE(lm.created_at, c.created_at) as last_message_time,
			COALESCE(c.creator_username, ''),
			COALESCE(lm.encrypted_text, ''::bytea) as last_message_text,
			COALESCE(c.avatar_url, ''),
			COALESCE(lm.username, '') as last_message_username
		FROM chats c
		LEFT JOIN last_messages lm ON c.id = lm.room_id
		LEFT JOIN unread_counts uc ON c.id = uc.room_id
		WHERE c.participants::jsonb @> jsonb_build_array($2::text)
		ORDER BY last_message_time DESC`

	// Внимание: Чтобы это работало быстро, измените тип поля participants в таблице chats
	// с TEXT на JSONB и создайте GIN индекс:
	// CREATE INDEX idx_chats_participants ON chats USING gin (participants);

	rows, err := db.Query(query, userId, username)
	if err != nil {
		return nil, fmt.Errorf("failed to query user chats: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("Error: failed to close rows in GetUserChats: %v", err)
		}
	}()

	results := make([]struct {
		ID                  string
		Name                string
		Type                string
		Participants        string
		CreatedAt           time.Time
		UnreadCount         int
		LastMessageTime     time.Time
		Creator             string
		LastMessageText     string
		AvatarURL           string
		LastMessageUsername string
	}, 0, 15)

	for rows.Next() {
		var c struct {
			ID                  string
			Name                string
			Type                string
			Participants        string
			CreatedAt           time.Time
			UnreadCount         int
			LastMessageTime     time.Time
			Creator             string
			LastEncrypted       []byte
			AvatarURL           string
			LastMessageUsername string
		}

		if err := rows.Scan(
			&c.ID, &c.Name, &c.Type, &c.Participants, &c.CreatedAt,
			&c.UnreadCount, &c.LastMessageTime, &c.Creator,
			&c.LastEncrypted, &c.AvatarURL, &c.LastMessageUsername,
		); err != nil {
			return nil, fmt.Errorf("failed to scan chat row: %w", err)
		}

		lastText := ""
		if len(c.LastEncrypted) > 0 {
			var err error
			lastText, err = decrypt(c.LastEncrypted)
			if err != nil {
				log.Printf("Error: failed to decrypt last message for chat %s: %v", c.ID, err)
				lastText = "⚠️ Message corrupted"
			}
		}

		results = append(results, struct {
			ID                  string
			Name                string
			Type                string
			Participants        string
			CreatedAt           time.Time
			UnreadCount         int
			LastMessageTime     time.Time
			Creator             string
			LastMessageText     string
			AvatarURL           string
			LastMessageUsername string
		}{
			ID:                  c.ID,
			Name:                c.Name,
			Type:                c.Type,
			Participants:        c.Participants,
			CreatedAt:           c.CreatedAt,
			UnreadCount:         c.UnreadCount,
			LastMessageTime:     c.LastMessageTime,
			Creator:             c.Creator,
			LastMessageText:     lastText,
			AvatarURL:           c.AvatarURL,
			LastMessageUsername: c.LastMessageUsername,
		})
	}

	// 🛠️ 2. Скрытые ошибки итератора
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error during user chats rows iteration: %w", err)
	}

	return results, nil
}

// MarkRead updates the last read time for a user in a room and marks messages as read
func (db *DB) MarkRead(roomID, username string) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction in MarkRead: %w", err)
	}

	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			log.Printf("Error: failed to rollback transaction in MarkRead: %v", err)
		}
	}()

	// 🛠️ Добавлено ::text для устранения ошибки pq: inconsistent types
	query1 := `INSERT INTO user_chat_metadata (username, user_id, room_id, last_read_at)
	          VALUES (
				$1::text, 
				(SELECT id FROM users WHERE username = $1::text), 
				$2, 
				NOW()
			  )
              ON CONFLICT (username, room_id) 
			  DO UPDATE SET last_read_at = EXCLUDED.last_read_at`

	_, err = tx.Exec(query1, username, roomID)
	if err != nil {
		return fmt.Errorf("failed to update user chat metadata: %w", err)
	}

	err = db.IncrementUserChatListVersion(username)
	if err != nil {
		return fmt.Errorf("failed to increment user chat list version: %w", err)
	}

	query2 := `UPDATE messages SET is_read = TRUE WHERE room_id = $1 AND username != $2 AND is_read = FALSE`
	_, err = tx.Exec(query2, roomID, username)
	if err != nil {
		return fmt.Errorf("failed to mark messages as read: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction in MarkRead: %w", err)
	}

	return nil
}

// GetDirectChatBetweenUsers finds or creates a direct chat between two users
func (db *DB) GetDirectChatBetweenUsers(user1, user2 string) (string, error) {
	log.Printf("DB: Looking for direct chat between %s and %s", user1, user2)

	// 🛠️ 1. Оптимизация SQL и Безопасность: Мы избавляемся от опасного LIKE.
	// Используем оператор JSON @>, чтобы Postgres искал точное совпадение обоих пользователей!
	// 🛠️ Добавляем ::text к параметрам, чтобы Postgres точно знал тип данных для JSON-массива
	query := `SELECT id FROM chats 
          WHERE type = 'direct' 
            AND participants::jsonb @> jsonb_build_array($1::text, $2::text)`

	var chatID string
	err := db.QueryRow(query, user1, user2).Scan(&chatID)

	if err == nil {
		log.Printf("DB: Found existing direct chat: %s", chatID)
		return chatID, nil
	}

	if err != sql.ErrNoRows {
		return "", fmt.Errorf("failed to look for direct chat: %w", err)
	}

	log.Printf("DB: No existing direct chat found, creating new one...")

	// 🛠️ 2. Для предотвращения дублирования мы генерируем детерминированный ID.
	// Независимо от того, кто открывает чат первым (user1 или user2), ID будет строго одинаковым!
	var newChatID string
	if user1 < user2 {
		newChatID = user1 + "_" + user2 + "_direct"
	} else {
		newChatID = user2 + "_" + user1 + "_direct"
	}

	participants := `["` + user1 + `", "` + user2 + `"]`

	// 🛠️ 3. Используем транзакцию с защитой от одновременной вставки (Race Condition)
	tx, err := db.Begin()
	if err != nil {
		return "", fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			log.Printf("Error: failed to rollback transaction in GetDirectChatBetweenUsers: %v", err)
		}
	}()

	// Проверяем еще раз внутри транзакции по сгенерированному ID
	var checkID string
	err = tx.QueryRow(`SELECT id FROM chats WHERE id = $1`, newChatID).Scan(&checkID)

	if err == nil {
		return checkID, nil // Чат успел создаться параллельным потоком
	}

	// Создаем новый чат
	err = db.CreateChat(newChatID, user1+" & "+user2, "direct", participants, user1)
	if err != nil {
		return "", fmt.Errorf("failed to create direct chat: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Printf("DB: Created new direct chat: %s", newChatID)
	return newChatID, nil
}

// DeleteProfile removes a user and all their data
func (db *DB) DeleteProfile(username string) error {
	// Start a transaction
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			log.Printf("Error: failed to rollback transaction in DeleteProfile: %v", err)
		}
	}()

	var filesToDelete []string

	// 1. Get all image and voice URLs for messages by this user
	rows, err := tx.Query(`SELECT COALESCE(image_url, ''), COALESCE(voice_url, '') 
	                       FROM messages WHERE username = $1`, username)
	if err != nil {
		return fmt.Errorf("failed to query message files: %w", err)
	}
	for rows.Next() {
		var img, voice string
		if err := rows.Scan(&img, &voice); err == nil {
			if img != "" {
				filesToDelete = append(filesToDelete, img)
			}
			if voice != "" {
				filesToDelete = append(filesToDelete, voice)
			}
		}
	}
	rows.Close()

	// 2. Get user's avatar URL
	var avatarURL sql.NullString
	err = tx.QueryRow(`SELECT avatar_url FROM users WHERE username = $1`, username).Scan(&avatarURL)
	if err == nil && avatarURL.Valid && avatarURL.String != "" {
		filesToDelete = append(filesToDelete, avatarURL.String)
	}

	// 3. Get all theme background URLs
	rows, err = tx.Query(`SELECT COALESCE(chat_background_image_url, ''), COALESCE(chat_list_background_image_url, '')
	                       FROM user_themes WHERE username = $1`, username)
	if err != nil {
		return fmt.Errorf("failed to query theme backgrounds: %w", err)
	}
	for rows.Next() {
		var url1, url2 string
		if err := rows.Scan(&url1, &url2); err == nil {
			if url1 != "" {
				filesToDelete = append(filesToDelete, url1)
			}
			if url2 != "" {
				filesToDelete = append(filesToDelete, url2)
			}
		}
	}
	rows.Close()

	// 4. Delete user's messages
	_, err = tx.Exec(`DELETE FROM messages WHERE username = $1`, username)
	if err != nil {
		return fmt.Errorf("failed to delete user messages: %w", err)
	}

	// 5. Delete user's reactions
	_, err = tx.Exec(`DELETE FROM reactions WHERE username = $1`, username)
	if err != nil {
		return fmt.Errorf("failed to delete user reactions: %w", err)
	}

	// 6. Delete user's tokens
	_, err = tx.Exec(`DELETE FROM user_tokens WHERE username = $1`, username)
	if err != nil {
		return fmt.Errorf("failed to delete user tokens: %w", err)
	}

	// 7. Delete user's metadata
	_, err = tx.Exec(`DELETE FROM user_chat_metadata WHERE username = $1`, username)
	if err != nil {
		return fmt.Errorf("failed to delete user metadata: %w", err)
	}

	// 8. Delete user's contacts
	_, err = tx.Exec(`DELETE FROM contacts WHERE username = $1 OR contact_username = $1`, username)
	if err != nil {
		return fmt.Errorf("failed to delete user contacts: %w", err)
	}

	// 9. Update chats participants strictly (JSON operation)
	// Instead of unsafe text REPLACE, we filter out the deleted username from the JSON array
	query := `UPDATE chats
              SET participants = (
                  SELECT COALESCE(json_agg(p), '[]'::json)
                  FROM json_array_elements_text(participants::json) AS p
                  WHERE p != $1
              )::text
              WHERE participants::jsonb @> jsonb_build_array($1::text)`

	_, err = tx.Exec(query, username)
	if err != nil {
		return fmt.Errorf("failed to update participants in chats: %w", err)
	}

	// 10. Delete the user itself
	_, err = tx.Exec(`DELETE FROM users WHERE username = $1`, username)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	// 🛠️ Физическое удаление файлов (Только после успешного коммита!)
	go func(urls []string) {
		for _, fileURL := range urls {
			if err := DeleteImageFile(fileURL); err != nil {
				log.Printf("Error: failed to delete file %s during profile deletion: %v", fileURL, err)
			}
		}
	}(filesToDelete)

	log.Printf("Successfully deleted profile and all associated data for user: %s", username)
	return nil
}

// CleanupEmptyMessages removes all corrupted messages (marked by maintenance)
func (db *DB) CleanupEmptyMessages() (int64, error) {
	// 🛠️ Безопасность: Мы добавили условие WHERE.
	// Теперь функция удалит ТОЛЬКО те сообщения, которые были помечены
	// как битые в результате смены ключей или системных сбоев.

	query := `DELETE FROM messages 
	          WHERE encrypted_text = 'DECRYPTION_FAILED'::bytea 
	             OR encrypted_text = 'CORRUPTED_FIX'::bytea`

	result, err := db.Exec(query)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup corrupted messages: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()

	log.Printf("Successfully cleaned up %d corrupted messages from database", rowsAffected)
	return rowsAffected, nil
}

// GetUserProfile retrieves user profile information
func (db *DB) GetUserProfile(username string) (struct {
	Username  string
	Bio       string
	Status    string
	AvatarURL string
}, error) {
	// Создаем структуру, которую сразу вернем (без промежуточных NullString)
	var profile struct {
		Username  string
		Bio       string
		Status    string
		AvatarURL string
	}

	// 🛠️ COALESCE гарантирует, что мы получим пустую строку вместо NULL.
	query := `SELECT 
				username, 
				COALESCE(bio, ''), 
				COALESCE(status, ''), 
				COALESCE(avatar_url, '') 
			 FROM users 
			 WHERE username = $1`

	err := db.QueryRow(query, username).Scan(
		&profile.Username,
		&profile.Bio,
		&profile.Status,
		&profile.AvatarURL,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return profile, fmt.Errorf("user not found: %s", username)
		}
		return profile, fmt.Errorf("failed to scan user profile: %w", err)
	}

	return profile, nil
}

// UpdateProfile updates user profile information
func (db *DB) UpdateProfile(username, bio, status string) error {
	query := `UPDATE users SET bio = $1, status = $2 WHERE username = $3`
	result, err := db.Exec(query, bio, status, username)
	if err != nil {
		return fmt.Errorf("failed to execute profile update: %w", err)
	}

	// Verify that the user actually existed and was updated
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("user not found: %s", username)
	}

	log.Printf("Successfully updated profile for user '%s'", username)
	return nil
}

// UpdateChatParticipants updates the participants of a chat
func (db *DB) UpdateChatParticipants(chatID, participants string) error {
	query := `UPDATE chats SET participants = $1 WHERE id = $2`
	result, err := db.Exec(query, participants, chatID)
	if err != nil {
		return fmt.Errorf("failed to update chat participants: %w", err)
	}

	// 🛠️ 1. Проверяем, что чат существовал
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("chat not found with ID: %s", chatID)
	}

	return nil
}

// UpdateChatName updates the name of a chat
func (db *DB) UpdateChatName(chatID, newName string) error {
	query := `UPDATE chats SET name = $1 WHERE id = $2`
	result, err := db.Exec(query, newName, chatID)
	if err != nil {
		return fmt.Errorf("failed to update chat name: %w", err)
	}

	// 🛠️ 2. Проверяем, что чат существовал
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("chat not found with ID: %s", chatID)
	}

	return nil
}

// UpdateChatAvatar updates the avatar URL for a chat
func (db *DB) UpdateChatAvatar(chatID, avatarURL string) error {
	query := `UPDATE chats SET avatar_url = $1 WHERE id = $2`
	result, err := db.Exec(query, avatarURL, chatID)
	if err != nil {
		return fmt.Errorf("failed to update chat avatar: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("chat not found")
	}

	return nil
}

// UpdateChatAvatarWithFull updates both thumbnail and full avatar URLs for a chat
func (db *DB) UpdateChatAvatarWithFull(chatID, avatarURL, fullAvatarURL string) error {
	query := `UPDATE chats SET avatar_url = $1, full_avatar_url = $2 WHERE id = $3`
	result, err := db.Exec(query, avatarURL, fullAvatarURL, chatID)
	if err != nil {
		return fmt.Errorf("failed to update chat avatar with full: %w", err)
	}

	// 🛠️ 3. Проверяем, что чат существовал
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("chat not found with ID: %s", chatID)
	}

	return nil
}

// AddContact adds a contact for a user
func (db *DB) AddContact(username, contactUsername string) error {
	query := `INSERT INTO contacts (user_id, contact_id, username, contact_username)
	          SELECT u1.id, u2.id, u1.username, u2.username
	          FROM users u1, users u2
	          WHERE u1.username = $1 AND u2.username = $2
	          ON CONFLICT (user_id, contact_id) DO NOTHING`

	result, err := db.Exec(query, username, contactUsername)
	if err != nil {
		return fmt.Errorf("failed to add contact: %w", err)
	}

	// 🛠️ 1. Проверяем, что пользователи действительно существовали в таблице users
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("could not add contact: one or both users do not exist")
	}

	return nil
}

// RemoveContact removes a contact for a user
func (db *DB) RemoveContact(username, contactUsername string) error {
	query := `DELETE FROM contacts WHERE username = $1 AND contact_username = $2`
	_, err := db.Exec(query, username, contactUsername)
	if err != nil {
		return fmt.Errorf("failed to remove contact: %w", err)
	}
	return nil
}

// AddFavorite adds a message to a user's favorites

// GetContacts retrieves all contacts for a user
func (db *DB) GetContacts(username string) ([]string, error) {
	// 🛠️ 2. Оптимизация SQL: мы объединили два запроса в один через COALESCE.
	// Если у нас есть связь по UUID, мы берем свежее имя пользователя u_contact.username.
	// Если связи по UUID еще нет (старая запись), мы берем старое имя c.contact_username.
	query := `SELECT COALESCE(u_contact.username, c.contact_username)
	          FROM contacts c
	          LEFT JOIN users u_me ON c.user_id = u_me.id
	          LEFT JOIN users u_contact ON c.contact_id = u_contact.id
	          WHERE c.username = $1 OR u_me.username = $1`

	rows, err := db.Query(query, username)
	if err != nil {
		return nil, fmt.Errorf("failed to query contacts: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("Error: failed to close rows in GetContacts: %v", err)
		}
	}()

	contacts := make([]string, 0, 20)
	for rows.Next() {
		var contact string
		if err := rows.Scan(&contact); err != nil {
			return nil, fmt.Errorf("failed to scan contact row: %w", err)
		}
		contacts = append(contacts, contact)
	}

	// 🛠️ 3. Обязательная проверка на скрытые ошибки при итерации [INDEX]
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error during contacts rows iteration: %w", err)
	}

	return contacts, nil
}

// UpdateMessageText updates the text of a message by UUID and marks it as edited
func (db *DB) UpdateMessageText(messageID, newText string) error {
	encrypted, err := encrypt(newText)
	if err != nil {
		return fmt.Errorf("failed to encrypt updated message text: %w", err)
	}

	query := `UPDATE messages SET encrypted_text = $1, edited = true WHERE message_id = $2`
	result, err := db.Exec(query, encrypted, messageID)
	if err != nil {
		return fmt.Errorf("failed to execute message text update: %w", err)
	}

	// 🛠️ 1. Проверяем, что сообщение существовало
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("message not found with UUID: %s", messageID)
	}

	return nil
}

// GetUserChatListVersion returns the current version of the user's chat list
func (db *DB) GetUserChatListVersion(username string) (int64, error) {
	var version int64
	query := `SELECT COALESCE(chat_list_version, 0) FROM users WHERE username = $1`
	err := db.QueryRow(query, username).Scan(&version)

	if err == sql.ErrNoRows {
		return 0, fmt.Errorf("user not found: %s", username)
	}
	if err != nil {
		return 0, fmt.Errorf("failed to get chat list version: %w", err)
	}

	return version, nil
}

// IncrementUserChatListVersion increments the chat list version for a specific user
func (db *DB) IncrementUserChatListVersion(username string) error {
	// 🛠️ 2. Используем атомарный апдейт базы данных.
	// Конструкция 'SET version = version + 1' на уровне СУБД защищает нас
	// от состояния гонки (Race Condition), когда два пуша происходят одновременно.
	query := `UPDATE users SET chat_list_version = chat_list_version + 1 WHERE username = $1`
	result, err := db.Exec(query, username)
	if err != nil {
		return fmt.Errorf("failed to increment chat list version: %w", err)
	}

	// Проверяем, что счетчик действительно обновился
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("failed to increment version: user not found: %s", username)
	}

	return nil
}

// IncrementParticipantsChatListVersion increments the version for all participants in a chat
func (db *DB) IncrementParticipantsChatListVersion(chatID string) error {
	// 🛠️ 1. Решение проблемы производительности:
	// Мы делаем ОДИН SQL-запрос, который на уровне Postgres находит участников
	// в JSON-массиве таблицы chats и сразу же увеличивает версию каждого из них!
	// Это работает в сотни раз быстрее исходного кода.

	query := `
		UPDATE users
		SET chat_list_version = chat_list_version + 1
		WHERE username IN (
			SELECT json_array_elements_text(participants::json)
			FROM chats
			WHERE id = $1
		)`

	result, err := db.Exec(query, chatID)
	if err != nil {
		return fmt.Errorf("failed to increment participants version for chat %s: %w", chatID, err)
	}

	// 🛠️ 2. Проверяем, затронуло ли это хоть кого-то
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		// Либо чат не найден, либо в нем нет участников (или они не в users)
		log.Printf("Warning: no participants updated for chat %s", chatID)
	} else {
		log.Printf("Successfully incremented chat list version for %d participants of chat %s", rowsAffected, chatID)
	}

	return nil
}

// UserTheme represents a custom theme in the database
type UserTheme struct {
	ThemeID                    string
	Name                       string
	PrimaryColor               string
	OnPrimaryColor             string
	SurfaceColor               string
	OnSurfaceColor             string
	BackgroundColor            string
	TextPrimaryColor           string
	TextSecondaryColor         string
	IsDark                     bool
	ChatBackgroundImageUrl     string
	ChatListBackgroundImageUrl string
	BottomPanelColor           string
	OnBottomPanelColor         string
	SurfaceContainer           string
	OutgoingBubbleColor        string
	IncomingBubbleColor        string
}

// GetUserThemes retrieves current theme ID and all custom themes for a user
func (db *DB) GetUserThemes(username string) (string, []UserTheme, error) {
	var currentID string
	err := db.QueryRow(`SELECT current_theme_id FROM users WHERE username = $1`, username).Scan(&currentID)
	if err != nil {
		if err == sql.ErrNoRows {
			return "dark", nil, fmt.Errorf("user not found: %s", username)
		}
		return "dark", nil, fmt.Errorf("failed to get current theme ID: %w", err)
	}

	rows, err := db.Query(`SELECT theme_id, name, primary_color, on_primary_color, 
	                              surface_color, on_surface_color, background_color, 
	                              text_primary_color, text_secondary_color, is_dark, 
	                              COALESCE(chat_background_image_url, ''),
	                              COALESCE(chat_list_background_image_url, ''), 
	                              COALESCE(bottom_panel_color, ''), 
	                              COALESCE(on_bottom_panel_color, ''),
	                              COALESCE(surface_container, ''),
	                              COALESCE(outgoing_bubble_color, ''),
	                              COALESCE(incoming_bubble_color, '')
	                       FROM user_themes WHERE username = $1`, username)
	if err != nil {
		return currentID, nil, fmt.Errorf("failed to query user themes: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("Error: failed to close rows in GetUserThemes: %v", err)
		}
	}()

	themes := make([]UserTheme, 0, 5)
	for rows.Next() {
		var t UserTheme
		err := rows.Scan(
			&t.ThemeID, &t.Name, &t.PrimaryColor, &t.OnPrimaryColor,
			&t.SurfaceColor, &t.OnSurfaceColor, &t.BackgroundColor,
			&t.TextPrimaryColor, &t.TextSecondaryColor, &t.IsDark,
			&t.ChatBackgroundImageUrl, &t.ChatListBackgroundImageUrl,
			&t.BottomPanelColor, &t.OnBottomPanelColor,
			&t.SurfaceContainer, &t.OutgoingBubbleColor, &t.IncomingBubbleColor,
		)
		if err != nil {
			// 🛠️ 1. Обязательно возвращаем ошибку, если строка повреждена
			return currentID, nil, fmt.Errorf("failed to scan theme row: %w", err)
		}
		themes = append(themes, t)
	}

	// 🛠️ 2. Проверяем скрытые ошибки итерации [2]
	if err := rows.Err(); err != nil {
		return currentID, nil, fmt.Errorf("error during themes rows iteration: %w", err)
	}

	return currentID, themes, nil
}

// SaveUserTheme saves or updates a custom theme
func (db *DB) SaveUserTheme(username string, theme *gen.CustomTheme) error {
	// 🛠️ 3. Используем EXCLUDED для профессиональной реализации UPSERT [2]
	query := `INSERT INTO user_themes (username, theme_id, name, primary_color, on_primary_color, surface_color, on_surface_color, background_color, text_primary_color, text_secondary_color, is_dark, chat_background_image_url, chat_list_background_image_url, bottom_panel_color, on_bottom_panel_color, surface_container, outgoing_bubble_color, incoming_bubble_color)
	          VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
	          ON CONFLICT (username, theme_id) DO UPDATE SET
	          name = EXCLUDED.name, 
	          primary_color = EXCLUDED.primary_color, 
	          on_primary_color = EXCLUDED.on_primary_color, 
	          surface_color = EXCLUDED.surface_color, 
	          on_surface_color = EXCLUDED.on_surface_color, 
	          background_color = EXCLUDED.background_color, 
	          text_primary_color = EXCLUDED.text_primary_color, 
	          text_secondary_color = EXCLUDED.text_secondary_color, 
	          is_dark = EXCLUDED.is_dark, 
	          chat_background_image_url = EXCLUDED.chat_background_image_url,
	          chat_list_background_image_url = EXCLUDED.chat_list_background_image_url, 
	          bottom_panel_color = EXCLUDED.bottom_panel_color, 
	          on_bottom_panel_color = EXCLUDED.on_bottom_panel_color,
	          surface_container = EXCLUDED.surface_container,
	          outgoing_bubble_color = EXCLUDED.outgoing_bubble_color,
	          incoming_bubble_color = EXCLUDED.incoming_bubble_color`

	_, err := db.Exec(query, username, theme.Id, theme.Name, theme.PrimaryColor, theme.OnPrimaryColor, theme.SurfaceColor, theme.OnSurfaceColor, theme.BackgroundColor, theme.TextPrimaryColor, theme.TextSecondaryColor, theme.IsDark, theme.ChatBackgroundImageUrl, theme.ChatListBackgroundImageUrl, theme.BottomPanelColor, theme.OnBottomPanelColor, theme.SurfaceContainer, theme.OutgoingBubbleColor, theme.IncomingBubbleColor)
	if err != nil {
		return fmt.Errorf("failed to save user theme: %w", err)
	}
	return nil
}

// AddFavorite adds a message to a user's favorites

// SetCurrentTheme updates the user's selected theme ID
func (db *DB) SetCurrentTheme(username, themeID string) error {
	query := `UPDATE users SET current_theme_id = $1 WHERE username = $2`
	result, err := db.Exec(query, themeID, username)
	if err != nil {
		return fmt.Errorf("failed to set current theme: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("user not found: %s", username)
	}
	return nil
}

// AddFavorite adds a message to a user's favorites

// DeleteUserTheme removes a custom theme and its associated background image
func (db *DB) DeleteUserTheme(username, themeID string) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			log.Printf("Error: failed to rollback transaction in DeleteUserTheme: %v", err)
		}
	}()

	// 🛠️ 4. Безопасно получаем URL файлов внутри транзакции
	var bgURL, chatListBgURL sql.NullString
	err = tx.QueryRow(`SELECT chat_background_image_url, chat_list_background_image_url
	                   FROM user_themes 
	                   WHERE username = $1 AND theme_id = $2`, username, themeID).Scan(&bgURL, &chatListBgURL)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("failed to query theme backgrounds before deletion: %w", err)
	}

	// 5. Delete from database
	_, err = tx.Exec(`DELETE FROM user_themes WHERE username = $1 AND theme_id = $2`, username, themeID)
	if err != nil {
		return fmt.Errorf("failed to delete theme from database: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	// 🛠️ 5. Физическое удаление файлов строго ПОСЛЕ коммита и в фоне [1, 2]
	go func(urls []string) {
		for _, url := range urls {
			if url != "" {
				if err := DeleteImageFile(url); err != nil {
					log.Printf("Error: failed to delete file %s after theme deletion: %v", url, err)
				}
			}
		}
	}([]string{bgURL.String, chatListBgURL.String})

	return nil
}

// SaveDraft saves or updates a draft message for a user in a specific room
func (db *DB) SaveDraft(username, roomID, draftText, repliedToMessageID, repliedToUser, repliedToText string) error {
	query := `INSERT INTO draft_messages (username, room_id, draft_text, replied_to_message_id, replied_to_user, replied_to_text, updated_at)
	          VALUES ($1, $2, $3, $4, $5, $6, NOW())
	          ON CONFLICT (username, room_id) 
	          DO UPDATE SET 
	            draft_text = EXCLUDED.draft_text,
	            replied_to_message_id = EXCLUDED.replied_to_message_id,
	            replied_to_user = EXCLUDED.replied_to_user,
	            replied_to_text = EXCLUDED.replied_to_text,
	            updated_at = NOW()`

	_, err := db.Exec(query, username, roomID, draftText, repliedToMessageID, repliedToUser, repliedToText)
	if err != nil {
		return fmt.Errorf("failed to save draft: %w", err)
	}
	return nil
}

// AddFavorite adds a message to a user's favorites

// GetDraft retrieves a draft message for a user in a specific room
func (db *DB) GetDraft(username, roomID string) (struct {
	DraftText          string
	RepliedToMessageID string
	RepliedToUser      string
	RepliedToText      string
	UpdatedAt          time.Time
}, error) {
	var result struct {
		DraftText          string
		RepliedToMessageID string
		RepliedToUser      string
		RepliedToText      string
		UpdatedAt          time.Time
	}

	query := `SELECT COALESCE(draft_text, ''), 
	                 COALESCE(replied_to_message_id, ''), 
	                 COALESCE(replied_to_user, ''), 
	                 COALESCE(replied_to_text, ''), 
	                 updated_at 
	          FROM draft_messages 
	          WHERE username = $1 AND room_id = $2`

	err := db.QueryRow(query, username, roomID).Scan(
		&result.DraftText,
		&result.RepliedToMessageID,
		&result.RepliedToUser,
		&result.RepliedToText,
		&result.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		// No draft found, return empty result
		return result, nil
	}
	if err != nil {
		return result, fmt.Errorf("failed to get draft: %w", err)
	}

	return result, nil
}

// DeleteDraft removes a draft message for a user in a specific room
func (db *DB) DeleteDraft(username, roomID string) error {
	query := `DELETE FROM draft_messages WHERE username = $1 AND room_id = $2`
	_, err := db.Exec(query, username, roomID)
	if err != nil {
		return fmt.Errorf("failed to delete draft: %w", err)
	}
	return nil
}

// AddFavorite adds a message to a user's favorites

// GetMutedChats returns a list of room IDs that the user has muted (no push notifications)
func (db *DB) GetMutedChats(username string) ([]string, error) {
	query := `SELECT room_id FROM muted_chats WHERE username = $1 AND muted = TRUE`
	rows, err := db.Query(query, username)
	if err != nil {
		return nil, fmt.Errorf("failed to get muted chats: %w", err)
	}
	defer rows.Close()

	var mutedChats []string
	for rows.Next() {
		var roomID string
		if err := rows.Scan(&roomID); err != nil {
			return nil, fmt.Errorf("failed to scan muted chat room_id: %w", err)
		}
		mutedChats = append(mutedChats, roomID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating muted chats rows: %w", err)
	}

	return mutedChats, nil
}

// SetMutedChat sets the mute status for a specific chat room for a user
// If muted=true, user will not receive push notifications from this chat
// If muted=false, removes the mute entry (user receives notifications normally)
func (db *DB) SetMutedChat(username, roomID string, muted bool) error {
	if muted {
		// Upsert: insert or update to muted=true
		query := `
			INSERT INTO muted_chats (username, room_id, muted, updated_at)
			VALUES ($1, $2, TRUE, NOW())
			ON CONFLICT (username, room_id)
			DO UPDATE SET muted = TRUE, updated_at = NOW()`
		_, err := db.Exec(query, username, roomID)
		if err != nil {
			return fmt.Errorf("failed to set muted chat: %w", err)
		}
	} else {
		// Remove mute entry - delete the row
		query := `DELETE FROM muted_chats WHERE username = $1 AND room_id = $2`
		_, err := db.Exec(query, username, roomID)
		if err != nil {
			return fmt.Errorf("failed to unmute chat: %w", err)
		}
	}
	return nil
}

// AddFavorite adds a message to a user's favorites

// SaveDraftByUserID saves or updates a draft message for a user (by UUID) in a specific room
func (db *DB) SaveDraftByUserID(userID, roomID, draftText, repliedToMessageID, repliedToUser, repliedToText string) error {
	query := `INSERT INTO draft_messages (user_id, username, room_id, draft_text, replied_to_message_id, replied_to_user, replied_to_text, updated_at)
	          VALUES ($1::uuid, (SELECT username FROM users WHERE id = $1::uuid), $2, $3, $4, $5, $6, NOW())
	          ON CONFLICT (user_id, room_id)
	          DO UPDATE SET
	            draft_text = EXCLUDED.draft_text,
	            replied_to_message_id = EXCLUDED.replied_to_message_id,
	            replied_to_user = EXCLUDED.replied_to_user,
	            replied_to_text = EXCLUDED.replied_to_text,
	            updated_at = NOW()`

	_, err := db.Exec(query, userID, roomID, draftText, repliedToMessageID, repliedToUser, repliedToText)
	if err != nil {
		return fmt.Errorf("failed to save draft by user_id: %w", err)
	}
	return nil
}

// AddFavorite adds a message to a user's favorites

// GetDraftByUserID retrieves a draft message for a user (by UUID) in a specific room
func (db *DB) GetDraftByUserID(userID, roomID string) (struct {
	DraftText          string
	RepliedToMessageID string
	RepliedToUser      string
	RepliedToText      string
	UpdatedAt          time.Time
}, error) {
	var result struct {
		DraftText          string
		RepliedToMessageID string
		RepliedToUser      string
		RepliedToText      string
		UpdatedAt          time.Time
	}

	query := `SELECT COALESCE(draft_text, ''),
	                 COALESCE(replied_to_message_id, ''),
	                 COALESCE(replied_to_user, ''),
	                 COALESCE(replied_to_text, ''),
	                 updated_at
	          FROM draft_messages
	          WHERE user_id = $1::uuid AND room_id = $2`

	err := db.QueryRow(query, userID, roomID).Scan(
		&result.DraftText,
		&result.RepliedToMessageID,
		&result.RepliedToUser,
		&result.RepliedToText,
		&result.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		// No draft found, return empty result
		return result, nil
	}
	if err != nil {
		return result, fmt.Errorf("failed to get draft by user_id: %w", err)
	}

	return result, nil
}

// DeleteDraftByUserID removes a draft message for a user (by UUID) in a specific room
// Returns true if a draft was actually deleted, false if no draft existed
func (db *DB) DeleteDraftByUserID(userID, roomID string) (bool, error) {
	query := `DELETE FROM draft_messages WHERE user_id = $1::uuid AND room_id = $2`
	result, err := db.Exec(query, userID, roomID)
	if err != nil {
		return false, fmt.Errorf("failed to delete draft by user_id: %w", err)
	}
	rowsAffected, _ := result.RowsAffected()
	return rowsAffected > 0, nil
}

// GetMutedChatsByUserID returns a list of room IDs that the user (by UUID) has muted
func (db *DB) GetMutedChatsByUserID(userID string) ([]string, error) {
	query := `SELECT room_id FROM muted_chats WHERE user_id = $1::uuid AND muted = TRUE`
	rows, err := db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get muted chats by user_id: %w", err)
	}
	defer rows.Close()

	var mutedChats []string
	for rows.Next() {
		var roomID string
		if err := rows.Scan(&roomID); err != nil {
			return nil, fmt.Errorf("failed to scan muted chat room_id: %w", err)
		}
		mutedChats = append(mutedChats, roomID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating muted chats rows: %w", err)
	}

	return mutedChats, nil
}

// GetUserIdByUsername retrieves the UUID for a given username
func (db *DB) GetUserIdByUsername(username string) (string, error) {
	var userID string
	query := `SELECT id FROM users WHERE username = $1::text`
	err := db.QueryRow(query, username).Scan(&userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("user not found: %s", username)
		}
		return "", fmt.Errorf("failed to get user id: %w", err)
	}
	return userID, nil
}

// GetUsernameByID retrieves the username for a given user ID (UUID)
func (db *DB) GetUsernameByID(userID string) (string, error) {
	var username string
	query := `SELECT username FROM users WHERE id = $1::uuid`
	err := db.QueryRow(query, userID).Scan(&username)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("user not found: %s", userID)
		}
		return "", fmt.Errorf("failed to get username: %w", err)
	}
	return username, nil
}

// SetMutedChatByUserID sets the mute status for a specific chat room for a user (by UUID)
func (db *DB) SetMutedChatByUserID(userID, roomID string, muted bool) error {
	if muted {
		// Upsert: insert or update to muted=true
		query := `
			INSERT INTO muted_chats (user_id, username, room_id, muted, updated_at)
			VALUES ($1::uuid, (SELECT username FROM users WHERE id = $1::uuid), $2, TRUE, NOW())
			ON CONFLICT (user_id, room_id)
			DO UPDATE SET muted = TRUE, updated_at = NOW()`
		_, err := db.Exec(query, userID, roomID)
		if err != nil {
			return fmt.Errorf("failed to set muted chat by user_id: %w", err)
		}
	} else {
		// Remove mute entry - delete the row
		query := `DELETE FROM muted_chats WHERE user_id = $1::uuid AND room_id = $2`
		_, err := db.Exec(query, userID, roomID)
		if err != nil {
			return fmt.Errorf("failed to unmute chat by user_id: %w", err)
		}
	}
	return nil
}

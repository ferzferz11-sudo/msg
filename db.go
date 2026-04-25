// Lavender Messenger - A secure messaging application
// Author: Pavel Davydov (ferz)
//
// This file handles database operations for the Lavender Messenger.
// It manages PostgreSQL connections, message storage, and table creation.

package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
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

	// Test the connection to ensure it's valid and reachable
	err = db.Ping()
	if err != nil {
		if db != nil {
			if closeErr := db.Close(); closeErr != nil {
				log.Printf("Warning: error closing database connection: %v", closeErr)
			}
		}
		return nil, fmt.Errorf("unable to ping database: %w\nDATABASE_URL: %s", err, maskPassword(dbUrl))
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
		`CREATE TABLE IF NOT EXISTS user_tokens (
			username VARCHAR(255) PRIMARY KEY,
			fcm_token TEXT NOT NULL,
			updated_at TIMESTAMP NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS users (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			username VARCHAR(255) UNIQUE NOT NULL,
			password_hash VARCHAR(255) NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			avatar_url VARCHAR(512),
			bio TEXT,
			status VARCHAR(255)
		);`,
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
	}

	for _, query := range queries {
		_, err = db.Exec(query)
		if err != nil {
			if db != nil {
				if closeErr := db.Close(); closeErr != nil {
					log.Printf("Warning: error closing database connection: %v", closeErr)
				}
			}
			return nil, fmt.Errorf("failed to execute query: %w\nQuery: %s", err, query)
		}
	}

	log.Println("Database connected, tables ready")

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
func (db *DB) SaveMessage(messageID string, username string, encryptedText []byte, createdAt time.Time, repliedToMessageID string, repliedToUser string, repliedToText string, roomID string, imageURL string) error {
	query := `INSERT INTO messages (message_id, username, user_id, encrypted_text, created_at, replied_to_message_id, replied_to_user, replied_to_text, room_id, is_read, image_url)
	          VALUES ($1, $2::text, (SELECT id FROM users WHERE username = $2::text), $3, $4, $5, $6, $7, $8, FALSE, $9)`
	_, err := db.Exec(query, messageID, username, encryptedText, createdAt, repliedToMessageID, repliedToUser, repliedToText, roomID, imageURL)
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
}, error) {
	query := `SELECT COALESCE(m.message_id, ''), m.username, m.encrypted_text, m.created_at, COALESCE(m.replied_to_message_id, ''), COALESCE(m.replied_to_user, ''), COALESCE(m.replied_to_text, ''), COALESCE(m.room_id, ''), m.is_read, u.avatar_url, COALESCE(m.image_url, ''), COALESCE(m.edited, false)
	             FROM messages m
	             LEFT JOIN users u ON m.username = u.username
	             WHERE m.room_id = $1 ORDER BY m.created_at DESC LIMIT $2`
	rows, err := db.Query(query, roomID, limit)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("Warning: error closing rows: %v", err)
		}
	}()

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
			AvatarURL          sql.NullString
			ImageURL           string
			Edited             bool
		}
		if err := rows.Scan(&r.MessageID, &r.Username, &r.Encrypted, &r.CreatedAt, &r.RepliedToMessageID, &r.RepliedToUser, &r.RepliedToText, &r.RoomID, &r.IsRead, &r.AvatarURL, &r.ImageURL, &r.Edited); err != nil {
			return nil, err
		}
		avatarURL := ""
		if r.AvatarURL.Valid {
			avatarURL = r.AvatarURL.String
		}
		results = append(results, struct {
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
		}{
			MessageID:          r.MessageID,
			Username:           r.Username,
			Encrypted:          r.Encrypted,
			Edited:             r.Edited,
			CreatedAt:          r.CreatedAt,
			RepliedToMessageID: r.RepliedToMessageID,
			RepliedToUser:      r.RepliedToUser,
			RepliedToText:      r.RepliedToText,
			RoomID:             r.RoomID,
			IsRead:             r.IsRead,
			AvatarURL:          avatarURL,
			ImageURL:           r.ImageURL,
		})
	}
	return results, nil
}

// SetReaction saves or updates a reaction
func (db *DB) SetReaction(messageID string, username string, emoji string) error {
	// Check if message exists first to avoid foreign key constraint violation
	var exists bool
	checkQuery := `SELECT EXISTS(SELECT 1 FROM messages WHERE message_id = $1)`
	err := db.QueryRow(checkQuery, messageID).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("message not found")
	}

	query := `INSERT INTO reactions (message_id, username, user_id, emoji)
	          VALUES ($1, $2::text, (SELECT id FROM users WHERE username = $2::text), $3)
              ON CONFLICT (message_id, username) DO UPDATE SET emoji = $3`
	_, err = db.Exec(query, messageID, username, emoji)
	return err
}

// GetReactionsForMessage retrieves reactions for a specific message
func (db *DB) GetReactionsForMessage(messageID string) ([]struct {
	Username string
	Emoji    string
}, error) {
	query := `SELECT username, emoji FROM reactions WHERE message_id = $1`
	rows, err := db.Query(query, messageID)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("Warning: error closing rows: %v", err)
		}
	}()

	var results []struct {
		Username string
		Emoji    string
	}

	for rows.Next() {
		var r struct {
			Username string
			Emoji    string
		}
		if err := rows.Scan(&r.Username, &r.Emoji); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, nil
}

// GetMessagesByUserAndTime retrieves messages matching username and time window
func (db *DB) GetMessagesByUserAndTime(username string, createdAt time.Time) ([]struct {
	ID        int
	Encrypted []byte
	ImageURL  string
}, error) {
	query := `SELECT id, encrypted_text, image_url FROM messages WHERE username = $1 AND ABS(EXTRACT(EPOCH FROM (created_at - $2))) < 2`
	rows, err := db.Query(query, username, createdAt)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("Warning: error closing rows: %v", err)
		}
	}()

	var results []struct {
		ID        int
		Encrypted []byte
		ImageURL  string
	}

	for rows.Next() {
		var r struct {
			ID        int
			Encrypted []byte
			ImageURL  string
		}
		if err := rows.Scan(&r.ID, &r.Encrypted, &r.ImageURL); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, nil
}

// GetMessageImageURL retrieves the image URL for a message by its UUID
func (db *DB) GetMessageImageURL(messageID string) (string, error) {
	var imageURL string
	query := `SELECT image_url FROM messages WHERE message_id = $1`
	err := db.QueryRow(query, messageID).Scan(&imageURL)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return imageURL, err
}

// DeleteMessageByUUID deletes a message by its unique message_id
func (db *DB) DeleteMessageByUUID(messageID string) error {
	query := `DELETE FROM messages WHERE message_id = $1`
	_, err := db.Exec(query, messageID)
	return err
}

// DeleteMessageByID deletes a message by its serial database ID
func (db *DB) DeleteMessageByID(id int) error {
	query := `DELETE FROM messages WHERE id = $1`
	_, err := db.Exec(query, id)
	return err
}

// GetChatMessagesImageURLs returns all image URLs for messages in a specific chat
func (db *DB) GetChatMessagesImageURLs(roomID string) ([]string, error) {
	query := `SELECT image_url FROM messages WHERE room_id = $1 AND image_url != ''`
	rows, err := db.Query(query, roomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var urls []string
	for rows.Next() {
		var url string
		if err := rows.Scan(&url); err != nil {
			return nil, err
		}
		urls = append(urls, url)
	}
	return urls, nil
}

// DeleteChat removes a chat and all its associated data
func (db *DB) DeleteChat(chatID string) error {
	// 1. Delete messages (reactions will be deleted via ON DELETE CASCADE)
	_, err := db.Exec(`DELETE FROM messages WHERE room_id = $1`, chatID)
	if err != nil {
		return fmt.Errorf("failed to delete messages: %w", err)
	}

	// 2. Delete chat metadata
	_, err = db.Exec(`DELETE FROM user_chat_metadata WHERE room_id = $1`, chatID)
	if err != nil {
		return fmt.Errorf("failed to delete chat metadata: %w", err)
	}

	// 3. Delete the chat itself
	_, err = db.Exec(`DELETE FROM chats WHERE id = $1`, chatID)
	if err != nil {
		return fmt.Errorf("failed to delete chat: %w", err)
	}

	return nil
}

// SaveUserToken сохраняет или обновляет FCM токен пользователя
func (db *DB) SaveUserToken(username, token string) error {
	query := `INSERT INTO user_tokens (username, user_id, fcm_token, updated_at)
	          VALUES ($1::text, (SELECT id FROM users WHERE username = $1::text), $2, $3)
              ON CONFLICT (username) DO UPDATE SET fcm_token = $2, updated_at = $3`
	_, err := db.Exec(query, username, token, time.Now())
	return err
}

// GetUserToken получает токен пользователя для отправки пуша
func (db *DB) GetUserToken(username string) (string, error) {
	var token string
	query := `SELECT fcm_token FROM user_tokens WHERE username = $1`
	err := db.QueryRow(query, username).Scan(&token)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return token, err
}

// SaveUser сохраняет или обновляет хеш пароля пользователя
func (db *DB) SaveUser(username, passwordHash string) error {
	query := `INSERT INTO users (username, password_hash, created_at)
			  VALUES ($1, $2, NOW())
			  ON CONFLICT (username) DO UPDATE SET password_hash = $2`
	_, err := db.Exec(query, username, passwordHash)
	return err
}

// GetUserPasswordHash получает хеш пароля пользователя
func (db *DB) GetUserPasswordHash(username string) (string, error) {
	var passwordHash string
	query := `SELECT password_hash FROM users WHERE username = $1`
	err := db.QueryRow(query, username).Scan(&passwordHash)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("user not found")
	}
	return passwordHash, err
}

// UserExists проверяет, существует ли пользователь
func (db *DB) UserExists(username string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM users WHERE username = $1)`
	err := db.QueryRow(query, username).Scan(&exists)
	return exists, err
}

// GetAllUsers возвращает список всех зарегистрированных пользователей
func (db *DB) GetAllUsers() ([]string, error) {
	query := `SELECT username FROM users ORDER BY username`
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("Warning: error closing rows: %v", err)
		}
	}()

	var users []string
	for rows.Next() {
		var username string
		if err := rows.Scan(&username); err != nil {
			return nil, err
		}
		users = append(users, username)
	}
	return users, nil
}

// UpdateUsername обновляет имя пользователя
func (db *DB) UpdateUsername(oldUsername, newUsername string) error {
	// Проверяем, что новое имя не занято
	exists, err := db.UserExists(newUsername)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("username already taken")
	}

	// Начинаем транзакцию для атомарного обновления во всех таблицах
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Обновляем основную таблицу пользователей
	_, err = tx.Exec(`UPDATE users SET username = $1 WHERE username = $2`, newUsername, oldUsername)
	if err != nil {
		return err
	}

	// 2. Обновляем сообщения
	_, err = tx.Exec(`UPDATE messages SET username = $1 WHERE username = $2`, newUsername, oldUsername)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`UPDATE messages SET replied_to_user = $1 WHERE replied_to_user = $2`, newUsername, oldUsername)
	if err != nil {
		return err
	}

	// 3. Обновляем контакты (обе стороны)
	_, err = tx.Exec(`UPDATE contacts SET username = $1 WHERE username = $2`, newUsername, oldUsername)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`UPDATE contacts SET contact_username = $1 WHERE contact_username = $2`, newUsername, oldUsername)
	if err != nil {
		return err
	}

	// 4. Обновляем токены уведомлений
	_, err = tx.Exec(`UPDATE user_tokens SET username = $1 WHERE username = $2`, newUsername, oldUsername)
	if err != nil {
		return err
	}

	// 5. Обновляем метаданные чатов (last_read_at и т.д.)
	_, err = tx.Exec(`UPDATE user_chat_metadata SET username = $1 WHERE username = $2`, newUsername, oldUsername)
	if err != nil {
		return err
	}

	// 6. Обновляем реакции
	_, err = tx.Exec(`UPDATE reactions SET username = $1 WHERE username = $2`, newUsername, oldUsername)
	if err != nil {
		return err
	}

	// 7. Обновляем участников в чатах (самое сложное из-за JSON)
	// Используем безопасную замену в JSON массиве
	oldJson := fmt.Sprintf("\"%s\"", oldUsername)
	newJson := fmt.Sprintf("\"%s\"", newUsername)

	query := `UPDATE chats
              SET participants = (
                  SELECT json_agg(CASE WHEN elem::text = $1 THEN $2::json ELSE elem END)
                  FROM json_array_elements(participants::json) AS elem
              )::text
              WHERE participants::json @> $1::json`

	_, err = tx.Exec(query, oldJson, newJson)
	if err != nil {
		log.Printf("Warning: failed to update username in chats via JSON: %v. Falling back to REPLACE.", err)
		// Фолбек на простой REPLACE если JSON функции не сработали (хотя в Postgres они должны быть)
		_, err = tx.Exec(`UPDATE chats SET participants = REPLACE(participants, $1, $2) WHERE participants LIKE $3`,
			oldJson, newJson, "%"+oldJson+"%")
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// UpdatePassword обновляет пароль пользователя
func (db *DB) UpdatePassword(username, newPassword string) error {
	passwordHash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}

	query := `UPDATE users SET password_hash = $1 WHERE username = $2`
	result, err := db.Exec(query, passwordHash, username)
	if err != nil {
		return err
	}

	// Проверяем, что пользователь существовал
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}

// UpdateAvatar обновляет URL аватара пользователя
func (db *DB) UpdateAvatar(username, avatarURL string) error {
	query := `UPDATE users SET avatar_url = $1 WHERE username = $2`
	result, err := db.Exec(query, avatarURL, username)
	if err != nil {
		return err
	}

	// Проверяем, что пользователь существовал
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}

// GetUserAvatar получает URL аватара пользователя
func (db *DB) GetUserAvatar(username string) (string, error) {
	var avatarURL sql.NullString
	query := `SELECT avatar_url FROM users WHERE username = $1`
	err := db.QueryRow(query, username).Scan(&avatarURL)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if avatarURL.Valid {
		return avatarURL.String, nil
	}
	return "", nil
}

// CreateChat creates a new chat room
func (db *DB) CreateChat(id, name, chatType, participants string) error {
	query := `INSERT INTO chats (id, name, type, participants, created_at) VALUES ($1, $2, $3, $4, NOW())`
	_, err := db.Exec(query, id, name, chatType, participants)
	return err
}

// GetChat retrieves a chat by ID
func (db *DB) GetChat(id string) (struct {
	ID           string
	Name         string
	Type         string
	Participants string
	CreatedAt    time.Time
}, error) {
	var chat struct {
		ID           string
		Name         string
		Type         string
		Participants string
		CreatedAt    time.Time
	}
	query := `SELECT id, name, type, participants, created_at FROM chats WHERE id = $1`
	err := db.QueryRow(query, id).Scan(&chat.ID, &chat.Name, &chat.Type, &chat.Participants, &chat.CreatedAt)
	return chat, err
}

// GetUserChats retrieves all chats for a specific user with unread count and last message time
func (db *DB) GetUserChats(username string) ([]struct {
	ID              string
	Name            string
	Type            string
	Participants    string
	CreatedAt       time.Time
	UnreadCount     int
	LastMessageTime time.Time
}, error) {
	query := `
		SELECT c.id, c.name, c.type, c.participants, c.created_at,
		(SELECT COUNT(*) FROM messages m
		 WHERE m.room_id = c.id
		 AND m.is_read = FALSE
		 AND m.username != $1) as unread_count,
		(SELECT MAX(m.created_at) FROM messages m WHERE m.room_id = c.id) as last_message_time
		FROM chats c
		WHERE c.participants LIKE $2 OR (c.type = 'group' AND c.participants LIKE $2)`

	rows, err := db.Query(query, username, "%"+username+"%")
	if err != nil {
		log.Printf("Error in GetUserChats query: %v", err)
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("Warning: error closing rows: %v", err)
		}
	}()

	var results []struct {
		ID              string
		Name            string
		Type            string
		Participants    string
		CreatedAt       time.Time
		UnreadCount     int
		LastMessageTime time.Time
	}

	for rows.Next() {
		var c struct {
			ID              string
			Name            string
			Type            string
			Participants    string
			CreatedAt       time.Time
			UnreadCount     int
			LastMessageTime sql.NullTime
		}
		if err := rows.Scan(&c.ID, &c.Name, &c.Type, &c.Participants, &c.CreatedAt, &c.UnreadCount, &c.LastMessageTime); err != nil {
			log.Printf("Error scanning chat row: %v", err)
			return nil, err
		}
		lastMessageTime := time.Time{}
		if c.LastMessageTime.Valid {
			lastMessageTime = c.LastMessageTime.Time
		}
		results = append(results, struct {
			ID              string
			Name            string
			Type            string
			Participants    string
			CreatedAt       time.Time
			UnreadCount     int
			LastMessageTime time.Time
		}{
			ID:              c.ID,
			Name:            c.Name,
			Type:            c.Type,
			Participants:    c.Participants,
			CreatedAt:       c.CreatedAt,
			UnreadCount:     c.UnreadCount,
			LastMessageTime: lastMessageTime,
		})
	}
	return results, nil
}

// MarkRead updates the last read time for a user in a room and marks messages as read
func (db *DB) MarkRead(roomID, username string) error {
	// 1. Update user_chat_metadata
	query1 := `INSERT INTO user_chat_metadata (username, user_id, room_id, last_read_at)
	          VALUES ($1::text, (SELECT id FROM users WHERE username = $1::text), $2, NOW())
              ON CONFLICT (username, room_id) DO UPDATE SET last_read_at = NOW()`
	_, err := db.Exec(query1, username, roomID)
	if err != nil {
		return err
	}

	// 2. Update messages table for the "double check"
	query2 := `UPDATE messages SET is_read = TRUE WHERE room_id = $1 AND username != $2 AND is_read = FALSE`
	_, err = db.Exec(query2, roomID, username)
	return err
}

// GetDirectChatBetweenUsers finds or creates a direct chat between two users
func (db *DB) GetDirectChatBetweenUsers(user1, user2 string) (string, error) {
	// Try to find existing direct chat
	query := `SELECT id FROM chats WHERE type = 'direct' AND participants LIKE $1 AND participants LIKE $2`
	var chatID string
	err := db.QueryRow(query, "%"+user1+"%", "%"+user2+"%").Scan(&chatID)
	if err == nil {
		return chatID, nil
	}
	if err != sql.ErrNoRows {
		return "", err
	}

	// Create new direct chat
	newChatID := user1 + "_" + user2 + "_direct"
	participants := `["` + user1 + `", "` + user2 + `"]`
	err = db.CreateChat(newChatID, user1+" & "+user2, "direct", participants)
	if err != nil {
		return "", err
	}
	return newChatID, nil
}

// DeleteProfile removes a user and all their data
func (db *DB) DeleteProfile(username string) error {
	// Start a transaction
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Get all message IDs for this user to delete reactions (optional if using cascade)
	// 2. Get all image URLs for messages by this user
	rows, err := tx.Query(`SELECT image_url FROM messages WHERE username = $1 AND image_url != ''`, username)
	if err == nil {
		var urls []string
		for rows.Next() {
			var url string
			if err := rows.Scan(&url); err == nil {
				urls = append(urls, url)
			}
		}
		rows.Close()
		// Delete image files (outside transaction is fine as files are not transactional)
		for _, url := range urls {
			_ = DeleteImageFile(url)
		}
	}

	// 3. Get user's avatar URL and delete it
	var avatarURL sql.NullString
	err = tx.QueryRow(`SELECT avatar_url FROM users WHERE username = $1`, username).Scan(&avatarURL)
	if err == nil && avatarURL.Valid && avatarURL.String != "" {
		_ = DeleteImageFile(avatarURL.String)
	}

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

	// 8. Delete user's contacts (both as owner and as contact)
	_, err = tx.Exec(`DELETE FROM contacts WHERE username = $1 OR contact_username = $1`, username)
	if err != nil {
		return fmt.Errorf("failed to delete user contacts: %w", err)
	}

	// 9. Delete the user itself
	_, err = tx.Exec(`DELETE FROM users WHERE username = $1`, username)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	// 9. Update chats where user was a participant
	// This is a bit complex since participants is a JSON string.
	// For simplicity, we'll mark them as "deleted_user" or just leave as is
	// but the app should handle missing users.
	_, err = tx.Exec(`UPDATE chats SET participants = REPLACE(participants, $1, '"deleted_user"') WHERE participants LIKE $2`, "\""+username+"\"", "%\""+username+"\"%")
	if err != nil {
		log.Printf("Warning: failed to update participants in chats: %v", err)
	}

	return tx.Commit()
}

// CleanupEmptyMessages removes all old messages (encrypted with old key)
func (db *DB) CleanupEmptyMessages() (int64, error) {
	query := `DELETE FROM messages`
	result, err := db.Exec(query)
	if err != nil {
		return 0, err
	}
	rowsAffected, _ := result.RowsAffected()
	return rowsAffected, nil
}

// GetUserProfile retrieves user profile information
func (db *DB) GetUserProfile(username string) (struct {
	Username  string
	Bio       string
	Status    string
	AvatarURL string
}, error) {
	var profile struct {
		Username  string
		Bio       sql.NullString
		Status    sql.NullString
		AvatarURL sql.NullString
	}

	query := `SELECT username, bio, status, avatar_url FROM users WHERE username = $1`
	err := db.QueryRow(query, username).Scan(&profile.Username, &profile.Bio, &profile.Status, &profile.AvatarURL)
	if err != nil {
		return struct {
			Username  string
			Bio       string
			Status    string
			AvatarURL string
		}{}, err
	}

	bio := ""
	if profile.Bio.Valid {
		bio = profile.Bio.String
	}

	status := ""
	if profile.Status.Valid {
		status = profile.Status.String
	}

	avatar := ""
	if profile.AvatarURL.Valid {
		avatar = profile.AvatarURL.String
	}

	return struct {
		Username  string
		Bio       string
		Status    string
		AvatarURL string
	}{
		Username:  profile.Username,
		Bio:       bio,
		Status:    status,
		AvatarURL: avatar,
	}, nil
}

// UpdateProfile updates user profile information
func (db *DB) UpdateProfile(username, bio, status string) error {
	query := `UPDATE users SET bio = $1, status = $2 WHERE username = $3`
	result, err := db.Exec(query, bio, status, username)
	if err != nil {
		return err
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

// UpdateChatParticipants updates the participants of a chat
func (db *DB) UpdateChatParticipants(chatID, participants string) error {
	query := `UPDATE chats SET participants = $1 WHERE id = $2`
	_, err := db.Exec(query, participants, chatID)
	return err
}

// AddContact adds a contact for a user
func (db *DB) AddContact(username, contactUsername string) error {
	query := `INSERT INTO contacts (user_id, contact_id, username, contact_username)
	          SELECT u1.id, u2.id, u1.username, u2.username
	          FROM users u1, users u2
	          WHERE u1.username = $1 AND u2.username = $2
	          ON CONFLICT (user_id, contact_id) DO NOTHING`
	_, err := db.Exec(query, username, contactUsername)
	return err
}

// RemoveContact removes a contact for a user
func (db *DB) RemoveContact(username, contactUsername string) error {
	_, err := db.Exec(`DELETE FROM contacts WHERE username = $1 AND contact_username = $2`, username, contactUsername)
	return err
}

// GetContacts retrieves all contacts for a user
func (db *DB) GetContacts(username string) ([]string, error) {
	// We join with users to get the CURRENT username of the contact,
	// in case they changed it but we are still linked by contact_id
	query := `SELECT u_contact.username
	          FROM contacts c
	          JOIN users u_me ON c.user_id = u_me.id
	          JOIN users u_contact ON c.contact_id = u_contact.id
	          WHERE u_me.username = $1`

	rows, err := db.Query(query, username)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var contacts []string
	for rows.Next() {
		var contact string
		if err := rows.Scan(&contact); err != nil {
			return nil, err
		}
		contacts = append(contacts, contact)
	}

	// Fallback for old records without user_id links
	if len(contacts) == 0 {
		rows, err := db.Query(`SELECT contact_username FROM contacts WHERE username = $1`, username)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var contact string
				if err := rows.Scan(&contact); err == nil {
					contacts = append(contacts, contact)
				}
			}
		}
	}

	return contacts, nil
}

// UpdateMessageText updates the text of a message by UUID and marks it as edited
func (db *DB) UpdateMessageText(messageID, newText string) error {
	encrypted, err := encrypt(newText)
	if err != nil {
		return err
	}

	query := `UPDATE messages SET encrypted_text = $1, edited = true WHERE message_id = $2`
	_, err = db.Exec(query, encrypted, messageID)
	return err
}

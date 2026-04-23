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
		    ALTER TABLE messages ADD COLUMN room_id VARCHAR(255) DEFAULT 'general';
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
			username VARCHAR(255) PRIMARY KEY,
			password_hash VARCHAR(255) NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW()
		);`,
		// Migration: Add avatar_url to users
		`DO $$
		 BEGIN
		  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='users' AND column_name='avatar_url') THEN
		    ALTER TABLE users ADD COLUMN avatar_url VARCHAR(512);
		  END IF;
		 END $$;`,
		`CREATE TABLE IF NOT EXISTS chats (
			id VARCHAR(255) PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			type VARCHAR(50) NOT NULL, -- 'general' or 'direct'
			participants TEXT NOT NULL, -- JSON array of usernames
			created_at TIMESTAMP NOT NULL DEFAULT NOW()
		);`,
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
	query := `INSERT INTO messages (message_id, username, encrypted_text, created_at, replied_to_message_id, replied_to_user, replied_to_text, room_id, is_read, image_url) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, FALSE, $9)`
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
}, error) {
	query := `SELECT COALESCE(m.message_id, ''), m.username, m.encrypted_text, m.created_at, COALESCE(m.replied_to_message_id, ''), COALESCE(m.replied_to_user, ''), COALESCE(m.replied_to_text, ''), COALESCE(m.room_id, 'general'), m.is_read, u.avatar_url, COALESCE(m.image_url, '')
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
		}
		if err := rows.Scan(&r.MessageID, &r.Username, &r.Encrypted, &r.CreatedAt, &r.RepliedToMessageID, &r.RepliedToUser, &r.RepliedToText, &r.RoomID, &r.IsRead, &r.AvatarURL, &r.ImageURL); err != nil {
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
		}{
			MessageID:          r.MessageID,
			Username:           r.Username,
			Encrypted:          r.Encrypted,
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
	query := `INSERT INTO reactions (message_id, username, emoji)
              VALUES ($1, $2, $3)
              ON CONFLICT (message_id, username) DO UPDATE SET emoji = $3`
	_, err := db.Exec(query, messageID, username, emoji)
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
	query := `INSERT INTO user_tokens (username, fcm_token, updated_at)
              VALUES ($1, $2, $3)
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

	// Обновляем имя пользователя
	query := `UPDATE users SET username = $1 WHERE username = $2`
	result, err := db.Exec(query, newUsername, oldUsername)
	if err != nil {
		return err
	}

	// Проверяем, что пользователь существовал
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("user not found")
	}

	// Обновляем имя пользователя в таблице чатов
	query = `UPDATE chats SET participants = REPLACE(participants, $1, $2) WHERE participants LIKE $1`
	_, err = db.Exec(query, oldUsername, newUsername)
	if err != nil {
		log.Printf("Warning: failed to update username in chats: %v", err)
	}

	return nil
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

// GetUserChats retrieves all chats for a specific user with unread count
func (db *DB) GetUserChats(username string) ([]struct {
	ID           string
	Name         string
	Type         string
	Participants string
	CreatedAt    time.Time
	UnreadCount  int
}, error) {
	query := `
		SELECT c.id, c.name, c.type, c.participants, c.created_at,
		(SELECT COUNT(*) FROM messages m
		 WHERE m.room_id = c.id
		 AND m.is_read = FALSE
		 AND m.username != $1) as unread_count
		FROM chats c
		WHERE c.participants LIKE $2 OR c.type = 'general' OR (c.type = 'group' AND c.participants LIKE $2)`

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
		ID           string
		Name         string
		Type         string
		Participants string
		CreatedAt    time.Time
		UnreadCount  int
	}

	for rows.Next() {
		var c struct {
			ID           string
			Name         string
			Type         string
			Participants string
			CreatedAt    time.Time
			UnreadCount  int
		}
		if err := rows.Scan(&c.ID, &c.Name, &c.Type, &c.Participants, &c.CreatedAt, &c.UnreadCount); err != nil {
			log.Printf("Error scanning chat row: %v", err)
			return nil, err
		}
		results = append(results, c)
	}
	return results, nil
}

// MarkRead updates the last read time for a user in a room and marks messages as read
func (db *DB) MarkRead(roomID, username string) error {
	// 1. Update user_chat_metadata
	query1 := `INSERT INTO user_chat_metadata (username, room_id, last_read_at)
              VALUES ($1, $2, NOW())
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

	// 8. Delete the user itself
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

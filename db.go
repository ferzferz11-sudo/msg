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
		`CREATE TABLE IF NOT EXISTS reactions (
			id SERIAL PRIMARY KEY,
			message_id VARCHAR(255) NOT NULL REFERENCES messages(message_id) ON DELETE CASCADE,
			username VARCHAR(255) NOT NULL,
			emoji VARCHAR(50) NOT NULL,
			UNIQUE(message_id, username)
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
func (db *DB) SaveMessage(messageID string, username string, encryptedText []byte, createdAt time.Time) error {
	query := `INSERT INTO messages (message_id, username, encrypted_text, created_at) VALUES ($1, $2, $3, $4)`
	_, err := db.Exec(query, messageID, username, encryptedText, createdAt)
	return err
}

// GetMessages retrieves recent messages from the database
func (db *DB) GetMessages(limit int) ([]struct {
	MessageID string
	Username  string
	Encrypted []byte
	CreatedAt time.Time
}, error) {
	query := `SELECT message_id, username, encrypted_text, created_at FROM messages ORDER BY created_at DESC LIMIT $1`
	rows, err := db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []struct {
		MessageID string
		Username  string
		Encrypted []byte
		CreatedAt time.Time
	}

	for rows.Next() {
		var r struct {
			MessageID string
			Username  string
			Encrypted []byte
			CreatedAt time.Time
		}
		if err := rows.Scan(&r.MessageID, &r.Username, &r.Encrypted, &r.CreatedAt); err != nil {
			return nil, err
		}
		results = append(results, r)
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
	defer rows.Close()

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
}, error) {
	query := `SELECT id, encrypted_text FROM messages WHERE username = $1 AND ABS(EXTRACT(EPOCH FROM (created_at - $2))) < 2`
	rows, err := db.Query(query, username, createdAt)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []struct {
		ID        int
		Encrypted []byte
	}

	for rows.Next() {
		var r struct {
			ID        int
			Encrypted []byte
		}
		if err := rows.Scan(&r.ID, &r.Encrypted); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, nil
}

// DeleteMessageByUUID deletes a message by its unique message_id
func (db *DB) DeleteMessageByUUID(messageID string) error {
	query := `DELETE FROM messages WHERE message_id = $1`
	_, err := db.Exec(query, messageID)
	return err
}

// DeleteMessageByID deletes a message by its serial internal ID
func (db *DB) DeleteMessageByID(id int) error {
	query := `DELETE FROM messages WHERE id = $1`
	_, err := db.Exec(query, id)
	return err
}

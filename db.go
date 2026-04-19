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
	// sql.Open doesn't actually establish a connection, just prepares it
	db, err := sql.Open("postgres", dbUrl)
	if err != nil {
		return nil, fmt.Errorf("unable to open database connection: %w", err)
	}

	// Test the connection to ensure it's valid and reachable
	err = db.Ping()
	if err != nil {
		// Clean up the connection if ping fails
		if db != nil {
			// Safely close the connection, ignoring any errors during cleanup
			if closeErr := db.Close(); closeErr != nil {
				log.Printf("Warning: error closing database connection: %v", closeErr)
			}
		}
		return nil, fmt.Errorf("unable to ping database: %w\nDATABASE_URL: %s", err, maskPassword(dbUrl))
	}

	// Create the messages table if it doesn't already exist
	// This table stores encrypted messages with metadata
	createTableQuery := `
	CREATE TABLE IF NOT EXISTS messages (
		id SERIAL PRIMARY KEY,                    -- Auto-incrementing unique identifier
		username VARCHAR(255) NOT NULL,           -- Sender's username
		encrypted_text BYTEA NOT NULL,            -- Encrypted message content (binary data)
		created_at TIMESTAMP NOT NULL             -- Message timestamp
	);`

	// Execute the table creation query
	_, err = db.Exec(createTableQuery)
	if err != nil {
		// Clean up connection if table creation fails
		if db != nil {
			// Safely close the connection, ignoring any errors during cleanup
			if closeErr := db.Close(); closeErr != nil {
				log.Printf("Warning: error closing database connection: %v", closeErr)
			}
		}
		return nil, fmt.Errorf("failed to create messages table: %w\nQuery: %s", err, createTableQuery)
	}

	log.Println("Database connected, messages table ready")

	// Return wrapped database connection for use throughout the application
	return &DB{db}, nil
}

// maskPassword obscures sensitive password information in database URLs
// This is used for logging purposes to prevent credentials from being exposed
func maskPassword(dbUrl string) string {
	if len(dbUrl) > 20 {
		// Simple masking - keep first 20 chars and last 10 chars, mask the middle
		return dbUrl[:20] + "***" + dbUrl[len(dbUrl)-10:]
	}
	// For very short URLs, mask everything
	return "***"
}

// Close terminates the database connection
// This method delegates to the embedded sql.DB's Close method
func (db *DB) Close() error {
	if db == nil || db.DB == nil {
		return nil
	}
	return db.DB.Close()
}

// SaveMessage stores an encrypted message in the database
// Parameters:
// - username: the sender's username
// - encryptedText: the encrypted message content as byte array
// - createdAt: timestamp when the message was created
// Returns any error that occurs during the database operation
func (db *DB) SaveMessage(username string, encryptedText []byte, createdAt time.Time) error {
	// SQL query to insert a new message into the messages table
	// Using parameterized queries to prevent SQL injection
	query := `INSERT INTO messages (username, encrypted_text, created_at) VALUES ($1, $2, $3)`
	_, err := db.Exec(query, username, encryptedText, createdAt)
	return err
}

// GetMessages retrieves recent messages from the database
func (db *DB) GetMessages(limit int) ([]struct {
	Username  string
	Encrypted []byte
	CreatedAt time.Time
}, error) {
	query := `SELECT username, encrypted_text, created_at FROM messages ORDER BY created_at DESC LIMIT $1`
	rows, err := db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("Warning: failed to close rows: %v", err)
		}
	}()

	var results []struct {
		Username  string
		Encrypted []byte
		CreatedAt time.Time
	}

	for rows.Next() {
		var r struct {
			Username  string
			Encrypted []byte
			CreatedAt time.Time
		}
		if err := rows.Scan(&r.Username, &r.Encrypted, &r.CreatedAt); err != nil {
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
	// Используем небольшой интервал для времени (2 секунды), так как при сохранении/загрузке
	// точность может незначительно отличаться.
	query := `SELECT id, encrypted_text FROM messages WHERE username = $1 AND ABS(EXTRACT(EPOCH FROM (created_at - $2))) < 2`
	rows, err := db.Query(query, username, createdAt)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("Warning: failed to close rows: %v", err)
		}
	}()

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

// DeleteMessageByID удаляет сообщение по его уникальному идентификатору
func (db *DB) DeleteMessageByID(id int) error {
	query := `DELETE FROM messages WHERE id = $1`
	_, err := db.Exec(query, id)
	return err
}

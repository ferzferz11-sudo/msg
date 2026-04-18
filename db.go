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

	log.Printf("Attempting to connect to database...")

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

	log.Println("Database connection successful")

	// Create the messages table if it doesn't already exist
	// This table stores encrypted messages with metadata
	createTableQuery := `
	CREATE TABLE IF NOT EXISTS messages (
		id SERIAL PRIMARY KEY,                    -- Auto-incrementing unique identifier
		username VARCHAR(255) NOT NULL,           -- Sender's username
		encrypted_text BYTEA NOT NULL,            -- Encrypted message content (binary data)
		created_at TIMESTAMP NOT NULL             -- Message timestamp
	);`

	log.Printf("Creating messages table...")
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

	log.Println("Messages table created/verified successfully")

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

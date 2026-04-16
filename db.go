package main

import (
	"database/sql"
	"log"
	"os"
	"time"

	_ "github.com/lib/pq"
)

type DB struct {
	*sql.DB
}

func ConnectDB() *DB {
	dbUrl := os.Getenv("DATABASE_URL")
	if dbUrl == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	db, err := sql.Open("postgres", dbUrl)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v", err)
	}

	err = db.Ping()
	if err != nil {
		log.Fatalf("Unable to ping database: %v", err)
	}

	log.Println("Connected to PostgreSQL")

	// Создаем таблицу для сообщений, если ее нет
	createTableQuery := `
	CREATE TABLE IF NOT EXISTS messages (
		id SERIAL PRIMARY KEY,
		username VARCHAR(255) NOT NULL,
		encrypted_text BYTEA NOT NULL,
		created_at TIMESTAMP NOT NULL
	);`

	_, err = db.Exec(createTableQuery)
	if err != nil {
		log.Fatalf("Failed to create table: %v", err)
	}

	return &DB{db}
}

func (db *DB) SaveMessage(username string, encryptedText []byte, createdAt time.Time) error {
	query := `INSERT INTO messages (username, encrypted_text, created_at) VALUES ($1, $2, $3)`
	_, err := db.Exec(query, username, encryptedText, createdAt)
	return err
}

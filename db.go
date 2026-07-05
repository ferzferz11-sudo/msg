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

	// ======= CORE MIGRATIONS =======
	if err := runCoreMigrations(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migration failed: %w", err)
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

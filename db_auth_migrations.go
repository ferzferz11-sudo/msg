package main

import (
	"fmt"
)

// MigrateDeviceTables adds missing columns to user_devices if they don't exist
// and creates device_auth_log if missing.
func (db *DB) MigrateDeviceTables() error {
	// Step 1: Drop old PK if exists — must be done before adding new PK
	// Use IF EXISTS to avoid error when constraint doesn't exist
	db.Exec(`ALTER TABLE user_devices DROP CONSTRAINT IF EXISTS user_devices_pkey`)

	// Step 2: Add id column if not exists (without PK constraint first)
	db.Exec(`ALTER TABLE user_devices ADD COLUMN IF NOT EXISTS id UUID DEFAULT gen_random_uuid()`)

	// Step 3: Add all missing columns (safe to run multiple times)
	db.Exec(`ALTER TABLE user_devices ADD COLUMN IF NOT EXISTS device_type VARCHAR(50) DEFAULT 'unknown'`)
	db.Exec(`ALTER TABLE user_devices ADD COLUMN IF NOT EXISTS refresh_token_jti VARCHAR(255) DEFAULT ''`)
	db.Exec(`ALTER TABLE user_devices ADD COLUMN IF NOT EXISTS refresh_token_expires_at TIMESTAMP`)
	db.Exec(`ALTER TABLE user_devices ADD COLUMN IF NOT EXISTS is_active BOOLEAN DEFAULT TRUE`)
	db.Exec(`ALTER TABLE user_devices ADD COLUMN IF NOT EXISTS created_at TIMESTAMP NOT NULL DEFAULT NOW()`)
	db.Exec(`ALTER TABLE user_devices ADD COLUMN IF NOT EXISTS user_agent TEXT DEFAULT ''`)

	// Step 4: Add PK on id if no PK exists
	var hasPK bool
	db.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.table_constraints
			WHERE table_name = 'user_devices' AND constraint_type = 'PRIMARY KEY'
		)
	`).Scan(&hasPK)

	if !hasPK {
		db.Exec(`UPDATE user_devices SET id = gen_random_uuid() WHERE id IS NULL`)
		db.Exec(`ALTER TABLE user_devices ADD PRIMARY KEY (id)`)
	}

	// Step 5: Ensure unique constraint on (user_id, device_id)
	db.Exec(`ALTER TABLE user_devices DROP CONSTRAINT IF EXISTS user_devices_user_id_device_id_key`)
	db.Exec(`ALTER TABLE user_devices ADD CONSTRAINT user_devices_user_id_device_id_key UNIQUE (user_id, device_id)`)

	// Step 6: Indexes
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_user_devices_user ON user_devices(user_id)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_user_devices_jti ON user_devices(refresh_token_jti)`)

	// Step 7: Create device_auth_log if not exists
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS device_auth_log (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			device_id VARCHAR(255) DEFAULT '',
			action VARCHAR(50) NOT NULL,
			ip_address VARCHAR(45) DEFAULT '',
			client_version VARCHAR(50) DEFAULT '',
			success BOOLEAN NOT NULL,
			error_message TEXT DEFAULT '',
			created_at TIMESTAMP NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		logger.Warnf("Migration: could not create device_auth_log: %v", err)
	}
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_device_auth_log_user ON device_auth_log(user_id)`)

	// Verify
	var colCount int
	db.QueryRow(`SELECT COUNT(*) FROM information_schema.columns WHERE table_name = 'user_devices'`).Scan(&colCount)
	logger.Infof("Migration: user_devices table has %d columns", colCount)
	fmt.Println("Migration complete")

	return nil
}

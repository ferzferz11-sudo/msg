package main

// MigrateDeviceTables adds missing columns to user_devices if they don't exist
// and creates device_auth_log if missing
func (db *DB) MigrateDeviceTables() error {
	// Check and add missing columns to user_devices
	alterStatements := []string{
		`ALTER TABLE user_devices ADD COLUMN IF NOT EXISTS id UUID PRIMARY KEY DEFAULT gen_random_uuid()`,
		`ALTER TABLE user_devices ADD COLUMN IF NOT EXISTS device_type VARCHAR(50) DEFAULT 'unknown'`,
		`ALTER TABLE user_devices ADD COLUMN IF NOT EXISTS refresh_token_jti VARCHAR(255) DEFAULT ''`,
		`ALTER TABLE user_devices ADD COLUMN IF NOT EXISTS refresh_token_expires_at TIMESTAMP`,
		`ALTER TABLE user_devices ADD COLUMN IF NOT EXISTS is_active BOOLEAN DEFAULT TRUE`,
		`ALTER TABLE user_devices ADD COLUMN IF NOT EXISTS created_at TIMESTAMP NOT NULL DEFAULT NOW()`,
		`ALTER TABLE user_devices ADD COLUMN IF NOT EXISTS user_agent TEXT DEFAULT ''`,
	}

	for _, stmt := range alterStatements {
		if _, err := db.Exec(stmt); err != nil {
			// Column might already exist with different definition — log and continue
			logger.Warnf("Migration warning: %v", err)
		}
	}

	// If username column exists (old schema), we need to fix the primary key
	// The old schema had device_id as PK, we need (user_id, device_id) as unique
	var hasUsername bool
	err := db.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns 
			WHERE table_name = 'user_devices' AND column_name = 'username'
		)
	`).Scan(&hasUsername)
	if err != nil {
		logger.Warnf("Migration: could not check username column: %v", err)
	}

	if hasUsername {
		// Remove old PK if exists, add new unique constraint
		db.Exec(`ALTER TABLE user_devices DROP CONSTRAINT IF EXISTS user_devices_pkey`)
		db.Exec(`ALTER TABLE user_devices DROP COLUMN IF EXISTS username`)
		db.Exec(`ALTER TABLE user_devices DROP COLUMN IF EXISTS id`)
	}

	// Create id if not exists
	db.Exec(`ALTER TABLE user_devices ADD COLUMN IF NOT EXISTS id UUID DEFAULT gen_random_uuid()`)

	// Fix primary key — use id
	var hasIDPK bool
	db.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.table_constraints 
			WHERE table_name = 'user_devices' AND constraint_type = 'PRIMARY KEY'
			AND constraint_name = 'user_devices_pkey'
		)
	`).Scan(&hasIDPK)

	if !hasIDPK {
		db.Exec(`ALTER TABLE user_devices ADD PRIMARY KEY (id)`)
	}

	// Ensure unique constraint on (user_id, device_id)
	db.Exec(`ALTER TABLE user_devices DROP CONSTRAINT IF EXISTS user_devices_user_id_device_id_key`)
	db.Exec(`ALTER TABLE user_devices ADD CONSTRAINT user_devices_user_id_device_id_key UNIQUE (user_id, device_id)`)

	// Recreate indexes
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_user_devices_user ON user_devices(user_id)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_user_devices_jti ON user_devices(refresh_token_jti)`)

	// Create device_auth_log if not exists
	_, err = db.Exec(`
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

	return nil
}

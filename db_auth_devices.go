package main

import (
	"time"
)

// UserDevice represents a registered device for a user
type UserDevice struct {
	ID                   string    `json:"id"`
	UserID               string    `json:"user_id"`
	DeviceID             string    `json:"device_id"`
	DeviceName           string    `json:"device_name"`
	DeviceType           string    `json:"device_type"`
	ClientVersion        string    `json:"client_version"`
	IPAddress            string    `json:"ip_address"`
	UserAgent            string    `json:"user_agent"`
	RefreshTokenJTI      string    `json:"refresh_token_jti"`
	RefreshTokenExpiresAt *time.Time `json:"refresh_token_expires_at"`
	IsActive             bool      `json:"is_active"`
	CreatedAt            time.Time `json:"created_at"`
	LastSeenAt           time.Time `json:"last_seen_at"`
}

// DeviceAuthLog represents an audit entry for device authentication events
type DeviceAuthLog struct {
	ID            string    `json:"id"`
	UserID        string    `json:"user_id"`
	DeviceID      string    `json:"device_id"`
	Action        string    `json:"action"`
	IPAddress     string    `json:"ip_address"`
	ClientVersion string    `json:"client_version"`
	Success       bool      `json:"success"`
	ErrorMessage  string    `json:"error_message"`
	CreatedAt     time.Time `json:"created_at"`
}

// CreateDeviceTables creates the new auth tables if they don't exist
func (db *DB) CreateDeviceTables() error {
	query := `
	CREATE TABLE IF NOT EXISTS user_devices (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		device_id VARCHAR(255) NOT NULL,
		device_name VARCHAR(255) DEFAULT '',
		device_type VARCHAR(50) DEFAULT 'unknown',
		client_version VARCHAR(50) DEFAULT '',
		ip_address VARCHAR(45) DEFAULT '',
		user_agent TEXT DEFAULT '',
		refresh_token_jti VARCHAR(255) DEFAULT '',
		refresh_token_expires_at TIMESTAMP,
		is_active BOOLEAN DEFAULT TRUE,
		created_at TIMESTAMP NOT NULL DEFAULT NOW(),
		last_seen_at TIMESTAMP NOT NULL DEFAULT NOW(),
		UNIQUE(user_id, device_id)
	);

	CREATE INDEX IF NOT EXISTS idx_user_devices_user ON user_devices(user_id);
	CREATE INDEX IF NOT EXISTS idx_user_devices_jti ON user_devices(refresh_token_jti);

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
	);

	CREATE INDEX IF NOT EXISTS idx_device_auth_log_user ON device_auth_log(user_id);
	`

	_, err := db.Exec(query)
	if err != nil {
		return err
	}

	// Migration: add UNIQUE constraint if missing (for tables created before this constraint was added)
	_, err = db.Exec(`
		DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM pg_constraint 
				WHERE conname = 'user_devices_user_id_device_id_key'
				AND conrelid = 'user_devices'::regclass
			) THEN
				ALTER TABLE user_devices 
				ADD CONSTRAINT user_devices_user_id_device_id_key 
				UNIQUE (user_id, device_id);
			END IF;
		EXCEPTION WHEN duplicate_table THEN
			-- constraint already exists, ignore
		END $$;
	`)
	return err
}

// UpsertDevice inserts or updates a device on sign-in
func (db *DB) UpsertDevice(userID, deviceID, deviceName, deviceType, clientVersion, ipAddress, userAgent string) (*UserDevice, error) {
	var device UserDevice
	err := db.QueryRow(`
		INSERT INTO user_devices (user_id, device_id, device_name, device_type, client_version, ip_address, user_agent, is_active, last_seen_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, TRUE, NOW())
		ON CONFLICT (user_id, device_id) DO UPDATE SET
			device_name = EXCLUDED.device_name,
			device_type = EXCLUDED.device_type,
			client_version = EXCLUDED.client_version,
			ip_address = EXCLUDED.ip_address,
			user_agent = EXCLUDED.user_agent,
			is_active = TRUE,
			last_seen_at = NOW()
		RETURNING id, user_id, device_id, device_name, device_type, client_version, ip_address, user_agent,
		          refresh_token_jti, refresh_token_expires_at, is_active, created_at, last_seen_at
	`, userID, deviceID, deviceName, deviceType, clientVersion, ipAddress, userAgent).Scan(
		&device.ID, &device.UserID, &device.DeviceID, &device.DeviceName, &device.DeviceType,
		&device.ClientVersion, &device.IPAddress, &device.UserAgent,
		&device.RefreshTokenJTI, &device.RefreshTokenExpiresAt, &device.IsActive,
		&device.CreatedAt, &device.LastSeenAt,
	)
	if err != nil {
		return nil, err
	}
	return &device, nil
}

// UpdateDeviceRefreshToken stores the JTI and expiry of the current refresh token
func (db *DB) UpdateDeviceRefreshToken(userID, deviceID, jti string, expiresAt time.Time) error {
	_, err := db.Exec(`
		UPDATE user_devices SET refresh_token_jti = $1, refresh_token_expires_at = $2, last_seen_at = NOW()
		WHERE user_id = $3 AND device_id = $4
	`, jti, expiresAt, userID, deviceID)
	return err
}

// GetDevices returns all active devices for a user
func (db *DB) GetDevices(userID string) ([]UserDevice, error) {
	rows, err := db.Query(`
		SELECT id, user_id, device_id, device_name, device_type, client_version, ip_address, user_agent,
		       refresh_token_jti, refresh_token_expires_at, is_active, created_at, last_seen_at
		FROM user_devices WHERE user_id = $1 AND is_active = TRUE
		ORDER BY last_seen_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var devices []UserDevice
	for rows.Next() {
		var d UserDevice
		err := rows.Scan(&d.ID, &d.UserID, &d.DeviceID, &d.DeviceName, &d.DeviceType,
			&d.ClientVersion, &d.IPAddress, &d.UserAgent,
			&d.RefreshTokenJTI, &d.RefreshTokenExpiresAt, &d.IsActive,
			&d.CreatedAt, &d.LastSeenAt)
		if err != nil {
			return nil, err
		}
		devices = append(devices, d)
	}
	return devices, nil
}

// RevokeDevice deactivates a specific device for a user
func (db *DB) RevokeDevice(userID, deviceID string) error {
	_, err := db.Exec(`UPDATE user_devices SET is_active = FALSE WHERE user_id = $1 AND device_id = $2`, userID, deviceID)
	return err
}

// RevokeAllDevices deactivates all devices for a user
func (db *DB) RevokeAllDevices(userID string) error {
	_, err := db.Exec(`UPDATE user_devices SET is_active = FALSE WHERE user_id = $1`, userID)
	return err
}

// IsDeviceActive checks if a device is still active (not revoked)
func (db *DB) IsDeviceActive(userID, deviceID string) (bool, error) {
	var active bool
	err := db.QueryRow(`SELECT is_active FROM user_devices WHERE user_id = $1 AND device_id = $2`, userID, deviceID).Scan(&active)
	if err != nil {
		return false, err
	}
	return active, nil
}

// ValidateRefreshToken checks if the refresh token JTI matches the stored one for the device
func (db *DB) ValidateRefreshToken(userID, deviceID, jti string) (bool, error) {
	var storedJTI string
	var expiresAt time.Time
	err := db.QueryRow(
		`SELECT refresh_token_jti, refresh_token_expires_at FROM user_devices WHERE user_id = $1 AND device_id = $2 AND is_active = TRUE`,
		userID, deviceID,
	).Scan(&storedJTI, &expiresAt)
	if err != nil {
		return false, err
	}
	if storedJTI != jti {
		return false, nil
	}
	if time.Now().After(expiresAt) {
		return false, nil
	}
	return true, nil
}

// UpdateDeviceLastSeen updates the last_seen_at timestamp for a device
func (db *DB) UpdateDeviceLastSeen(userID, deviceID string) error {
	_, err := db.Exec(`UPDATE user_devices SET last_seen_at = NOW() WHERE user_id = $1 AND device_id = $2`, userID, deviceID)
	return err
}

// LogAuthEvent records an authentication event in the audit log
func (db *DB) LogAuthEvent(userID, deviceID, action, ipAddress, clientVersion string, success bool, errorMessage string) {
	_, _ = db.Exec(`
		INSERT INTO device_auth_log (user_id, device_id, action, ip_address, client_version, success, error_message)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, userID, deviceID, action, ipAddress, clientVersion, success, errorMessage)
}

// CleanupDeviceAuthLog deletes old auth log entries and deactivates expired devices
func (db *DB) CleanupDeviceAuthLog() {
	_, _ = db.Exec(`DELETE FROM device_auth_log WHERE created_at < NOW() - INTERVAL '90 days'`)
	_, _ = db.Exec(`UPDATE user_devices SET is_active = FALSE WHERE refresh_token_expires_at < NOW() AND is_active = TRUE`)
}

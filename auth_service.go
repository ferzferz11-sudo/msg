package main

import (
	"time"
)

// authDB defines the database methods used by authServerV2.
// *DB implements this interface.
type authDB interface {
	UserExists(user string) (bool, error)
	EmailExists(email string) (bool, error)
	GetUserPasswordHash(user string) (string, error)
	SaveUserWithEmail(user, hash, email string) error
	GetUserIdByUsername(user string) (string, error)
	GetUserAvatar(user string) (string, error)
	UpdateLastSeen(user string) error
	queryUserProfile(username string) (email, bio, status string, createdAt, lastSeenAt time.Time, err error)

	UpsertDevice(userID, deviceID, deviceName, deviceType, clientVersion, ipAddress, userAgent string) (*UserDevice, error)
	UpdateDeviceRefreshToken(userID, deviceID, jti string, expiresAt time.Time) error
	GetDevices(userID string) ([]UserDevice, error)
	RevokeDevice(userID, deviceID string) error
	RevokeAllDevices(userID string) error
	IsDeviceActive(userID, deviceID string) (bool, error)
	ValidateRefreshToken(userID, deviceID, jti string) (bool, error)
	UpdateDeviceLastSeen(userID, deviceID string) error
	LogAuthEvent(userID, deviceID, action, ipAddress, clientVersion string, success bool, errorMessage string)
}

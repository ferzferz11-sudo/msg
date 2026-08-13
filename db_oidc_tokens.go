package main

import (
	"crypto/sha256"
	"fmt"
	"time"
)

func oidcHashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return fmt.Sprintf("%x", h)
}

// --- Auth Codes ---

type OAuthAuthCode struct {
	ID                   string
	CodeHash             string
	UserID               string
	ClientID             string
	RedirectURI          string
	Scope                string
	Nonce                string
	CodeChallenge        string
	CodeChallengeMethod  string
	IsUsed               bool
	IsSSO                bool
	CreatedAt            time.Time
	ExpiresAt            time.Time
}

func (db *DB) StoreAuthCode(code *OAuthAuthCode) error {
	_, err := db.Exec(
		`INSERT INTO oauth_auth_codes (code_hash, user_id, client_id, redirect_uri, scope, nonce,
			code_challenge, code_challenge_method, is_sso, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		code.CodeHash, code.UserID, code.ClientID, code.RedirectURI, code.Scope,
		code.Nonce, code.CodeChallenge, code.CodeChallengeMethod, code.IsSSO, code.ExpiresAt,
	)
	return err
}

func (db *DB) GetAuthCode(codeHash string) (*OAuthAuthCode, error) {
	var c OAuthAuthCode
	err := db.QueryRow(
		`SELECT id::text, code_hash, user_id::text, client_id, redirect_uri, scope, nonce,
			code_challenge, code_challenge_method, is_used, is_sso, created_at, expires_at
		 FROM oauth_auth_codes WHERE code_hash = $1`, codeHash,
	).Scan(&c.ID, &c.CodeHash, &c.UserID, &c.ClientID, &c.RedirectURI, &c.Scope,
		&c.Nonce, &c.CodeChallenge, &c.CodeChallengeMethod, &c.IsUsed, &c.IsSSO,
		&c.CreatedAt, &c.ExpiresAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (db *DB) MarkAuthCodeUsed(codeHash string) error {
	_, err := db.Exec(`UPDATE oauth_auth_codes SET is_used=TRUE WHERE code_hash=$1`, codeHash)
	return err
}

func (db *DB) IsAuthCodeUsed(codeHash string) (bool, error) {
	var used bool
	err := db.QueryRow(`SELECT is_used FROM oauth_auth_codes WHERE code_hash=$1`, codeHash).Scan(&used)
	return used, err
}

// --- Refresh Tokens ---

type OAuthRefreshToken struct {
	ID            string
	TokenHash     string
	UserID        string
	ClientID      string
	Scope         string
	DeviceID      string
	IsRevoked     bool
	ExpiresAt     time.Time
	CreatedAt     time.Time
	ReplacedByID  *string
	UseCount      int
}

func (db *DB) StoreRefreshToken(t *OAuthRefreshToken) error {
	_, err := db.Exec(
		`INSERT INTO oauth_refresh_tokens (token_hash, user_id, client_id, scope, device_id, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		t.TokenHash, t.UserID, t.ClientID, t.Scope, t.DeviceID, t.ExpiresAt,
	)
	return err
}

func (db *DB) GetRefreshToken(tokenHash string) (*OAuthRefreshToken, error) {
	var t OAuthRefreshToken
	err := db.QueryRow(
		`SELECT id::text, token_hash, user_id::text, client_id, scope, device_id,
			is_revoked, expires_at, created_at, replaced_by_id::text, use_count
		 FROM oauth_refresh_tokens WHERE token_hash = $1`, tokenHash,
	).Scan(&t.ID, &t.TokenHash, &t.UserID, &t.ClientID, &t.Scope, &t.DeviceID,
		&t.IsRevoked, &t.ExpiresAt, &t.CreatedAt, &t.ReplacedByID, &t.UseCount)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// RevokeRefreshToken atomically revokes a refresh token. Returns true if token was valid and revoked.
func (db *DB) RevokeRefreshToken(tokenHash, clientID string) (bool, error) {
	result, err := db.Exec(
		`UPDATE oauth_refresh_tokens SET is_revoked=TRUE
		 WHERE token_hash=$1 AND client_id=$2 AND is_revoked=FALSE`,
		tokenHash, clientID,
	)
	if err != nil {
		return false, err
	}
	n, _ := result.RowsAffected()
	return n > 0, nil
}

// RotateRefreshToken atomically revokes old token and records replacement. Returns true if old token was valid.
func (db *DB) RotateRefreshToken(oldHash, newID, clientID string) (bool, error) {
	result, err := db.Exec(
		`UPDATE oauth_refresh_tokens SET is_revoked=TRUE, replaced_by_id=$1
		 WHERE token_hash=$2 AND client_id=$3 AND is_revoked=FALSE`,
		newID, oldHash, clientID,
	)
	if err != nil {
		return false, err
	}
	n, _ := result.RowsAffected()
	return n > 0, nil
}

func (db *DB) RevokeAllRefreshTokens(userID, clientID string) error {
	_, err := db.Exec(
		`UPDATE oauth_refresh_tokens SET is_revoked=TRUE
		 WHERE user_id=$1::uuid AND client_id=$2 AND is_revoked=FALSE`,
		userID, clientID,
	)
	return err
}

// --- Access Token Audit ---

type OAuthAccessTokenAudit struct {
	ID        string
	JTI       string
	UserID    string
	ClientID  string
	Scope     string
	IsRevoked bool
	ExpiresAt time.Time
	CreatedAt time.Time
}

func (db *DB) StoreAccessTokenAudit(t *OAuthAccessTokenAudit) error {
	_, err := db.Exec(
		`INSERT INTO oauth_access_tokens (jti, user_id, client_id, scope, expires_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		t.JTI, t.UserID, t.ClientID, t.Scope, t.ExpiresAt,
	)
	return err
}

func (db *DB) RevokeAccessToken(jti string) error {
	_, err := db.Exec(`UPDATE oauth_access_tokens SET is_revoked=TRUE WHERE jti=$1`, jti)
	return err
}

func (db *DB) IsAccessTokenRevoked(jti string) (bool, error) {
	var revoked bool
	err := db.QueryRow(`SELECT is_revoked FROM oauth_access_tokens WHERE jti=$1`, jti).Scan(&revoked)
	if err != nil {
		return false, err
	}
	return revoked, nil
}

// --- Grants ---

type OAuthGrant struct {
	ID         string
	UserID     string
	ClientID   string
	Scope      string
	GrantedAt  time.Time
	LastUsedAt time.Time
}

func (db *DB) GetOAuthGrant(userID, clientID string) (*OAuthGrant, error) {
	var g OAuthGrant
	err := db.QueryRow(
		`SELECT id::text, user_id::text, client_id, scope, granted_at, last_used_at
		 FROM oauth_grants WHERE user_id=$1::uuid AND client_id=$2`,
		userID, clientID,
	).Scan(&g.ID, &g.UserID, &g.ClientID, &g.Scope, &g.GrantedAt, &g.LastUsedAt)
	if err != nil {
		return nil, err
	}
	return &g, nil
}

func (db *DB) UpsertOAuthGrant(userID, clientID, scope string) error {
	_, err := db.Exec(
		`INSERT INTO oauth_grants (user_id, client_id, scope)
		 VALUES ($1::uuid, $2, $3)
		 ON CONFLICT (user_id, client_id)
		 DO UPDATE SET scope=$3, last_used_at=NOW()`,
		userID, clientID, scope,
	)
	return err
}

func (db *DB) CleanupOIDCTokens() {
	// Delete expired refresh tokens
	db.Exec(`DELETE FROM oauth_refresh_tokens WHERE expires_at < NOW() - INTERVAL '7 days' OR (is_revoked=TRUE AND created_at < NOW() - INTERVAL '30 days')`)
	// Delete expired auth codes
	db.Exec(`DELETE FROM oauth_auth_codes WHERE expires_at < NOW() - INTERVAL '1 day'`)
	// Delete expired access token records
	db.Exec(`DELETE FROM oauth_access_tokens WHERE expires_at < NOW() - INTERVAL '7 days'`)
}

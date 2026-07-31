package main

import (
	"encoding/json"
	"time"
)

type OAuthClient struct {
	ID                      string
	ClientID                string
	ClientSecretHash        *string
	ClientName              string
	ClientType              string // "public" or "confidential"
	RedirectURIs            []string
	AllowedScopes           []string
	TokenEndpointAuthMethod string
	GrantTypes              []string
	AllowedSSO              bool
	IsActive                bool
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

func (db *DB) CreateOAuthClient(c *OAuthClient) error {
	redirectURIs, _ := json.Marshal(c.RedirectURIs)
	_, err := db.Exec(
		`INSERT INTO oauth_clients (client_id, client_secret_hash, client_name, client_type,
			redirect_uris, allowed_scopes, token_endpoint_auth_method, grant_types, allowed_sso)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		c.ClientID, c.ClientSecretHash, c.ClientName, c.ClientType,
		redirectURIs, pqStringArray(c.AllowedScopes), c.TokenEndpointAuthMethod,
		pqStringArray(c.GrantTypes), c.AllowedSSO,
	)
	return err
}

func (db *DB) GetOAuthClient(clientID string) (*OAuthClient, error) {
	var c OAuthClient
	var redirectURISJSON, allowedScopes, grantTypes string
	err := db.QueryRow(
		`SELECT id::text, client_id, client_secret_hash, client_name, client_type,
			redirect_uris::text, allowed_scopes::text, token_endpoint_auth_method,
			grant_types::text, allowed_sso, is_active, created_at, updated_at
		 FROM oauth_clients WHERE client_id = $1`, clientID,
	).Scan(&c.ID, &c.ClientID, &c.ClientSecretHash, &c.ClientName, &c.ClientType,
		&redirectURISJSON, &allowedScopes, &c.TokenEndpointAuthMethod,
		&grantTypes, &c.AllowedSSO, &c.IsActive, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(redirectURISJSON), &c.RedirectURIs)
	_ = json.Unmarshal([]byte(allowedScopes), &c.AllowedScopes)
	_ = json.Unmarshal([]byte(grantTypes), &c.GrantTypes)
	return &c, nil
}

func (db *DB) ListOAuthClients() ([]*OAuthClient, error) {
	rows, err := db.Query(
		`SELECT id::text, client_id, client_name, client_type, allowed_scopes::text,
			allowed_sso, is_active, created_at
		 FROM oauth_clients ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var clients []*OAuthClient
	for rows.Next() {
		var c OAuthClient
		var scopesJSON string
		if err := rows.Scan(&c.ID, &c.ClientID, &c.ClientName, &c.ClientType,
			&scopesJSON, &c.AllowedSSO, &c.IsActive, &c.CreatedAt); err != nil {
			continue
		}
		_ = json.Unmarshal([]byte(scopesJSON), &c.AllowedScopes)
		clients = append(clients, &c)
	}
	return clients, nil
}

func (db *DB) UpdateOAuthClient(clientID string, updates map[string]interface{}) error {
	// Simple approach: re-read and update specific fields
	if name, ok := updates["client_name"].(string); ok {
		_, err := db.Exec(`UPDATE oauth_clients SET client_name=$1, updated_at=NOW() WHERE client_id=$2`, name, clientID)
		if err != nil {
			return err
		}
	}
	if active, ok := updates["is_active"].(bool); ok {
		_, err := db.Exec(`UPDATE oauth_clients SET is_active=$1, updated_at=NOW() WHERE client_id=$2`, active, clientID)
		if err != nil {
			return err
		}
	}
	if sso, ok := updates["allowed_sso"].(bool); ok {
		_, err := db.Exec(`UPDATE oauth_clients SET allowed_sso=$1, updated_at=NOW() WHERE client_id=$2`, sso, clientID)
		if err != nil {
			return err
		}
	}
	return nil
}

func (db *DB) DeleteOAuthClient(clientID string) error {
	_, err := db.Exec(`UPDATE oauth_clients SET is_active=FALSE, updated_at=NOW() WHERE client_id=$1`, clientID)
	return err
}

func (db *DB) RotateOAuthClientSecret(clientID string, newHash string) error {
	_, err := db.Exec(`UPDATE oauth_clients SET client_secret_hash=$1, updated_at=NOW() WHERE client_id=$2`, newHash, clientID)
	return err
}

// pqStringArray is a helper for scanning PostgreSQL TEXT[] into []string
type pqStringArray []string

func (a pqStringArray) Value() interface{} {
	if len(a) == 0 {
		return "{}"
	}
	b, _ := json.Marshal(a)
	return string(b)
}

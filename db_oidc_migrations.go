package main

import "database/sql"

func runOIDCMigrations(db *sql.DB) error {
	migrations := []string{
		// 1. OAuth Clients
		`CREATE TABLE IF NOT EXISTS oauth_clients (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			client_id VARCHAR(255) UNIQUE NOT NULL,
			client_secret_hash VARCHAR(255),
			client_name VARCHAR(255) NOT NULL,
			client_type VARCHAR(20) NOT NULL CHECK (client_type IN ('public', 'confidential')),
			redirect_uris JSONB NOT NULL DEFAULT '[]'::jsonb,
			allowed_scopes TEXT[] NOT NULL DEFAULT '{}',
			token_endpoint_auth_method VARCHAR(50) NOT NULL DEFAULT 'none',
			grant_types TEXT[] NOT NULL DEFAULT '{authorization_code,refresh_token}',
			allowed_sso BOOLEAN NOT NULL DEFAULT FALSE,
			is_active BOOLEAN NOT NULL DEFAULT TRUE,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_oauth_clients_client_id ON oauth_clients(client_id)`,

		// 2. Refresh Tokens
		`CREATE TABLE IF NOT EXISTS oauth_refresh_tokens (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			token_hash VARCHAR(255) UNIQUE NOT NULL,
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			client_id VARCHAR(255) NOT NULL REFERENCES oauth_clients(client_id) ON DELETE CASCADE,
			scope TEXT NOT NULL,
			device_id VARCHAR(255) DEFAULT '',
			is_revoked BOOLEAN NOT NULL DEFAULT FALSE,
			expires_at TIMESTAMP NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			replaced_by_id UUID,
			use_count INT NOT NULL DEFAULT 0
		)`,
		`CREATE INDEX IF NOT EXISTS idx_oauth_refresh_tokens_hash ON oauth_refresh_tokens(token_hash)`,
		`CREATE INDEX IF NOT EXISTS idx_oauth_refresh_tokens_user ON oauth_refresh_tokens(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_oauth_refresh_tokens_client ON oauth_refresh_tokens(client_id)`,

		// 3. User Consent Grants
		`CREATE TABLE IF NOT EXISTS oauth_grants (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			client_id VARCHAR(255) NOT NULL REFERENCES oauth_clients(client_id) ON DELETE CASCADE,
			scope TEXT NOT NULL,
			granted_at TIMESTAMP NOT NULL DEFAULT NOW(),
			last_used_at TIMESTAMP NOT NULL DEFAULT NOW(),
			UNIQUE(user_id, client_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_oauth_grants_user_client ON oauth_grants(user_id, client_id)`,

		// 4. Auth Code Audit
		`CREATE TABLE IF NOT EXISTS oauth_auth_codes (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			code_hash VARCHAR(255) UNIQUE NOT NULL,
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			client_id VARCHAR(255) NOT NULL REFERENCES oauth_clients(client_id) ON DELETE CASCADE,
			redirect_uri TEXT NOT NULL,
			scope TEXT NOT NULL,
			nonce VARCHAR(255) DEFAULT '',
			code_challenge VARCHAR(255) NOT NULL,
			code_challenge_method VARCHAR(10) NOT NULL DEFAULT 'S256',
			is_used BOOLEAN NOT NULL DEFAULT FALSE,
			is_sso BOOLEAN NOT NULL DEFAULT FALSE,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			expires_at TIMESTAMP NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_oauth_auth_codes_hash ON oauth_auth_codes(code_hash)`,

		// 5. Access Token Audit
		`CREATE TABLE IF NOT EXISTS oauth_access_tokens (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			jti VARCHAR(255) UNIQUE NOT NULL,
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			client_id VARCHAR(255) NOT NULL REFERENCES oauth_clients(client_id) ON DELETE CASCADE,
			scope TEXT NOT NULL,
			is_revoked BOOLEAN NOT NULL DEFAULT FALSE,
			expires_at TIMESTAMP NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_oauth_access_tokens_jti ON oauth_access_tokens(jti)`,
		`CREATE INDEX IF NOT EXISTS idx_oauth_access_tokens_expires ON oauth_access_tokens(expires_at)`,
	}

	for _, m := range migrations {
		if _, err := db.Exec(m); err != nil {
			logger.Errorf("OIDC migration error: %v", err)
		}
	}
	logger.Info("OIDC tables ready")
	return nil
}

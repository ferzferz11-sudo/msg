package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

func oidcTokenHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		oidcError(w, "invalid_request", "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		oidcError(w, "invalid_request", "invalid form data", http.StatusBadRequest)
		return
	}

	grantType := r.FormValue("grant_type")
	switch grantType {
	case "authorization_code":
		handleAuthCodeExchange(w, r)
	case "refresh_token":
		handleRefreshTokenGrant(w, r)
	default:
		oidcError(w, "unsupported_grant_type", "grant_type must be authorization_code or refresh_token", http.StatusBadRequest)
	}
}

func handleAuthCodeExchange(w http.ResponseWriter, r *http.Request) {
	code := r.FormValue("code")
	redirectURI := r.FormValue("redirect_uri")
	clientID := r.FormValue("client_id")
	codeVerifier := r.FormValue("code_verifier")

	if code == "" || redirectURI == "" || clientID == "" || codeVerifier == "" {
		oidcError(w, "invalid_request", "missing required parameters", http.StatusBadRequest)
		return
	}

	db := httpDB
	if db == nil {
		oidcError(w, "server_error", "database not available", http.StatusInternalServerError)
		return
	}

	// Authenticate client
	client, err := db.GetOAuthClient(clientID)
	if err != nil || !client.IsActive {
		oidcError(w, "invalid_client", "unknown or inactive client", http.StatusUnauthorized)
		return
	}

	// Check client secret for confidential clients
	if client.ClientType == "confidential" {
		if !authenticateClient(r, client) {
			oidcError(w, "invalid_client", "client authentication failed", http.StatusUnauthorized)
			return
		}
	}

	// Look up auth code
	codeHash := oidcHashToken(code)
	authCode, err := db.GetAuthCode(codeHash)
	if err != nil {
		// Check if code was already used (replay detection)
		used, _ := db.IsAuthCodeUsed(codeHash)
		if used {
			// Replay detected: revoke all tokens for this code
			logger.Warnf("OIDC auth code replay detected for client %s", clientID)
			oidcError(w, "invalid_grant", "authorization code already used", http.StatusBadRequest)
			return
		}
		oidcError(w, "invalid_grant", "authorization code not found or expired", http.StatusBadRequest)
		return
	}

	// Validate code
	if authCode.IsUsed {
		oidcError(w, "invalid_grant", "authorization code already used", http.StatusBadRequest)
		return
	}
	if authCode.ClientID != clientID {
		oidcError(w, "invalid_grant", "client_id mismatch", http.StatusBadRequest)
		return
	}
	if authCode.RedirectURI != redirectURI {
		oidcError(w, "invalid_grant", "redirect_uri mismatch", http.StatusBadRequest)
		return
	}
	if timeNow().After(authCode.ExpiresAt) {
		oidcError(w, "invalid_grant", "authorization code expired", http.StatusBadRequest)
		return
	}

	// Validate PKCE
	expectedChallenge := computePKCEChallenge(codeVerifier)
	if expectedChallenge != authCode.CodeChallenge {
		oidcError(w, "invalid_grant", "PKCE verification failed", http.StatusBadRequest)
		return
	}

	// Mark code as used
	db.MarkAuthCodeUsed(codeHash)

	// Get user info for tokens
	userID := authCode.UserID
	profile, err := db.GetUserProfileById(userID)
	username, email, name, avatar := "", "", "", ""
	if err == nil {
		username = profile.Username
		name = profile.Username
		avatar = profile.AvatarURL
	}
	// Get email separately (not in profile struct)
	var emailScan string
	db.QueryRow(`SELECT COALESCE(email, '') FROM users WHERE id=$1::uuid`, userID).Scan(&emailScan)
	email = emailScan

	// Generate tokens
	idToken, err := GenerateIDToken(userID, clientID, authCode.Nonce, email, username, name, avatar)
	if err != nil {
		oidcError(w, "server_error", "failed to generate ID token", http.StatusInternalServerError)
		return
	}

	accessToken, jti, exp, err := GenerateAccessToken(userID, clientID, authCode.Scope)
	if err != nil {
		oidcError(w, "server_error", "failed to generate access token", http.StatusInternalServerError)
		return
	}

	// Store access token audit
	db.StoreAccessTokenAudit(&OAuthAccessTokenAudit{
		JTI:       jti,
		UserID:    userID,
		ClientID:  clientID,
		Scope:     authCode.Scope,
		ExpiresAt: exp,
	})

	// Generate refresh token
	refreshToken, refreshHash, err := GenerateOpaqueRefreshToken()
	if err != nil {
		oidcError(w, "server_error", "failed to generate refresh token", http.StatusInternalServerError)
		return
	}

	// Store refresh token
	db.StoreRefreshToken(&OAuthRefreshToken{
		TokenHash: refreshHash,
		UserID:    userID,
		ClientID:  clientID,
		Scope:     authCode.Scope,
		ExpiresAt: timeNow().Add(30 * 24 * time.Hour),
	})

	// Response
	response := map[string]interface{}{
		"access_token":  accessToken,
		"token_type":    "Bearer",
		"expires_in":    3600,
		"refresh_token": refreshToken,
		"id_token":      idToken,
		"scope":         authCode.Scope,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func handleRefreshTokenGrant(w http.ResponseWriter, r *http.Request) {
	refreshToken := r.FormValue("refresh_token")
	clientID := r.FormValue("client_id")
	requestedScope := r.FormValue("scope")

	if refreshToken == "" || clientID == "" {
		oidcError(w, "invalid_request", "missing refresh_token or client_id", http.StatusBadRequest)
		return
	}

	db := httpDB
	if db == nil {
		oidcError(w, "server_error", "database not available", http.StatusInternalServerError)
		return
	}

	// Authenticate client
	client, err := db.GetOAuthClient(clientID)
	if err != nil || !client.IsActive {
		oidcError(w, "invalid_client", "unknown or inactive client", http.StatusUnauthorized)
		return
	}

	// Hash refresh token
	refreshHash := fmt.Sprintf("%x", sha256.Sum256([]byte(refreshToken)))

	// Validate refresh token
	oldToken, err := ValidateOIDCRefreshToken(db, refreshHash, clientID)
	if err != nil {
		oidcError(w, "invalid_grant", err.Error(), http.StatusBadRequest)
		return
	}

	// Validate scope (must be subset of original)
	scope := oldToken.Scope
	if requestedScope != "" {
		if !isSubsetOf(strings.Fields(requestedScope), strings.Fields(oldToken.Scope)) {
			oidcError(w, "invalid_scope", "requested scope exceeds original grant", http.StatusBadRequest)
			return
		}
		scope = requestedScope
	}

	// Rotate: revoke old, issue new
	newID := uuid.New().String()
	revoked, err := db.RotateRefreshToken(refreshHash, newID, clientID)
	if err != nil || !revoked {
		// Token was already revoked = replay detected
		logger.Warnf("OIDC refresh token replay detected for client %s user %s", clientID, oldToken.UserID)
		db.RevokeAllRefreshTokens(oldToken.UserID, clientID)
		oidcError(w, "invalid_grant", "refresh token revoked (replay detected)", http.StatusBadRequest)
		return
	}

	// Get user info
	userID := oldToken.UserID
	profile, err := db.GetUserProfileById(userID)
	username, email, name, avatar := "", "", "", ""
	if err == nil {
		username = profile.Username
		name = profile.Username
		avatar = profile.AvatarURL
	}
	var emailScan2 string
	db.QueryRow(`SELECT COALESCE(email, '') FROM users WHERE id=$1::uuid`, userID).Scan(&emailScan2)
	email = emailScan2

	// Generate new tokens
	idToken, _ := GenerateIDToken(userID, clientID, "", email, username, name, avatar)
	accessToken, jti, exp, _ := GenerateAccessToken(userID, clientID, scope)

	db.StoreAccessTokenAudit(&OAuthAccessTokenAudit{
		JTI: jti, UserID: userID, ClientID: clientID, Scope: scope, ExpiresAt: exp,
	})

	newRefreshToken, newRefreshHash, _ := GenerateOpaqueRefreshToken()
	db.StoreRefreshToken(&OAuthRefreshToken{
		TokenHash: newRefreshHash, UserID: userID, ClientID: clientID,
		Scope: scope, ExpiresAt: timeNow().Add(30 * 24 * time.Hour),
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"access_token":  accessToken,
		"token_type":    "Bearer",
		"expires_in":    3600,
		"refresh_token": newRefreshToken,
		"id_token":      idToken,
		"scope":         scope,
	})
}

func authenticateClient(r *http.Request, client *OAuthClient) bool {
	// Check Authorization: Basic header
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Basic ") {
		decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(authHeader, "Basic "))
		if err != nil {
			return false
		}
		parts := strings.SplitN(string(decoded), ":", 2)
		if len(parts) == 2 && parts[0] == client.ClientID {
			// Verify client secret
			h := sha256.Sum256([]byte(parts[1]))
			storedHash := ""
			if client.ClientSecretHash != nil {
				storedHash = *client.ClientSecretHash
			}
			return fmt.Sprintf("%x", h) == storedHash
		}
	}

	// Check client_secret_post
	secret := r.FormValue("client_secret")
	if secret != "" && client.ClientSecretHash != nil {
		h := sha256.Sum256([]byte(secret))
		return fmt.Sprintf("%x", h) == *client.ClientSecretHash
	}

	return false
}

func isSubsetOf(requested, granted []string) bool {
	grantedSet := make(map[string]bool)
	for _, g := range granted {
		grantedSet[g] = true
	}
	for _, r := range requested {
		if !grantedSet[r] {
			return false
		}
	}
	return true
}

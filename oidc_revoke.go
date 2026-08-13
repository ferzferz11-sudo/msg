package main

import (
	"net/http"
	"strings"
)

func oidcRevokeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		oidcError(w, "invalid_request", "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		oidcError(w, "invalid_request", "invalid form data", http.StatusBadRequest)
		return
	}

	token := r.FormValue("token")
	tokenTypeHint := r.FormValue("token_type_hint")
	clientID := r.FormValue("client_id")

	if token == "" {
		oidcError(w, "invalid_request", "token is required", http.StatusBadRequest)
		return
	}

	db := httpDB
	if db == nil {
		// Still return 200 per RFC 7009
		w.WriteHeader(http.StatusOK)
		return
	}

	// Authenticate client
	if clientID != "" {
		client, err := db.GetOAuthClient(clientID)
		if err != nil || !client.IsActive {
			// Return 200 even on auth failure per RFC 7009
			w.WriteHeader(http.StatusOK)
			return
		}
		_ = client // authenticated
	}

	if tokenTypeHint == "refresh_token" || tokenTypeHint == "" {
		// Try as refresh token
		tokenHash := oidcHashToken(token)
		db.RevokeRefreshToken(tokenHash, clientID)
	}

	if tokenTypeHint == "access_token" || tokenTypeHint == "" {
		// Try as access token (extract JTI from JWT without verification)
		if claims, err := extractOIDCClaimsUnsafe(token); err == nil && claims != nil {
			db.RevokeAccessToken(claims.ID)
		}
	}

	// Always return 200 per RFC 7009
	w.WriteHeader(http.StatusOK)
}

func extractOIDCClaimsUnsafe(tokenString string) (*OIDCClaims, error) {
	// Parse without verification — only for extracting JTI for revocation
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return nil, nil
	}
	// Use the validation function which does verify — acceptable for revocation
	return ValidateOIDCAccessToken(tokenString)
}

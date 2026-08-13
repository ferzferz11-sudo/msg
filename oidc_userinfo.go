package main

import (
	"encoding/json"
	"net/http"
	"strings"
)

func oidcUserInfoHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		oidcError(w, "invalid_request", "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract Bearer token
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		oidcError(w, "invalid_request", "missing or invalid Authorization header", http.StatusUnauthorized)
		return
	}
	tokenString := strings.TrimPrefix(authHeader, "Bearer ")

	// Validate OIDC access token
	claims, err := ValidateOIDCAccessToken(tokenString)
	if err != nil {
		oidcError(w, "invalid_token", "access token is invalid or expired", http.StatusUnauthorized)
		return
	}

	// Check if token is revoked
	if httpDB != nil {
		revoked, _ := httpDB.IsAccessTokenRevoked(claims.ID)
		if revoked {
			oidcError(w, "invalid_token", "access token has been revoked", http.StatusUnauthorized)
			return
		}
	}

	// Build response based on scopes
	response := map[string]interface{}{
		"sub": claims.UserID,
	}

	if strings.Contains(claims.Scope, "profile") || strings.Contains(claims.Scope, "read:profile") {
		if claims.Username != "" {
			response["preferred_username"] = claims.Username
		}
		if claims.Name != "" {
			response["name"] = claims.Name
		}
		if claims.Picture != "" {
			response["picture"] = claims.Picture
		}
	}

	if strings.Contains(claims.Scope, "email") {
		if claims.Email != "" {
			response["email"] = claims.Email
			response["email_verified"] = true
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

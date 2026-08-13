package main

import (
	"encoding/json"
	"net/http"
)

func oidcIntrospectHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		oidcError(w, "invalid_request", "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		oidcError(w, "invalid_request", "invalid form data", http.StatusBadRequest)
		return
	}

	token := r.FormValue("token")
	if token == "" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"active": false})
		return
	}

	claims, err := ValidateOIDCAccessToken(token)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"active": false})
		return
	}

	// Check if revoked
	db := httpDB
	if db != nil {
		revoked, _ := db.IsAccessTokenRevoked(claims.ID)
		if revoked {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"active": false})
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"active":     true,
		"sub":        claims.UserID,
		"client_id":  claims.ClientID,
		"scope":      claims.Scope,
		"exp":        claims.ExpiresAt.Unix(),
		"iat":        claims.IssuedAt.Unix(),
		"iss":        claims.Issuer,
		"token_type": "Bearer",
		"aud":        claims.ClientID,
	})
}

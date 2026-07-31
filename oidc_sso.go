package main

import (
	"encoding/json"
	"net/http"
)

// oidcSSOCheckHandler handles deep link SSO checks
// GET /oidc/sso-check?client_id=...&redirect_uri=...&scope=...&state=...&code_challenge=...&code_challenge_method=S256
func oidcSSOCheckHandler(w http.ResponseWriter, r *http.Request) {
	// This endpoint is hit by the Lavender app via deep link
	// It checks if user has an active session and returns an auth code
	clientID := r.URL.Query().Get("client_id")
	redirectURI := r.URL.Query().Get("redirect_uri")
	scope := r.URL.Query().Get("scope")
	state := r.URL.Query().Get("state")
	codeChallenge := r.URL.Query().Get("code_challenge")
	codeChallengeMethod := r.URL.Query().Get("code_challenge_method")

	if clientID == "" || redirectURI == "" || codeChallenge == "" {
		oidcError(w, "invalid_request", "missing required parameters", http.StatusBadRequest)
		return
	}

	// Check for existing Lavender session (via cookie or token)
	userID := getOIDCSessionUser(r)
	if userID == "" {
		oidcError(w, "login_required", "no active Lavender session", http.StatusUnauthorized)
		return
	}

	db := httpDB
	if db == nil {
		oidcError(w, "server_error", "database not available", http.StatusInternalServerError)
		return
	}

	// Validate client
	client, err := db.GetOAuthClient(clientID)
	if err != nil || !client.IsActive || !client.AllowedSSO {
		oidcError(w, "unauthorized_client", "client not authorized for SSO", http.StatusUnauthorized)
		return
	}

	// Generate auth code
	params := &oidcAuthorizeParams{
		ClientID:            clientID,
		RedirectURI:         redirectURI,
		Scope:               scope,
		State:               state,
		CodeChallenge:       codeChallenge,
		CodeChallengeMethod: codeChallengeMethod,
	}

	code, err := generateAndStoreAuthCode(db, userID, params, true)
	if err != nil {
		oidcError(w, "server_error", "failed to generate code", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"code":  code,
		"state": state,
	})
}

// oidcSSOExchangeHandler exchanges a Lavender JWT for OIDC tokens
// POST /oidc/sso-exchange
func oidcSSOExchangeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		oidcError(w, "invalid_request", "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ClientID            string `json:"client_id"`
		CodeChallenge       string `json:"code_challenge"`
		CodeChallengeMethod string `json:"code_challenge_method"`
		Scope               string `json:"scope"`
		State               string `json:"state"`
		LavenderToken       string `json:"lavender_token"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		oidcError(w, "invalid_request", "invalid JSON body", http.StatusBadRequest)
		return
	}

	if req.ClientID == "" || req.CodeChallenge == "" || req.LavenderToken == "" {
		oidcError(w, "invalid_request", "missing required fields", http.StatusBadRequest)
		return
	}

	// Validate Lavender JWT (HS256, existing system)
	lavenderClaims, err := ValidateToken(req.LavenderToken)
	if err != nil {
		oidcError(w, "invalid_token", "invalid Lavender token", http.StatusUnauthorized)
		return
	}

	// Only access tokens allowed for SSO
	if lavenderClaims.Type != "access" {
		oidcError(w, "invalid_token", "only access tokens allowed for SSO", http.StatusUnauthorized)
		return
	}

	db := httpDB
	if db == nil {
		oidcError(w, "server_error", "database not available", http.StatusInternalServerError)
		return
	}

	// Validate client
	client, err := db.GetOAuthClient(req.ClientID)
	if err != nil || !client.IsActive || !client.AllowedSSO {
		oidcError(w, "unauthorized_client", "client not authorized for SSO", http.StatusUnauthorized)
		return
	}

	// Validate redirect URI (for SSO, use a pre-registered redirect or skip)
	redirectURI := ""
	if len(client.RedirectURIs) > 0 {
		redirectURI = client.RedirectURIs[0] // Use first registered URI
	}

	// Generate auth code
	params := &oidcAuthorizeParams{
		ClientID:            req.ClientID,
		RedirectURI:         redirectURI,
		Scope:               req.Scope,
		State:               req.State,
		CodeChallenge:       req.CodeChallenge,
		CodeChallengeMethod: req.CodeChallengeMethod,
	}

	code, err := generateAndStoreAuthCode(db, lavenderClaims.UserID, params, true)
	if err != nil {
		oidcError(w, "server_error", "failed to generate code", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"code":  code,
		"state": req.State,
	})
}

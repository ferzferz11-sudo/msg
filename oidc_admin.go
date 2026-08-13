package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// requireAdminAuth checks that the request has a valid Lavender admin JWT
func requireAdminAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			oidcError(w, "unauthorized", "missing authorization", http.StatusUnauthorized)
			return
		}
		token := strings.TrimPrefix(authHeader, "Bearer ")
		claims, err := ValidateToken(token)
		if err != nil || claims.Type != "access" {
			oidcError(w, "unauthorized", "invalid token", http.StatusUnauthorized)
			return
		}
		// Check if user is super admin
		if httpDB != nil {
			if !httpDB.IsSuperAdmin(claims.UserID) {
				oidcError(w, "forbidden", "admin access required", http.StatusForbidden)
				return
			}
		}
		next(w, r)
	}
}

// oidcAdminClientsHandler handles GET (list) and POST (create) for /oidc/admin/clients
func oidcAdminClientsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		listOAuthClients(w, r)
	case http.MethodPost:
		createOAuthClient(w, r)
	default:
		oidcError(w, "invalid_request", "method not allowed", http.StatusMethodNotAllowed)
	}
}

func listOAuthClients(w http.ResponseWriter, r *http.Request) {
	db := httpDB
	if db == nil {
		oidcError(w, "server_error", "database not available", http.StatusInternalServerError)
		return
	}
	clients, err := db.ListOAuthClients()
	if err != nil {
		oidcError(w, "server_error", "failed to list clients", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"clients": clients})
}

func createOAuthClient(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ClientName              string   `json:"client_name"`
		ClientType              string   `json:"client_type"`
		RedirectURIs            []string `json:"redirect_uris"`
		AllowedScopes           []string `json:"allowed_scopes"`
		TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
		GrantTypes              []string `json:"grant_types"`
		AllowedSSO              bool     `json:"allowed_sso"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		oidcError(w, "invalid_request", "invalid JSON", http.StatusBadRequest)
		return
	}

	if req.ClientName == "" || len(req.RedirectURIs) == 0 {
		oidcError(w, "invalid_request", "client_name and redirect_uris are required", http.StatusBadRequest)
		return
	}
	if req.ClientType == "" {
		req.ClientType = "public"
	}
	if req.TokenEndpointAuthMethod == "" {
		req.TokenEndpointAuthMethod = "none"
	}
	if len(req.GrantTypes) == 0 {
		req.GrantTypes = []string{"authorization_code", "refresh_token"}
	}
	if len(req.AllowedScopes) == 0 {
		req.AllowedScopes = []string{"openid", "profile", "email"}
	}

	db := httpDB
	if db == nil {
		oidcError(w, "server_error", "database not available", http.StatusInternalServerError)
		return
	}

	// Generate client_id
	b := make([]byte, 16)
	rand.Read(b)
	clientID := hex.EncodeToString(b)

	client := &OAuthClient{
		ClientID:                clientID,
		ClientName:              req.ClientName,
		ClientType:              req.ClientType,
		RedirectURIs:            req.RedirectURIs,
		AllowedScopes:           req.AllowedScopes,
		TokenEndpointAuthMethod: req.TokenEndpointAuthMethod,
		GrantTypes:              req.GrantTypes,
		AllowedSSO:              req.AllowedSSO,
		IsActive:                true,
	}

	// Generate client secret for confidential clients
	var clientSecret string
	if req.ClientType == "confidential" {
		secretBytes := make([]byte, 32)
		rand.Read(secretBytes)
		clientSecret = hex.EncodeToString(secretBytes)
		h := sha256.Sum256([]byte(clientSecret))
		hash := fmt.Sprintf("%x", h)
		client.ClientSecretHash = &hash
	}

	if err := db.CreateOAuthClient(client); err != nil {
		oidcError(w, "server_error", "failed to create client", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"client_id":     clientID,
		"client_name":   client.ClientName,
		"client_type":   client.ClientType,
		"redirect_uris": client.RedirectURIs,
		"created_at":    client.CreatedAt,
	}
	if clientSecret != "" {
		response["client_secret"] = clientSecret // Only shown once!
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// oidcAdminClientHandler handles single client operations (GET, PUT, DELETE)
func oidcAdminClientHandler(w http.ResponseWriter, r *http.Request) {
	// Extract client_id from URL path: /oidc/admin/clients/{client_id}
	path := strings.TrimPrefix(r.URL.Path, "/oidc/admin/clients/")
	path = strings.TrimSuffix(path, "/rotate-secret")
	clientID := strings.Split(path, "/")[0]

	if clientID == "" {
		oidcError(w, "invalid_request", "client_id is required", http.StatusBadRequest)
		return
	}

	db := httpDB
	if db == nil {
		oidcError(w, "server_error", "database not available", http.StatusInternalServerError)
		return
	}

	switch r.Method {
	case http.MethodGet:
		client, err := db.GetOAuthClient(clientID)
		if err != nil {
			oidcError(w, "not_found", "client not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(client)

	case http.MethodPut:
		var updates map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
			oidcError(w, "invalid_request", "invalid JSON", http.StatusBadRequest)
			return
		}
		if err := db.UpdateOAuthClient(clientID, updates); err != nil {
			oidcError(w, "server_error", "failed to update client", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "updated"})

	case http.MethodDelete:
		if err := db.DeleteOAuthClient(clientID); err != nil {
			oidcError(w, "server_error", "failed to delete client", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})

	default:
		oidcError(w, "invalid_request", "method not allowed", http.StatusMethodNotAllowed)
	}
}

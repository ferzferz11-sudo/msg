package main

import (
	"net/http"
	"strings"
)

func oidcLogoutHandler(w http.ResponseWriter, r *http.Request) {
	idTokenHint := r.URL.Query().Get("id_token_hint")
	postLogoutRedirectURI := r.URL.Query().Get("post_logout_redirect_uri")
	state := r.URL.Query().Get("state")

	// Extract user from ID token hint if provided
	if idTokenHint != "" {
		claims, err := ValidateOIDCAccessToken(idTokenHint)
		if err == nil && httpDB != nil {
			// Revoke all refresh tokens for this user+client
			httpDB.RevokeAllRefreshTokens(claims.UserID, claims.ClientID)
		}
	}

	// Clear OIDC session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "oidc_session",
		Value:    "",
		Path:     "/oidc",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})

	// Redirect if post_logout_redirect_uri is provided
	if postLogoutRedirectURI != "" {
		// Validate redirect URI (basic check)
		if strings.HasPrefix(postLogoutRedirectURI, "http://") || strings.HasPrefix(postLogoutRedirectURI, "https://") {
			sep := "?"
			if strings.Contains(postLogoutRedirectURI, "?") {
				sep = "&"
			}
			redirectURL := postLogoutRedirectURI
			if state != "" {
				redirectURL += sep + "state=" + state
			}
			http.Redirect(w, r, redirectURL, http.StatusFound)
			return
		}
	}

	// Show logout confirmation
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`<!DOCTYPE html><html><head><meta charset="UTF-8"><title>Lavender — Выход</title>
<style>body{font-family:sans-serif;display:flex;justify-content:center;align-items:center;min-height:100vh;margin:0;background:#1a1a2e;color:#e0e0e0}
.card{background:#16213e;padding:40px;border-radius:12px;text-align:center}
h2{color:#a78bfa}</style></head>
<body><div class="card"><h2>Вы вышли из аккаунта</h2><p>Вы успешно вышли из Lavender.</p></div></body></html>`))
}

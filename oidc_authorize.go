package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"time"
)

// oidcAuthorizeParams holds parsed authorization request parameters
type oidcAuthorizeParams struct {
	ResponseType        string
	ClientID            string
	RedirectURI         string
	Scope               string
	State               string
	CodeChallenge       string
	CodeChallengeMethod string
	Nonce               string
	Prompt              string
	LoginHint           string
}

func parseAuthorizeParams(r *http.Request) (*oidcAuthorizeParams, error) {
	p := &oidcAuthorizeParams{
		ResponseType:        r.URL.Query().Get("response_type"),
		ClientID:            r.URL.Query().Get("client_id"),
		RedirectURI:         r.URL.Query().Get("redirect_uri"),
		Scope:               r.URL.Query().Get("scope"),
		State:               r.URL.Query().Get("state"),
		CodeChallenge:       r.URL.Query().Get("code_challenge"),
		CodeChallengeMethod: r.URL.Query().Get("code_challenge_method"),
		Nonce:               r.URL.Query().Get("nonce"),
		Prompt:              r.URL.Query().Get("prompt"),
		LoginHint:           r.URL.Query().Get("login_hint"),
	}

	if p.ResponseType != "code" {
		return nil, fmt.Errorf("unsupported response_type: %s", p.ResponseType)
	}
	if p.ClientID == "" {
		return nil, fmt.Errorf("client_id is required")
	}
	if p.RedirectURI == "" {
		return nil, fmt.Errorf("redirect_uri is required")
	}
	if p.State == "" {
		return nil, fmt.Errorf("state is required")
	}
	if p.CodeChallenge == "" {
		return nil, fmt.Errorf("code_challenge is required (PKCE)")
	}
	if p.CodeChallengeMethod != "S256" {
		return nil, fmt.Errorf("only S256 code_challenge_method is supported")
	}
	if !strings.Contains(p.Scope, "openid") {
		return nil, fmt.Errorf("scope must include 'openid'")
	}
	return p, nil
}

func oidcAuthorizeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		oidcError(w, "invalid_request", "method not allowed", http.StatusBadRequest)
		return
	}

	params, err := parseAuthorizeParams(r)
	if err != nil {
		oidcError(w, "invalid_request", err.Error(), http.StatusBadRequest)
		return
	}

	// Validate client
	db := httpDB
	if db == nil {
		oidcError(w, "server_error", "database not available", http.StatusInternalServerError)
		return
	}
	client, err := db.GetOAuthClient(params.ClientID)
	if err != nil || !client.IsActive {
		oidcErrorRedirect(w, r, params.RedirectURI, params.State, "unauthorized_client", "unknown or inactive client")
		return
	}

	// Validate redirect URI (exact match)
	validRedirect := false
	for _, uri := range client.RedirectURIs {
		if uri == params.RedirectURI {
			validRedirect = true
			break
		}
	}
	if !validRedirect {
		oidcError(w, "invalid_request", "redirect_uri not registered for this client", http.StatusBadRequest)
		return
	}

	// Check if user is logged in (via existing Lavender session cookie or query param)
	userID := getOIDCSessionUser(r)

	if params.Prompt == "none" && userID == "" {
		oidcErrorRedirect(w, r, params.RedirectURI, params.State, "login_required", "user not authenticated")
		return
	}

	if userID == "" || params.Prompt == "login" {
		// Show login form
		showOIDCLoginForm(w, r, params)
		return
	}

	// Check if consent is needed
	grant, _ := db.GetOAuthGrant(userID, params.ClientID)
	needsConsent := grant == nil || params.Prompt == "consent"

	if needsConsent {
		showOIDCConsentForm(w, r, params, userID, client)
		return
	}

	// Auto-approve: generate auth code
	code, err := generateAndStoreAuthCode(db, userID, params, false)
	if err != nil {
		oidcErrorRedirect(w, r, params.RedirectURI, params.State, "server_error", "failed to generate code")
		return
	}

	// Redirect with code
	redirectURL := fmt.Sprintf("%s?code=%s&state=%s", params.RedirectURI, code, params.State)
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

func generateAndStoreAuthCode(db *DB, userID string, params *oidcAuthorizeParams, isSSO bool) (string, error) {
	code, err := generateOpaqueToken(32)
	if err != nil {
		return "", err
	}
	codeHash := oidcHashToken(code)

	authCode := &OAuthAuthCode{
		CodeHash:            codeHash,
		UserID:              userID,
		ClientID:            params.ClientID,
		RedirectURI:         params.RedirectURI,
		Scope:               params.Scope,
		Nonce:               params.Nonce,
		CodeChallenge:       params.CodeChallenge,
		CodeChallengeMethod: params.CodeChallengeMethod,
		IsSSO:               isSSO,
		ExpiresAt:           timeNow().Add(10 * time.Minute),
	}

	if err := db.StoreAuthCode(authCode); err != nil {
		return "", err
	}
	return code, nil
}

func oidcConsentHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		oidcError(w, "invalid_request", "method not allowed", http.StatusBadRequest)
		return
	}

	// Parse form
	if err := r.ParseForm(); err != nil {
		oidcError(w, "invalid_request", "invalid form data", http.StatusBadRequest)
		return
	}

	state := r.FormValue("state")
	action := r.FormValue("action")

	db := httpDB
	if db == nil {
		oidcError(w, "server_error", "database not available", http.StatusInternalServerError)
		return
	}

	// Retrieve stored params from session (simplified: stored in form hidden fields)
	clientID := r.FormValue("client_id")
	redirectURI := r.FormValue("redirect_uri")
	scope := r.FormValue("scope")
	challenge := r.FormValue("code_challenge")
	challengeMethod := r.FormValue("code_challenge_method")
	nonce := r.FormValue("nonce")
	userID := getOIDCSessionUser(r)

	if userID == "" {
		oidcError(w, "invalid_request", "no session", http.StatusBadRequest)
		return
	}

	if action == "deny" {
		oidcErrorRedirect(w, r, redirectURI, state, "access_denied", "user denied consent")
		return
	}

	params := &oidcAuthorizeParams{
		ClientID:            clientID,
		RedirectURI:         redirectURI,
		Scope:               scope,
		State:               state,
		CodeChallenge:       challenge,
		CodeChallengeMethod: challengeMethod,
		Nonce:               nonce,
	}

	// Upsert grant
	db.UpsertOAuthGrant(userID, clientID, scope)

	// Generate auth code
	authCode, err := generateAndStoreAuthCode(db, userID, params, false)
	if err != nil {
		oidcErrorRedirect(w, r, redirectURI, state, "server_error", "failed to generate code")
		return
	}

	redirectURL := fmt.Sprintf("%s?code=%s&state=%s", redirectURI, authCode, state)
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

func getOIDCSessionUser(r *http.Request) string {
	// Check cookie
	cookie, err := r.Cookie("oidc_session")
	if err != nil || cookie.Value == "" {
		return ""
	}
	// In production, verify HMAC signature. For now, decode simple session
	// TODO: proper session validation
	return ""
}

// --- Login/Consent Form Rendering ---

var loginTemplate = template.Must(template.New("login").Parse(`
<!DOCTYPE html>
<html lang="ru">
<head><meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Lavender — Вход</title>
<style>
body{font-family:-apple-system,BlinkMacSystemFont,sans-serif;display:flex;justify-content:center;align-items:center;min-height:100vh;margin:0;background:#1a1a2e;color:#e0e0e0}
.card{background:#16213e;padding:40px;border-radius:12px;width:360px;box-shadow:0 8px 32px rgba(0,0,0,0.3)}
h2{margin:0 0 24px;text-align:center;color:#a78bfa}
input{width:100%;padding:12px;margin:8px 0;border:1px solid #334155;border-radius:8px;background:#0f172a;color:#e0e0e0;font-size:14px;box-sizing:border-box}
button{width:100%;padding:12px;margin:16px 0 0;border:none;border-radius:8px;background:#7c3aed;color:white;font-size:16px;cursor:pointer;font-weight:600}
button:hover{background:#6d28d9}
.error{color:#f87171;font-size:13px;text-align:center;margin-bottom:12px}
.app-name{text-align:center;font-size:13px;color:#64748b;margin-top:16px}
</style></head>
<body>
<div class="card">
<h2>Вход в Lavender</h2>
{{if .Error}}<div class="error">{{.Error}}</div>{{end}}
<form method="POST" action="/oidc/authorize/consent">
<input type="hidden" name="client_id" value="{{.ClientID}}">
<input type="hidden" name="redirect_uri" value="{{.RedirectURI}}">
<input type="hidden" name="scope" value="{{.Scope}}">
<input type="hidden" name="state" value="{{.State}}">
<input type="hidden" name="code_challenge" value="{{.CodeChallenge}}">
<input type="hidden" name="code_challenge_method" value="{{.CodeChallengeMethod}}">
<input type="hidden" name="nonce" value="{{.Nonce}}">
<input type="text" name="username" placeholder="Имя пользователя" required autofocus value="{{.LoginHint}}">
<input type="password" name="password" placeholder="Пароль" required>
<button type="submit" name="action" value="login">Войти</button>
</form>
<div class="app-name">{{.ClientName}}</div>
</div></body></html>
`))

var consentTemplate = template.Must(template.New("consent").Parse(`
<!DOCTYPE html>
<html lang="ru">
<head><meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Lavender — Доступ</title>
<style>
body{font-family:-apple-system,BlinkMacSystemFont,sans-serif;display:flex;justify-content:center;align-items:center;min-height:100vh;margin:0;background:#1a1a2e;color:#e0e0e0}
.card{background:#16213e;padding:40px;border-radius:12px;width:400px;box-shadow:0 8px 32px rgba(0,0,0,0.3)}
h2{margin:0 0 8px;text-align:center;color:#a78bfa}
.desc{text-align:center;color:#94a3b8;font-size:14px;margin-bottom:24px}
.scope-list{list-style:none;padding:0;margin:0 0 24px}
.scope-list li{padding:8px 12px;margin:4px 0;background:#0f172a;border-radius:6px;font-size:14px}
.btn-group{display:flex;gap:12px}
button{flex:1;padding:12px;border:none;border-radius:8px;font-size:15px;cursor:pointer;font-weight:600}
.btn-approve{background:#7c3aed;color:white}
.btn-approve:hover{background:#6d28d9}
.btn-deny{background:#334155;color:#94a3b8}
.btn-deny:hover{background:#475569}
</style></head>
<body>
<div class="card">
<h2>Запрос доступа</h2>
<p class="desc"><strong>{{.ClientName}}</strong> запрашивает доступ к вашему аккаунту</p>
<ul class="scope-list">
{{range .Scopes}}<li>{{.}}</li>{{end}}
</ul>
<form method="POST" action="/oidc/authorize/consent">
<input type="hidden" name="client_id" value="{{.ClientID}}">
<input type="hidden" name="redirect_uri" value="{{.RedirectURI}}">
<input type="hidden" name="scope" value="{{.Scope}}">
<input type="hidden" name="state" value="{{.State}}">
<input type="hidden" name="code_challenge" value="{{.CodeChallenge}}">
<input type="hidden" name="code_challenge_method" value="{{.CodeChallengeMethod}}">
<input type="hidden" name="nonce" value="{{.Nonce}}">
<div class="btn-group">
<button type="submit" name="action" value="deny" class="btn-deny">Отклонить</button>
<button type="submit" name="action" value="approve" class="btn-approve">Разрешить</button>
</div>
</form>
</div></body></html>
`))

func showOIDCLoginForm(w http.ResponseWriter, r *http.Request, params *oidcAuthorizeParams) {
	data := map[string]string{
		"ClientID":       params.ClientID,
		"RedirectURI":    params.RedirectURI,
		"Scope":          params.Scope,
		"State":          params.State,
		"CodeChallenge":  params.CodeChallenge,
		"CodeChallengeMethod": params.CodeChallengeMethod,
		"Nonce":          params.Nonce,
		"LoginHint":      params.LoginHint,
		"ClientName":     params.ClientID,
	}
	loginTemplate.Execute(w, data)
}

func showOIDCConsentForm(w http.ResponseWriter, r *http.Request, params *oidcAuthorizeParams, userID string, client *OAuthClient) {
	scopeDescriptions := map[string]string{
		"openid":          "Идентификация (ваш ID)",
		"profile":         "Имя пользователя и аватар",
		"email":           "Email адрес",
		"offline_access":  "Долгосрочный доступ",
		"read:profile":    "Чтение профиля",
		"read:messages":   "Чтение сообщений",
		"push:send":       "Отправка уведомлений",
	}
	var scopes []string
	for _, s := range strings.Split(params.Scope, " ") {
		if desc, ok := scopeDescriptions[s]; ok {
			scopes = append(scopes, fmt.Sprintf("%s — %s", s, desc))
		} else {
			scopes = append(scopes, s)
		}
	}

	data := map[string]interface{}{
		"ClientID":            params.ClientID,
		"ClientName":          client.ClientName,
		"RedirectURI":         params.RedirectURI,
		"Scope":               params.Scope,
		"State":               params.State,
		"CodeChallenge":       params.CodeChallenge,
		"CodeChallengeMethod": params.CodeChallengeMethod,
		"Nonce":               params.Nonce,
		"Scopes":              scopes,
	}
	consentTemplate.Execute(w, data)
}

func oidcError(w http.ResponseWriter, errCode, description string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{
		"error":             errCode,
		"error_description": description,
	})
}

func oidcErrorRedirect(w http.ResponseWriter, req *http.Request, redirectURI, state, errCode, description string) {
	sep := "?"
	if strings.Contains(redirectURI, "?") {
		sep = "&"
	}
	url := fmt.Sprintf("%s%serror=%s&error_description=%s&state=%s",
		redirectURI, sep, errCode, description, state)
	http.Redirect(w, req, url, http.StatusFound)
}

// computePKCEChallenge computes S256 code_challenge from code_verifier
func computePKCEChallenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

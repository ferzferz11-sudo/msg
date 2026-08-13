package auth

// auth/jwt.go — генерация и валидация JWT токенов для удалённых агентов
//
// Токены используются для аутентификации hermes-agent daemon при подключении
// к Orchestrator через HermesAgentService.Connect.
//
// Алгоритм: HS256 (HMAC-SHA256)
// Секрет: из переменной окружения JWT_SECRET (минимум 32 байта)
// Claims: agent_id, agent_name, capabilities, issued_at, expires_at

import (
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

// db is the database connection for token revocation (injected via SetDB)
var db *sql.DB

// AgentRevocationCheck is an optional hook called during ValidateAgentToken
// to check if an agent has been revoked in external systems (e.g. agent_tokens table).
// Set via SetAgentRevocationCheck.
var AgentRevocationCheck func(agentID string) (bool, error)

// SetDB sets the database connection for the auth package.
// Must be called before using RevokeToken or blacklist checks.
func SetDB(database *sql.DB) {
	db = database
}

// SetAgentRevocationCheck sets a hook that checks agent-level revocation.
// Called during ValidateAgentToken after blacklist check, before signature validation.
func SetAgentRevocationCheck(fn func(agentID string) (bool, error)) {
	AgentRevocationCheck = fn
}

// AgentClaims — данные внутри JWT токена агента
type AgentClaims struct {
	AgentID      string   `json:"agent_id"`
	AgentName    string   `json:"agent_name"`
	Capabilities []string `json:"capabilities,omitempty"`
	IssuedAt     int64    `json:"iat"`
	ExpiresAt    int64    `json:"exp"`
}

// AgentToken — полная информация о токене для хранения в БД
type AgentToken struct {
	ID           int64
	AgentID      string
	AgentName    string
	Token        string
	Capabilities []string
	CreatedAt    time.Time
	ExpiresAt    time.Time
	Revoked      bool
	CreatedBy    string
}

// getTokenSecret загружает секретный ключ из окружения
func getTokenSecret() []byte {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		log.Fatal("FATAL: JWT_SECRET environment variable is not set. Tokens cannot be generated securely.")
	}
	if len(secret) < 32 {
		log.Fatalf("FATAL: JWT_SECRET must be at least 32 bytes, got %d", len(secret))
	}
	return []byte(secret)
}

// base64URLEncode кодирует данные в base64url без padding
func base64URLEncode(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}

// base64URLDecode декодирует base64url без padding
func base64URLDecode(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}

// signHS256 создаёт HMAC-SHA256 подпись
func signHS256(data string, secret []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(data))
	return base64URLEncode(mac.Sum(nil))
}

// GenerateAgentToken генерирует JWT токен для агента
// agentID — уникальный идентификатор агента
// agentName — человекочитаемое имя
// capabilities — список возможностей (shell, git, build, etc.)
// ttl — время жизни токена (0 = 24 часа по умолчанию)
func GenerateAgentToken(agentID, agentName string, capabilities []string, ttl time.Duration) (string, error) {
	if agentID == "" {
		return "", fmt.Errorf("agent_id is required")
	}

	if ttl == 0 {
		ttl = 24 * time.Hour
	}

	now := time.Now()
	claims := AgentClaims{
		AgentID:      agentID,
		AgentName:    agentName,
		Capabilities: capabilities,
		IssuedAt:     now.Unix(),
		ExpiresAt:    now.Add(ttl).Unix(),
	}

	// Формируем JWT: header.payload.signature
	header := map[string]string{"alg": "HS256", "typ": "JWT"}
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", fmt.Errorf("marshal header: %w", err)
	}

	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("marshal claims: %w", err)
	}

	encodedHeader := base64URLEncode(headerJSON)
	encodedClaims := base64URLEncode(claimsJSON)
	signingInput := encodedHeader + "." + encodedClaims

	signature := signHS256(signingInput, getTokenSecret())

	token := signingInput + "." + signature
	return token, nil
}

// ValidateAgentToken проверяет JWT токен и возвращает claims
// Возвращает ошибку если токен невалидный, просроченный или подпись неверна
func ValidateAgentToken(token string) (*AgentClaims, error) {
	if token == "" {
		return nil, fmt.Errorf("empty token")
	}

	// Check blacklist before signature validation
	if revoked, err := isTokenRevoked(token); err == nil && revoked {
		return nil, fmt.Errorf("token has been revoked")
	}

	// Check agent-level revocation (e.g. agent_tokens.revoked flag)
	if AgentRevocationCheck != nil {
		if quickClaims, err := parseAgentClaims(token); err == nil {
			if revoked, err := AgentRevocationCheck(quickClaims.AgentID); err == nil && revoked {
				return nil, fmt.Errorf("agent %s has been revoked", quickClaims.AgentID)
			}
		}
	}

	// Разбираем на части
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid token format: expected 3 parts, got %d", len(parts))
	}

	encodedHeader, encodedClaims, encodedSignature := parts[0], parts[1], parts[2]

	// Проверяем подпись
	signingInput := encodedHeader + "." + encodedClaims
	expectedSig := signHS256(signingInput, getTokenSecret())
	if !hmac.Equal([]byte(encodedSignature), []byte(expectedSig)) {
		return nil, fmt.Errorf("invalid signature")
	}

	// Декодируем claims
	claimsBytes, err := base64URLDecode(encodedClaims)
	if err != nil {
		return nil, fmt.Errorf("decode claims: %w", err)
	}

	var claims AgentClaims
	if err := json.Unmarshal(claimsBytes, &claims); err != nil {
		return nil, fmt.Errorf("unmarshal claims: %w", err)
	}

	// Проверяем expiration
	if claims.ExpiresAt > 0 && time.Now().Unix() > claims.ExpiresAt {
		return nil, fmt.Errorf("token expired (exp=%d, now=%d)", claims.ExpiresAt, time.Now().Unix())
	}

	// Проверяем что алгоритм HS256
	headerBytes, err := base64URLDecode(encodedHeader)
	if err != nil {
		return nil, fmt.Errorf("decode header: %w", err)
	}
	var header map[string]string
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, fmt.Errorf("unmarshal header: %w", err)
	}
	if header["alg"] != "HS256" {
		return nil, fmt.Errorf("unsupported algorithm: %s", header["alg"])
	}

	return &claims, nil
}

// RevokeToken adds a token to the revoked_tokens blacklist.
// The token hash is stored with its expiry for automatic cleanup.
func RevokeToken(token string) error {
	if db == nil {
		return fmt.Errorf("database not initialized — call auth.SetDB first")
	}

	hash := sha256.Sum256([]byte(token))
	hashHex := hex.EncodeToString(hash[:])

	// Parse claims to extract expiry (reuse existing parser)
	claims, err := parseAgentClaims(token)
	if err != nil {
		return fmt.Errorf("parse token for revocation: %w", err)
	}

	expiresAt := time.Unix(claims.ExpiresAt, 0)

	_, err = db.Exec(`INSERT INTO revoked_tokens (token_hash, expires_at) VALUES ($1, $2) ON CONFLICT (token_hash) DO NOTHING`, hashHex, expiresAt)
	if err != nil {
		return fmt.Errorf("insert revoked token: %w", err)
	}
	return nil
}

// isTokenRevoked checks if a token hash exists in the revoked_tokens blacklist.
func isTokenRevoked(token string) (bool, error) {
	if db == nil {
		return false, nil // no DB = no blacklist
	}

	hash := sha256.Sum256([]byte(token))
	hashHex := hex.EncodeToString(hash[:])

	var exists bool
	err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM revoked_tokens WHERE token_hash = $1 AND expires_at > NOW())`, hashHex).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

// parseAgentClaims extracts AgentClaims from a JWT token without signature validation.
func parseAgentClaims(token string) (*AgentClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid token format")
	}

	claimsBytes, err := base64URLDecode(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decode claims: %w", err)
	}

	var claims AgentClaims
	if err := json.Unmarshal(claimsBytes, &claims); err != nil {
		return nil, fmt.Errorf("unmarshal claims: %w", err)
	}

	return &claims, nil
}

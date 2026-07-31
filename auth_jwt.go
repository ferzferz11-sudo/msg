package main

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	AccessTokenTTL  = 15 * time.Minute    // 15 minutes
	RefreshTokenTTL = 30 * 24 * time.Hour // 30 days
)

// authClaims defines the JWT claims for both access and refresh tokens
type authClaims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	DeviceID string `json:"device_id"`
	Type     string `json:"type"` // "access" or "refresh"
	jwt.RegisteredClaims
}

// getJWTSecret returns the JWT signing secret from env (cached, re-reads if env changes)
var (
	cachedJWTSecret    []byte
	cachedJWTSecretEnv string
	jwtSecretMu        sync.Mutex
)

func getJWTSecret() ([]byte, error) {
	jwtSecretMu.Lock()
	defer jwtSecretMu.Unlock()

	secret := os.Getenv("JWT_SECRET")
	if secret == cachedJWTSecretEnv && cachedJWTSecret != nil {
		return cachedJWTSecret, nil
	}
	if len(secret) < 32 {
		return nil, fmt.Errorf("JWT_SECRET must be at least 32 bytes, got %d", len(secret))
	}
	cachedJWTSecret = []byte(secret)
	cachedJWTSecretEnv = secret
	return cachedJWTSecret, nil
}

// GenerateTokenPair creates a new access + refresh token pair for a user+device
func GenerateTokenPair(userID, username, deviceID string) (accessToken, refreshToken string, accessExp, refreshExp time.Time, err error) {
	secret, err := getJWTSecret()
	if err != nil {
		return "", "", time.Time{}, time.Time{}, err
	}

	now := time.Now()

	// Access token
	accessExp = now.Add(AccessTokenTTL)
	accessClaims := authClaims{
		UserID:   userID,
		Username: username,
		DeviceID: deviceID,
		Type:     "access",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(accessExp),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        uuid.New().String(),
			Issuer:    "lavender-server",
			Audience:  jwt.ClaimStrings{"lavender-server"},
		},
	}
	accessTokenObj := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessToken, err = accessTokenObj.SignedString(secret)
	if err != nil {
		return "", "", time.Time{}, time.Time{}, fmt.Errorf("failed to sign access token: %w", err)
	}

	// Refresh token
	refreshExp = now.Add(RefreshTokenTTL)
	refreshJTI := uuid.New().String()
	refreshClaims := authClaims{
		UserID:   userID,
		Username: username,
		DeviceID: deviceID,
		Type:     "refresh",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(refreshExp),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        refreshJTI,
			Issuer:    "lavender-server",
			Audience:  jwt.ClaimStrings{"lavender-server"},
		},
	}
	refreshTokenObj := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshToken, err = refreshTokenObj.SignedString(secret)
	if err != nil {
		return "", "", time.Time{}, time.Time{}, fmt.Errorf("failed to sign refresh token: %w", err)
	}

	return accessToken, refreshToken, accessExp, refreshExp, nil
}

// ValidateToken parses and validates a JWT token string
// Returns the claims and nil error on success
func ValidateToken(tokenString string) (*authClaims, error) {
	secret, err := getJWTSecret()
	if err != nil {
		return nil, err
	}

	token, err := jwt.ParseWithClaims(tokenString, &authClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return secret, nil
	}, jwt.WithIssuer("lavender-server"), jwt.WithAudience("lavender-server"))
	if err != nil {
		return nil, fmt.Errorf("token validation failed: %w", err)
	}

	claims, ok := token.Claims.(*authClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	return claims, nil
}

// ExtractJTI extracts the JWT ID (jti) from a token without full validation
// Used for refresh token rotation — we need the JTI to check against DB
func ExtractJTI(tokenString string) (string, error) {
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, &authClaims{})
	if err != nil {
		return "", err
	}
	claims, ok := token.Claims.(*authClaims)
	if !ok {
		return "", fmt.Errorf("invalid claims")
	}
	return claims.ID, nil
}

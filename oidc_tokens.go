package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// OIDCClaims represents claims in OIDC tokens (RS256 signed)
type OIDCClaims struct {
	UserID   string `json:"sub"`
	ClientID string `json:"aud"`
	Issuer   string `json:"iss"`
	Nonce    string `json:"nonce,omitempty"`
	Email    string `json:"email,omitempty"`
	Username string `json:"preferred_username,omitempty"`
	Name     string `json:"name,omitempty"`
	Picture  string `json:"picture,omitempty"`
	Scope    string `json:"scope,omitempty"`
	jwt.RegisteredClaims
}

// GenerateIDToken creates an OIDC ID Token (RS256)
func GenerateIDToken(userID, clientID, nonce, email, username, name, avatarURL string) (string, error) {
	issuer := getOIDCIssuer()
	now := timeNow()

	claims := OIDCClaims{
		UserID:   userID,
		ClientID: clientID,
		Issuer:   issuer,
		Nonce:    nonce,
		Email:    email,
		Username: username,
		Name:     name,
		Picture:  avatarURL,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Subject:   userID,
			Audience:  jwt.ClaimStrings{clientID},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = oidcKID
	return token.SignedString(oidcPrivateKey)
}

// GenerateAccessToken creates an OIDC Access Token (RS256)
func GenerateAccessToken(userID, clientID, scope string) (string, string, time.Time, error) {
	issuer := getOIDCIssuer()
	now := timeNow()
	jti := uuid.New().String()

	claims := OIDCClaims{
		UserID:   userID,
		ClientID: clientID,
		Issuer:   issuer,
		Scope:    scope,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			Issuer:    issuer,
			Subject:   userID,
			Audience:  jwt.ClaimStrings{clientID},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = oidcKID
	signed, err := token.SignedString(oidcPrivateKey)
	if err != nil {
		return "", "", time.Time{}, err
	}
	return signed, jti, now.Add(time.Hour), nil
}

// GenerateOpaqueRefreshToken creates a random refresh token and its hash
func GenerateOpaqueRefreshToken() (token string, hash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", err
	}
	token = base64.RawURLEncoding.EncodeToString(b)
	h := sha256.Sum256([]byte(token))
	hash = fmt.Sprintf("%x", h)
	return token, hash, nil
}

// ValidateOIDCAccessToken validates an RS256 OIDC access token and returns claims
func ValidateOIDCAccessToken(tokenString string) (*OIDCClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &OIDCClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return oidcPublicKey, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*OIDCClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	return claims, nil
}

// ValidateOIDCRefreshToken validates an opaque refresh token (checks DB)
func ValidateOIDCRefreshToken(db *DB, tokenHash, clientID string) (*OAuthRefreshToken, error) {
	t, err := db.GetRefreshToken(tokenHash)
	if err != nil {
		return nil, fmt.Errorf("refresh token not found")
	}
	if t.IsRevoked {
		return nil, fmt.Errorf("refresh token revoked")
	}
	if t.ClientID != clientID {
		return nil, fmt.Errorf("client mismatch")
	}
	if timeNow().After(t.ExpiresAt) {
		return nil, fmt.Errorf("refresh token expired")
	}
	return t, nil
}

package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	oidcPrivateKey *rsa.PrivateKey
	oidcPublicKey  *rsa.PublicKey
	oidcKID        string
	oidcKeysOnce   sync.Once
)

type JWKSKey struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

type JWKS struct {
	Keys []JWKSKey `json:"keys"`
}

func initOIDCKeys() error {
	var initErr error
	oidcKeysOnce.Do(func() {
		keysDir := getEnvOrDefault("OIDC_KEYS_DIR", ".keys")
		if err := os.MkdirAll(keysDir, 0700); err != nil {
			initErr = fmt.Errorf("create keys dir: %w", err)
			return
		}

		privPath := filepath.Join(keysDir, "oidc-private.pem")
		pubPath := filepath.Join(keysDir, "oidc-public.pem")
		kidPath := filepath.Join(keysDir, "oidc-kid")

		// Try loading existing keys
		if data, err := os.ReadFile(privPath); err == nil {
			block, _ := pem.Decode(data)
			if block != nil {
				key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
				if err == nil {
					oidcPrivateKey = key
					oidcPublicKey = &key.PublicKey
				}
			}
		}

		// Generate new keys if loading failed
		if oidcPrivateKey == nil {
			key, err := rsa.GenerateKey(rand.Reader, 2048)
			if err != nil {
				initErr = fmt.Errorf("generate RSA key: %w", err)
				return
			}
			oidcPrivateKey = key
			oidcPublicKey = &key.PublicKey

			// Save private key
			privFile, err := os.Create(privPath)
			if err != nil {
				initErr = fmt.Errorf("create priv key file: %w", err)
				return
			}
			pem.Encode(privFile, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
			privFile.Close()
			os.Chmod(privPath, 0600)

			// Save public key
			pubFile, err := os.Create(pubPath)
			if err != nil {
				initErr = fmt.Errorf("create pub key file: %w", err)
				return
			}
			pubBytes, _ := x509.MarshalPKIXPublicKey(&key.PublicKey)
			pem.Encode(pubFile, &pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes})
			pubFile.Close()

			logger.Info("Generated new OIDC RSA key pair")
		}

		// Load or generate KID
		if data, err := os.ReadFile(kidPath); err == nil && len(data) > 0 {
			oidcKID = string(data)
		} else {
			b := make([]byte, 16)
			rand.Read(b)
			oidcKID = fmt.Sprintf("%x", b)
			os.WriteFile(kidPath, []byte(oidcKID), 0600)
		}
	})
	return initErr
}

func getOIDCPublicKeyJWKS() JWKS {
	e := big.NewInt(int64(oidcPublicKey.E))
	return JWKS{
		Keys: []JWKSKey{{
			Kty: "RSA",
			Kid: oidcKID,
			Use: "sig",
			Alg: "RS256",
			N:   base64.RawURLEncoding.EncodeToString(oidcPublicKey.N.Bytes()),
			E:   base64.RawURLEncoding.EncodeToString(e.Bytes()),
		}},
	}
}

func oidcJWKSHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "max-age=3600")
	json.NewEncoder(w).Encode(getOIDCPublicKeyJWKS())
}

// generateOpaqueToken generates a cryptographically random opaque token
func generateOpaqueToken(byteLength int) (string, error) {
	b := make([]byte, byteLength)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// OIDCDiscoveryDocument represents the OpenID Connect discovery metadata
type OIDCDiscoveryDocument struct {
	Issuer                           string   `json:"issuer"`
	AuthorizationEndpoint            string   `json:"authorization_endpoint"`
	TokenEndpoint                    string   `json:"token_endpoint"`
	UserinfoEndpoint                 string   `json:"userinfo_endpoint"`
	JWKSURI                          string   `json:"jwks_uri"`
	RevocationEndpoint               string   `json:"revocation_endpoint"`
	IntrospectionEndpoint            string   `json:"introspection_endpoint"`
	EndSessionEndpoint               string   `json:"end_session_endpoint"`
	ResponseTypesSupported           []string `json:"response_types_supported"`
	SubjectTypesSupported            []string `json:"subject_types_supported"`
	IDTokenSigningAlgValuesSupported []string `json:"id_token_signing_alg_values_supported"`
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported"`
	ScopesSupported                  []string `json:"scopes_supported"`
	ClaimsSupported                  []string `json:"claims_supported"`
	CodeChallengeMethodsSupported    []string `json:"code_challenge_methods_supported"`
	GrantTypesSupported              []string `json:"grant_types_supported"`
	ClaimsParameterSupported         bool     `json:"claims_parameter_supported"`
}

func getOIDCIssuer() string {
	return getEnvOrDefault("OIDC_ISSUER_URL", "http://localhost:8082")
}

func oidcDiscoveryHandler(w http.ResponseWriter, r *http.Request) {
	issuer := getOIDCIssuer()
	doc := OIDCDiscoveryDocument{
		Issuer:                           issuer,
		AuthorizationEndpoint:            issuer + "/oidc/authorize",
		TokenEndpoint:                    issuer + "/oidc/token",
		UserinfoEndpoint:                 issuer + "/oidc/userinfo",
		JWKSURI:                          issuer + "/.well-known/jwks.json",
		RevocationEndpoint:               issuer + "/oidc/revoke",
		IntrospectionEndpoint:            issuer + "/oidc/introspect",
		EndSessionEndpoint:               issuer + "/oidc/logout",
		ResponseTypesSupported:           []string{"code"},
		SubjectTypesSupported:            []string{"public"},
		IDTokenSigningAlgValuesSupported: []string{"RS256"},
		TokenEndpointAuthMethodsSupported: []string{"client_secret_basic", "client_secret_post", "none"},
		ScopesSupported:                  []string{"openid", "profile", "email", "offline_access", "read:profile", "read:messages", "push:send"},
		ClaimsSupported:                  []string{"sub", "iss", "aud", "exp", "iat", "nonce", "email", "email_verified", "preferred_username", "name", "picture"},
		CodeChallengeMethodsSupported:    []string{"S256"},
		GrantTypesSupported:              []string{"authorization_code", "refresh_token"},
		ClaimsParameterSupported:         true,
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "max-age=3600")
	json.NewEncoder(w).Encode(doc)
}

// timeNow is a package-level function for testing overrides
var timeNow = time.Now

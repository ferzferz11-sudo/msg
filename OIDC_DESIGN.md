# OIDC SSO System Design — Lavender Messenger

**Version:** 1.0  
**Date:** 2026-07-21  
**Status:** Design Document

---

## Table of Contents

1. [Architecture Overview](#1-architecture-overview)
2. [OIDC Provider Implementation](#2-oidc-provider-implementation)
3. [SSO Flow Design](#3-sso-flow-design)
4. [New App Architecture](#4-new-app-architecture)
5. [Database Schema](#5-database-schema)
6. [Security Considerations](#6-security-considerations)
7. [API Specification](#7-api-specification)
8. [Implementation Plan](#8-implementation-plan)
9. [Edge Cases](#9-edge-cases)

---

## 1. Architecture Overview

### Design Principles

- **Additive only**: No changes to existing auth flows. The OIDC system is layered on top.
- **Dual JWT strategy**: Keep HS256 for internal Lavender auth; use RS256 for OIDC tokens so relying parties can verify without the signing secret.
- **PKCE mandatory**: All authorization code flows require PKCE (no exceptions for public clients).
- **Redis for ephemeral state**: Authorization codes, OIDC session state, and nonce verification go in Redis with TTL. PostgreSQL stores durable records (clients, grants, audit).
- **Device-aware**: OIDC tokens are tied to an OAuth client registration, not a device. The Lavender "device" concept is orthogonal — an OIDC session maps a user + client + browser/app context.

### System Context

```
┌─────────────────────┐     ┌──────────────────────────┐
│  Lavender Messenger │     │  Relying Party (RP)      │
│  (OIDC Provider)    │◄────│  - Android Kotlin App     │
│                     │     │  - React/Next.js Web App  │
│  Existing Auth:     │     │  - Future apps            │
│  - HS256 JWT        │     └──────────────────────────┘
│  - Device mgmt      │
│  - Rate limiting    │
│                     │     ┌──────────────────────────┐
│  New OIDC Layer:    │     │  Lavender Messenger App   │
│  - RS256 JWT        │────►│  (Existing Android/iOS)   │
│  - Auth codes       │     │  SSO via Intent/DeepLink  │
│  - Client mgmt      │     └──────────────────────────┘
│  - PKCE             │
└─────────────────────┘
```

### Token Strategy

| Token Type | Signing | Audience | Lifetime | Purpose |
|---|---|---|---|---|
| Internal Lavender JWT | HS256 | Lavender server | 15min / 30d | Existing auth (unchanged) |
| OIDC Access Token | RS256 | RP client_id | 1 hour | API access at RP |
| OIDC ID Token | RS256 | RP client_id | 1 hour | User identity assertion |
| OIDC Refresh Token | opaque | RP client_id | 30 days | Token renewal |
| Authorization Code | opaque | N/A | 10 min | One-time code exchange |

### RS256 Key Management

The server generates an RSA 2048-bit key pair on first startup (or if keys are missing). Keys are stored at:

```
.keys/oidc-private.pem   (RS256 signing, never leaves server)
.keys/oidc-public.pem    (served via JWKS endpoint)
.keys/oidc-kid           (key ID, UUID rotated on key change)
```

Key rotation: generate new key pair, publish both old + new in JWKS for a transition period (48 hours), then remove old key.

---

## 2. OIDC Provider Implementation

### 2.1 New HTTP Endpoints

All OIDC endpoints are added to the existing HTTP server on port 8082. They are **unauthenticated** (except UserInfo and Token with refresh grant).

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/.well-known/openid-configuration` | No | Discovery document |
| GET | `/.well-known/jwks.json` | No | JSON Web Key Set |
| GET | `/oidc/authorize` | No | Authorization endpoint (redirects to consent/login) |
| POST | `/oidc/authorize/consent` | OIDC session cookie | User submits consent |
| POST | `/oidc/token` | No | Token exchange |
| GET | `/oidc/userinfo` | Bearer (OIDC AT) | Returns claims |
| GET | `/oidc/logout` | OIDC session cookie | Ends OIDC session |
| POST | `/oidc/revoke` | Basic auth or body auth | Revokes refresh tokens |
| POST | `/oidc/introspect` | Basic auth | Token introspection (RFC 7662) |
| GET | `/oidc/sso-check` | No | Deep link handler for SSO |
| POST | `/oidc/sso-exchange` | Bearer (Lavender JWT) | Exchange Lavender session for OIDC tokens |

### 2.2 Discovery Document

```
GET /.well-known/openid-configuration
```

```json
{
  "issuer": "https://messenger.lavenderapp.com",
  "authorization_endpoint": "https://messenger.lavenderapp.com/oidc/authorize",
  "token_endpoint": "https://messenger.lavenderapp.com/oidc/token",
  "userinfo_endpoint": "https://messenger.lavenderapp.com/oidc/userinfo",
  "jwks_uri": "https://messenger.lavenderapp.com/.well-known/jwks.json",
  "revocation_endpoint": "https://messenger.lavenderapp.com/oidc/revoke",
  "introspection_endpoint": "https://messenger.lavenderapp.com/oidc/introspect",
  "end_session_endpoint": "https://messenger.lavenderapp.com/oidc/logout",
  "response_types_supported": ["code"],
  "subject_types_supported": ["public"],
  "id_token_signing_alg_values_supported": ["RS256"],
  "token_endpoint_auth_methods_supported": ["client_secret_basic", "client_secret_post", "none"],
  "scopes_supported": ["openid", "profile", "email", "offline_access", "read:profile", "read:messages", "push:send"],
  "claims_supported": ["sub", "iss", "aud", "exp", "iat", "nonce", "email", "email_verified", "preferred_username", "name", "picture"],
  "code_challenge_methods_supported": ["S256"],
  "grant_types_supported": ["authorization_code", "refresh_token"],
  "claims_parameter_supported": true,
  "service_documentation": "https://docs.lavenderapp.com/oidc"
}
```

**Implementation note:** The `issuer` field must exactly match the URL used in requests. Use `ISSUER_URL` env var (default: `http://localhost:8082` for dev).

### 2.3 JWKS Endpoint

```
GET /.well-known/jwks.json
```

```json
{
  "keys": [
    {
      "kty": "RSA",
      "kid": "<key-id>",
      "use": "sig",
      "alg": "RS256",
      "n": "<modulus-base64url>",
      "e": "<exponent-base64url>"
  }
}
```

### 2.4 Authorization Endpoint

```
GET /oidc/authorize?response_type=code&client_id=...&redirect_uri=...&scope=openid+profile&state=...&code_challenge=...&code_challenge_method=S256&nonce=...
```

**Request parameters:**

| Parameter | Required | Description |
|---|---|---|
| `response_type` | Yes | Must be `code` |
| `client_id` | Yes | Registered client identifier |
| `redirect_uri` | Yes | Must match a registered URI exactly |
| `scope` | Yes | Space-delimited scopes; must include `openid` |
| `state` | Yes | Opaque value returned to RP for CSRF protection |
| `code_challenge` | Yes | PKCE challenge (base64url-encoded SHA-256 of `code_verifier`) |
| `code_challenge_method` | Yes | Must be `S256` |
| `nonce` | Recommended | Value included in ID token |
| `prompt` | No | `none` (error if not logged in), `login` (force re-auth), `consent` (force consent) |
| `login_hint` | No | Pre-fill username |
| `max_age` | No | Max seconds since last authentication |
| `prompt` | No | `none`, `login`, `consent` |

**Processing flow:**

1. Validate `client_id` exists and is active.
2. Validate `redirect_uri` is an exact match against registered URIs.
3. Validate `response_type=code`, `code_challenge_method=S256`.
4. Check if user has an active OIDC session cookie (see §2.4.1).
5. If no session or `prompt=login`: render login form or redirect to RP with error.
6. If session exists and `prompt=consent` or first time for this client: render consent page.
7. Generate authorization code (256-bit random, base64url), store in Redis with:
   - `code` → `{user_id, client_id, redirect_uri, scope, nonce, code_challenge, expires_at, code_challenge_method}`
   - TTL: 10 minutes
8. Store in `oauth_grants` table: `{user_id, client_id, scope, granted_at}` for consent tracking.
9. Redirect to `redirect_uri?code=...&state=...`

#### 2.4.1 OIDC Session Management

During the authorization flow, the server manages a short-lived session via a signed, HttpOnly, Secure cookie:

```
Set-Cookie: oidc_session=<signed-token>; Path=/oidc; HttpOnly; Secure; SameSite=Lax; Max-Age=600
```

The session token contains: `{session_id, user_id, ip, nonce, created_at}`. It's signed with a separate HMAC key derived from `OIDC_SESSION_SECRET` env var. Max lifetime: 10 minutes. This is NOT a long-lived session — it only covers the auth redirect flow.

**If user already has an active Lavender session** (access token valid), skip login form entirely — go straight to consent (or auto-consent for previously approved clients).

### 2.5 Token Endpoint

```
POST /oidc/token
```

**Content-Type:** `application/x-www-form-urlencoded`

#### Authorization Code Exchange

```
grant_type=authorization_code
code=<auth_code>
redirect_uri=<redirect_uri>
client_id=<client_id>
code_verifier=<code_verifier>
```

or with client_secret (confidential clients):

```
Authorization: Basic base64(client_id:client_secret)
```

**Processing:**

1. Authenticate client (client_secret_basic, client_secret_post, or public client with PKCE only).
2. Look up auth code in Redis. Delete immediately (one-time use).
3. Validate: not expired, redirect_uri matches, client_id matches.
4. Validate PKCE: `SHA256(code_verifier) == stored code_challenge`.
5. Generate OIDC tokens (see §2.5.1).
6. Store refresh token hash in `oauth_refresh_tokens` table.
7. Return token response.

**Error on code reuse:** If code not found in Redis, check `oauth_auth_codes` table for audit. If the code was already used, revoke all tokens for this grant (token replay protection).

#### 2.5.1 Token Generation

**ID Token** (JWT, RS256):
```json
{
  "iss": "https://messenger.lavenderapp.com",
  "sub": "user-uuid",
  "aud": "client_id",
  "exp": 3600,
  "iat": 1721568000,
  "nonce": "from-authorization-request",
  "email": "user@example.com",
  "email_verified": true,
  "preferred_username": "alice",
  "name": "Alice",
  "picture": "https://messenger.lavenderapp.com/avatars/user-uuid/avatar.jpg"
}
```

**Access Token** (JWT, RS256):
```json
{
  "iss": "https://messenger.lavenderapp.com",
  "sub": "user-uuid",
  "aud": "client_id",
  "exp": 3600,
  "iat": 1721568000,
  "scope": "openid profile email",
  "client_id": "my-android-app",
  "jti": "unique-token-id"
}
```

**Refresh Token:** Opaque 256-bit random value, stored as SHA-256 hash in DB.

**Token response:**
```json
{
  "access_token": "eyJhbGciOiJSUzI1NiIs...",
  "token_type": "Bearer",
  "expires_in": 3600,
  "refresh_token": "dGhpcyBpcyBhIHJlZnJlc2ggdG9rZW4...",
  "id_token": "eyJhbGciOiJSUzI1NiIs...",
  "scope": "openid profile email"
}
```

#### Refresh Token Grant

```
grant_type=refresh_token
refresh_token=<token>
client_id=<client_id>
scope=<optional_new_scope>   // must be subset of original grant
```

1. Authenticate client.
2. Hash the refresh token, look up in `oauth_refresh_tokens`.
3. Validate: not expired, not revoked, client matches.
4. Rotate: issue new access + refresh token, revoke old refresh token.
5. If refresh token reuse detected (old token used after rotation): revoke ALL refresh tokens for this client+user grant (security measure).

### 2.6 UserInfo Endpoint

```
GET /oidc/userinfo
Authorization: Bearer <access_token>
```

**Response:**
```json
{
  "sub": "user-uuid",
  "name": "Alice",
  "preferred_username": "alice",
  "email": "alice@example.com",
  "email_verified": true,
  "picture": "https://messenger.lavenderapp.com/avatars/user-uuid/avatar.jpg"
}
```

Claims returned depend on the scopes in the access token:

| Scope | Claims |
|---|---|
| `openid` | `sub` |
| `profile` | `name`, `preferred_username`, `picture` |
| `email` | `email`, `email_verified` |
| `read:profile` | (same as profile, for API delegation) |
| `read:messages` | (no additional claims; scope used for authorization) |
| `push:send` | (no additional claims; scope used for authorization) |

### 2.7 Token Revocation Endpoint

```
POST /oidc/revoke
Content-Type: application/x-www-form-urlencoded

token=<refresh_token_or_access_token>
token_type_hint=refresh_token
```

Authentication: `client_secret_basic` or `client_secret_post`.

**Processing:**
1. Authenticate client.
2. If hint is `refresh_token`: hash and revoke in `oauth_refresh_tokens`.
3. If hint is `access_token` or no hint: look up JTI in `oauth_access_tokens` and mark revoked.
4. Always return 200 (per RFC 7009, even if token was invalid).

### 2.8 Token Introspection Endpoint

```
POST /oidc/introspect
Content-Type: application/x-www-form-urlencoded

token=<token>
token_type_hint=access_token
```

Authentication: `client_secret_basic`.

**Response (active):**
```json
{
  "active": true,
  "sub": "user-uuid",
  "client_id": "my-android-app",
  "scope": "openid profile email",
  "exp": 1721571600,
  "iat": 1721568000,
  "iss": "https://messenger.lavenderapp.com",
  "token_type": "Bearer"
}
```

**Response (inactive):**
```json
{ "active": false }
```

### 2.9 Logout Endpoint

```
GET /oidc/logout?id_token_hint=<id_token>&post_logout_redirect_uri=<uri>&state=<state>
```

1. Validate `id_token_hint` (extract `sub` and `aud`).
2. Revoke all refresh tokens for this client+user.
3. Clear OIDC session cookie.
4. Redirect to `post_logout_redirect_uri` (must be registered for the client) with `?state=<state>`.

### 2.10 Client Registration

**Initial approach:** Admin-managed via a database seed script and a protected admin endpoint.

**Admin endpoint** (protected by internal auth — Lavender admin only):

```
POST /oidc/admin/clients
Authorization: Bearer <lavender_admin_token>

{
  "client_name": "My Android App",
  "client_type": "public",
  "redirect_uris": [
    "com.lavender.myapp:/callback",
    "https://myapp.lavenderapp.com/auth/callback"
  ],
  "allowed_scopes": ["openid", "profile", "email", "read:profile"],
  "token_endpoint_auth_method": "none",
  "grant_types": ["authorization_code", "refresh_token"]
}
```

**Response:**
```json
{
  "client_id": "generated-uuid",
  "client_secret": null,     // null for public clients
  "client_name": "My Android App",
  "client_type": "public",
  "redirect_uris": [...],
  "allowed_scopes": [...],
  "created_at": "2026-07-21T10:00:00Z"
}
```

For **confidential** clients (server-side web apps), a `client_secret` is generated and returned once.

**Management endpoints:**

```
GET    /oidc/admin/clients/:client_id       - Get client details
PUT    /oidc/admin/clients/:client_id       - Update client
DELETE /oidc/admin/clients/:client_id       - Deactivate client
POST   /oidc/admin/clients/:client_id/rotate-secret - Rotate secret
GET    /oidc/admin/clients                  - List all clients
```

---

## 3. SSO Flow Design

### 3.1 SSO Detection and Flow

#### Android: Intent-based Detection

The new Android app checks if Lavender is installed:

```kotlin
// Check if Lavender Messenger is installed
val lavenderPackage = "com.lavender.messenger"
val isLavenderInstalled = try {
    packageManager.getPackageInfo(lavenderPackage, 0)
    true
} catch (e: PackageManager.NameNotFoundException) {
    false
}
```

**SSO Flow (Lavender installed):**

```
┌──────────────┐     ┌──────────────────┐     ┌──────────────────┐
│ New App      │     │ Lavender App     │     │ Lavender Server  │
│ (RP)         │     │ (via Intent)     │     │ (OIDC Provider)  │
└──────┬───────┘     └────────┬─────────┘     └────────┬─────────┘
       │                      │                        │
       │  1. Check Lavender   │                        │
       │  installed           │                        │
       │─────────────────────►│                        │
       │                      │                        │
       │  2. Lavender found   │                        │
       │  Send SSO Intent     │                        │
       │  with:               │                        │
       │  - client_id         │                        │
       │  - code_challenge    │                        │
       │  - state             │                        │
       │  - redirect_uri      │                        │
       │─────────────────────►│                        │
       │                      │                        │
       │                      │  3. Lavender validates │
       │                      │  user session (local   │
       │                      │  JWT or refresh token) │
       │                      │                        │
       │                      │  4. Calls POST         │
       │                      │  /oidc/sso-exchange    │
       │                      │  with:                 │
       │                      │  - Lavender JWT        │
       │                      │  - client_id           │
       │                      │  - code_challenge      │
       │                      │───────────────────────►│
       │                      │                        │
       │                      │  5. Server validates    │
       │                      │  Lavender JWT, checks   │
       │                      │  client is authorized   │
       │                      │  for SSO, generates     │
       │                      │  auth code              │
       │                      │◄───────────────────────│
       │                      │  returns auth_code      │
       │                      │                        │
       │  6. Lavender returns │                        │
       │  auth_code via Intent│                        │
       │  result              │                        │
       │◄─────────────────────│                        │
       │                      │                        │
       │  7. New app exchanges│                        │
       │  auth_code for tokens│                        │
       │  POST /oidc/token    │                        │
       │─────────────────────────────────────────────►│
       │                      │                        │
       │  8. Returns OIDC     │                        │
       │  tokens              │                        │
       │◄─────────────────────────────────────────────│
```

**SSO Check Intent:**

```kotlin
// In new app
val intent = Intent("com.lavender.messenger.SSO_CHECK").apply {
    setPackage("com.lavender.messenger")
    putExtra("client_id", OIDC_CLIENT_ID)
    putExtra("code_challenge", pkceChallenge)
    putExtra("code_challenge_method", "S256")
    putExtra("state", state)
    putExtra("scope", "openid profile email")
    putExtra("redirect_uri", "com.newapp:/sso-callback")
}
startActivityForResult(intent, SSO_REQUEST_CODE)
```

**Lavender app receives and handles:**
- Checks if user is logged in locally
- If logged in: calls `/oidc/sso-exchange` endpoint, returns auth code via `setResult(RESULT_OK, intent)`
- If not logged in: returns `RESULT_CANCELED` with error

**New app falls back to credential mode if:**
- Lavender not installed
- Intent fails / timeout (2 seconds)
- User not logged in on Lavender
- Error response from Lavender

#### Web: Deep Link Flow

For web apps, SSO detection uses a redirect-based flow:

```
┌──────────────┐     ┌──────────────────┐     ┌──────────────────┐
│ Browser      │     │ Lavender Web     │     │ Lavender Server  │
│ (New App)    │     │ (if open)        │     │ (OIDC Provider)  │
└──────┬───────┘     └────────┬─────────┘     └────────┬─────────┘
       │                      │                        │
       │  1. New app checks   │                        │
       │  if user has active  │                        │
       │  Lavender session    │                        │
       │  (try GET /health    │                        │
       │  with credentials)   │                        │
       │─────────────────────────────────────────────►│
       │                      │                        │
       │  2a. If session valid: use SSO flow           │
       │  2b. If not: show login form                 │
       │◄─────────────────────────────────────────────│
```

**Web SSO Flow (simplified):**

1. New app redirects to Lavender's `/oidc/sso-check` endpoint with RP's OIDC parameters.
2. Lavender checks if user has an active session (via existing Lavender cookie/token).
3. If session exists: generate auth code, redirect back to RP with `?code=...&state=...`
4. If no session: show Lavender login page, then generate auth code after login.

```
GET /oidc/sso-check?
  client_id=my-web-app&
  redirect_uri=https://myapp.com/auth/callback&
  scope=openid+profile&
  state=xyz&
  code_challenge=abc123&
  code_challenge_method=S256
```

### 3.2 Session Bridging: Lavender JWT → OIDC Tokens

The SSO exchange endpoint bridges existing Lavender sessions to OIDC tokens:

```
POST /oidc/sso-exchange
Content-Type: application/json

{
  "client_id": "my-android-app",
  "code_challenge": "abc123...",
  "code_challenge_method": "S256",
  "scope": "openid profile email",
  "state": "xyz",
  "lavender_token": "existing-lavender-jwt"
}
```

**Processing:**
1. Validate the Lavender JWT (HS256, existing `ValidateToken()`).
2. Verify the user exists and is active.
3. Validate `client_id` exists and is authorized for SSO.
4. Validate `redirect_uri` is registered for this client (or use a pre-registered SSO redirect URI).
5. Generate authorization code with the user's ID, store PKCE challenge.
6. Return `{ "code": "...", "state": "..." }` (not a redirect — the Lavender app handles the redirect back).

### 3.3 SSO Redirect URI Convention

Each RP registers a "canonical" SSO redirect URI:

| Platform | URI Pattern |
|---|---|
| Android | `com.newapp.sso:/callback` (custom scheme) |
| Web | `https://myapp.com/auth/callback` |
| iOS (future) | `com.newapp.ios:/callback` |

---

## 4. New App Architecture

### 4.1 Android Kotlin Integration

**Library:** [AppAuth for Android](https://github.com/openid/AppAuth-Android) (net.openid.appauth)

**Setup:**

```kotlin
// AuthService.kt
object LavenderAuthService {
    private const val ISSUER = "https://messenger.lavenderapp.com"
    private const val CLIENT_ID = "my-android-app"
    private const val REDIRECT_URI = "com.newapp:/callback"
    private const val SCOPE = "openid profile email offline_access"

    private var authState: AuthState = AuthState()

    // Discover Lavender OIDC endpoints
    suspend fun discoverConfig(): AuthorizationServiceConfiguration {
        return AuthorizationServiceConfiguration.fetchFromIssuer(
            ISSUER.toUri()
        ) { config, ex ->
            // Config cached by AppAuth
        }
    }

    // SSO Flow: check Lavender installed, use Intent
    suspend fun ssoLogin(activity: Activity): AuthState {
        // 1. Check if Lavender is installed
        val lavenderInstalled = isLavenderInstalled(activity)
        if (!lavenderInstalled) return credentialLogin(activity)

        // 2. Generate PKCE
        val request = AuthorizationRequest.Builder(
            config, CLIENT_ID, ResponseTypeValues.CODE, REDIRECT_URI.toUri()
        )
            .setScope(SCOPE)
            .setCodeVerifier(pkceVerifier, pkceChallenge)
            .build()

        // 3. Send SSO Intent to Lavender
        val intent = Intent("com.lavender.messenger.SSO_CHECK").apply {
            setPackage("com.lavender.messenger")
            putExtra("authorization_request", request.jsonSerializeString())
        }

        // 4. Handle response
        val result = activity.startActivityForResult(intent, SSO_REQUEST_CODE)
        // ... parse result, exchange code for tokens
    }

    // Credential Flow: login form, then OIDC authorize
    suspend fun credentialLogin(activity: Activity): AuthState {
        val request = AuthorizationRequest.Builder(
            config, CLIENT_ID, ResponseTypeValues.CODE, REDIRECT_URI.toUri()
        )
            .setScope(SCOPE)
            .setCodeVerifier(pkceVerifier, pkceChallenge)
            .build()

        // Show custom login UI or use AppAuth browser flow
        // ...
    }
}
```

**Token Storage:**

```kotlin
// TokenStore.kt - EncryptedSharedPreferences
class TokenStore(context: Context) {
    private val prefs = EncryptedSharedPreferences.create(
        "lavender_tokens",
        MasterKey.Builder(context).setKeyScheme(MasterKey.KeyScheme.AES256_GCM).build(),
        EncryptedSharedPreferences.PrefKeyEncryptionScheme.AES256_SIV,
        EncryptedSharedPreferences.PrefValueEncryptionScheme.AES256_GCM
    )

    fun saveAuthState(authState: AuthState) {
        prefs.edit().putString("auth_state", authState.jsonSerializeString()).apply()
    }

    fun loadAuthState(): AuthState? {
        val json = prefs.getString("auth_state", null) ?: return null
        return AuthState.jsonDeserialize(json)
    }
}
```

**Token Refresh:**

```kotlin
// Auto-refresh using AppAuth's performActionWithFreshTokens
authService.performActionWithFreshTokens(authState) { authState, ex ->
    if (ex != null) {
        // Token refresh failed, re-authenticate
        handleAuthError(ex)
    } else {
        // Use authState.accessToken for API calls
        apiCall(authState.accessToken)
    }
}
```

### 4.2 React/Next.js Integration

**Library:** [next-auth](https://next-auth.js.org/) with a custom OIDC provider, or [@auth/core](https://authjs.dev/) with the OIDC provider.

**Simpler approach:** Use `openid-client` (oidc-client-ts) directly for maximum control.

```typescript
// lib/auth/lavender-provider.ts
import { generators, Issuer } from 'openid-client';

const issuer = await Issuer.discover('https://messenger.lavenderapp.com');

const client = new issuer.Client({
  client_id: process.env.LAVENDER_CLIENT_ID!,
  client_secret: process.env.LAVENDER_CLIENT_SECRET,  // null for public clients
  redirect_uris: [process.env.REDIRECT_URI!],
  response_types: ['code'],
});

export function getAuthorizationUrl(state: string, codeChallenge: string) {
  return client.authorizationUrl({
    scope: 'openid profile email offline_access',
    state,
    code_challenge: codeChallenge,
    code_challenge_method: 'S256',
    nonce: generators.nonce(),
  });
}

export async function exchangeCode(params: CallbackParams) {
  const tokenSet = await client.callback(process.env.REDIRECT_URI!, params);
  return tokenSet;
}

export async function refreshTokens(refreshToken: string) {
  const tokenSet = await client.refresh(refreshToken);
  return tokenSet;
}
```

**Next.js Route Handlers:**

```typescript
// app/api/auth/[...nextauth]/route.ts (simplified)
import { getAuthorizationUrl } from '@/lib/auth/lavender-provider';

// GET /api/auth/login - initiates OIDC flow
// GET /api/auth/callback - handles code exchange
// POST /api/auth/refresh - refreshes tokens
// POST /api/auth/logout - ends session
```

**Token Storage:** HTTP-only cookies (via Next.js cookie helpers or next-auth).

**Session Management:** Server-side session via `next-auth` session or a lightweight Redis session store.

### 4.3 Token Refresh Strategy

Both apps implement:

1. **Proactive refresh:** 5 minutes before access token expiry, attempt refresh.
2. **Reactive refresh:** If API returns 401, attempt refresh once, retry request.
3. **Refresh token rotation:** Server issues new refresh token on each use; old one is invalidated.
4. **Graceful degradation:** If refresh fails, redirect to login. No lockout.

---

## 5. Database Schema

### 5.1 New Tables

#### `oauth_clients`

```sql
CREATE TABLE IF NOT EXISTS oauth_clients (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id VARCHAR(255) UNIQUE NOT NULL,
    client_secret_hash VARCHAR(255),           -- NULL for public clients; bcrypt hash
    client_name VARCHAR(255) NOT NULL,
    client_type VARCHAR(20) NOT NULL CHECK (client_type IN ('public', 'confidential')),
    redirect_uris JSONB NOT NULL DEFAULT '[]'::jsonb,
    allowed_scopes TEXT[] NOT NULL DEFAULT '{}',
    token_endpoint_auth_method VARCHAR(50) NOT NULL DEFAULT 'none',
    grant_types TEXT[] NOT NULL DEFAULT '{authorization_code,refresh_token}',
    allowed_sso BOOLEAN NOT NULL DEFAULT FALSE, -- can use SSO exchange endpoint
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_oauth_clients_client_id ON oauth_clients(client_id);
```

#### `oauth_refresh_tokens`

```sql
CREATE TABLE IF NOT EXISTS oauth_refresh_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    token_hash VARCHAR(255) UNIQUE NOT NULL,    -- SHA-256 of the opaque refresh token
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    client_id VARCHAR(255) NOT NULL REFERENCES oauth_clients(client_id) ON DELETE CASCADE,
    scope TEXT NOT NULL,
    device_id VARCHAR(255) DEFAULT '',          -- optional: link to Lavender device
    is_revoked BOOLEAN NOT NULL DEFAULT FALSE,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    replaced_by_id UUID,                        -- points to new token after rotation
    use_count INT NOT NULL DEFAULT 0
);

CREATE INDEX idx_oauth_refresh_tokens_hash ON oauth_refresh_tokens(token_hash);
CREATE INDEX idx_oauth_refresh_tokens_user ON oauth_refresh_tokens(user_id);
CREATE INDEX idx_oauth_refresh_tokens_client ON oauth_refresh_tokens(client_id);
CREATE INDEX idx_oauth_refresh_tokens_expires ON oauth_refresh_tokens(expires_at);
```

#### `oauth_grants` (user consent records)

```sql
CREATE TABLE IF NOT EXISTS oauth_grants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    client_id VARCHAR(255) NOT NULL REFERENCES oauth_clients(client_id) ON DELETE CASCADE,
    scope TEXT NOT NULL,
    granted_at TIMESTAMP NOT NULL DEFAULT NOW(),
    last_used_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, client_id)
);

CREATE INDEX idx_oauth_grants_user_client ON oauth_grants(user_id, client_id);
```

#### `oauth_auth_codes` (audit trail — codes are stored in Redis for lookup)

```sql
CREATE TABLE IF NOT EXISTS oauth_auth_codes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code_hash VARCHAR(255) UNIQUE NOT NULL,     -- SHA-256 of the auth code
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    client_id VARCHAR(255) NOT NULL REFERENCES oauth_clients(client_id) ON DELETE CASCADE,
    redirect_uri TEXT NOT NULL,
    scope TEXT NOT NULL,
    nonce VARCHAR(255) DEFAULT '',
    code_challenge VARCHAR(255) NOT NULL,
    code_challenge_method VARCHAR(10) NOT NULL DEFAULT 'S256',
    is_used BOOLEAN NOT NULL DEFAULT FALSE,
    is_sso BOOLEAN NOT NULL DEFAULT FALSE,      -- generated via SSO exchange
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMP NOT NULL
);

CREATE INDEX idx_oauth_auth_codes_hash ON oauth_auth_codes(code_hash);
```

#### `oauth_access_tokens` (audit — actual tokens are stateless JWTs)

```sql
CREATE TABLE IF NOT EXISTS oauth_access_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    jti VARCHAR(255) UNIQUE NOT NULL,           -- JWT ID claim
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    client_id VARCHAR(255) NOT NULL REFERENCES oauth_clients(client_id) ON DELETE CASCADE,
    scope TEXT NOT NULL,
    is_revoked BOOLEAN NOT NULL DEFAULT FALSE,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_oauth_access_tokens_jti ON oauth_access_tokens(jti);
CREATE INDEX idx_oauth_access_tokens_expires ON oauth_access_tokens(expires_at);
```

### 5.2 Modifications to Existing Tables

**None required.** The OIDC system uses its own tables and references `users(id)` via foreign key. No changes to `users`, `user_devices`, or `device_auth_log` tables.

### 5.3 Cleanup Migration

```sql
-- Cleanup function for expired tokens (run periodically)
CREATE OR REPLACE FUNCTION cleanup_oidc_tokens() RETURNS void AS $$
BEGIN
    -- Delete expired refresh tokens
    DELETE FROM oauth_refresh_tokens
    WHERE expires_at < NOW() - INTERVAL '7 days'
       OR (is_revoked = TRUE AND created_at < NOW() - INTERVAL '30 days');

    -- Delete expired auth codes
    DELETE FROM oauth_auth_codes
    WHERE expires_at < NOW() - INTERVAL '1 day';

    -- Delete expired access token records
    DELETE FROM oauth_access_tokens
    WHERE expires_at < NOW() - INTERVAL '7 days';
END;
$$ LANGUAGE plpgsql;
```

---

## 6. Security Considerations

### 6.1 PKCE Enforcement

- **All authorization code flows** require PKCE (`S256` method only).
- `plain` method is NOT supported (rejected with `invalid_request`).
- The `code_challenge_method` parameter is validated at both authorization and token endpoints.
- Server stores `code_challenge` in Redis (auth code data) and validates at token exchange.

### 6.2 CORS Configuration

The current `Access-Control-Allow-Origin: *` is incompatible with OIDC endpoints that may need credentials. **Strategy:**

```go
// OIDC-specific CORS (applied only to /oidc/* and /.well-known/*)
func oidcCorsMiddleware(next http.Handler) http.Handler {
    allowedOrigins := map[string]bool{
        "https://myapp.com":           true,
        "https://admin.lavenderapp.com": true,
        // ... registered RP origins
    }

    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        origin := r.Header.Get("Origin")
        if allowedOrigins[origin] {
            w.Header().Set("Access-Control-Allow-Origin", origin)
            w.Header().Set("Access-Control-Allow-Credentials", "true")
        }
        // ... standard CORS headers
    })
}
```

**Discovery and JWKS endpoints** (`/.well-known/*`) keep `Access-Control-Allow-Origin: *` since they are public.

### 6.3 Token Audience Validation

- **OIDC Access Tokens:** `aud` claim = `client_id`. RPs must validate `aud` matches their own `client_id`.
- **OIDC ID Tokens:** `aud` claim = `client_id`. Same validation.
- **Internal Lavender JWTs:** `aud` is not currently set. OIDC tokens always include it.
- **Token endpoint:** validates `aud` matches the authenticating client.

### 6.4 Client Secret Management

- **Public clients** (mobile apps): no secret. Security comes from PKCE + redirect URI validation.
- **Confidential clients** (server-side web apps): `client_secret` stored as bcrypt hash in DB.
- Secret is shown **once** at registration time.
- Rotation: admin endpoint generates new secret, old one valid for 24h overlap.
- Never log client secrets. Never include in URLs. Only accepted via `Authorization: Basic` header or `client_secret_post`.

### 6.5 Redirect URI Validation

- **Exact string match** (no wildcards, no pattern matching).
- Must include scheme, host, port, path. `https://myapp.com/callback` ≠ `https://myapp.com/callback/`.
- Custom schemes for mobile: `com.newapp:/callback` is valid.
- `http://localhost` allowed only for development (flag-gated).
- Redirect URIs are immutable after client creation (must delete and recreate client to change).

### 6.6 Additional Security Measures

- **State parameter:** Required. Server validates it matches what was sent. Prevents CSRF on the callback.
- **Nonce:** Optional but recommended. Server includes it in ID token. RP validates on receipt.
- **Token binding:** OIDC tokens are NOT bound to Lavender devices. They are bound to the client registration.
- **Rate limiting:** Apply `authLimiter` (IP-based) to `/oidc/token` and `/oidc/authorize` endpoints.
- **Brute force protection:** After 5 failed authorization attempts per IP per 10 minutes, return `temporarily_unavailable`.
- **Token replay protection:** If an authorization code that was already used is submitted again, revoke all tokens issued from that code.
- **Refresh token rotation detection:** If a refresh token is used after rotation, revoke ALL refresh tokens for that client+user grant.

### 6.7 Key Security: Separate Signing Keys

- Internal Lavender JWTs (HS256) and OIDC tokens (RS256) use **completely different keys**.
- Compromising one does not compromise the other.
- The OIDC private key never leaves the server. Only the public key is published via JWKS.

---

## 7. API Specification

### 7.1 Discovery & Keys

#### `GET /.well-known/openid-configuration`
- **Response:** JSON (see §2.2)
- **Auth:** None
- **Cache-Control:** `max-age=3600`

#### `GET /.well-known/jwks.json`
- **Response:** JSON JWKS (see §2.3)
- **Auth:** None
- **Cache-Control:** `max-age=3600`

### 7.2 Authorization

#### `GET /oidc/authorize`
- **Query params:** `response_type`, `client_id`, `redirect_uri`, `scope`, `state`, `code_challenge`, `code_challenge_method`, `nonce` (optional), `prompt` (optional)
- **Response:** Redirect to `redirect_uri` with `code` and `state`, or error redirect, or HTML login/consent page
- **Auth:** OIDC session cookie (if present)
- **Errors:**
  - `invalid_request` — missing/invalid params
  - `unauthorized_client` — client not authorized for this flow
  - `access_denied` — user denied consent
  - `server_error` — internal error

#### `POST /oidc/authorize/consent`
- **Request body:** `{ "code": "..." }` or `{ "deny": true }`
- **Auth:** OIDC session cookie
- **Response:** Redirect to `redirect_uri`

#### `POST /oidc/sso-exchange`
- **Request body:** `{ "client_id", "code_challenge", "code_challenge_method", "scope", "state", "lavender_token" }`
- **Response:** `{ "code": "...", "state": "..." }`
- **Auth:** Bearer (Lavender JWT in body)
- **Errors:** `invalid_token`, `unauthorized_client`, `invalid_request`

### 7.3 Token

#### `POST /oidc/token`
- **Content-Type:** `application/x-www-form-urlencoded`
- **Request body (auth code):** `grant_type=authorization_code&code=...&redirect_uri=...&client_id=...&code_verifier=...`
- **Request body (refresh):** `grant_type=refresh_token&refresh_token=...&client_id=...`
- **Auth:** `client_secret_basic` (for confidential) or `client_id` in body (for public)
- **Response (success):** Token response JSON (see §2.5)
- **Response (error):** `{ "error": "invalid_grant", "error_description": "..." }`
- **Rate limited:** 10/min per IP

#### `POST /oidc/revoke`
- **Request body:** `token=...&token_type_hint=refresh_token`
- **Auth:** `client_secret_basic`
- **Response:** HTTP 200 (always)

#### `POST /oidc/introspect`
- **Request body:** `token=...&token_type_hint=access_token`
- **Auth:** `client_secret_basic`
- **Response:** Introspection JSON (see §2.8)

### 7.4 UserInfo

#### `GET /oidc/userinfo`
- **Auth:** `Authorization: Bearer <access_token>`
- **Response:** Claims JSON (see §2.6)
- **Errors:** `invalid_token` (401)

### 7.5 Logout

#### `GET /oidc/logout`
- **Query params:** `id_token_hint` (optional), `post_logout_redirect_uri` (optional), `state` (optional)
- **Response:** Redirect to `post_logout_redirect_uri` or HTML confirmation page
- **Side effects:** Revokes tokens, clears session cookie

### 7.6 Admin

#### `POST /oidc/admin/clients`
- **Auth:** Lavender admin JWT (existing auth system)
- **Request body:** Client registration JSON
- **Response:** Client JSON with `client_id` (and `client_secret` for confidential)

#### `GET /oidc/admin/clients`
- **Auth:** Lavender admin JWT
- **Response:** `{ "clients": [...] }`

#### `PUT /oidc/admin/clients/:client_id`
- **Auth:** Lavender admin JWT
- **Request body:** Partial update JSON
- **Response:** Updated client JSON

#### `DELETE /oidc/admin/clients/:client_id`
- **Auth:** Lavender admin JWT
- **Response:** `{ "success": true }`

### 7.7 Proto Changes

**None required.** The OIDC system is entirely HTTP-based. The existing gRPC auth system continues to serve Lavender's native clients. The OIDC layer is a parallel HTTP API that bridges to the same user database.

---

## 8. Implementation Plan

### Phase 1: Core OIDC Provider (files to create/modify)

| Order | File | Action | Description |
|---|---|---|---|
| 1 | `db_oidc_migrations.go` | **Create** | SQL migrations for all OIDC tables |
| 2 | `db_oidc_clients.go` | **Create** | OAuth client CRUD operations |
| 3 | `db_oidc_tokens.go` | **Create** | Refresh token, auth code, access token audit operations |
| 4 | `db_oidc_grants.go` | **Create** | User consent grant operations |
| 5 | `oidc_keys.go` | **Create** | RSA key pair generation, loading, JWKS serving |
| 6 | `oidc_tokens.go` | **Create** | RS256 JWT generation for access/ID tokens, opaque refresh token generation |
| 7 | `oidc_discovery.go` | **Create** | `/.well-known/openid-configuration` handler |
| 8 | `oidc_authorize.go` | **Create** | Authorization endpoint + login/consent page rendering |
| 9 | `oidc_token.go` | **Create** | Token endpoint (auth code exchange, refresh) |
| 10 | `oidc_userinfo.go` | **Create** | UserInfo endpoint |
| 11 | `oidc_revoke.go` | **Create** | Token revocation endpoint |
| 12 | `oidc_introspect.go` | **Create** | Token introspection endpoint |
| 13 | `oidc_logout.go` | **Create** | Logout endpoint |
| 14 | `oidc_sso.go` | **Create** | SSO check and exchange endpoints |
| 15 | `oidc_admin.go` | **Create** | Admin client management endpoints |
| 16 | `http_server.go` | **Modify** | Register all OIDC routes, add OIDC CORS middleware, add admin auth check |
| 17 | `main.go` | **Modify** | Run OIDC migrations on startup |
| 18 | `redis_rate_limiter.go` | **Modify** | Add rate limiters for OIDC token/authorize endpoints |

### Phase 2: OIDC Route Registration in `http_server.go`

Add to the existing `setupRoutes()` function (or equivalent):

```go
// OIDC Discovery (no auth)
http.HandleFunc("/.well-known/openid-configuration", oidcDiscoveryHandler)
http.HandleFunc("/.well-known/jwks.json", oidcJWKSHandler)

// OIDC Authorization
http.HandleFunc("/oidc/authorize", oidcAuthorizeHandler)
http.HandleFunc("/oidc/authorize/consent", oidcConsentHandler)

// OIDC Token
http.HandleFunc("/oidc/token", oidcTokenHandler)

// OIDC UserInfo (auth required)
http.HandleFunc("/oidc/userinfo", requireAuthOIDC(oidcUserInfoHandler))

// OIDC Management
http.HandleFunc("/oidc/revoke", oidcRevokeHandler)
http.HandleFunc("/oidc/introspect", oidcIntrospectHandler)
http.HandleFunc("/oidc/logout", oidcLogoutHandler)

// OIDC SSO
http.HandleFunc("/oidc/sso-check", oidcSSOCheckHandler)
http.HandleFunc("/oidc/sso-exchange", oidcSSOExchangeHandler)

// OIDC Admin (admin auth required)
http.HandleFunc("/oidc/admin/clients", requireAdminAuth(oidcAdminClientsHandler))
http.HandleFunc("/oidc/admin/clients/", requireAdminAuth(oidcAdminClientHandler))
```

### Phase 3: Dependencies

```
Phase 1 steps 1-4 can be done in parallel (all DB layer).
Phase 1 steps 5-6 depend on nothing (crypto layer).
Phase 1 steps 7-15 depend on 1-6 (handlers use DB + crypto).
Phase 1 step 16 depends on 7-15 (route registration).
Phase 1 step 17 depends on 1 (migration).
Phase 1 step 18 is independent.
```

### Testing Strategy

#### Unit Tests

| Test | Coverage |
|---|---|
| `oidc_keys_test.go` | Key generation, JWKS format, key rotation |
| `oidc_tokens_test.go` | RS256 signing/verification, claim generation, opaque token generation |
| `db_oidc_clients_test.go` | Client CRUD, secret hashing, redirect URI validation |
| `db_oidc_tokens_test.go` | Auth code storage/retrieval, refresh token rotation, revocation |
| `oidc_authorize_test.go` | Parameter validation, PKCE validation, consent flow |
| `oidc_token_test.go` | Code exchange, refresh, error cases, replay detection |

#### Integration Tests

| Test | Scenario |
|---|---|
| Full auth code flow | Authorize → login → consent → code → token → userinfo |
| PKCE validation | Reject wrong verifier, reject missing challenge, reject `plain` method |
| Refresh token rotation | Use refresh token → get new pair → old one fails |
| Token replay protection | Use auth code twice → second use returns error + revokes tokens |
| SSO exchange | Valid Lavender JWT → auth code → OIDC tokens |
| Redirect URI validation | Exact match enforcement, reject mismatched URIs |
| Client authentication | Basic auth, post body, public client |
| Rate limiting | Exceed limits → 429 |

#### Manual Testing

1. Start Lavender server with OIDC enabled
2. Register a test client via admin endpoint
3. Use `curl` to simulate auth code flow
4. Verify tokens at JWKS endpoint
5. Test UserInfo endpoint
6. Test refresh flow
7. Test revocation

#### Security Tests

1. Replay old auth code → verify token revocation
2. Use refresh token after rotation → verify all tokens revoked
3. Send wrong `code_verifier` → verify rejection
4. Send `code_challenge_method=plain` → verify rejection
5. Send mismatched `redirect_uri` → verify rejection
6. Send invalid `client_id` → verify rejection
7. Send expired auth code → verify rejection

---

## 9. Edge Cases

### 9.1 Lavender Server Down

When the OIDC provider is unreachable:

- **Android SSO flow:** Intent to Lavender app fails or times out (2s). App falls back to credential mode (direct login form, which calls Lavender's own auth — also fails if server is down). User sees "Server unavailable" message.
- **Web SSO flow:** Redirect to `/oidc/authorize` fails. RP shows error page.
- **Token refresh:** RP gets network error. Retry with exponential backoff (1s, 2s, 4s, max 3 retries). If all fail, clear local session and prompt re-login.
- **UserInfo call:** RP gets network error. Use cached profile data (stored in local DB). Show stale indicator.

### 9.2 Token Revocation Propagation

When a Lavender admin revokes a user:

1. Admin calls `POST /oidc/revoke` for the user's tokens (or revokes via Lavender admin panel).
2. All refresh tokens for that user are marked `is_revoked = TRUE` in DB.
3. Next time RP tries to refresh → rejected with `invalid_grant`.
4. **Propagation gap:** Access tokens already issued remain valid until expiry (max 1 hour). This is acceptable — OIDC tokens are short-lived.
5. **Instant revocation:** For immediate revocation, the RP can call `/oidc/introspect` before sensitive operations. If `active: false`, the RP forces re-authentication.

### 9.3 Race Conditions in Token Refresh

**Scenario:** RP sends two concurrent refresh requests with the same refresh token.

**Mitigation:**
1. Server acquires a row-level lock (or uses optimistic locking) on the `oauth_refresh_tokens` row.
2. First request succeeds: issues new tokens, marks old as revoked, sets `replaced_by_id`.
3. Second request finds the token is now revoked → returns `invalid_grant`.
4. If the first request hasn't committed yet, the second request waits (or fails fast with a 409 Conflict).

**Implementation:**
```sql
-- Atomic refresh: use a single UPDATE with WHERE clause
UPDATE oauth_refresh_tokens
SET is_revoked = TRUE, replaced_by_id = $new_token_id
WHERE token_hash = $hash
  AND is_revoked = FALSE
  AND client_id = $client_id
  AND expires_at > NOW()
RETURNING id;
-- If 0 rows returned → token already revoked → replay detected
```

### 9.4 Consent UI

When the consent page needs to be shown:

1. Server renders an HTML page (not a redirect). The page is served from the Lavender server.
2. The page shows: "App X wants to access: profile, email" with Approve/Deny buttons.
3. The page includes CSRF protection (hidden nonce field).
4. If the user has previously granted consent (record in `oauth_grants`), skip the consent page and redirect automatically.

### 9.5 Clock Skew

- Server and RP clocks may differ. OIDC tokens include `iat` and `exp` claims.
- RPs should allow 5 minutes of clock skew when validating `exp` claims.
- Server should allow 5 minutes of clock skew when validating token expiry.

### 9.6 Client Deactivation

When an admin deactivates a client (`is_active = FALSE`):

1. All pending authorization codes are rejected.
2. All refresh tokens for that client are revoked.
3. Access tokens remain valid until expiry (short-lived).
4. RPs using this client see `invalid_client` errors on next token request.

### 9.7 Multiple Concurrent OIDC Sessions

A single user can have OIDC sessions with multiple RP clients simultaneously. Each client has independent consent grants. Revoking one client's tokens does not affect others.

---

## Appendix A: Environment Variables

```bash
# OIDC Configuration
OIDC_ISSUER_URL=https://messenger.lavenderapp.com  # Must match external URL
OIDC_SESSION_SECRET=<random-64-bytes-hex>           # For OIDC session cookies

# Key Storage
OIDC_KEYS_DIR=.keys                                  # Directory for RSA key pair

# Feature Flags
OIDC_ENABLED=true                                    # Enable/disable OIDC endpoints
OIDC_SSO_ENABLED=true                                # Enable SSO exchange endpoint
OIDC_CONSENT_SKIP_FOR_RETURNING=true                 # Skip consent for previously approved clients

# CORS
OIDC_ALLOWED_ORIGINS=https://myapp.com,https://admin.lavenderapp.com
```

## Appendix B: Library Dependencies (Go)

```
# Existing (no changes needed)
github.com/golang-jwt/jwt/v5          # JWT (extend for RS256)
github.com/google/uuid                # UUID generation
github.com/lib/pq                     # PostgreSQL
github.com/redis/go-redis/v9          # Redis

# New
golang.org/x/crypto                   # Already in go.mod (for bcrypt + potentially ed25519)
```

**Note:** `golang-jwt/jwt/v5` already supports RS256 via `jwt.SigningMethodRS256`. No new JWT library needed.

## Appendix C: Migration SQL (Complete)

```sql
-- Migration: Add OIDC tables
-- File: db_oidc_migrations.go

BEGIN;

-- 1. OAuth Clients
CREATE TABLE IF NOT EXISTS oauth_clients (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id VARCHAR(255) UNIQUE NOT NULL,
    client_secret_hash VARCHAR(255),
    client_name VARCHAR(255) NOT NULL,
    client_type VARCHAR(20) NOT NULL CHECK (client_type IN ('public', 'confidential')),
    redirect_uris JSONB NOT NULL DEFAULT '[]'::jsonb,
    allowed_scopes TEXT[] NOT NULL DEFAULT '{}',
    token_endpoint_auth_method VARCHAR(50) NOT NULL DEFAULT 'none',
    grant_types TEXT[] NOT NULL DEFAULT '{authorization_code,refresh_token}',
    allowed_sso BOOLEAN NOT NULL DEFAULT FALSE,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_oauth_clients_client_id ON oauth_clients(client_id);

-- 2. Refresh Tokens
CREATE TABLE IF NOT EXISTS oauth_refresh_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    token_hash VARCHAR(255) UNIQUE NOT NULL,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    client_id VARCHAR(255) NOT NULL REFERENCES oauth_clients(client_id) ON DELETE CASCADE,
    scope TEXT NOT NULL,
    device_id VARCHAR(255) DEFAULT '',
    is_revoked BOOLEAN NOT NULL DEFAULT FALSE,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    replaced_by_id UUID,
    use_count INT NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_oauth_refresh_tokens_hash ON oauth_refresh_tokens(token_hash);
CREATE INDEX IF NOT EXISTS idx_oauth_refresh_tokens_user ON oauth_refresh_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_oauth_refresh_tokens_client ON oauth_refresh_tokens(client_id);

-- 3. User Consent Grants
CREATE TABLE IF NOT EXISTS oauth_grants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    client_id VARCHAR(255) NOT NULL REFERENCES oauth_clients(client_id) ON DELETE CASCADE,
    scope TEXT NOT NULL,
    granted_at TIMESTAMP NOT NULL DEFAULT NOW(),
    last_used_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, client_id)
);

-- 4. Auth Code Audit
CREATE TABLE IF NOT EXISTS oauth_auth_codes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code_hash VARCHAR(255) UNIQUE NOT NULL,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    client_id VARCHAR(255) NOT NULL REFERENCES oauth_clients(client_id) ON DELETE CASCADE,
    redirect_uri TEXT NOT NULL,
    scope TEXT NOT NULL,
    nonce VARCHAR(255) DEFAULT '',
    code_challenge VARCHAR(255) NOT NULL,
    code_challenge_method VARCHAR(10) NOT NULL DEFAULT 'S256',
    is_used BOOLEAN NOT NULL DEFAULT FALSE,
    is_sso BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMP NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_oauth_auth_codes_hash ON oauth_auth_codes(code_hash);

-- 5. Access Token Audit
CREATE TABLE IF NOT EXISTS oauth_access_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    jti VARCHAR(255) UNIQUE NOT NULL,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    client_id VARCHAR(255) NOT NULL REFERENCES oauth_clients(client_id) ON DELETE CASCADE,
    scope TEXT NOT NULL,
    is_revoked BOOLEAN NOT NULL DEFAULT FALSE,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_oauth_access_tokens_jti ON oauth_access_tokens(jti);

-- 6. Version tracking
CREATE TABLE IF NOT EXISTS schema_migrations (
    version INT PRIMARY KEY,
    applied_at TIMESTAMP NOT NULL DEFAULT NOW()
);

INSERT INTO schema_migrations (version) VALUES (100)
ON CONFLICT (version) DO NOTHING;

COMMIT;
```

---

*End of OIDC SSO System Design Document*

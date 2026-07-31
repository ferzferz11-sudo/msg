# Задачи: Security Fixes — JWT, Rate Limiter, CORS

**Приоритет:** CRITICAL / HIGH
**Создано:** 2026-07-31
**Обновлено:** 2026-07-31
**Файлы:** `auth_jwt.go`, `auth/jwt.go`, `auth_service_v2.go`, `http_server.go`, `db_migrations.go`, `main.go`

---

## Статус

| # | Задача | Приоритет | Статус |
|---|--------|-----------|--------|
| 1 | HTTP requireAuth: проверка claims.Type | CRITICAL | ✅ Done (http_server.go:86) |
| 2 | JWT issuer/audience claims | CRITICAL | ✅ Done (auth_jwt.go:70-71,92-93,118) |
| 3 | Agent JWT RevokeToken | HIGH | ✅ Done (auth/jwt.go:210+, db_migrations.go:204+) |
| 4 | IP Extraction: заменить "unknown" | MEDIUM | ✅ Done (auth_service_v2.go:186) |
| 5 | Rate limiter cleanup goroutine | HIGH | ✅ Done (auth_service_v2.go:30+) |
| 6 | CORS restriction для OIDC | HIGH | ✅ Done (http_server.go:806+) |

**Все задачи выполнены. Сборка и тесты пройдены. Готово к деплою.**

---

## Задача 2 (CRITICAL): JWT issuer/audience claims

### Проблема
`auth_jwt.go:66,86` — `RegisteredClaims` не содержит `Issuer`/`Audience`. Токен, выпущенный для одного сервиса, может быть использован против другого (confused deputy).

### Решение
В `auth_jwt.go`, функция `GenerateTokenPair`, в обоих блоках `RegisteredClaims` добавить:

Для access token (строка ~66):
```go
RegisteredClaims: jwt.RegisteredClaims{
    ExpiresAt: jwt.NewNumericDate(accessExp),
    IssuedAt:  jwt.NewNumericDate(now),
    ID:        uuid.New().String(),
    Issuer:    "lavender-server",
    Audience:  jwt.ClaimStrings{"lavender-server"},
},
```

Для refresh token (строка ~86):
```go
RegisteredClaims: jwt.RegisteredClaims{
    ExpiresAt: jwt.NewNumericDate(refreshExp),
    IssuedAt:  jwt.NewNumericDate(now),
    ID:        refreshJTI,
    Issuer:    "lavender-server",
    Audience:  jwt.ClaimStrings{"lavender-server"},
},
```

В `ValidateToken` (строка ~109) — добавить опции валидации:
```go
token, err := jwt.ParseWithClaims(tokenString, &authClaims{}, func(token *jwt.Token) (interface{}, error) {
    // ... существующий код ...
}, jwt.WithIssuer("lavender-server"), jwt.WithAudience("lavender-server"))
```

### Файл: `auth_jwt.go`

---

## Задача 3 (HIGH): Agent JWT RevokeToken

### Проблема
`auth/jwt.go:176` — `RevokeToken` возвращает `nil` (заглушка). Компрометированный agent-токен живёт 24ч без возможности отзыва. Также `ValidateAgentToken` не проверяет blacklist.

### Решение

1. Создать таблицу в БД (добавить миграцию в `db_migrations.go`):
```sql
CREATE TABLE IF NOT EXISTS revoked_tokens (
    token_hash VARCHAR(64) PRIMARY KEY,
    revoked_at TIMESTAMP DEFAULT NOW(),
    expires_at TIMESTAMP NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_revoked_tokens_expires ON revoked_tokens(expires_at);
```

2. В `auth/jwt.go` — реализовать `RevokeToken`:
```go
import (
    "crypto/sha256"
    "encoding/hex"
)

func RevokeToken(token string) error {
    hash := sha256.Sum256([]byte(token))
    hashHex := hex.EncodeToString(hash[:])

    // Парсим токен чтобы достать expiry (без валидации подписи)
    parser := jwt.NewParser(jwt.WithoutClaimsValidation())
    claims := &jwt.RegisteredClaims{}
    _, _, err := parser.ParseUnverified(token, claims)
    if err != nil {
        return fmt.Errorf("parse token for revocation: %w", err)
    }

    expiresAt := claims.ExpiresAt.Time

    _, err = db.Exec(`INSERT INTO revoked_tokens (token_hash, expires_at) VALUES ($1, $2) ON CONFLICT (token_hash) DO NOTHING`, hashHex, expiresAt)
    if err != nil {
        return fmt.Errorf("insert revoked token: %w", err)
    }
    return nil
}
```

3. В `ValidateAgentToken` (строка ~122) — добавить проверку blacklist перед валидацией подписи:
```go
// Check blacklist
hash := sha256.Sum256([]byte(token))
hashHex := hex.EncodeToString(hash[:])
var exists bool
err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM revoked_tokens WHERE token_hash = $1 AND expires_at > NOW())`, hashHex).Scan(&exists)
if err == nil && exists {
    return nil, fmt.Errorf("token has been revoked")
}
```

4. Cleanup-горутина в `main.go` (рядом с другими background-горутинами, ~строка 262):
```go
// Periodic revoked token cleanup (every 1 hour)
go func() {
    ticker := time.NewTicker(1 * time.Hour)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            db.Exec(`DELETE FROM revoked_tokens WHERE expires_at < NOW()`)
        }
    }
}()
```

**Важно:** Cleanup-горутина должна использовать `ctx.Done()` для корректного завершения при shutdown (как другие горутины в main.go).

### Файлы: `auth/jwt.go`, `db_migrations.go`, `main.go`

---

## Задача 4 (MEDIUM): IP Extraction — заменить "unknown" (3 места)

### Проблема
`auth_service_v2.go` — IP-адрес записывается как `"unknown"` в трёх местах вместо реального IP из gRPC контекста. Функция `getIPFromContext` уже существует (строка 56) и используется в строках 92 и 223, но не в остальных местах.

**Места:**
- `auth_service_v2.go:158` — `SignInV2`, `ipAddress := "unknown"`
- `auth_service_v2.go:300` — `SignUpV2`, `UpsertDevice(..., "unknown", ...)`
- `auth_service_v2.go:321` — `SignUpV2`, `LogAuthEvent(..., "unknown", ...)`

### Решение

В `auth_service_v2.go:158` заменить:
```go
ipAddress := "unknown" // TODO: extract from gRPC context if needed
```
На:
```go
ipAddress := getIPFromContext(ctx)
```

В `auth_service_v2.go:300` заменить:
```go
_, err = a.db.UpsertDevice(userID, deviceID, deviceName, deviceType, clientVersion, "unknown", clientVersion)
```
На:
```go
ip := getIPFromContext(ctx)
_, err = a.db.UpsertDevice(userID, deviceID, deviceName, deviceType, clientVersion, ip, clientVersion)
```

В `auth_service_v2.go:321` заменить:
```go
a.db.LogAuthEvent(userID, deviceID, "signup_v2", "unknown", clientVersion, true, "")
```
На:
```go
a.db.LogAuthEvent(userID, deviceID, "signup_v2", ip, clientVersion, true, "")
```

**Примечание:** Переменная `ip` уже определена на строке 223 в `SignUpV2` для rate limiter, но она локальна в блоке `if`. Нужно вынести или переиспользовать.

### Файл: `auth_service_v2.go`

---

## Задача 5 (HIGH): Rate limiter cleanup goroutine

### Проблема
`auth_service_v2.go:15-52` — `ipRateLimiter` хранит timestamp в `map[string][]time.Time` без cleanup-горутины. Unbounded memory growth — при большом количестве IP записи никогда не удаляются.

### Решение
Добавить метод `cleanup` в `auth_service_v2.go`:

```go
func (r *ipRateLimiter) cleanup(ctx context.Context) {
    ticker := time.NewTicker(5 * time.Minute)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            r.mu.Lock()
            now := time.Now()
            for key, times := range r.requests {
                valid := times[:0]
                for _, t := range times {
                    if now.Sub(t) < r.window {
                        valid = append(valid, t)
                    }
                }
                if len(valid) == 0 {
                    delete(r.requests, key)
                } else {
                    r.requests[key] = valid
                }
            }
            r.mu.Unlock()
        }
    }
}
```

В `main.go` (рядом с другими init-горутинами, ~строка 262):
```go
go authLimiter.cleanup(ctx)
```

**Важно:** Принимать `ctx` для корректного завершения при shutdown.

### Файлы: `auth_service_v2.go`, `main.go`

---

## Задача 6 (HIGH): CORS restriction для OIDC

### Проблема
`http_server.go:808` — `Access-Control-Allow-Origin: *` для всех запросов. OIDC-эндпоинты с credentials (`/oidc/authorize`, `/oidc/token`, `/oidc/userinfo`) не работают с wildcard — браузер блокирует credentialed cross-origin requests.

### Решение
В `corsMiddleware` (строка ~806) — разделить CORS-политику:

```go
func corsMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        path := r.URL.Path

        // OIDC-эндпоинты: restricted CORS с credentials
        if strings.HasPrefix(path, "/oidc/") {
            origin := r.Header.Get("Origin")
            allowedOrigins := []string{
                "https://13.140.25.249",
                "http://13.140.25.249",
                "http://localhost:3000",
                "http://localhost:8080",
            }
            for _, allowed := range allowedOrigins {
                if origin == allowed {
                    w.Header().Set("Access-Control-Allow-Origin", origin)
                    w.Header().Set("Access-Control-Allow-Credentials", "true")
                    break
                }
            }
        } else {
            // .well-known, uploads, health, info — wildcard
            w.Header().Set("Access-Control-Allow-Origin", "*")
        }

        w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
        w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

        if r.Method == http.MethodOptions {
            w.WriteHeader(http.StatusNoContent)
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

**Примечание:** Список `allowedOrigins` нужно вынести в env-переменную (`CORS_ALLOWED_ORIGINS`) для гибкости между dev/prod.

### Файл: `http_server.go`

---

## Порядок выполнения

1. Задача 4 — 2 минуты (3 замены строки)
2. Задача 2 — 5 минут
3. Задача 5 — 10 минут
4. Задача 6 — 10 минут
5. Задача 3 — 20 минут (миграция БД + реализация + тесты)

## Проверка

```bash
# Сборка
go build -o lavender-server .

# Тесты
go test ./... -v

# Race detector
go test -race -count=1 .
```

Ручная проверка:
1. Login → токен содержит `iss: lavender-server`, `aud: lavender-server` (jwt.io)
2. Login → в `user_devices` реальный IP (не "unknown")
3. SignUp → в `user_devices` и `device_auth_log` реальный IP
4. SignOut agent → токен в `revoked_tokens`
5. Повторный use отозванного agent-токена → 401
6. OIDC flow работает end-to-end
7. Rate limiter memory не растёт (grafana / pprof)
8. CORS: `curl -H "Origin: https://evil.com" /oidc/authorize` → нет `Access-Control-Allow-Origin`

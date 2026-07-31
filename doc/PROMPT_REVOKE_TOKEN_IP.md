# Задачи: RevokeToken + IP Extraction + OIDC Security Fixes

**⚠️ SUPERSEDED by `PROMPT_SECURITY_FIXES.md` (2026-07-31) — актуальный промт там.**

**Приоритет:** CRITICAL / HIGH
**Создано:** 2026-07-05 | **Обновлено:** 2026-07-31
**Файлы:** `auth/jwt.go`, `auth_service_v2.go`, `http_server.go`, `auth_jwt.go`

---

## Краткий статус

| Задача | Приоритет | Статус |
|--------|-----------|--------|
| HTTP requireAuth: проверка claims.Type | CRITICAL | ✅ Done |
| JWT issuer/audience claims | CRITICAL | ✅ Done |
| Agent JWT RevokeToken | HIGH | ✅ Done |
| OIDC SSO система | HIGH | ✅ Done |
| IP Extraction из gRPC context | MEDIUM | ✅ Done |
| Rate limiter eviction | HIGH | ✅ Done |
| CORS restriction для OIDC | HIGH | ✅ Done |

**Все задачи выполнены (2026-07-31). Актуальный промт: `PROMPT_SECURITY_FIXES.md`**

---

## Задача 1 (CRITICAL): HTTP requireAuth — проверка claims.Type

### Проблема
`http_server.go:80-88` — `requireAuth` не проверяет `claims.Type`. Refresh token (30д) проходит валидацию и даёт доступ к upload-эндпоинтам и TURN credentials.

### Решение
В `http_server.go`, функция `requireAuth`, после `ValidateToken`:
```go
if claims.Type != "access" {
    http.Error(w, `{"error":"invalid token type"}`, http.StatusUnauthorized)
    return
}
```

### Файл: `http_server.go:80-88`

---

## Задача 2 (CRITICAL): JWT issuer/audience claims

### Проблема
`auth_jwt.go` — `authClaims` не устанавливает `Issuer`/`Audience`. Токен для одного сервиса может быть использован против другого.

### Решение
В `auth_jwt.go`, `GenerateTokenPair`:
```go
claims.Issuer = getEnvOrDefault("OIDC_ISSUER_URL", "lavender-server")
claims.Audience = jwt.ClaimStrings{"lavender-server"}
```

### Файл: `auth_jwt.go`

---

## Задача 3 (HIGH): Agent JWT RevokeToken

### Проблема
`auth/jwt.go:176` — `RevokeToken` возвращает `nil`. Компрометированный agent-токен живёт 24ч без возможности отзыва.

### Решение
1. Использовать существующую таблицу `oauth_access_tokens` (OIDC audit) или создать `revoked_tokens`
2. При `RevokeToken` — сохранять SHA-256 хеш токена + expiry
3. При `ValidateAgentToken` — проверять хеш в blacklist
4. Очистка: удалять записи с `expires_at < NOW()`

### Файл: `auth/jwt.go`

---

## Задача 4 (MEDIUM): IP Extraction из gRPC context

### Проблема
`auth_service_v2.go:158`:
```go
ipAddress := "unknown" // TODO: extract from gRPC context if needed
```

### Решение
1. Создать `auth/ip_extract.go`:
```go
func ExtractClientIP(ctx context.Context) string {
    // 1. x-forwarded-for (первый IP = клиент)
    // 2. x-real-ip
    // 3. peer.Addr
    // 4. "unknown"
}
```
2. Заменить `ipAddress := "unknown"` на `ipAddress := ExtractClientIP(ctx)`
3. Интегрировать во все места с `UpsertDevice` и `LogAuthEvent`

### Файлы: `auth_service_v2.go:158`, новый `auth/ip_extract.go`

---

## Задача 5 (HIGH): Rate limiter eviction

### Проблема
`auth_service_v2.go:15-52` — `ipRateLimiter` хранит timestamp в `map[string][]time.Time` без cleanup-горутины. Unbounded memory growth.

### Решение
Добавить background goroutine для очистки:
```go
func (r *ipRateLimiter) cleanup() {
    ticker := time.NewTicker(5 * time.Minute)
    for range ticker.C {
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
```
Запускать в `main.go` как `go authLimiter.cleanup()`.

### Файл: `auth_service_v2.go`

---

## Задача 6 (HIGH): CORS restriction для OIDC

### Проблема
`http_server.go:771` — `Access-Control-Allow-Origin: *`. OIDC-эндпоинты с credentials не могут работать с wildcard.

### Решение
OIDC-эндпоинты (`/oidc/*`) используют свою CORS-политику с конкретными origins. Discovery (`/.well-known/*`) остаётся с `*`.

### Файл: `http_server.go`

---

## Порядок выполнения

1. Задача 1 (CRITICAL) — 5 минут
2. Задача 2 (CRITICAL) — 5 минут
3. Задача 5 (HIGH) — 15 минут
4. Задача 3 (HIGH) — 30 минут (нужна миграция БД)
5. Задача 4 (MEDIUM) — 15 минут
6. Задача 6 (HIGH) — 10 минут (совместно с OIDC)

## Проверка после реализации

1. `go build -o lavender-server .` — компиляция
2. `go test ./... -v` — все тесты
3. Ручная проверка:
   - Login → в `user_devices` реальный IP (не "unknown")
   - SignOut → токен в blacklist
   - Повторный use отозванного токена → 401
   - Refresh token через HTTP upload → 401 (не 200)
   - OIDC flow работает end-to-end

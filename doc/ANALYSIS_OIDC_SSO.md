# Анализ Lavender Messenger v1.3.4.0 + OIDC SSO

**Дата:** 2026-07-21
**Версия сервера:** 1.3.4.0
**Статус:** Анализ завершён, OIDC SSO — реализация завершена, session validation исправлена

---

## 1. Анализ текущей версии

### Сильные стороны

| Область | Описание |
|---------|----------|
| Архитектура | 117 файлов, разделение по domain-принципу (20+ хендлеров). Чёткая изоляция модулей (чаты, AI, компании, стикеры) |
| Безопасность (основная) | JWT access+refresh с device binding, bcrypt cost 12, E2EE (AES-256-GCM + ECDH), gRPC interceptors, panic recovery в streams |
| AI-система v2 | 9 провайдеров (OpenRouter, MiMo, Hermes ACP, Webhook, WS, Subprocess, MCP, Reve, Local), 11 пресетов, marketplace, RAG pipeline (Qdrant + OpenAI embeddings), tool calling loop (max 10 iterations) |
| Компании | Multi-tenant: иерархия позиций (Owner>TopManager>Manager>Employee), chat access control, invite codes, FCM push |
| Производительность | Cursor pagination (O(log n)), push batching (500 tokens), push debouncing, O(1) online presence, DB pool 50/25 |
| Тесты | ~88 тестов: auth, ChatV2 integration, reactions, stability, company |
| Документация | ARCHITECTURE.md, CLIENT_INTEGRATION.md (2086 строк), CHANGELOG.md (80+ записей) |

### Слабые стороны

| Приоритет | Проблема | Файл:строка | Описание |
|-----------|----------|-------------|----------|
| **CRITICAL** | HTTP middleware пропускает refresh tokens | `http_server.go:80-88` | `requireAuth` не проверяет `claims.Type == "access"`. Refresh token (30д) даёт доступ к upload и TURN |
| **CRITICAL** | Нет issuer/audience в JWT | `auth_jwt.go` | `authClaims` не устанавливает `Issuer`/`Audience`. Токен для одного сервиса → другой |
| **HIGH** | Agent JWT — заглушка RevokeToken | `auth/jwt.go:176` | `RevokeToken` возвращает nil. Компрометированный agent-токен живёт 24ч |
| **HIGH** | CORS `*` | `http_server.go:771` | Любая страница может делать authenticated requests |
| **HIGH** | Rate limiter без eviction | `auth_service_v2.go:15-52` | `ipRateLimiter` — unbounded memory growth при нагрузке |
| **HIGH** | Access token не отзываем | — | 15 минут без mechanism отмены. Смена пароля не аннулирует AT |
| **MEDIUM** | IP не сохраняется | `auth_service_v2.go:158` | `ipAddress := "unknown"` вместо реального IP (TODO) |
| **MEDIUM** | `user_agent` = `clientVersion` | `auth_service_v2.go` | Не настоящий HTTP User-Agent |
| **MEDIUM** | SignOut без аутентификации | `auth_service_v2.go:393` | Извлекает identity только из refresh token в body |
| **MEDIUM** | Двух-identity система | codebase | Username→UUID миграция не завершена, дублирование DB-функций |
| **LOW** | README отстаёт | `README.md` | Ссылается на v1.1.2.4, реальная — 1.3.4.0 |
| **LOW** | Миграции без error check | `db_auth_migrations.go:12-45` | Отдельные `db.Exec()` молча игнорируют ошибки |

### Устранённые инциденты

| Дата | Проблема | Решение |
|------|----------|---------|
| 2026-07-21 | PostgreSQL PANIC: No space left on device | Очищено 3.7G (journal 2G, coturn 706M, syslog 860M). PG перезапущен |

---

## 2. OIDC SSO — Дизайн

### Концепция

Лава становится **OpenID Connect Provider**. Новые приложения аутентифицируют пользователей через Lavender credentials. Два режима:
- **SSO**: Lavender установлена → автоматический вход через Intent/DeepLink
- **Credentials**: Lavender не установлена → форма логина → OIDC authorize flow

### Стратегия токенов

| Тип | Алгоритм | Audience | TTL | Назначение |
|-----|----------|----------|-----|------------|
| Internal Lavender JWT | HS256 | Lavender | 15мин/30д | Существующая auth (без изменений) |
| OIDC Access Token | RS256 | client_id | 1 час | API доступ |
| OIDC ID Token | RS256 | client_id | 1 час | Identity assertion |
| OIDC Refresh Token | opaque | client_id | 30 дней | Обновление |
| Auth Code | opaque | N/A | 10 мин | One-time exchange |

### Новые HTTP-эндпоинты (порт 8082)

| Метод | Путь | Auth | Описание |
|-------|------|------|----------|
| GET | `/.well-known/openid-configuration` | — | Discovery |
| GET | `/.well-known/jwks.json` | — | JWKS |
| GET | `/oidc/authorize` | Сессия | Authorization |
| POST | `/oidc/authorize/consent` | Сессия | Consent |
| POST | `/oidc/token` | client auth | Token exchange |
| GET | `/oidc/userinfo` | Bearer AT | User claims |
| GET | `/oidc/logout` | Сессия | End session |
| POST | `/oidc/revoke` | Basic auth | Revocation |
| POST | `/oidc/introspect` | Basic auth | Introspection |
| GET | `/oidc/sso-check` | — | Deep link SSO |
| POST | `/oidc/sso-exchange` | Lavender JWT | SSO → OIDC |
| POST | `/oidc/admin/clients` | Admin JWT | Client CRUD |
| GET | `/oidc/admin/clients` | Admin JWT | List clients |

### SSO Flow (Android)

```
New App → check packageManager → Intent → Lavender App
  → validate local JWT → POST /oidc/sso-exchange → auth code
  → return code to New App → POST /oidc/token → OIDC tokens
```

Fallback: Lavender не установлена → credential mode.

### PostgreSQL таблицы (5 новых)

| Таблица | Назначение |
|---------|------------|
| `oauth_clients` | OAuth2 клиенты (public/confidential) |
| `oauth_refresh_tokens` | Refresh tokens (hashed, rotation chain) |
| `oauth_grants` | User consent |
| `oauth_auth_codes` | Auth code audit trail |
| `oauth_access_tokens` | AT audit (actual tokens — stateless JWT) |

**Изменений в существующие таблицы: НЕТ.**

### Scopes

| Scope | Claims |
|-------|--------|
| `openid` | `sub` |
| `profile` | `name`, `preferred_username`, `picture` |
| `email` | `email`, `email_verified` |
| `offline_access` | Refresh token |
| `read:profile`, `read:messages`, `push:send` | API delegation |

---

## 3. Статус реализации OIDC

### Файлы DB-слоя

| Файл | Статус | Описание |
|------|--------|----------|
| `db_oidc_migrations.go` | ✅ Создан | SQL миграции для 5 таблиц |
| `db_oidc_clients.go` | ✅ Создан | OAuth client CRUD |
| `db_oidc_tokens.go` | ✅ Создан | Refresh token, auth code, AT audit, grants |
| `oidc_keys.go` | ✅ Создан | RSA key pair, JWKS, discovery document |

### Файлы хендлеров

| Файл | Статус | Описание |
|------|--------|----------|
| `oidc_tokens.go` | ✅ Создан | RS256 JWT generation, validation |
| `oidc_authorize.go` | ✅ Создан | Authorization endpoint + login/consent forms + HMAC session validation |
| `oidc_token.go` | ✅ Создан | Token endpoint (code exchange + refresh) |
| `oidc_userinfo.go` | ✅ Создан | UserInfo endpoint |
| `oidc_revoke.go` | ✅ Создан | Token revocation |
| `oidc_introspect.go` | ✅ Создан | Token introspection |
| `oidc_logout.go` | ✅ Создан | Logout endpoint |
| `oidc_sso.go` | ✅ Создан | SSO check + exchange |
| `oidc_admin.go` | ✅ Создан | Admin client management |

### Интеграция

| Файл | Статус | Описание |
|------|--------|----------|
| `http_server.go` | ✅ Изменён | OIDC routes registered, requireAuth fixed |
| `main.go` | ✅ Изменён | OIDC migrations + key init on startup |

### Исправления безопасности

| # | Задача | Приоритет | Статус |
|---|--------|-----------|--------|
| 1 | HTTP requireAuth: проверка claims.Type | CRITICAL | ✅ Исправлено |
| 2 | JWT issuer/audience claims | CRITICAL | ✅ Исправлено (v1.3.4.0) |
| 3 | Agent JWT RevokeToken | HIGH | ✅ Исправлено (v1.3.4.0) |
| 4 | IP Extraction из gRPC context | MEDIUM | ✅ Исправлено (v1.3.4.0) |
| 5 | Rate limiter eviction | HIGH | ✅ Исправлено (v1.3.4.0) |
| 6 | CORS restriction для OIDC | HIGH | ✅ Исправлено (v1.3.4.0) |

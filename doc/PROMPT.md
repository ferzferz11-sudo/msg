# Промпт для серверных сессий — v1.2.0.x

**Дата:** 2026-06-18 | **Ветка:** feat/1.2.0.x
**Статус:** Безопасность частично завершена, dev работает

---

## ТЕКУЩИЙ СТАТУС

| | Версия | Статус |
|---|--------|--------|
| **Сервер prod** | v1.1.3.0 | Работает на порту 50051 |
| **Сервер dev** | v1.2.0.2 | Работает на порту 50052 |
| **Android** | v1.1.3.37 | Использует JWT (v2 auth) |

---

## АРХИТЕКТУРА

### Сервер (/root/msg)
```
main.go                    — Entry point, gRPC server
server.go                  — ServerVersion = "1.2.0.2"
auth_service.go            — AuthService v1 (deprecated)
auth_service_v2.go         — AuthService v2 (JWT)
auth_interceptor.go        — gRPC Bearer token interceptor
auth_jwt.go                — JWT генерация/валидация (библиотека golang-jwt)
auth/jwt.go                — JWT для agent tokens (кастомный)
db.go                      — Database layer (~1428 LOC)
db_hermes.go               — Hermes DB migrations + CRUD
db_chatlist_v2.go          — ChatList v2 DB methods
db_auth_devices.go         — user_devices + device_auth_log
server_chat.go             — Chat, Typing, CallStream
server_ai.go               — AI chat (OWL + Hermes)
server_chatlist_v2.go      — ChatList v2 RPC
server_profile_v2.go       — ProfileService v2
hermes_orchestrator.go     — Agent routing + orchestration
hub.go                     — Connection management
http_server.go             — HTTP uploads + TURN
messenger.proto            — All proto definitions
```

### Android (/root/msg.client.android)
```
data/grpc/GrpcClient.kt         — Facade
data/grpc/RealGrpcClient.kt     — gRPC implementation (JWT auth)
data/grpc/BearerTokenInterceptor.kt — JWT Bearer injection
data/session/SessionManager.kt  — loginV2 (JWT) + loginV1 (fallback)
data/auth/AuthManager.kt        — Token storage
ui/chatlist/ChatListActivity.kt — Chat list with tabs
```

---

## ЭТАПЫ РЕАЛИЗАЦИИ

### ЭТАП 1: Безопасность (ТЕКУЩИЙ) — v1.2.1.0

**Статус:** 5/9 задач выполнено

#### Выполнено ✅
1. Path traversal fix — `http_server.go:375`
2. JWT fallback → log.Fatal — `auth/jwt.go:47`
3. Hardcoded admin → one-time — `db.go:161`
4. TURN secret → .env — `http_server.go:32`
5. MarkReadAndCheck ON CONFLICT bug — `db.go:631`

#### Осталось
6. **User impersonation fix** — `server_ai.go` (6 мест)
   - Replace `req.UserId` → `auth.UserID(ctx)` в AI handlers
   - Файлы: ChatWithOWL, CreateOwlChat, DeleteOwlChat, ChatWithAI, SetFreeModel, RemoveFreeModel
7. **Auth на uploads** — `http_server.go`
   - JWT token в query param или header для upload endpoints
8. **Fix secret_chat caller** — `secret_chat.go:14`
   - Replace `getCallerUsernameSecret` → `auth.UserID(ctx)`
9. **Firebase → .gitignore** — добавить `*.json` credentials
10. **Auth на TURN** — `http_server.go:443`
    - Ограничить доступ к TURN credentials

---

### ЭТАП 2: Concurrency — v1.2.1.1

1. **stream.Send race** — `hermes_orchestrator.go:402`
   - Очередь с mutex для parallel agents
2. **Blocking broadcast** — `hub.go:284-317`
   - Copy-then-send паттерн
3. **DB query under lock** — `hermes_orchestrator.go:107`
   - Unlock before DB call
4. **IsUserOnline O(n)** — `hub.go:254`
   - Reverse index userId → streams

---

### ЭТАП 3: DB рефакторинг — v1.2.1.2

1. **MessageRecord type** — убрать 8 дублей в db.go
2. **Split db.go** → db_users.go, db_chats.go, db_messages.go
3. **DB indexes** — messages(room_id), contacts(username), user_tokens(username)
4. **Scan error handling** — 14+ мест в db.go

---

### ЭТАП 4: Code Quality — v1.2.1.3

1. **Unified RateLimiter** — owl.go + bot_commands.go
2. **Shared http.Client** — owl.go OpenRouter calls
3. **N+1 fixes** — reactions, AI settings

---

## ПРАВИЛА

1. ⚠️ **НЕ компилировать Android на сервере** — OOM kill
2. Версия сервера в `server.go:33`
3. userId (UUID) — всегда как ключ, НЕ username
4. Auth context → `auth.UserID(ctx)`, NEVER `req.UserId`
5. DB миграции: `IF NOT EXISTS`, NEVER `DROP`
6. Коммитить после каждого изменения
7. Деплой: dev → тест → prod

---

## КОМАНДЫ

```bash
# === СЕРВЕР ===
cd /root/msg && export PATH=$PATH:/usr/local/go/bin:~/go/bin

# Сборка + деплой dev
go build -o /tmp/lavender-server-dev .
systemctl stop lavender-server-dev
cp /tmp/lavender-server-dev /root/LavenderMessenger/run/lavender-server-dev
systemctl start lavender-server-dev

# Сборка + деплой prod
go build -o /tmp/lavender-server .
systemctl stop lavender-server
cp /tmp/lavender-server /root/LavenderMessenger/run/lavender-server
systemctl start lavender-server

# Proto gen
protoc --go_out=./gen --go_opt=paths=source_relative \
  --go-grpc_out=./gen --go-grpc_opt=paths=source_relative messenger.proto

# Тесты
go test ./...

# Логи
journalctl -u lavender-server-dev -f
```

---

## DEV vs PROD

| | Dev | Prod |
|---|-----|------|
| Порт gRPC | 50052 | 50051 |
| Порт HTTP | 8083 | 8082 |
| DB | chat_db_dev | chat_db |
| Config | .env.dev | .env |
| ProfileService | v2 (JWT) | v1 (legacy) |
| ChatStream | v2 (JWT) | v1 (password) |
| Версия | v1.2.0.2 | v1.1.3.0 |

---

## ДОКУМЕНТАЦИЯ

- План оптимизации: `/root/msg/doc/OPTIMIZATION_PLAN.md`
- Интеграция: `/root/msg/doc/INTEGRATION_SESSION.md`
- Задачи: `/root/msg/doc/TASKS.md`
- Pitfalls: `/root/msg/doc/PITFALLS.md`
- AI сервисы: `/root/msg/doc/AI_SERVICES.md`
- Архитектура: `/root/msg/doc/ARCHITECTURE.md`

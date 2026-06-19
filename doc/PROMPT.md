# Промпт для серверных сессий — v1.2.0.9

**Дата:** 2026-06-19 | **Ветка:** feat/1.2.0.x
**Статус:** P0+P1+P2 оптимизации задеплоены. Фокус на FCM batching и стабильности.

---

## ТЕКУЩИЙ СТАТУС

| | Версия | Статус |
|---|--------|--------|
| **Сервер prod** | v1.2.0.9 | ✅ Работает на порту 50051 |
| **Сервер dev** | v1.2.0.9 | ✅ Работает на порту 50052 |

**Android:** `/root/msg.client.android` — документация там, сборка ТОЛЬКО локально.

---

## АРХИТЕКТУРА

### Сервер (/root/msg)
```
main.go                    — Entry point, gRPC server, GracefulStop 30s timeout
server.go                  — ServerVersion = "1.2.0.8"
auth_service.go            — AuthService v1 (deprecated)
auth_service_v2.go         — AuthService v2 (JWT)
auth_interceptor.go        — gRPC Bearer token + v1 username fallback
auth_jwt.go                — JWT генерация/валидация (кэш secret, sync.Mutex)
auth/jwt.go                — JWT для agent tokens (кастомный)
db.go                      — Database layer (~80+ CRUD методов)
db_hermes.go               — Hermes DB migrations + CRUD
db_chatlist_v2.go          — ChatList v2 DB (PinChat, SearchChats, ArchiveChat, getMutedRoomsSet)
db_auth_devices.go         — user_devices + device_auth_log
server_chat.go             — Chat, Typing, CallStream, cleanupRecentMsgs
server_chatlist_v2.go      — ChatList v2 RPC (PinChat, SearchChats, ArchiveChat)
server_ai.go               — AI chat (OWL + Hermes) — OWL response теперь сохраняется
server_profile_v2.go       — ProfileService v2 (JWT)
hermes_orchestrator.go     — Agent routing — TTL cleanup 30мин, message cap 50
hermes_agent_service.go    — Agent token management — auth check через GetUserID(ctx)
hub.go                     — Connection management — Broadcast snapshot+send без лока
http_server.go             — HTTP uploads + TURN (path traversal fix, env vars)
secret_chat.go             — E2EE secret chats — membership verification
owl.go                     — OWL AI — shared http.Client, LimitReader(10MB)
bot_commands.go            — Bot commands — auth через GetUserID(ctx)
messenger.proto            — All proto definitions
```

---

## ЧТО СДЕЛАНО (v1.2.0.8)

### P1+P2 Оптимизации (Сессия 41)
- **getAIChatManager sync.Once** — thread-safe lazy init
- **backfillLastMessageText SQL fix** — operator precedence скобки
- **Stream interceptor injection** — usernameKey + deviceIDKey в stream context
- **device_auth_log TTL** — CleanupDeviceAuthLog(), cron 24ч
- **IncrementParticipantsChatListVersion → UUID[]** — unnest(participant_ids)
- **Rate limiter cleanup** — cleanup() + periodic goroutine 10мин
- **ResolveUserID cache** — in-memory cache TTL 5мин
- **IsUserOnline O(1)** — reverse-lookup sets
- **DB pool tuning** — MaxIdleConns=15, ConnMaxIdleTime=5min
- **owl.go ctx cancellation** — ctx.Err() в SSE loop
- **main.go goroutine leak** — context.WithCancel + ticker

### P0 Оптимизации (Сессия 40)
- **Broadcast deadlock fix** — snapshot streams под RLock, отправка без лока
- **isChatMuted N+1** — batch getMutedRoomsSet() вместо N запросов
- **Push N+1** — GetChat вынесен до цикла, participantSet O(1), senderNotifiesOthers рано
- **Hermes sessions** — TTL cleanup 30мин + message cap 50
- **recentMsgs** — periodic cleanup >10s
- **JWT secret** — кэш с env-change detection
- **io.LimitReader(10MB)** — OOM protection
- **OWL response saved** — AddMessage после стрима
- **GracefulStop 30s timeout** — fix deploy hang

### v1.2.0.7 (Сессия 39)
- UserInfo: user_id + is_super_admin в GetAllUsers
- deploy-dev-local.sh, deploy-prod-local.sh
- CreateChat fix (CTE для PostgreSQL parameter type conflict)
- CLIENT_INTEGRATION.md (127 gRPC методов)

---

## ПРАВИЛА

1. ⚠️ **НЕ компилировать Android на сервере** — OOM kill. Android собирается ТОЛЬКО локально.
2. Версия сервера в `server.go:33`
3. userId (UUID) — всегда как ключ, НЕ username
4. Auth context → `GetUserID(ctx)`, NEVER `req.UserId`
5. DB миграции: `IF NOT EXISTS`, NEVER `DROP`
6. Коммитить после каждого изменения
7. **Стабильность > фичи** — деплоим сразу на prod, ошибки критичны

---

## КОМАНДЫ

```bash
# === СЕРВЕР ===
cd /root/msg && export PATH=$PATH:/usr/local/go/bin:~/go/bin

# Деплой dev (с сервера)
./scripts/deploy-dev.sh

# Деплой dev (с локальной машины)
./scripts/deploy-dev-local.sh

# Деплой prod (с локальной машины)
./scripts/deploy-prod-local.sh

# Proto gen
protoc --go_out=./gen --go_opt=paths=source_relative \
  --go-grpc_out=./gen --go-grpc_opt=paths=source_relative messenger.proto

# Тесты
go test ./...
```

---

## DEV vs PROD

| | Dev | Prod |
|---|-----|------|
| Порт gRPC | 50052 | 50051 |
| Порт HTTP | 8083 | 8082 |
| DB | chat_db_dev | chat_db |
| Config | .env.dev | .env |

---

## ДОКУМЕНТАЦИЯ

- Интеграция: `/root/msg/doc/CLIENT_INTEGRATION.md`
- Индекс: `/root/msg/doc/INDEX.md`
- Задачи: `/root/msg/doc/TASKS.md`
- Оптимизации: `/root/msg/doc/OPTIMIZATION_PLAN.md`
- Pitfalls: `/root/msg/doc/PITFALLS.md`
- Android: `/root/msg.client.android/doc/` (вся документация там)

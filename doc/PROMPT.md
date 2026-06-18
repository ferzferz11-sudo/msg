# Промпт для серверных сессий — v1.2.0.x

**Дата:** 2026-06-18 | **Ветка:** feat/1.2.0.x
**Статус:** userId migration завершена (Этапы 1-3), deployed на dev+prod

---

## ТЕКУЩИЙ СТАТУС

| | Версия | Статус |
|---|--------|--------|
| **Сервер prod** | v1.2.0.6 | ✅ Работает на порту 50051 |
| **Сервер dev** | v1.2.0.6 | ✅ Работает на порту 50052 |
| **Android** | v1.1.3.37 | Использует JWT (v2 auth) |

---

## АРХИТЕКТУРА

### Сервер (/root/msg)
```
main.go                    — Entry point, gRPC server
server.go                  — ServerVersion = "1.2.0.3"
auth_service.go            — AuthService v1 (deprecated)
auth_service_v2.go         — AuthService v2 (JWT)
auth_interceptor.go        — gRPC Bearer token + v1 username fallback
auth_jwt.go                — JWT генерация/валидация (библиотека golang-jwt)
auth/jwt.go                — JWT для agent tokens (кастомный)
db.go                      — Database layer (~1433 LOC)
db_hermes.go               — Hermes DB migrations + CRUD
db_chatlist_v2.go          — ChatList v2 DB (PinChat, SearchChats, ArchiveChat)
db_auth_devices.go         — user_devices + device_auth_log
server_chat.go             — Chat, Typing, CallStream
server_chatlist_v2.go      — ChatList v2 RPC (PinChat, SearchChats, ArchiveChat)
server_ai.go               — AI chat (OWL + Hermes) — все handlers используют GetUserID(ctx)
server_profile_v2.go       — ProfileService v2 (JWT)
hermes_orchestrator.go     — Agent routing — lock pattern оптимизирован
hermes_agent_service.go    — Agent token management — auth check через GetUserID(ctx)
hub.go                     — Connection management
http_server.go             — HTTP uploads + TURN (path traversal fix, env vars)
secret_chat.go             — E2EE secret chats — membership verification
owl.go                     — OWL AI — shared http.Client
bot_commands.go            — Bot commands — auth через GetUserID(ctx)
messenger.proto            — All proto definitions
```

---

## ЧТО СДЕЛАНО (v1.2.0.3)

### Безопасность
- Path traversal fix — `http_server.go:serveFileHandler`
- JWT_SECRET → log.Fatal если не задан
- TURN credentials → env variables
- Hardcoded admin → one-time bootstrap
- User impersonation — 25+ handlers используют `GetUserID(ctx)`
- Secret chat caller — auth context вместо hub перебора
- Secret chat membership verification
- Agent token RPC — admin auth check
- Firebase credentials — удалены из git tracking
- DeleteProfile — пароль обязателен
- AuthInterceptor — v1 fallback (username из metadata)
- DB indexes — 5 новых индексов
- Shared http.Client для OpenRouter
- Orchestrator lock — DB I/O вне lock
- bot_commands — admin auth через context

### Совместимость
- AuthInterceptor: v1 clients (без JWT) → username из gRPC metadata
- GetChatsV2: fallback на req.Username / ctx username
- Chat/Typing/CallSession: пропускают JWT auth (legacy)

---

## ПРАВИЛА

1. ⚠️ **НЕ компилировать Android на сервере** — OOM kill
2. Версия сервера в `server.go:33`
3. userId (UUID) — всегда как ключ, НЕ username
4. Auth context → `GetUserID(ctx)`, NEVER `req.UserId`
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
```

---

## DEV vs PROD

| | Dev | Prod |
|---|-----|------|
| Порт gRPC | 50052 | 50051 |
| Порт HTTP | 8083 | 8082 |
| DB | chat_db_dev | chat_db |
| Config | .env.dev | .env |
| Версия | v1.2.0.3 | v1.2.0.3 |

---

## ДОКУМЕНТАЦИЯ

- План оптимизации: `/root/msg/doc/OPTIMIZATION_PLAN.md`
- Интеграция: `/root/msg/doc/INTEGRATION_SESSION.md`
- Задачи: `/root/msg/doc/TASKS.md`
- Pitfalls: `/root/msg/doc/PITFALLS.md`
- Release: `/root/msg/doc/RELEASE.md`

# Промпт для серверных сессий — v1.2.0.7

**Дата:** 2026-06-18 | **Ветка:** feat/1.2.0.x
**Статус:** userId migration завершена, ChatList V2 lastmessage оптимизация завершена. Фокус на стабильности.

---

## ТЕКУЩИЙ СТАТУС

| | Версия | Статус |
|---|--------|--------|
| **Сервер prod** | v1.2.0.7 | ✅ Работает на порту 50051 |
| **Сервер dev** | v1.2.0.7 | ✅ Работает на порту 50052 |

**Android:** `/root/msg.client.android` — документация там, сборка ТОЛЬКО локально (нет памяти на сервере).

---

## АРХИТЕКТУРА

### Сервер (/root/msg)
```
main.go                    — Entry point, gRPC server
server.go                  — ServerVersion = "1.2.0.6"
auth_service.go            — AuthService v1 (deprecated)
auth_service_v2.go         — AuthService v2 (JWT)
auth_interceptor.go        — gRPC Bearer token + v1 username fallback
auth_jwt.go                — JWT генерация/валидация (библиотека golang-jwt)
auth/jwt.go                — JWT для agent tokens (кастомный)
db.go                      — Database layer (~80+ CRUD методов)
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

## ЧТО СДЕЛАНО (v1.2.0.6)

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

### userId Migration (Этапы 1-3)
- UUID-колонки в `reactions`, `contacts`, `user_tokens`, `user_themes`
- `chats.participant_ids UUID[]` + GIN индекс
- UUID-based DB методы (ByUserID варианты)
- Все handlers переключены с `resolveUsername()` на `GetUserID(ctx)`
- `resolveDisplayName()` helper для отображения

### ChatList V2 Last Message Optimization
- DB миграция: `last_message_username`, `last_message_has_image` в `chats`
- SaveMessage обновляет `chats.last_message_*` при отправке
- CTE `WITH last_messages` удалён из GetUserChatsV2/GetUserChats/GetAllChats
- Быстрее GetChatsV2 — убран full scan messages

### Совместимость
- AuthInterceptor: v1 clients (без JWT) → username из gRPC metadata
- GetChatsV2: fallback на req.Username / ctx username
- Chat/Typing/CallSession: пропускают JWT auth (legacy)

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

---

## ДОКУМЕНТАЦИЯ

- Интеграция: `/root/msg/doc/INTEGRATION_SESSION.md`
- Задачи: `/root/msg/doc/TASKS.md`
- Pitfalls: `/root/msg/doc/PITFALLS.md`
- Release: `/root/msg/doc/RELEASE.md`
- Android: `/root/msg.client.android/doc/` (вся документация там)

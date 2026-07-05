# Промпт для серверных сессий — v1.3.3.0

**Дата:** 2026-07-05 | **Ветка:** feat/1.3.0.x
**Статус:** v1.3.3.0 задеплоен. v1 совместимость удалена, бот-команды в ChatV2, auth fix.

---

## ТЕКУЩИЙ СТАТУС

| | Версия | Статус |
|---|--------|--------|
| **Сервер prod** | v1.3.3.0 | ✅ Работает на порту 50051 |
| **Сервер dev** | v1.3.3.0 | ✅ Работает на порту 50052 |
| **Web клиент** | v0.1.9.0 | ✅ https://13.140.25.249/web/ |

**Android:** `/root/msg.client.android` — документация там, сборка ТОЛЬКО локально.

---

## АРХИТЕКТУРА

### Файлы сервера
```
main.go                    — Entry point, gRPC, GracefulStop
server.go                  — Server struct, helpers, version (v1.3.3.0)
hub.go                     — ChatV2 streams, call streams, conferences, online status

=== Streams ===
server_chat.go             — ChatV2 stream (messaging, typing, bot commands)
server_call.go             — CallSession stream (WebRTC signaling), conference helpers

=== AI Services v2 ===
ai_v2.go                   — AI Gateway: sessions, streaming, RAG
ai_router.go               — Hybrid router
ai_agent_executor.go       — Agent execution + tool calling
ai_provider*.go            — 9 providers (openrouter, mimo, local, hermes_acp, webhook, ws, subprocess, mcp, reve)
ai_tool*.go                — 6 tools + registry
server_ai_v2.go            — 15 gRPC handlers
rate_limiter.go            — Rate limiter + OpenRouter calls
redis_rate_limiter.go      — Redis-backed rate limiter

=== Core DB ===
db.go                      — DB connection, proxy methods
db_migrations.go           — Core schema migrations (extracted)
db_messages_v2.go          — Messages v2 CRUD
db_users.go                — User operations
db_chats.go                — Chat operations
db_chatlist_v2.go          — ChatList v2: pin/unpin/archive/search
db_ai_v2.go                — AI v2: agents, chats, messages
db_auth_devices.go         — Device CRUD
db_hermes.go               — Hermes DB

=== Handlers ===
server_chatlist_v2.go      — GetChatsV2, Pin/Unpin, Search, Archive
server_messages_v2.go      — GetHistoryV2, SendMessageV2, Edit/Delete/ReactionV2, SearchMessages
server_push.go             — FCM push, call push, conference push
server_contacts.go, server_themes.go, server_drafts.go, server_muted.go, server_favorites.go
server_profile_v2.go       — ProfileService v2
server_company.go          — CompanyService
server_management.go, server_remote.go
secret_chat.go, bot_commands.go, http_server.go, email.go, crypto.go
```

---

## ПРАВИЛА

1. ⚠️ **НЕ компилировать Android на сервере** — OOM kill
2. Версия сервера в `server.go`
3. userId (UUID) — всегда как ключ, НЕ username
4. Auth context → `GetUserID(ctx)`, NEVER `req.UserId` (T7: системный fix pending)
5. DB миграции: `IF NOT EXISTS`, NEVER `DROP`
6. Коммитить после каждого изменения
7. **Актуальный код сервера всегда доступен локально** — перед работой всегда читай файлы из `/Users/paveld/LavenderMessenger-server/`, НЕ полагайся на кеш или предыдущие версии
8. **Стабильность > фичи** — деплоим на prod, ошибки критичны
9. **Тесты обязательны** — `go test ./...` перед каждым деплоем
10. **v1 удалён** — только ChatV2, ProfileService v2, Messages v2

---

## КОМАНДЫ

```bash
# Деплой dev (с локальной машины)
./scripts/deploy-dev-local.sh

# Деплой prod (с локальной машины)
./scripts/deploy-prod-local.sh

# Proto gen
PATH=$PATH:~/go/bin protoc --go_out=gen --go_opt=paths=source_relative \
  --go-grpc_out=gen --go-grpc_opt=paths=source_relative messenger.proto

# Тесты
go test ./...
```

---

## ДОКУМЕНТАЦИЯ

- Интеграция клиентов: `doc/CLIENT_INTEGRATION.md` (единый документ)
- AI сервисы: `doc/AI_SERVICES.md`
- Архитектура: `doc/ARCHITECTURE.md`
- Подводные камни: `doc/PITFALLS.md`
- Android: `/root/msg.client.android/doc/`

---

## ИЗВЕСТНЫЕ ПРОБЛЕМЫ

- **T7 (blocked):** ~15 хендлеров используют `req.UserId` вместо `GetUserID(ctx)` — системный fix на следующей сессии
- **DeleteChat:** уже исправлен (использует JWT context)

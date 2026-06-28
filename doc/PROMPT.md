# Промпт для серверных сессий — v1.3.0.31

**Дата:** 2026-06-28 | **Ветка:** feat/1.3.0.x
**Статус:** v1.3.0.31 задеплоен. Все фичи работают.

---

## ТЕКУЩИЙ СТАТУС

| | Версия | Статус |
|---|--------|--------|
| **Сервер prod** | v1.3.0.31 | ✅ Работает на порту 50051 |
| **Сервер dev** | v1.3.0.31 | ✅ Работает на порту 50052 |
| **Web клиент** | v0.1.9.0 | ✅ https://13.140.25.249/web/ |

**Android:** `/root/msg.client.android` — документация там, сборка ТОЛЬКО локально.

---

## АРХИТЕКТУРА

### Файлы сервера
```
main.go                    — Entry point, gRPC, GracefulStop
server.go                  — Server struct, helpers (v1.3.0.31)
hub.go                     — Connection management + BroadcastV2Reaction

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
db.go, db_messages.go, db_messages_v2.go, db_users.go, db_chats.go, db_chatlist_v2.go
db_ai_v2.go, db_auth_devices.go, db_hermes.go

=== Handlers ===
server_chat.go             — Chat/Typing/CallSession streams + ChatV2 stream
server_chatlist_v2.go      — GetChatsV2, Pin/Unpin, Search, Archive
server_messages.go         — History, reactions, editing (v1 deprecated)
server_messages_v2.go      — GetHistoryV2, SendMessageV2, Edit/Delete/ReactionV2, SearchMessages
server_push.go             — FCM push, call push
server_contacts.go, server_themes.go, server_drafts.go, server_muted.go, server_favorites.go
server_profile.go, server_profile_v2.go (ProfileService), server_management.go, server_remote.go
secret_chat.go, bot_commands.go, http_server.go, email.go, crypto.go
```

---

## ПРАВИЛА

1. ⚠️ **НЕ компилировать Android на сервере** — OOM kill
2. Версия сервера в `server.go`
3. userId (UUID) — всегда как ключ, НЕ username
4. Auth context → `GetUserID(ctx)`, NEVER `req.UserId`
5. DB миграции: `IF NOT EXISTS`, NEVER `DROP`
6. Коммитить после каждого изменения
7. **Актуальный код сервера всегда доступен локально** — перед работой всегда читай файлы из `/Users/paveld/LavenderMessenger-server/`, НЕ полагайся на кеш или предыдущие версии
8. **Стабильность > фичи** — деплоим на prod, ошибки критичны

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

# Промпт для серверных сессий — v1.3.3.1

**Дата:** 2026-07-05 | **Ветка:** feat/1.3.0.x
**Статус:** v1.3.3.5 задеплоен. Push debouncing 3s per room.

---

## ТЕКУЩИЙ СТАТУС

| | Версия | Статус |
|---|--------|--------|
| **Сервер prod** | v1.3.3.5 | ✅ Работает на порту 50051 |
| **Сервер dev** | v1.3.3.5 | ✅ Работает на порту 50052 |
| **Web клиент** | v0.1.9.0 | ✅ https://13.140.25.249/web/ |

**Android:** `/root/msg.client.android` — документация там, сборка ТОЛЬКО локально.

---

## ВЫПОЛНЕНО В ЭТОЙ СЕССИИ (v1.3.3.1)

| Задача | Описание | Статус |
|--------|----------|--------|
| Password Reset | `POST /api/request-password-reset` — публичный HTTP эндпоинт для web клиента | ✅ |
| MarkRead Handler | Реализован `ChatService.MarkRead` — помечает сообщения прочитанными, рассылает `READ_ALL` | ✅ |
| Systemic Auth Fix | 15+ хендлеров теперь используют `GetUserID(ctx)` вместо `req.UserId` | ✅ |
| ChatList Fields | `chatV2RowToProto` заполняет все поля ChatInfo: is_secret, e2ee_ready, company_*, agent_* | ✅ |
| AI v2 Fields | `ChatWithAIV2Response` теперь включает `has_rag_context`, `model_used` | ✅ |

---

## ОСТАВШИЕСЯ ЗАДАЧИ НА СЛЕДУЮЩУЮ СЕССИЮ

| Задача | Описание | Приоритет | Статус |
|--------|----------|-----------|--------|
| ~~Call disconnect Bug 2~~ | HANGUP не доставляется callee если call stream неактивен. Нужен fallback через push notification. Серверная часть (`server_push.go:484-486`). | Высокий | ✅ Исправлено |
| ~~Call disconnect push fallback~~ | Когда `BroadcastCall` возвращает `delivered=false` для HANGUP — отправлять FCM push вместо потери сигнала. (`handleAbruptDisconnect` + `server_call.go:179-181`). | Высокий | ✅ Исправлено |
| ~~Push messages bug~~ | Push для сообщений не приходил — `IsUserOnline` пропускал push для пользователей в пустой комнате. Заменено на `IsUserInRoom` (`hub.go:273-288`, `server_push.go:57,149`). | Высокий | ✅ Исправлено |
| ~~Push ChannelID mismatch~~ | Сервер отправлял `lavender_messages`, клиент создавал `lavender_messages_v2`. Android 8+ игнорировал push. (`server_push.go:108,208`). | Высокий | ✅ Исправлено |
| ~~SendMessageV2 push~~ | `SendMessageV2` не отправлял push-уведомления. Добавлена push-логика для offline-получателей (`server_messages_v2.go:155-189`). | Высокий | ✅ Исправлено |
| ~~Initiate echo fix (клиент)~~ | Клиентская часть починки — `CallManager.kt` INITIATE echo больше не портит receiverId. Собрано и протестировано. | Высокий | ✅ Исправлено |

---

## АРХИТЕКТУРА

### Файлы сервера
```
main.go                    — Entry point, gRPC, GracefulStop
server.go                  — Server struct, helpers, version (v1.3.3.1)
hub.go                     — ChatV2 streams, call streams, conferences, online status

=== Streams ===
server_chat.go             — ChatV2 stream (messaging, typing, bot commands) + MarkRead
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
db_chatlist_v2.go          — ChatList v2: pin/unpin/archive/search + migrations
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
4. Auth context → `GetUserID(ctx)`, NEVER `req.UserId`
5. DB миграции: `IF NOT EXISTS`, NEVER `DROP`
6. Коммитить после каждого изменения
7. **Актуальный код сервера всегда доступен локально** — перед работой всегда читай файлы из `/Users/paveld/LavenderMessenger-server/`
8. **Стабильность > фичи** — деплоим на prod, ошибки критичны
9. **Тесты обязательны** — `go test ./...` перед каждым деплоем
10. **v1 удалён** — только ChatV2, ProfileService v2, Messages v2
11. **Деплоить через скрипты** — `./scripts/deploy-dev-local.sh` и `./scripts/deploy-prod-local.sh`

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

- ~~**Call disconnect Bug 2:**~~ ✅ Исправлено. `handleAbruptDisconnect` теперь отправляет `CALL_ENDED` push когда HANGUP не доставляется через call stream.
- ~~**Push messages bug:**~~ ✅ Исправлено. `IsUserOnline` заменён на `IsUserInRoom` — push теперь приходит когда пользователь на главном экране (пустая комната).
- ~~**Push ChannelID mismatch:**~~ ✅ Исправлено. Сервер теперь использует `lavender_messages_v2` (вместо `lavender_messages`).
- ~~**SendMessageV2 no push:**~~ ✅ Исправлено. Добавлена push-логика в `SendMessageV2` для offline-получателей.
- ~~**Initiate echo (клиент):**~~ ✅ Исправлено. `CallManager.kt:46-57` — INITIATE echo обновляет только callId, receiverId сохраняется.

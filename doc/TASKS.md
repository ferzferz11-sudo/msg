# Лава — Задачи

**Версия:** v1.3.0.36
**Ветка:** feat/1.3.0.x
**Обновлено:** 2026-06-29

---

## ✅ Серверная часть завершена (50/50 задач + 2 test suites + Hermes ACP)

Прогресс: **100%**

### Тесты (crypto + AI v2)

| Файл | Что тестирует | Тестов |
|------|--------------|--------|
| `crypto_test.go` | encrypt/decrypt (roundtrip, unicode, markers, tamper), HashPassword/CheckPassword, GenerateResetToken, getSecretKey | 22 |
| `ai_v2_test.go` | ProviderRegistry, ToolRegistry (whitelist, cache), HybridRouter (keywords, binding), AgentExecutor (mock provider, tool calls, model override), resolveAPIKey, toolCache (TTL, LRU, concurrent), isURLSafe SSRF, OpenRouter SSE streaming, query_database security, all tool interfaces | 76 |

---

## История релизов

| Версия | Описание |
|--------|----------|
| v1.3.0.31 | BroadcastGlobalV2 for online status, BroadcastShutdown for v2, reaction broadcast to ChatV2 stream, GetAdminUserList RPC, DeleteMessageV2 physical delete |
| v1.3.0.29 | Hermes ACP provider (JSON-RPC 2.0, persistent sessions), crypto + AI v2 tests (98), last_seen_at tracking, 11 preset agents |
| v1.3.0.27 | v1 messages tables dropped, v1 DB functions removed |
| v1.3.0.26 | v1 RPCs rewritten to v2 internally (backward compat), marked deprecated |
| v1.3.0.25 | Messages v1→v2 migration complete, SearchMessages RPC, dual-write removed |
| v1.3.0.19 | Messages v2: MessageV2 proto, ChatV2 stream, cursor pagination, JSONB reactions, dual-write |
| v1.3.0.18 | Reve Image Generation, image_url in ChatWithAIV2Response |
| v1.3.0.16 | RAG message indexing, JWT rotation, upload validation, DeleteProfile cascade, dead code removal |
| v1.3.0.15 | Security audit: message logging, LIKE injection, bcrypt cost |
| v1.3.0.14 | Security audit: Firebase key, HTTP auth, SSRF, SQL hardening, context timeouts |
| v1.3.0.13 | Performance optimizations: cursor pagination, DB pool tuning, AI dedup, tool caching, indexes |
| v1.3.0.12 | Unread count optimization (read_at based) |
| v1.3.0.11 | ProfileService v2 on prod + unread count fix + docs consolidation |
| v1.3.0.10 | Production RAG (Qdrant + OpenAI embeddings) |
| v1.3.0.9 | Redis rate limiter wired in + cleanup |
| v1.3.0.8 | v1 compat removal + AI gateway bug fix + logging |
| v1.3.0.7 | Redis rate limiter + billing guide |
| v1.3.0.4 | Graceful shutdown (SERVER_SHUTTINGDOWN broadcast) |
| v1.3.0.3 | Bug fixes (MarkRead, ghost AI chats) |
| v1.3.0.2 | AI v2 Usage Stats + Marketplace (7 new RPCs) |
| v1.3.0.0 | AI Services v2 (7 providers, 6 tools, 8 presets) |
| v1.2.0.11 | DB index optimization |
| v1.2.0.10 | E2EE fixes + P1/P2 optimizations |
| v1.2.0.9 | Performance optimizations (P0) |
| v1.2.0.8 | ChatList v2 last message optimization |
| v1.2.0.7 | UserInfo UUID + deploy scripts |
| v1.2.0.6 | ChatList v2 last message columns |
| v1.2.0.5 | GetChatsV2 (pagination, filters, v2 fields) |
| v1.2.0.4 | Security fixes (auth bypass, path traversal) |
| v1.2.0.3 | Security fixes (JWT, TURN, admin, user impersonation) |
| v1.2.0.0 | AuthService v2 (JWT) |
| v1.2.1.0 | ProfileService v2 (dev only) |

---

## Следующая сессия

### T1: Android — SuperAdminActivity (клиент)

См. `doc/PROMPT_ADMIN_USER_LIST.md`:
- Клиент: SuperAdminActivity обновление
- Новый RPC `GetAdminUserList` уже реализован на сервере (v1.3.0.31)

### T2: Android — DeleteMessageV2 cleanup

См. `doc/CLIENT_DELETE_MESSAGE_v2.md`:
- Убрать проверку `content_type == 'deleted'` (таких записей больше нет)
- Сервер физически удаляет записи из messages_v2

### T3: Web — Admin Panel (как на Android)

Новый RPC `GetAdminUserList` уже реализован на сервере (v1.3.0.31).
Нужно создать web-клиент:
- Доступ: только для `isSuperAdmin` пользователей
- Экран: список пользователей с пагинацией (cursor-based)
- Информация: username, avatar, email, isSuperAdmin, lastClientVersion, lastSeenAt, isOnline, lastMessageText, lastMessageTime, chatCount
- Действия: поиск, сортировка, клик на пользователя → профиль

### T4: Web — E2EE Secret Chat UI

См. `doc/PROMPT_NEXT_SESSION.md` в веб-проекте:
- Secret Chat Creation Flow
- Key Exchange UI
- Encrypted Messaging pipeline

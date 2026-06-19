# Lavender Messenger — План оптимизации

**Версия сервера:** 1.3.0.0 | **Дата:** 2026-06-19 | **Ветка:** feat/1.2.0.x

---

## AI Services v2 (v1.3.0.0) ✅

| # | Что | Статус | Файл |
|---|-----|--------|------|
| 36 | Unified AI API (ChatWithAIV2) | ✅ Done | `ai_v2.go`, `server_ai_v2.go` |
| 37 | 7 LLM providers (openrouter, local, mimo, webhook, websocket, subprocess, mcp) | ✅ Done | `ai_provider_*.go` |
| 38 | Agent CRUD (gRPC API) | ✅ Done | `server_ai_v2.go`, `db_ai_v2.go` |
| 39 | Tool system v2 (registry + 5 built-in tools) | ✅ Done | `ai_tool_*.go` |
| 40 | Hybrid router (keyword + LLM fallback) | ✅ Done | `ai_router.go` |
| 41 | MiMo integration (HTTP API provider) | ✅ Done | `ai_provider_mimo.go` |
| 42 | MCP support (stdio, JSON-RPC 2.0) | ✅ Done | `ai_provider_mcp.go` |
| 43 | 8 preset agents | ✅ Done | `db_ai_v2.go` |
| 44 | DB schema v2 (agents_v2, ai_chats_v2, ai_messages_v2) | ✅ Done | `db_ai_v2.go` |
| 45 | Remove old AI code (owl.go, hermes_orchestrator.go, etc.) | ✅ Done | — |

---

## P0 — Критические ✅ 8/8 (v1.2.0.8)

| # | Что | Файл |
|---|-----|------|
| 1 | Push N+1 → batch `SendEachForMulticast` | `server_push.go` |
| 2 | Broadcast deadlock → snapshot under lock | `hub.go` |
| 3 | isChatMuted N+1 → `getMutedRoomsSet()` batch | `db_chatlist_v2.go` |
| 4 | Hermes sessions TTL cleanup + 50 msg cap | ~~`hermes_orchestrator.go`~~ (deleted in v1.3.0.0) |
| 5 | recentMsgs cleanup goroutine | `server_chat.go` |
| 6 | OWL response saved to DB | ~~`server_ai.go`~~ (deleted in v1.3.0.0) |
| 7 | `io.LimitReader(10MB)` for OpenRouter | ~~`owl.go`~~ (deleted in v1.3.0.0) |
| 8 | JWT secret cached via `sync.Once` | `auth_jwt.go` |

---

## P1 — Важные ✅ 10/10 (v1.2.0.10)

| # | Что | Файл |
|---|-----|------|
| 9 | FCM batching + retry + invalid token cleanup | `server_push.go` |
| 10 | Rate limiter cleanup goroutine | `rate_limiter.go` |
| 11 | device_auth_log TTL (90d) | `db_auth_devices.go` |
| 12 | ResolveUserID cache (TTL 5min) | `auth_interceptor.go` |
| 13 | backfillLastMessageText SQL parentheses fix | `db.go` |
| 14 | backfillLastMessageText N+1 → JOIN LATERAL | `db.go` |
| 15 | IncrementParticipantsChatListVersion → UUID[] unnest | `db.go` |
| 16 | Stream interceptor username/device_id injection | `auth_interceptor.go` |
| 17 | ~~getAIChatManager sync.Once~~ (deleted in v1.3.0.0) | — |
| 18 | PinMessage LIKE → UUID[] | `db_chatlist_v2.go` |

---

## P2 — Улучшения ✅ 10/11 (v1.3.0.0)

| # | Что | Файл |
|---|-----|------|
| 19 | MessageRow: 10 anonymous copies → 1 named type | `db.go` |
| 20 | SaveMessage: 3 round-trips → 1 transaction | `db.go` |
| 21 | MarkReadAndCheck: UPDATE+INSERT (PK migration pending) | `db.go` |
| 22 | ~~owl.go ctx.Err() check~~ (deleted in v1.3.0.0) | — |
| 23 | main.go goroutine leak fix | `main.go` |
| 24 | gRPC GracefulStop 30s timeout | `main.go` |
| 25 | DB pool MaxIdleConns=15, ConnMaxIdleTime=5m | `db.go` |
| 26 | messages(username, created_at) index | `db.go` |
| 27 | PinMessage pagination (limit/offset) | `db_chatlist_v2.go` |
| 28 | Proto reserved fields (password, register) | ✅ Done (v1.3.0.0) — removed reserved declarations |
| 29 | IsUserOnline O(1) reverse map | `hub.go` |

---

## P3 — Частично ✅ 2/6

| # | Что | Статус |
|---|-----|--------|
| 35 | Пагинация GetAllUsers/GetAllChats/GetContacts/GetFavorites | ✅ Done (v1.2.0.11) |

---

## Осталось (2/45)

### P3

| # | Что | Сложность | Причина |
|---|-----|-----------|---------|
| 30 | Qdrant + CLIP (production RAG) | Высокая | Нужна инфраструктура |
| 33 | Unified RateLimiter (Redis) | Средняя | Нужен Redis |

### AI v2 — Доработка

| # | Что | Сложность | Причина |
|---|-----|-----------|---------|
| 48 | Production RAG (Qdrant/CLIP) | Высокая | Нужна инфраструктура |
| 49 | Usage stats + billing | Средняя | Требует UI |
| 50 | Agent marketplace (sharing, ratings) | Высокая | Требует UI + модерацию |

---

## Итого

| Категория | Кол-во | Статус |
|-----------|--------|--------|
| **AI Services v2** | 10 | ✅ 10/10 |
| **P0** | 8 | ✅ 8/8 |
| **P1** | 10 | ✅ 10/10 |
| **P2** | 11 | ✅ 10/11 |
| **P3** | 6 | ✅ 6/6 |
| **AI v2 доработка** | 5 | ✅ 2/5 |
| **Итого** | **50** | **46/50** |

Все реализованные оптимизации v2 заменяют v1 — старые AI методы удалены.

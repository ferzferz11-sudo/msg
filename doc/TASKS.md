# Лава — Задачи

**Версия:** v1.3.0.2
**Ветка:** feat/1.3.0.x
**Обновлено:** 2026-06-20

---

## ✅ v1.3.0.0 — AI Services v2 (Сессия 43)

### Что сделано

#### DB Layer (db_ai_v2.go)
- Новые таблицы: `agents_v2`, `ai_chats_v2`, `ai_messages_v2`, `ai_rate_limits`
- CRUD операции для всех таблиц
- Миграции: `DropOldAIV1()` + `MigrateAIV2()`
- 8 пресетов агентов (seeded при старте)

#### Agent Provider System (7 провайдеров)
- `ai_provider.go` — AgentProvider interface
- `ai_provider_registry.go` — Provider factory registry
- `ai_provider_openrouter.go` — OpenRouter (SSE streaming + tool calls)
- `ai_provider_local.go` — Local Hermes (subprocess)
- `ai_provider_mimo.go` — MiMo (HTTP API)
- `ai_provider_webhook.go` — Webhook (HTTP POST)
- `ai_provider_websocket.go` — WebSocket (placeholder)
- `ai_provider_subprocess.go` — Subprocess (stdin/stdout)
- `ai_provider_mcp.go` — MCP (stdio, JSON-RPC 2.0)

#### Tool System v2
- `ai_tool.go` — Tool interface
- `ai_tool_registry.go` — Tool registry
- 5 встроенных инструментов: search_messages, search_users, web_search, web_fetch, get_chat_info

#### AI Gateway + Routing
- `ai_v2.go` — AIGateway (session management, streaming, chat flow)
- `ai_router.go` — Hybrid router (keyword + LLM fallback)
- `ai_agent_executor.go` — Agent execution + tool calling loop (max 10 iterations)

#### gRPC Handlers (server_ai_v2.go)
- `ChatWithAIV2` — Unified streaming для всех типов чатов
- `CreateAIAgent`, `UpdateAIAgent`, `DeleteAIAgent` — Agent CRUD
- `GetAIAgent`, `ListAIAgents` — Agent listing
- `CloneAIAgent` — Clone agent
- `ListAITools` — List available tools

#### Proto (messenger.proto)
- 8 новых RPC в ChatService
- 15 новых message типов

#### Cleanup
- Удалены: owl.go, ai_chat_manager.go, hermes_orchestrator.go, hermes_agents.go, core/tools/executor.go, server_ai.go
- Созданы: hermes_stubs.go, rate_limiter.go (с callOpenRouterContext)

#### Documentation
- `doc/AI_V2_CLIENT_INTEGRATION.md` — Client integration guide
- `doc/AI_SERVICES_V2_PLAN.md` — Architecture plan
- `doc/PROMPT.md` — Updated for v1.3.0.0
- `doc/OPTIMIZATION_PLAN.md` — Updated with AI v2 items

#### Deploy
- Dev server: v1.3.0.0 deployed, running on port 50052
- Old AI v1 tables dropped
- New AI v2 tables created
- 8 preset agents seeded
- Tests: all passing

---

## ✅ v1.2.0.11 — DB Index + Proto Reserved

- ✅ ServerVersion: 1.2.0.10 → 1.2.0.11
- ✅ db.go: добавлен индекс `idx_messages_username_time ON messages(username, created_at)`
- ✅ messenger.proto: добавлены reserved поля

---

## ✅ v1.2.0.10 — E2EE Secret Chat Fixes

- ✅ UserInfo: `user_id` (UUID) + `is_super_admin` (bool) в GetAllUsers
- ✅ deploy-dev-local.sh — кросс-компиляция с Mac → SCP → рестарт
- ✅ CHANGELOG, doc обновлены

---

## ✅ v1.2.0.9 — P1 + P2 Performance Optimizations

- ✅ getAIChatManager sync.Once
- ✅ backfillLastMessageText SQL fix
- ✅ Stream interceptor injection
- ✅ device_auth_log TTL
- ✅ IncrementParticipantsChatListVersion → UUID[]
- ✅ Rate limiter cleanup
- ✅ ResolveUserID cache
- ✅ IsUserOnline O(1)
- ✅ DB pool tuning
- ✅ owl.go ctx.Err() check
- ✅ main.go goroutine leak fix

---

## ✅ v1.2.0.8 — P0 Performance Optimizations

- ✅ Broadcast deadlock fix
- ✅ isChatMuted N+1 batch
- ✅ Push N+1 hoisted
- ✅ Hermes sessions TTL cleanup + message cap 50
- ✅ recentMsgs cleanup
- ✅ JWT secret cached
- ✅ io.LimitReader(10MB)
- ✅ GracefulStop 30s timeout

---

## ✅ v1.3.0.1 — P3 + AI v2 доработка (Сессия 44)

### Что сделано

#### DB Split (db.go → 4 файла)
- `db.go` — 223 строки (ядро: типы, подключение, миграции, прокси-методы)
- `db_messages.go` — 245 строк (SaveMessage, GetMessages, reactions, favorites)
- `db_users.go` — 552 строки (User CRUD, themes, tokens, devices)
- `db_chats.go` — 710 строки (Chat CRUD, contacts, drafts, muted, servers, calls)

#### Deprecated v1 Auth Removal
- Удален v1 ChatStream password auth (~80 строк)
- Оставлен v1 auth service для совместимости со старыми клиентами

#### Concurrency Fixes
- Hub broadcast уже оптимизирован (snapshot under RLock)
- IsUserOnline grace period: RLock вместо Lock

#### WebSocket Provider (#46)
- Добавлен gorilla/websocket
- Реализован full-duplex streaming
- Ping/pong keepalive
- Auth header support

#### MiMo Deep Integration (#47)
- Добавлен `query_database` tool (SELECT only, security whitelist)
- Max 1000 rows, 10s timeout
- Blocked dangerous SQL keywords

### Deploy
- Dev server: v1.3.0.1 deployed, running on port 50052
- Logs: clean, no errors

---

## 📋 Активные задачи

### AI v2 — Доработка
- [x] WebSocket provider (реализация) ✅
- [x] MiMo deep integration (DB, bash) ✅
- [x] Usage stats (token tracking, ai_usage_stats table) ✅
- [x] Agent marketplace (reviews, ratings, share, install) ✅
- [ ] Production RAG (Qdrant/CLIP) — нужна инфраструктура

### P3 оптимизации
- [x] DB split (db.go → 4 файла) ✅
- [x] Concurrency fixes (hub broadcast) ✅
- [x] Удаление deprecated v1 кода (auth, chat) ✅
- [x] Usage stats (token tracking) ✅
- [x] Agent marketplace (reviews, ratings, share) ✅
- [ ] Qdrant + CLIP (production RAG) — нужна инфраструктура
- [ ] Unified RateLimiter (Redis) — нужен Redis

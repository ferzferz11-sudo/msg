# Лава — Задачи

**Версия:** v1.3.0.9
**Ветка:** feat/1.3.0.x
**Обновлено:** 2026-06-20

---

## ✅ v1.3.0.9 — Redis Rate Limiter + Cleanup (Сессия 50)

### Что сделано

#### Redis Rate Limiter — Wired In
- **rate_limiter.go** — `owlRateLimiter` (10/min) и `freeTierRateLimiter` (20/hr) заменены на `NewRedisRateLimiter` с prefix `rl:owl:` и `rl:free:`
- **bot_commands.go** — `botCmdRateLimiter` (30/min) заменён на `NewRedisRateLimiter` с prefix `rl:bot:`; удалён `botRateLimiter` struct
- **ai_v2.go** — per-agent rate limiter заменён на `NewRedisRateLimiter` с prefix `rl:ai:<id>:`
- **redis_rate_limiter.go** — методы приведены к lowercase API (`allow`/`cancel`/`remaining`/`cleanup`)
- Все limiters: автоматический fallback на in-memory если Redis недоступен

#### Cleanup
- **db_ai_v2.go** — удалена `DropOldAIV1()` (v1 таблицы уже удалены в предыдущих сессиях)
- **main.go** — удалён вызов `DropOldAIV1(db.DB)` при старте

#### Configuration
- `.env` / `.env.dev` — добавлен `REDIS_ADDR=localhost:6379`
- Бекапы .env созданы перед изменениями

#### Tests
- **bot_commands_test.go** — обновлены тесты rate limiter на `NewRedisRateLimiter`

### Статус
- Go build: ✅
- Tests: all PASS
- Dev (50052): ✅ deployed
- Prod (50051): pending

---

## ✅ v1.3.0.8 — v1 Removal + Fixes + Logging (Сессия 48)

### Что сделано

#### v1 Compat Removal
- **auth_interceptor.go** — удалены v1 fallback, `extractUsernameFromMetadata`, `ResolveUserID` + кэш
- **auth_service.go** — удалены `authServer` struct, `SignIn`/`SignUp` v1 (остался `authDB` interface)
- **auth_service_v2.go** — `authServerV2` больше не embedит `*authServer`
- **server_chats.go** — удалён `GetChats` v1
- **server.go** — удалены `resolveUserId`/`resolveUsername`
- **server_chat.go**, **server_push.go**, **server_users.go**, **secret_chat.go**, **server_profile.go** — все вызовы удалённых методов заменены

#### Bug Fixes
- **server_ai_v2.go** — `getAIV2UserID`: исправлен баг с typed context key (всегда возвращал "")
- **server_push.go** — FCM push > 4KB: добавлен `truncateForFCM()` для data payload
- **server_push.go** — `isInvalidTokenError`: добавлена строка "Requested entity was not found"
- **server_chat.go** — Call stream: `context.Canceled` gRPC error больше не логируется как error

#### Logging
- **server_ai_v2.go** — добавлены `[AI]` логи для всех AI v2 хендлеров (Chat, ListAgents, Marketplace, Rate, Reviews, Share, Install, Usage)
- **server_chat.go** — объединены "Auth success" + "Device registered" в одно сообщение
- **server_chat.go** — удалён неинформативный "Stream for %s closed"

#### Documentation
- **doc/MARKETPLACE_AGENTS_SETUP.md** — quickstart, пресеты, создание агентов, marketplace, tool calling
- **doc/ANDROID_RATE_LIMIT_PROMPT.md** — кэширование лимитов, UX при превышении
- **doc/INDEX.md** — обновлён: v1 compat удалён, новые документы
- **doc/ARCHITECTURE.md** — v1 compat секция заменена на "УДАЛЕНО"
- **doc/CLIENT_INTEGRATION.md** — версия 1.3.0.8

#### Cleanup
- Удалена `client.android/` из репозитория (2 файла)

---

## ✅ v1.3.0.3 — Bug Fixes (Сессия 46)

### Что сделано

#### MarkRead NULL user_id Fix
- **server_profile.go** — `MarkRead()`: заменён `GetUserID(ctx)` на `ResolveUserID(ctx, s.db)` для v1 username → UUID fallback

#### Ghost AI Chat Fix
- **db_chats.go** — `GetUserChats` (v1): добавлен `'ai'` в `WHERE c.type NOT IN ('ai', 'owl', 'hermes')`
- **db_chats.go** — `backfillLastMessageText`: добавлен `'ai'` в `WHERE c.type NOT IN ('ai', 'owl', 'hermes')`

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
- `ai_provider_websocket.go` — WebSocket (gorilla/websocket)
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

## 📋 Активные задачи

## Осталось (инфра-зависимые)

### Production RAG
- [ ] Qdrant + CLIP — нужна инфраструктура (Qdrant + CLIP модели на сервере)

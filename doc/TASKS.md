# Лава — Задачи

**Версия:** v1.3.0.0
**Ветка:** feat/1.2.0.x
**Обновлено:** 2026-06-19

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

## 📋 Активные задачи

### AI v2 — Доработка
- [ ] WebSocket provider (реализация)
- [ ] MiMo deep integration (DB, bash)
- [ ] Production RAG (Qdrant/CLIP)
- [ ] Usage stats + billing

### P3 оптимизации
- [ ] Qdrant + CLIP (production RAG)
- [ ] Concurrency fixes (hub broadcast)
- [ ] DB split (db.go → 4 файла)
- [ ] Unified RateLimiter (Redis)
- [ ] Удаление deprecated v1 кода (auth, chat)

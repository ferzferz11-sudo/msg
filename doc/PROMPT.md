# Промпт для серверных сессий — v1.3.0.8

**Дата:** 2026-06-20 | **Ветка:** feat/1.3.0.x
**Статус:** v1 compat полностью удалён. AI v2 + marketplace работают. Остался только Qdrant RAG.

---

## ТЕКУЩИЙ СТАТУС

| | Версия | Статус |
|---|--------|--------|
| **Сервер prod** | v1.3.0.8 | ✅ Работает на порту 50051 |
| **Сервер dev** | v1.3.0.8 | ✅ Работает на порту 50052 |

**Android:** `/root/msg.client.android` — документация там, сборка ТОЛЬКО локально.

---

## АРХИТЕКТУРА v2

### Сервер (/root/msg)
```
main.go                    — Entry point, gRPC server, GracefulStop 30s timeout
server.go                  — ServerVersion = "1.3.0.3"

=== AI Services v2 (ПОЛНОСТЬЮ ГОТОВО) ===
db_ai_v2.go                — DB layer: agents_v2, ai_chats_v2, ai_messages_v2, ai_usage_stats, agent_reviews
ai_v2.go                   — AI Gateway: session mgmt, streaming, chat flow, usage recording
ai_router.go               — Hybrid router (keyword + LLM fallback)
ai_agent_executor.go       — Agent execution + tool calling loop (returns ExecutionResult)
ai_provider.go             — AgentProvider interface + StreamUsage
ai_provider_registry.go    — Provider factory registry (7 типов)
ai_provider_openrouter.go  — OpenRouter provider (SSE streaming + usage parsing)
ai_provider_local.go       — Local Hermes provider (subprocess)
ai_provider_mimo.go        — MiMo provider (HTTP API + deep integration)
ai_provider_webhook.go     — Webhook provider (HTTP POST)
ai_provider_websocket.go   — WebSocket provider (gorilla/websocket) ✅
ai_provider_subprocess.go  — Subprocess provider (stdin/stdout)
ai_provider_mcp.go         — MCP provider (stdio, JSON-RPC 2.0)
ai_tool.go                 — Tool interface
ai_tool_registry.go        — Tool registry + 6 built-in tools
ai_tool_search_messages.go — Search messages tool
ai_tool_search_users.go    — Search users tool
ai_tool_web_search.go      — Web search tool (DuckDuckGo)
ai_tool_web_fetch.go       — URL fetch tool
ai_tool_get_chat_info.go   — Chat info tool
ai_tool_query_db.go        — DB query tool (SELECT only, admin) ✅
server_ai_v2.go            — gRPC handlers (15 RPCs)
rate_limiter.go            — Rate limiter + callOpenRouterContext
hermes_stubs.go            — Stubs for hermes_agent_service.go

=== Core (DB split) ===
db.go                      — Ядро: типы, подключение, миграции (223 строки)
db_messages.go             — Message операции (245 строк)
db_users.go                — User операции (552 строки)
db_chats.go                — Chat операции (710 строк)
db_chatlist_v2.go          — ChatList v2
db_ai_v2.go                — AI v2 DB layer
db_auth_devices.go         — Auth devices
db_hermes.go               — Hermes DB
hub.go                     — Connection management
http_server.go             — HTTP uploads + TURN
secret_chat.go             — E2EE secret chats
bot_commands.go            — Bot commands
messenger.proto            — All proto definitions
```

---

## AI SERVICES v2 — СТАТУС ВСЕХ КОМПОНЕНТОВ

### 3 типа чатов
| Тип | Описание | RPC |
|-----|----------|-----|
| `simple` | Прямой LLM (как ChatGPT) | ChatWithAIV2 |
| `agent` | Multi-agent с роутингом | ChatWithAIV2 |
| `pipeline` | RAG + tools chain | ChatWithAIV2 |

### 7 типов провайдеров
| Тип | Описание | Статус |
|-----|----------|--------|
| `openrouter` | OpenRouter API | ✅ Работает |
| `local` | Локальный LLM (hermes binary) | ✅ Работает |
| `mimo` | MiMo API (HTTP + deep integration) | ✅ Работает |
| `webhook` | HTTP webhook | ✅ Работает |
| `websocket` | WebSocket (gorilla/websocket) | ✅ Работает |
| `subprocess` | Subprocess (Python, Node) | ✅ Работает |
| `mcp` | MCP (stdio, JSON-RPC 2.0) | ✅ Работает |

### 8 пресетов
| ID | Имя | Провайдер | Tools | RAG |
|----|-----|-----------|-------|-----|
| `mimo` | MiMo | mimo | ✅ | ✅ |
| `assistant` | Assistant | openrouter | ✅ | ✅ |
| `developer` | Developer | openrouter | ✅ | ❌ |
| `devops` | DevOps | openrouter | ✅ | ❌ |
| `architect` | Architect | openrouter | ❌ | ❌ |
| `writer` | Writer | openrouter | ❌ | ❌ |
| `analyst` | Analyst | openrouter | ✅ | ✅ |
| `translator` | Translator | openrouter | ❌ | ❌ |

### 6 встроенных инструментов
- `search_messages` — поиск сообщений
- `search_users` — поиск пользователей
- `web_search` — веб-поиск (DuckDuckGo)
- `web_fetch` — загрузка URL
- `get_chat_info` — метаданные чата
- `query_database` — SQL запросы (SELECT only, admin) ✅

### Usage Stats + Marketplace
- `ai_usage_stats` — агрегированная статистика токенов (per user/agent/hour)
- `agent_reviews` — отзывы и рейтинги агентов (1-5 звёзд)
- `ShareAIAgent` / `InstallAIAgent` — шаринг агентов через share_code
- `ListMarketplaceAgents` — каталог публичных агентов с поиском
- `GetAIUsageStats` — статистика использования для клиента

---

## ПРАВИЛА

1. ⚠️ **НЕ компилировать Android на сервере** — OOM kill
2. Версия сервера в `server.go:33`
3. userId (UUID) — всегда как ключ, НЕ username
4. Auth context → `GetUserID(ctx)`, NEVER `req.UserId`
5. DB миграции: `IF NOT EXISTS`, NEVER `DROP`
6. Коммитить после каждого изменения
7. **Стабильность > фичи** — деплоим сразу на prod, ошибки критичны
8. v1 compat полностью удалён — все клиенты на v2 JWT

---

## КОМАНДЫ

```bash
# === СЕРВЕР ===
cd /root/msg && export PATH=$PATH:/usr/local/go/bin:~/go/bin

# Деплой dev (с сервера)
./scripts/deploy-dev.sh

# Деплой dev (с локальной машины)
./scripts/deploy-dev-local.sh

# Деплой prod (с локальной машины)
./scripts/deploy-prod-local.sh

# Proto gen
protoc --go_out=gen --go_opt=paths=source_relative \
  --go-grpc_out=gen --go-grpc_opt=paths=source_relative messenger.proto

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

- Интеграция AI v2: `/root/msg/doc/AI_V2_CLIENT_INTEGRATION.md`
- План реализации AI v2: `/root/msg/doc/AI_V2_IMPLEMENTATION_PLAN.md`
- Интеграция клиентов: `/root/msg/doc/CLIENT_INTEGRATION.md`
- Индекс: `/root/msg/doc/INDEX.md`
- Задачи: `/root/msg/doc/TASKS.md`
- Оптимизации: `/root/msg/doc/OPTIMIZATION_PLAN.md`
- Pitfalls: `/root/msg/doc/PITFALLS.md`
- Android: `/root/msg.client.android/doc/`

---

## ОСТАЛОСЬ (1 задача)

| # | Задача | Блокер |
|---|--------|--------|
| 30 | Qdrant + CLIP (production RAG) | Нужна инфраструктура |

**Прогресс: 49/50 задач выполнено (98%)**

---

## КЛЮЧЕВЫЕ ФАЙЛЫ ДЛЯ НОВОЙ СЕССИИ

- `doc/MARKETPLACE_AGENTS_SETUP.md` — quickstart для Android клиента
- `doc/ANDROID_RATE_LIMIT_PROMPT.md` — rate limiting для Android
- `doc/AI_V2_CLIENT_INTEGRATION.md` — полная интеграция AI v2
- `doc/TASKS.md` — список задач

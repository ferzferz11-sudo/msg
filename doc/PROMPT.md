# Промпт для серверных сессий — v1.3.0.0

**Дата:** 2026-06-19 | **Ветка:** feat/1.2.0.x
**Статус:** AI Services v2 задеплоены на dev. Фокус на тестировании и доработке.

---

## ТЕКУЩИЙ СТАТУС

| | Версия | Статус |
|---|--------|--------|
| **Сервер prod** | v1.2.0.11 | ✅ Работает на порту 50051 |
| **Сервер dev** | v1.3.0.0 | ✅ Работает на порту 50052 |

**Android:** `/root/msg.client.android` — документация там, сборка ТОЛЬКО локально.

---

## АРХИТЕКТУРА v2

### Сервер (/root/msg)
```
main.go                    — Entry point, gRPC server, GracefulStop 30s timeout
server.go                  — ServerVersion = "1.3.0.0"

=== AI Services v2 (НОВОЕ) ===
db_ai_v2.go                — DB layer: agents_v2, ai_chats_v2, ai_messages_v2
ai_v2.go                   — AI Gateway: session mgmt, streaming, chat flow
ai_router.go               — Hybrid router (keyword + LLM fallback)
ai_agent_executor.go       — Agent execution + tool calling loop
ai_provider.go             — AgentProvider interface
ai_provider_registry.go    — Provider factory registry (7 types)
ai_provider_openrouter.go  — OpenRouter provider (SSE streaming)
ai_provider_local.go       — Local Hermes provider (subprocess)
ai_provider_mimo.go        — MiMo provider (HTTP API)
ai_provider_webhook.go     — Webhook provider (HTTP POST)
ai_provider_websocket.go   — WebSocket provider
ai_provider_subprocess.go  — Subprocess provider (stdin/stdout)
ai_provider_mcp.go         — MCP provider (stdio, JSON-RPC 2.0)
ai_tool.go                 — Tool interface
ai_tool_registry.go        — Tool registry + built-in tools
ai_tool_search_messages.go — Search messages tool
ai_tool_search_users.go    — Search users tool
ai_tool_web_search.go      — Web search tool (DuckDuckGo)
ai_tool_web_fetch.go       — URL fetch tool
ai_tool_get_chat_info.go   — Chat info tool
server_ai_v2.go            — gRPC handlers (8 RPCs)
rate_limiter.go            — Rate limiter + callOpenRouterContext
hermes_stubs.go            — Stubs for hermes_agent_service.go

=== Core (unchanged) ===
auth_service_v2.go         — AuthService v2 (JWT)
auth_interceptor.go        — gRPC Bearer token
db.go                      — Database layer (~80+ CRUD методов)
hub.go                     — Connection management
http_server.go             — HTTP uploads + TURN
secret_chat.go             — E2EE secret chats
bot_commands.go            — Bot commands
messenger.proto            — All proto definitions

=== Remote Agent (unchanged) ===
hermes_agent_service.go    — Agent token management
hermes_remote_manager.go   — Remote agent management
server_remote.go           — Remote agent RPC
```

---

## AI SERVICES v2 — КЛЮЧЕВЫЕ КОНЦЕПЦИИ

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
| `mimo` | MiMo API | ✅ Работает |
| `webhook` | HTTP webhook | ✅ Работает |
| `websocket` | WebSocket | 🔨 Placeholder |
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

### 5 встроенных инструментов
- `search_messages` — поиск сообщений
- `search_users` — поиск пользователей
- `web_search` — веб-поиск (DuckDuckGo)
- `web_fetch` — загрузка URL
- `get_chat_info` — метаданные чата

---

## ПРАВИЛА

1. ⚠️ **НЕ компилировать Android на сервере** — OOM kill
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

- Интеграция: `/root/msg/doc/CLIENT_INTEGRATION.md`
- Индекс: `/root/msg/doc/INDEX.md`
- Задачи: `/root/msg/doc/TASKS.md`
- Оптимизации: `/root/msg/doc/OPTIMIZATION_PLAN.md`
- AI Services v2 план: `/root/msg/doc/AI_SERVICES_V2_PLAN.md`
- Pitfalls: `/root/msg/doc/PITFALLS.md`
- Android: `/root/msg.client.android/doc/` (вся документация там)

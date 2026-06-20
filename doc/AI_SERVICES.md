# Лава — AI Services

Документация по AI-сервисам: AI Gateway v2, провайдеры, маркетплейс.

**Обновлено:** 2026-06-20
**Версия:** v1.3.0.8

---

## Архитектура

```
┌─────────────────────────────────────────────────────────────┐
│                        SERVER                                │
│                                                             │
│  ChatWithAIV2 ──► AIGateway ──► HybridRouter               │
│                      │              │                       │
│                      ▼              ▼                       │
│                 AgentExecutor   ToolRegistry                │
│                      │              │                       │
│                      ▼              ▼                       │
│              ProviderRegistry   6 tools                     │
│              ┌─────┴─────┐                                 │
│              │           │                                 │
│         OpenRouter    MiMo    Webhook  WS  Subprocess  MCP │
└─────────────────────────────────────────────────────────────┘
```

---

## Провайдеры (7 типов)

| Тип | Файл | Описание |
|-----|------|----------|
| `openrouter` | `ai_provider_openrouter.go` | OpenRouter API (SSE streaming + tool calls) |
| `mimo` | `ai_provider_mimo.go` | MiMo API (HTTP + deep integration) |
| `local` | `ai_provider_local.go` | Локальный LLM (hermes binary) |
| `webhook` | `ai_provider_webhook.go` | HTTP webhook |
| `websocket` | `ai_provider_websocket.go` | WebSocket (gorilla/websocket) |
| `subprocess` | `ai_provider_subprocess.go` | Subprocess (stdin/stdout) |
| `mcp` | `ai_provider_mcp.go` | MCP (stdio, JSON-RPC 2.0) |

---

## Пресеты (8 агентов)

| ID | Имя | Провайдер | Модель | Tools | RAG |
|----|-----|-----------|--------|-------|-----|
| `mimo` | MiMo | mimo | mimo-auto | ✅ | ✅ |
| `assistant` | Assistant | openrouter | claude-sonnet-4 | ✅ | ✅ |
| `developer` | Developer | openrouter | claude-sonnet-4 | ✅ | ❌ |
| `devops` | DevOps | openrouter | claude-sonnet-4 | ✅ | ❌ |
| `architect` | Architect | openrouter | claude-sonnet-4 | ❌ | ❌ |
| `writer` | Writer | openrouter | gpt-4o | ❌ | ❌ |
| `analyst` | Analyst | openrouter | claude-sonnet-4 | ✅ | ✅ |
| `translator` | Translator | openrouter | gpt-4o-mini | ❌ | ❌ |

---

## Инструменты (6 шт.)

| Инструмент | Описание | Параметры |
|------------|----------|-----------|
| `search_messages` | Поиск сообщений | `query`, `chat_id?`, `limit?` |
| `search_users` | Поиск пользователей | `query`, `limit?` |
| `web_search` | Веб-поиск (DuckDuckGo) | `query` |
| `web_fetch` | Загрузка URL | `url`, `max_chars?` |
| `get_chat_info` | Метаданные чата | `chat_id` |
| `query_database` | SQL запросы (SELECT only, admin) | `query` |

---

## Tool Calling Flow

```
1. Клиент → ChatWithAIV2Request{message, agent_id}
2. Сервер стримит токены
3. Агент вызывает инструмент → ChatWithAIV2Response{tool_calls=[{id, name, args}]}
4. Клиент выполняет инструмент
5. Клиент → ChatWithAIV2Request{tool_calls=[{id, name, args, result}]}
6. Сервер продолжает стриминг
7. Готово → ChatWithAIV2Response{finished=true}
```

---

## Rate Limiting

| Лимит | Значение | Окно |
|-------|----------|------|
| По умолчанию | 10 запросов | 1 минута |
| Кастомный | N запросов | 1 минута |

При превышении: `ChatWithAIV2Response{error: "rate limit exceeded"}`

---

## Marketplace

| RPC | Описание |
|-----|----------|
| `ListMarketplaceAgents` | Каталог публичных агентов (поиск, пагинация) |
| `RateAIAgent` | Оценка агента (1-5 звёзд) |
| `GetAIAgentReviews` | Отзывы на агента |
| `ShareAIAgent` | Генерация share code |
| `InstallAIAgent` | Установка по share code |
| `GetAIAgentStats` | Статистика (installs, rating) |
| `GetAIUsageStats` | Статистика использования (токены, запросы) |

---

## gRPC Handlers (server_ai_v2.go)

| Хендлер | Описание |
|---------|----------|
| `ChatWithAIV2` | Единый AI чат (simple/agent/pipeline) |
| `CreateAIAgent` | Создание агента |
| `UpdateAIAgent` | Обновление агента |
| `DeleteAIAgent` | Удаление агента |
| `GetAIAgent` | Получение агента |
| `ListAIAgents` | Список агентов (свои + пресеты + публичные) |
| `CloneAIAgent` | Клонирование агента |
| `ListAITools` | Список доступных инструментов |

---

## Логи

Все AI v2 запросы логируются с префиксом `[AI]`:

```
[AI] ChatWithAIV2: user=xxx agent=assistant session=xxx msg=42chars
[AI] ListAgents: user=xxx includePublic=true count=8
[AI] Marketplace: query="code" limit=20 offset=0 results=3
[AI] RateAgent: agent=xxx user=xxx rating=5
```

# Лава — AI Services

Документация по AI-сервисам: AI Gateway v2, провайдеры, маркетплейс.

**Обновлено:** 2026-06-27
**Версия:** v1.3.0.27

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
| `reve` | `ai_provider_reve.go` | Reve image generation (POST /v1/image/create) |

---

## Пресеты (8 агентов)

| ID | Имя | Провайдер | Модель | Tools | RAG |
|----|-----|-----------|--------|-------|-----|
| `mimo` | MiMo | mimo | mimo-auto | ✅ | ✅ |
| `assistant` | Assistant | openrouter | meta-llama/llama-3.3-70b-instruct:free | ✅ | ✅ |
| `developer` | Developer | openrouter | qwen/qwen3-coder:free | ✅ | ❌ |
| `devops` | DevOps | openrouter | meta-llama/llama-3.3-70b-instruct:free | ✅ | ❌ |
| `architect` | Architect | openrouter | nvidia/nemotron-3-super-120b-a12b:free | ❌ | ❌ |
| `writer` | Writer | openrouter | meta-llama/llama-3.3-70b-instruct:free | ❌ | ❌ |
| `analyst` | Analyst | openrouter | qwen/qwen3-next-80b-a3b-instruct:free | ✅ | ✅ |
| `translator` | Translator | openrouter | meta-llama/llama-3.3-70b-instruct:free | ❌ | ❌ |
| `vision` | Vision | openrouter | google/gemma-4-26b-a4b-it:free | ✅ | ❌ |
| `reve` | Reve Image | reve | reve-2.0 | ❌ | ❌ |

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
| `ChatWithAIV2` | Единый AI чат (simple/agent/pipeline) — теперь в каждом токене `agent_id` и `agent_name` |
| `GetAIV2ChatHistory` | История AI чата с `agent_id`, `token_count`, `model_used` на сообщение |
| `ListAIV2Chats` | Список всех AI v2 чатов пользователя |
| `CreateAIAgent` | Создание агента |
| `UpdateAIAgent` | Обновление агента |
| `DeleteAIAgent` | Удаление агента |
| `GetAIAgent` | Получение агента |
| `ListAIAgents` | Список агентов (свои + пресеты + публичные) |
| `CloneAIAgent` | Клонирование агента |
| `ListAITools` | Список доступных инструментов |

---

## Мультиагентные чаты (клиентская маршрутизация)

Для групповых AI чатов с несколькими агентами клиент отправляет отдельные `ChatWithAIV2` запросы для каждого агента:

1. Клиент хранит список `agent_ids` для чата
2. При отправке сообщения:
   ```kotlin
   for (agentId in agentIds) {
       scope.launch {
           chatStub.chatWithAIV2(ChatWithAIV2Request {
               sessionId = sessionId
               message = message
               this.agentId = agentId
               images = images
           }).collect { response ->
               // response.agent_id и response.agent_name идентифицируют агента
           }
       }
   }
   ```
3. Каждый ответ помечен `agent_id` в `ChatWithAIV2Response`
4. UI отображает ответы от разных агентов с именами/цветами

---

## Логи

Все AI v2 запросы логируются с префиксом `[AI]`:

```
[AI] ChatWithAIV2: user=xxx agent=assistant session=xxx msg=42chars
[AI] ListAgents: user=xxx includePublic=true count=8
[AI] Marketplace: query="code" limit=20 offset=0 results=3
[AI] RateAgent: agent=xxx user=xxx rating=5
```

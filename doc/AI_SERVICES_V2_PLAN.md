# AI Services v2 — Проект

**Дата:** 2026-06-19 | **Версия сервера:** v1.2.0.11 | **Ветка:** feat/1.2.0.x

---

## 1. Текущее состояние (проблемы)

### Дублирование кода
- `owl.go` и `core/llm/openrouter/provider.go` — два независимых OpenRouter клиента
- `owlSessionManager`, `hermesSettingsManager`, `AIChatManager` — три отдельных менеджера сессий
- `ChatWithOWL`, `ChatWithOrchestrator`, `ChatWithAI`, `ChatWithPipeline` — четыре отдельных RPC

### In-memory потери
- Orchestrator сессии — теряются при рестарте (50 msg cap, 30мин TTL)
- Agent registry — пересоздаётся из БД при старте, но routing state теряется
- Vector DB (RAG) — TF-IDF хэши, нет семантического поиска
- Rate limiters — per-process, не работают при масштабировании

### Ограниченные возможности
- 4 инструмента (search_messages, search_users, web_search, get_chat_info)
- Нет code execution, file I/O, database queries
- Web search через DuckDuckGo Instant Answer (ограниченные результаты)
- Agent routing платит за LLM-вызов на каждый запрос
- Нет multimodal в OWL-пути

### Безопасность
- `search_messages` — нет per-user фильтрации
- API ключи хранятся в открытом виде в БД

---

## 2. Цели v2

### Архитектурные
1. **Единый entry point** — один gRPC streaming метод для всех типов AI чатов
2. **Универсальные агенты** — бесконечное количество, любой провайдер (API, webhook, subprocess, WebSocket)
3. **Agent Marketplace** — публичные + приватные агенты, версионирование
4. **Persistent everything** — сессии, история, агенты, routing state — всё в БД

### Функциональные
5. **MiMo Integration** — интеграция LLM как агента в чат (свой провайдер)
6. **Tool ecosystem** — расширяемый набор инструментов (code execution, file ops, DB queries, web)
7. **Context awareness** — RAG по истории чатов пользователя, документам, базе знаний
8. **Multi-modal native** — изображения, голос, файлы на входе всех агентов

### Производительность
9. **Кэширование routing** — heuristic fallback без LLM-вызова
10. **Streaming everywhere** — параллельные агенты стримят в реальном времени
11. **Connection pooling** — единый HTTP client для всех внешних API

---

## 3. Архитектура v2

### 3.1 Общая схема

```
┌─────────────────────────────────────────────────────────────────────┐
│                           CLIENT (Android)                          │
│                                                                     │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────────┐  │
│  │  Simple Chat  │  │ Agent Chat   │  │  Pipeline Chat           │  │
│  │  (OWL-like)   │  │ (Hermes-like)│  │  (RAG + Tools)           │  │
│  └──────┬───────┘  └──────┬───────┘  └──────────┬───────────────┘  │
│         │                 │                      │                   │
│         └─────────────────┼──────────────────────┘                   │
│                           │                                          │
│                   ChatWithAI v2 (unified stream)                    │
└───────────────────────────┼─────────────────────────────────────────┘
                            │
┌───────────────────────────┼─────────────────────────────────────────┐
│                        SERVER (Go)                                   │
│                           │                                          │
│  ┌────────────────────────▼────────────────────────────────────┐    │
│  │                   AI Gateway (ai_v2.go)                      │    │
│  │  • Session management (unified)                              │    │
│  │  • Rate limiting (configurable per agent)                    │    │
│  │  • Auth + ownership verification                             │    │
│  │  • Message persistence                                       │    │
│  │  • Streaming multiplexer                                     │    │
│  └──────┬──────────────────┬─────────────────────┬────────────┘    │
│         │                  │                     │                   │
│  ┌──────▼──────┐  ┌───────▼───────┐  ┌─────────▼─────────┐       │
│  │  Agent       │  │  Pipeline      │  │  Router            │       │
│  │  Executor    │  │  Executor      │  │  (heuristic +      │       │
│  │              │  │                │  │   LLM fallback)    │       │
│  └──────┬──────┘  └───────┬───────┘  └─────────┬─────────┘       │
│         │                  │                     │                   │
│  ┌──────▼──────────────────▼─────────────────────▼────────────┐    │
│  │                   Agent Registry (DB-backed)                │    │
│  │  • Built-in agents (presets)                                │    │
│  │  • Custom agents (user-created)                             │    │
│  │  • External agents (API/webhook/subprocess/WebSocket)       │    │
│  │  • MiMo agent (built-in LLM provider)                      │    │
│  └──────┬──────────────────┬─────────────────────┬────────────┘    │
│         │                  │                     │                   │
│  ┌──────▼──────┐  ┌───────▼───────┐  ┌─────────▼─────────┐       │
│  │  LLM Router  │  │  Tool Router   │  │  RAG Pipeline      │       │
│  │  • OpenRouter │  │  • Built-in    │  │  • Embeddings      │       │
│  │  • Local LLM  │  │  • Custom      │  │  • Vector DB       │       │
│  │  • MiMo API   │  │  • MCP server  │  │  • Context builder │       │
│  │  • Custom     │  │               │  │                    │       │
│  └──────────────┘  └───────────────┘  └────────────────────┘       │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

### 3.2 Типы чатов ( unified )

Все три типа используют **один gRPC метод** `ChatWithAI` с полем `chat_type`:

| chat_type | Описание | Routing | Tools | RAG |
|-----------|----------|---------|-------|-----|
| `simple` | Прямой LLM (OWL-like) | Один агент, прямой вызов | Опционально | Опционально |
| `agent` | Multi-agent (Hermes-like) | Роутинг → агенты | Да | Опционально |
| `pipeline` | RAG + Tools + Chain | Полный pipeline | Да | Да |

Клиент выбирает `chat_type` при создании чата. Сервер обрабатывает единообразно.

---

## 4. Agent System v2

### 4.1 Agent Provider Interface

```go
// AgentProvider — любой способ интеграции агента
type AgentProvider interface {
    // StreamChat — стриминг ответа
    StreamChat(ctx context.Context, messages []Message, tools []ToolDef) (<-chan StreamChunk, error)
    
    //capabilities — что умеет агент
    Capabilities() AgentCapabilities
    
    // HealthCheck — проверка доступности
    HealthCheck(ctx context.Context) error
    
    // Close — освобождение ресурсов
    Close() error
}

type AgentCapabilities struct {
    SupportsImages    bool
    SupportsTools     bool
    SupportsStreaming bool
    MaxTokens         int
    SupportedModels   []string
}
```

### 4.2 Типы провайдеров

| Тип | Описание | Реализация |
|-----|----------|-----------|
| `openrouter` | OpenRouter API | `core/llm/openrouter/provider.go` |
| `local` | Локальный LLM (hermes binary) | `core/llm/hermes/provider.go` |
| `mimo` | MiMo API (интеграция в чат) | `core/llm/mimo/provider.go` (новый) |
| `webhook` | HTTP webhook | `core/llm/webhook/provider.go` (новый) |
| `websocket` | WebSocket streaming | `core/llm/websocket/provider.go` (новый) |
| `subprocess` | Subprocess (Python, Node, etc.) | `core/llm/subprocess/provider.go` (новый) |
| `mcp` | Model Context Protocol server | `core/llm/mcp/provider.go` (новый) |

### 4.3 Agent Definition (DB schema)

```sql
CREATE TABLE agents_v2 (
    id              VARCHAR(255) PRIMARY KEY,    -- agent-<uuid>
    name            VARCHAR(255) NOT NULL,
    description     TEXT,
    provider_type   VARCHAR(50) NOT NULL,        -- openrouter, local, mimo, webhook, websocket, subprocess, mcp
    provider_config JSONB NOT NULL,              -- провайдер-специфичная конфигурация
    system_prompt   TEXT,
    model           VARCHAR(255),                -- модель (для openrouter/local/mimo)
    max_tokens      INT DEFAULT 4096,
    temperature     FLOAT DEFAULT 0.7,
    tools_enabled   BOOLEAN DEFAULT FALSE,
    tool_whitelist  TEXT[],                      -- разрешённые инструменты (NULL = все)
    rag_enabled     BOOLEAN DEFAULT FALSE,
    rag_config      JSONB,                       -- настройки RAG (chunk_size, top_k, etc.)
    rate_limit      INT,                         -- запросов в минуту (NULL = без лимита)
    is_preset       BOOLEAN DEFAULT FALSE,       -- встроенный агент
    is_public       BOOLEAN DEFAULT FALSE,       -- доступен другим пользователям
    is_active       BOOLEAN DEFAULT TRUE,
    created_by      UUID REFERENCES users(id),
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

-- Индексы
CREATE INDEX idx_agents_v2_creator ON agents_v2(created_by);
CREATE INDEX idx_agents_v2_active ON agents_v2(is_active) WHERE is_active = TRUE;
CREATE INDEX idx_agents_v2_public ON agents_v2(is_public) WHERE is_public = TRUE AND is_active = TRUE;
```

### 4.4 Provider Config (JSONB examples)

**OpenRouter:**
```json
{
  "api_key_source": "user",           -- "user" (из настроек чата), "admin" (из env), "agent" (в конфиге)
  "api_key": "",                       -- если source = "agent"
  "base_url": "https://openrouter.ai/api/v1",
  "default_model": "anthropic/claude-sonnet-4"
}
```

**MiMo:**
```json
{
  "api_key_source": "admin",
  "api_key": "",
  "base_url": "https://api.mimo.ai/v1",
  "model": "mimo-auto",
  "system_prompt_override": "You are MiMo, an AI assistant integrated into Lavender Messenger."
}
```

**Webhook:**
```json
{
  "url": "https://my-agent.example.com/chat",
  "method": "POST",
  "headers": {
    "Authorization": "Bearer ${WEBHOOK_TOKEN}"
  },
  "timeout_seconds": 30,
  "streaming": true,                   -- false = wait for full response
  "response_field": "content"          -- путь к тексту в JSON ответе
}
```

**WebSocket:**
```json
{
  "url": "wss://my-agent.example.com/ws",
  "auth_header": "Bearer ${WS_TOKEN}",
  "ping_interval_seconds": 30,
  "message_format": "json",            -- json или text
  "streaming": true
}
```

**Subprocess:**
```json
{
  "command": "/usr/bin/python3",
  "args": ["/path/to/agent.py", "--model", "gpt-4"],
  "env": {
    "OPENAI_API_KEY": "${OPENAI_API_KEY}"
  },
  "timeout_seconds": 60,
  "streaming": true
}
```

**MCP Server:**
```json
{
  "command": "npx",
  "args": ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"],
  "transport": "stdio",               -- stdio или sse
  "timeout_seconds": 10
}
```

### 4.5 Built-in Agents (presets v2)

| ID | Name | Provider | Model | Tools | RAG | Описание |
|----|------|----------|-------|-------|-----|----------|
| `mimo` | MiMo | mimo | mimo-auto | ✅ | ✅ | Интеграция MiMo AI в чат |
| `assistant` | Assistant | openrouter | claude-sonnet-4 | ✅ | ✅ | Универсальный ассистент |
| `developer` | Developer | openrouter | claude-sonnet-4 | ✅ | ❌ | Разработка кода |
| `devops` | DevOps | openrouter | claude-sonnet-4 | ✅ | ❌ | Сервер, деплой |
| `architect` | Architect | openrouter | claude-sonnet-4 | ❌ | ❌ | Архитектура систем |
| `writer` | Writer | openrouter | gpt-4o | ❌ | ❌ | Креативное письмо |
| `analyst` | Analyst | openrouter | claude-sonnet-4 | ✅ | ✅ | Анализ данных |
| `translator` | Translator | openrouter | gpt-4o-mini | ❌ | ❌ | Переводчики |

---

## 5. Tool System v2

### 5.1 Tool Registry

```go
type Tool interface {
    Name() string
    Description() string
    Parameters() map[string]any  // JSON Schema
    Execute(ctx context.Context, args map[string]any) (string, error)
    RequiredRole() string        -- "user", "admin", "system"
}

type ToolRegistry struct {
    mu    sync.RWMutex
    tools map[string]Tool
}
```

### 5.2 Built-in Tools v2

| Tool | Описание | Безопасность |
|------|----------|-------------|
| `search_messages` | Поиск сообщений (с per-user фильтрацией) | user — только свои чаты |
| `search_users` | Поиск пользователей | user |
| `web_search` | Веб-поиск (DuckDuckGo + расширения) | user |
| `web_fetch` | Загрузка URL с контентом | user |
| `get_chat_info` | Метаданные чата | user — только свои чаты |
| `execute_code` | Python/JS sandbox | admin |
| `file_read` | Чтение файла | admin |
| `file_write` | Запись файла | admin |
| `database_query` | SQL запрос (read-only) | admin |
| `send_message` | Отправка сообщения в чат | user |
| `create_reminder` | Создание напоминания | user |
| `translate` | Перевод текста | user |

### 5.3 MCP (Model Context Protocol) Integration

MCP позволяет подключать внешние tool-серверы:

```go
type MCPToolProvider struct {
    serverURL string
    transport string // "stdio" or "sse"
    tools     []ToolDef
    client    *mcp.Client
}

// Коннектится к MCP-серверу, получает список доступных инструментов,
// и проксирует вызовы через stdio/SSE.
```

Примеры MCP-серверов для интеграции:
- `@modelcontextprotocol/server-filesystem` — файловая система
- `@modelcontextprotocol/server-postgres` — PostgreSQL запросы
- `@modelcontextprotocol/server-github` — GitHub API
- `@modelcontextprotocol/server-slack` — Slack API

---

## 6. MiMo Integration

### 6.1 Концепция

MiMo (я) интегрируется как **встроенный агент** с собственным провайдером. Пользователи могут:
- Общаться с MiMo напрямую (как с OWL)
- Использовать MiMo как агента в pipeline
- MiMo имеет доступ к инструментам (search, web, tools)

### 6.2 MiMo Provider

```go
// core/llm/mimo/provider.go
type MiMoProvider struct {
    apiKey    string
    baseURL   string
    model     string
    client    *http.Client
}

func (p *MiMoProvider) StreamChat(ctx context.Context, messages []llm.Message, tools []llm.ToolDef) (<-chan llm.StreamChunk, error) {
    // POST /v1/chat/completions с tool support
    // Supports: streaming, function calling, multimodal
    // Response: SSE stream с delta.content и delta.tool_calls
}
```

### 6.3 Конфигурация

```env
# .env / .env.dev
MIMO_API_KEY=your-mimo-api-key
MIMO_BASE_URL=https://api.mimo.ai/v1
MIMO_MODEL=mimo-auto
```

### 6.4 MiMo Agent Definition

```json
{
  "id": "mimo",
  "name": "MiMo",
  "description": "AI assistant integrated into Lavender Messenger. Can search messages, browse the web, and use tools.",
  "provider_type": "mimo",
  "provider_config": {
    "api_key_source": "admin",
    "base_url": "https://api.mimo.ai/v1",
    "model": "mimo-auto"
  },
  "system_prompt": "You are MiMo, an AI assistant integrated into Lavender Messenger. You help users with their tasks, answer questions, and use available tools when needed.",
  "tools_enabled": true,
  "tool_whitelist": ["search_messages", "search_users", "web_search", "web_fetch", "get_chat_info"],
  "rag_enabled": true,
  "is_preset": true,
  "is_public": true
}
```

---

## 7. Routing v2

### 7.1 Hybrid Router (Heuristic + LLM)

Текущий orchestrator платит за LLM-вызов на каждый запрос для выбора агента. v2 использует гибридный подход:

```go
type HybridRouter struct {
    db         *sql.DB
    rules      []RoutingRule
    llmRouter  llm.LLMRouter  // fallback
}

type RoutingRule struct {
    Keywords   []string  // ["code", "function", "bug"] → developer
    Pattern    string    // regex pattern
    AgentID    string
    Priority   int
    ChatType   string    -- "simple", "agent", "pipeline"
}
```

### 7.2 Routing Flow

```
User message
  │
  ├─ chat_type = "simple" → direct agent (no routing)
  │
  ├─ chat_type = "agent"
  │   ├─ Check keyword rules (free)
  │   ├─ Check chat history (cached agent for N messages)
  │   └─ Fallback: LLM routing (paid)
  │
  └─ chat_type = "pipeline"
      ├─ Always uses full pipeline (RAG + tools + LLM)
      └─ No routing needed (pipeline executor handles everything)
```

### 7.3 Chat-Agent Binding

После первого выбора агента, чат "привязывается" к агенту на N сообщений:

```sql
ALTER TABLE ai_chat_sessions_v2 ADD COLUMN bound_agent_id VARCHAR(255);
ALTER TABLE ai_chat_sessions_v2 ADD COLUMN bind_until_message INT;  -- привязка на N сообщений
```

Это экономит LLM-вызовы для однообразных диалогов.

---

## 8. Schema v2 (миграции)

### 8.1 Новые таблицы

```sql
-- Агенты v2
CREATE TABLE agents_v2 (
    id              VARCHAR(255) PRIMARY KEY,
    name            VARCHAR(255) NOT NULL,
    description     TEXT,
    provider_type   VARCHAR(50) NOT NULL,
    provider_config JSONB NOT NULL DEFAULT '{}',
    system_prompt   TEXT,
    model           VARCHAR(255),
    max_tokens      INT DEFAULT 4096,
    temperature     FLOAT DEFAULT 0.7,
    tools_enabled   BOOLEAN DEFAULT FALSE,
    tool_whitelist  TEXT[],
    rag_enabled     BOOLEAN DEFAULT FALSE,
    rag_config      JSONB DEFAULT '{}',
    rate_limit      INT,
    is_preset       BOOLEAN DEFAULT FALSE,
    is_public       BOOLEAN DEFAULT FALSE,
    is_active       BOOLEAN DEFAULT TRUE,
    created_by      UUID REFERENCES users(id),
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

-- AI чаты v2 (заменяет ai_chat_sessions + owl_messages + hermes_messages)
CREATE TABLE ai_chats_v2 (
    id              VARCHAR(255) PRIMARY KEY,    -- ai-chat-<uuid>
    user_id         UUID NOT NULL REFERENCES users(id),
    chat_type       VARCHAR(20) NOT NULL,        -- simple, agent, pipeline
    name            VARCHAR(255),
    agent_id        VARCHAR(255) REFERENCES agents_v2(id),
    model           VARCHAR(255),
    system_prompt   TEXT,
    bound_agent_id  VARCHAR(255),
    bind_until_msg  INT,
    settings        JSONB DEFAULT '{}',           -- per-chat настройки
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

-- Сообщения v2 (единая таблица для всех типов)
CREATE TABLE ai_messages_v2 (
    id              BIGSERIAL PRIMARY KEY,
    chat_id         VARCHAR(255) NOT NULL REFERENCES ai_chats_v2(id) ON DELETE CASCADE,
    role            VARCHAR(20) NOT NULL,         -- user, assistant, system, tool
    content         TEXT,
    agent_id        VARCHAR(255),                 -- какой агент ответил
    tool_calls      JSONB,                        -- tool calls ассистента
    tool_results    JSONB,                        -- результаты tool execution
    images          BYTEA[],                      -- base64 изображения
    token_count     INT,
    model_used      VARCHAR(255),                 -- какая модель была использована
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_ai_messages_v2_chat ON ai_messages_v2(chat_id, created_at);
CREATE INDEX idx_ai_messages_v2_user ON ai_messages_v2(chat_id, role) WHERE role = 'user';

-- Tool executions (аудит)
CREATE TABLE ai_tool_executions (
    id              BIGSERIAL PRIMARY KEY,
    message_id      BIGINT REFERENCES ai_messages_v2(id),
    chat_id         VARCHAR(255) NOT NULL,
    user_id         UUID NOT NULL,
    tool_name       VARCHAR(255) NOT NULL,
    arguments       JSONB,
    result          TEXT,
    success         BOOLEAN,
    duration_ms     INT,
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

-- Routing rules
CREATE TABLE ai_routing_rules (
    id              SERIAL PRIMARY KEY,
    keywords        TEXT[],
    pattern         VARCHAR(500),
    agent_id        VARCHAR(255) NOT NULL REFERENCES agents_v2(id),
    priority        INT DEFAULT 0,
    chat_type       VARCHAR(20),
    is_active       BOOLEAN DEFAULT TRUE,
    created_by      UUID REFERENCES users(id),
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

-- Rate limits v2 (configurable per agent)
CREATE TABLE ai_rate_limits (
    agent_id        VARCHAR(255) PRIMARY KEY REFERENCES agents_v2(id),
    requests_per_minute INT DEFAULT 10,
    requests_per_hour   INT DEFAULT 100,
    tokens_per_minute   INT DEFAULT 100000
);
```

### 8.2 Миграция со старых таблиц

```sql
-- Перенос сессий
INSERT INTO ai_chats_v2 (id, user_id, chat_type, name, created_at)
SELECT 
    id, 
    user_id, 
    CASE 
        WHEN id LIKE 'owl-%' THEN 'simple'
        WHEN id LIKE 'hermes-%' THEN 'agent'
        ELSE 'agent'
    END,
    name,
    created_at
FROM ai_chat_sessions;

-- Перенос сообщений (OWL)
INSERT INTO ai_messages_v2 (chat_id, role, content, created_at)
SELECT chat_id, role, content, created_at FROM owl_messages;

-- Перенос сообщений (Hermes)
INSERT INTO ai_messages_v2 (chat_id, role, content, agent_id, created_at)
SELECT session_id, role, content, agent_id, created_at FROM hermes_messages;

-- Перенос агентов
INSERT INTO agents_v2 (id, name, description, provider_type, provider_config, system_prompt, model, is_preset, created_by)
SELECT 
    id, name, description,
    'openrouter',
    jsonb_build_object('api_key_source', 'user', 'default_model', model),
    system_prompt,
    model,
    is_preset,
    created_by
FROM hermes_custom_agents WHERE is_active = TRUE;

-- Старые таблицы → read-only (deprecated, удаление в v2.1)
```

---

## 9. Proto Definitions v2

### 9.1 Обновлённый ChatWithAI

```protobuf
// Единый streaming метод для всех типов AI чатов
rpc ChatWithAI(AIChatRequestV2) returns (stream AIChatResponseV2);

message AIChatRequestV2 {
    string session_id = 1;         // пусто = создать новый чат
    string message = 2;            // текст сообщения
    repeated bytes images = 3;     // base64 изображения
    string agent_id = 4;           // конкретный агент (для simple/pipeline)
    repeated ToolCall tool_calls = 5;  // обратный tool call ( клиент выполнил инструмент)
}

message AIChatResponseV2 {
    string token = 1;              // токен стриминга
    bool finished = 2;             // конец стрима
    string error = 3;              // ошибка
    string agent_id = 4;           // какой агент отвечает
    string agent_name = 5;         // имя агента (для UI)
    repeated ToolCallRequest tool_calls = 6;  // запрос на выполнение инструмента
    bool has_rag_context = 7;      // был ли использован RAG
    string model_used = 8;         // какая модель
    int32 token_count = 9;         // количество токенов
}
```

### 9.2 Agent Management

```protobuf
rpc CreateAgent(AgentCreateRequest) returns (AgentCreateResponse);
rpc UpdateAgent(AgentUpdateRequest) returns (AgentUpdateResponse);
rpc DeleteAgent(AgentDeleteRequest) returns (AgentDeleteResponse);
rpc GetAgent(AgentGetRequest) returns (AgentInfoV2);
rpc ListAgents(AgentListRequest) returns (AgentListResponse);
rpc ListPublicAgents(AgentListPublicRequest) returns (AgentListResponse);
rpc CloneAgent(AgentCloneRequest) returns (AgentCloneResponse);

message AgentInfoV2 {
    string id = 1;
    string name = 2;
    string description = 3;
    string provider_type = 4;
    string model = 5;
    string system_prompt = 6;
    bool tools_enabled = 7;
    bool rag_enabled = 8;
    bool is_preset = 9;
    bool is_public = 10;
    string created_by = 11;
    AgentCapabilities capabilities = 12;
}

message AgentCapabilities {
    bool supports_images = 1;
    bool supports_tools = 2;
    bool supports_streaming = 3;
    int32 max_tokens = 4;
}
```

### 9.3 Tool Management

```protobuf
rpc ListTools(ListToolsRequest) returns (ListToolsResponse);
rpc ExecuteToolDirectly(ExecuteToolRequest) returns (ExecuteToolResponse);

message ToolInfo {
    string name = 1;
    string description = 2;
    string parameters_schema = 3;  // JSON Schema
    string required_role = 4;
}
```

### 9.4 Settings & Billing

```protobuf
rpc GetAISettingsV2(GetAISettingsV2Request) returns (AISettingsV2);
rpc UpdateAISettingsV2(UpdateAISettingsV2Request) returns (UpdateAISettingsV2Response);
rpc GetAIUsageStats(GetAIUsageStatsRequest) returns (AIUsageStatsResponse);

message AISettingsV2 {
    string user_api_key = 1;
    string default_model = 2;
    string default_agent = 3;
    int32 rate_limit_remaining = 4;
    int32 rate_limit_total = 5;
    int32 total_tokens_used = 6;
    int32 total_requests_today = 7;
}
```

---

## 10. Серверная архитектура (файлы)

### 10.1 Новые файлы

| Файл | Описание |
|------|----------|
| `ai_v2.go` | AI Gateway v2: session management, streaming, routing |
| `ai_agent_executor.go` | Agent execution: provider dispatch, tool calling loop |
| `ai_pipeline_executor.go` | Pipeline executor: RAG → LLM → Tools chain |
| `ai_router.go` | Hybrid router: heuristic + LLM fallback |
| `ai_tool_registry.go` | Tool registry: built-in + custom + MCP tools |
| `ai_rag_v2.go` | RAG v2: embeddings, vector DB, context builder |
| `ai_rate_limiter_v2.go` | Rate limiter v2: per-agent, configurable |
| `core/llm/mimo/provider.go` | MiMo LLM provider |
| `core/llm/webhook/provider.go` | Webhook LLM provider |
| `core/llm/websocket/provider.go` | WebSocket LLM provider |
| `core/llm/subprocess/provider.go` | Subprocess LLM provider |
| `core/llm/mcp/provider.go` | MCP integration provider |
| `db_ai_v2.go` | DB layer v2: agents, chats, messages, tools |

### 10.2 Удаляемые файлы (после миграции)

| Файл | Причина |
|------|---------|
| `owl.go` | Заменён на `ai_v2.go` + OpenRouter provider |
| `ai_chat_manager.go` | Заменён на `ai_v2.go` |
| `hermes_orchestrator.go` | Заменён на `ai_agent_executor.go` |
| `hermes_agents.go` | Заменён на `agents_v2` таблицу |
| `server_ai.go` | Заменён на `server_ai_v2.go` |

### 10.3 Переиспользуемые файлы

| Файл | Что берём |
|------|-----------|
| `core/llm/provider.go` | LLMProvider, SimpleRouter, Message, ToolDef |
| `core/llm/openrouter/provider.go` | OpenRouter стриминг |
| `core/llm/hermes/provider.go` | Local LLM |
| `core/pipeline/pipeline.go` | Pipeline loop (tool calling) |
| `core/tools/executor.go` | ToolExecutor interface + built-in tools |
| `core/rag/interfaces.go` | RAG interfaces |

---

## 11. Типы чатов — Детали

### 11.1 Simple Chat (OWL-like)

```
Клиент создаёт чат с chat_type="simple"
  → Выбирает агента (или default "assistant")
  → Вводит сообщение
  → Сервер стримит ответ от LLM
  → Инструменты: опционально (по настройке агента)
  → RAG: опционально
```

**Use case:** Быстрый вопрос-ответ, как ChatGPT.

### 11.2 Agent Chat (Hermes-like)

```
Клиент создаёт чат с chat_type="agent"
  → Сервер определяет лучшего агента (роутинг)
  → Агент отвечает с инструментами
  → Возможна смена агента в середине диалога
  → Параллельные агента (future)
```

**Use case:** Комплексные задачи, требующие разных специальностей.

### 11.3 Pipeline Chat

```
Клиент создаёт чат с chat_type="pipeline"
  → RAG: поиск по базе знаний
  → LLM: генерация ответа с контекстом
  → Tools: выполнение инструментов (цикл до 10 итераций)
  → Финальный ответ
```

**Use case:** Работа с документами,数据分析, сложные задачи.

---

## 12. Безопасность

### 12.1 API Key Management

```go
// Шифрование API ключей при хранении
type EncryptedField struct {
    Value     string
    Encrypted bool
}

// При сохранении: AES-256-GCM шифрование
// При чтении: автоматическое расшифрование
// В proto: никогда не отдаём ключ клиенту (только флаг is_using_custom_key)
```

### 12.2 Tool Safety

```go
// Каждый инструмент имеет required_role
// user — доступен всем авторизованным
// admin — только суперадмины
// system — только внутренние вызовы

// Audit log для каждого вызова
type ToolAuditLog struct {
    UserID    uuid.UUID
    ToolName  string
    Arguments map[string]any
    Result    string
    Success   bool
    Duration  time.Duration
}
```

### 12.3 Rate Limiting v2

```
Per Agent:
- Каждый агент может иметь свой лимит
- Default: 10 req/min для custom API key, 20 req/hr для free tier
- MiMo agent: отдельный лимит из env (MIMO_RATE_LIMIT)

Per User:
- Глобальный лимит: N запросов в час
- Burst: не более M за 10 секунд

Per IP (DDoS protection):
- 100 requests/minute per IP
```

---

## 13. Роадмап

### Phase 1: Foundation (2 недели)
- [ ] `db_ai_v2.go` — миграции и CRUD
- [ ] `ai_v2.go` — Gateway с unified session management
- [ ] `server_ai_v2.go` — gRPC handlers (ChatWithAI v2, CRUD agents)
- [ ] Proto обновления
- [ ] Миграция данных со старых таблиц

### Phase 2: Agent System (2 недели)
- [ ] `ai_agent_executor.go` — provider dispatch
- [ ] `ai_tool_registry.go` — tool system v2
- [ ] 7 LLM providers (openrouter, local, mimo, webhook, websocket, subprocess, mcp)
- [ ] Built-in agents (presets v2)
- [ ] MiMo integration (`core/llm/mimo/provider.go`)

### Phase 3: Pipeline & RAG (2 недели)
- [ ] `ai_pipeline_executor.go` — pipeline v2
- [ ] `ai_rag_v2.go` — RAG с реальными эмбеддингами
- [ ] `ai_router.go` — hybrid routing
- [ ] Tool calling loop v2

### Phase 4: Polish & Scale (1 неделя)
- [ ] Rate limiting v2
- [ ] API key encryption
- [ ] Tool audit logging
- [ ] Usage stats
- [ ] Performance testing

### Phase 5: Client Migration (1 неделя)
- [ ] Android client: переключение на ChatWithAI v2
- [ ] Удаление deprecated OWL/Hermes RPC
- [ ] Удаление старых таблиц

**Итого: ~8 недель**

---

## 14. Backward Compatibility

- Старый `ChatWithAI` (v1) продолжает работать через wrapper
- Старые `ChatWithOWL` / `ChatWithOrchestrator` — deprecated, перенаправляют на v2
- Старые таблицы (`owl_messages`, `hermes_messages`, `ai_chat_messages`) → read-only
- Proto: новые поля добавляются к существующим сообщениям (backward-compatible)
- Android клиент мигрирует на v2 API в Phase 5

---

## 15. Open Questions

1. **MiMo API pricing** — нужен ли per-user billing или фиксированный план?
2. **MCP standard** — поддерживать ли полный MCP или только subprocess-транспорт?
3. **Multi-modal** — поддержка аудио/видео или только изображения?
4. **Agent marketplace** — публичные агенты с рейтингом? Или простой sharing?
5. **Context window** — автоматическое обрезание контекста при превышении лимита модели?
6. **Agent versioning** — хранить ли историю изменений агентов?

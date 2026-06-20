# AI Services v2 — Client Integration Guide

**Сервер:** v1.3.0.2 | **Протокол:** gRPC + Protocol Buffers | **Дата:** 2026-06-20

Документ описывает новые AI RPC методы v2. Заменяет старые `ChatWithAI`, `ChatWithOWL`, `ChatWithOrchestrator`, `ChatWithPipeline`.

---

## Обзор

AI Services v2提供единый API для всех типов AI чатов:

| Тип чата | Описание | Когда использовать |
|----------|----------|-------------------|
| `simple` | Прямой LLM (как ChatGPT) | Быстрые вопросы-ответы |
| `agent` | Multi-agent с роутингом | Задачи требующие разных специальностей |
| `pipeline` | RAG + tools chain | Работа с документами, сложные задачи |

**Ключевые изменения v2:**
- Один RPC `ChatWithAIV2` для всех типов
- Агенты создаются через gRPC API (бесконечное количество)
- 7 типов провайдеров: openrouter, local, mimo, webhook, websocket, subprocess, mcp
- Tool calling loop (агент может вызывать инструменты)
- Встроенные инструменты: search_messages, search_users, web_search, web_fetch, get_chat_info

---

## Новые RPC методы

### ChatWithAIV2 — Основной AI чат

```protobuf
rpc ChatWithAIV2(ChatWithAIV2Request) returns (stream ChatWithAIV2Response);
```

**Запрос:**
```protobuf
message ChatWithAIV2Request {
  string session_id = 1;        // пусто = создать новый чат
  string message = 2;           // текст сообщения
  repeated bytes images = 3;    // base64 изображения (multimodal)
  string agent_id = 4;          // принудительно выбрать агента
  repeated ToolCallV2 tool_calls = 5;  // результаты tool execution (для агентного цикла)
}
```

**Ответ (стриминг):**
```protobuf
message ChatWithAIV2Response {
  string token = 1;             // токен стриминга
  bool finished = 2;            // конец стрима
  string error = 3;             // ошибка
  string agent_id = 4;          // какой агент ответил
  string agent_name = 5;        // имя агента (для UI)
  repeated ToolCallRequestV2 tool_calls = 6;  // запрос на выполнение инструмента
  bool has_rag_context = 7;     // был ли использован RAG
  string model_used = 8;        // какая модель
  int32 token_count = 9;        // количество токенов
}
```

**Flow:**
```
1. Клиент отправляет ChatWithAIV2Request{message="...", agent_id="assistant"}
2. Сервер стримит ChatWithAIV2Response{token="Привет"} по одному слову
3. Если агент хочет вызвать инструмент:
   - Сервер отправляет ChatWithAIV2Response{tool_calls=[{id, name, arguments}]}
   - Клиент выполняет инструмент и отправляет результат обратно
   - Сервер продолжает стриминг с результатом
4. Когда ответ готов: ChatWithAIV2Response{finished=true}
```

**Примеры:**

Создать новый чат:
```kotlin
val request = ChatWithAIV2Request.newBuilder()
    .setMessage("Привет! Помоги с кодом")
    .setAgentId("developer")
    .build()
```

Продолжить существующий чат:
```kotlin
val request = ChatWithAIV2Request.newBuilder()
    .setSessionId("ai-chat-abc123")
    .setMessage("А теперь добавь тесты")
    .build()
```

С изображением:
```kotlin
val request = ChatWithAIV2Request.newBuilder()
    .setMessage("Что на этом скриншоте?")
    .addImages(imageBytes)
    .build()
```

---

### Agent CRUD — Управление агентами

#### CreateAIAgent

```protobuf
rpc CreateAIAgent(CreateAIAgentRequest) returns (CreateAIAgentResponse);

message CreateAIAgentRequest {
  string name = 1;                    // имя агента
  string description = 2;             // описание
  string provider_type = 3;           // openrouter, mimo, webhook, websocket, subprocess, mcp
  string provider_config = 4;         // JSON: конфигурация провайдера
  string system_prompt = 5;           // системный промпт
  string model = 6;                   // модель
  int32 max_tokens = 7;               // макс. токенов (default 4096)
  float temperature = 8;              // температура (default 0.7)
  bool tools_enabled = 9;             // включены ли инструменты
  repeated string tool_whitelist = 10; // список разрешённых инструментов (пусто = все)
  bool rag_enabled = 11;              // включён ли RAG
  string rag_config = 12;             // JSON: настройки RAG
  int32 rate_limit = 13;              // лимит запросов в минуту (0 = без лимита)
  bool is_public = 14;                // доступен другим пользователям
}

message CreateAIAgentResponse {
  bool success = 1;
  string agent_id = 2;                // ID созданного агента
  string error = 3;
}
```

**Пример:**
```kotlin
val config = """{"api_key_source": "user", "default_model": "anthropic/claude-sonnet-4"}"""
val request = CreateAIAgentRequest.newBuilder()
    .setName("My Custom Agent")
    .setDescription("Custom assistant for my tasks")
    .setProviderType("openrouter")
    .setProviderConfig(config)
    .setSystemPrompt("You are a helpful assistant specialized in...")
    .setModel("anthropic/claude-sonnet-4")
    .setMaxTokens(4096)
    .setTemperature(0.7f)
    .setToolsEnabled(true)
    .build()
```

#### UpdateAIAgent

```protobuf
rpc UpdateAIAgent(UpdateAIAgentRequest) returns (UpdateAIAgentResponse);

message UpdateAIAgentRequest {
  string agent_id = 1;
  string name = 2;
  string description = 3;
  string provider_config = 4;      // JSON
  string system_prompt = 5;
  string model = 6;
  int32 max_tokens = 7;
  float temperature = 8;
  bool tools_enabled = 9;
  repeated string tool_whitelist = 10;
  bool rag_enabled = 11;
  string rag_config = 12;          // JSON
  int32 rate_limit = 13;
  bool is_public = 14;
}
```

#### DeleteAIAgent

```protobuf
rpc DeleteAIAgent(DeleteAIAgentRequest) returns (DeleteAIAgentResponse);

message DeleteAIAgentRequest {
  string agent_id = 1;
}
```

#### GetAIAgent

```protobuf
rpc GetAIAgent(GetAIAgentRequest) returns (GetAIAgentResponse);

message GetAIAgentRequest {
  string agent_id = 1;
}

message GetAIAgentResponse {
  AgentInfoV2 agent = 1;
}
```

#### ListAIAgents

```protobuf
rpc ListAIAgents(ListAIAgentsRequest) returns (ListAIAgentsResponse);

message ListAIAgentsRequest {
  bool include_public = 1;  // включить публичные агенты других пользователей
}

message ListAIAgentsResponse {
  repeated AgentInfoV2 agents = 1;
}
```

**AgentInfoV2:**
```protobuf
message AgentInfoV2 {
  string id = 1;
  string name = 2;
  string description = 3;
  string provider_type = 4;
  string model = 5;
  string system_prompt = 6;
  bool tools_enabled = 7;
  bool rag_enabled = 8;
  bool is_preset = 9;           // встроенный агент
  bool is_public = 10;
  int32 max_tokens = 11;
  float temperature = 12;
  string created_by = 13;
  AgentCapabilitiesV2 capabilities = 14;
}

message AgentCapabilitiesV2 {
  bool supports_images = 1;
  bool supports_tools = 2;
  bool supports_streaming = 3;
  int32 max_tokens = 4;
}
```

#### CloneAIAgent

```protobuf
rpc CloneAIAgent(CloneAIAgentRequest) returns (CloneAIAgentResponse);

message CloneAIAgentRequest {
  string agent_id = 1;
  string new_name = 2;
}
```

---

### ListAITools — Список доступных инструментов

```protobuf
rpc ListAITools(ListAIToolsRequest) returns (ListAIToolsResponse);

message ListAIToolsRequest {}

message ListAIToolsResponse {
  repeated ToolInfoV2 tools = 1;
}

message ToolInfoV2 {
  string name = 1;
  string description = 2;
  string parameters_schema = 3;   // JSON Schema параметров
  string required_role = 4;       // user, admin, system
}
```

---

## Tool Calling (Агентный цикл)

Когда агент с `tools_enabled = true` решает вызвать инструмент:

1. Сервер отправляет `ChatWithAIV2Response{tool_calls=[{id, name, arguments}]}`
2. Клиент **выполняет** инструмент (локально или через свой API)
3. Клиент отправляет результат обратно:
```protobuf
ChatWithAIV2Request{
  session_id = "ai-chat-xxx",
  tool_calls = [{
    id = "call_abc123",
    name = "web_search",
    arguments = "{\"query\": \"Go concurrency\"}",
    result = "Go goroutines are lightweight threads..."
  }]
}
```
4. Сервер продолжает стриминг с учётом результата

**Встроенные инструменты:**

| Инструмент | Описание | Параметры |
|------------|----------|-----------|
| `search_messages` | Поиск сообщений | `query`, `chat_id?`, `limit?` |
| `search_users` | Поиск пользователей | `query`, `limit?` |
| `web_search` | Веб-поиск | `query` |
| `web_fetch` | Загрузка URL | `url`, `max_chars?` |
| `get_chat_info` | Метаданные чата | `chat_id` |

---

## Provider Config (JSON)

### OpenRouter
```json
{
  "api_key_source": "user",
  "default_model": "anthropic/claude-sonnet-4"
}
```

### MiMo
```json
{
  "api_key_source": "admin",
  "base_url": "https://api.mimo.ai/v1",
  "model": "mimo-auto"
}
```

### Webhook
```json
{
  "url": "https://my-agent.example.com/chat",
  "method": "POST",
  "headers": {"Authorization": "Bearer token"},
  "timeout_seconds": 30,
  "streaming": true
}
```

### Subprocess
```json
{
  "command": "/usr/bin/python3",
  "args": ["/path/to/agent.py"],
  "env": {"OPENAI_API_KEY": "sk-..."},
  "timeout_seconds": 60,
  "streaming": true
}
```

### MCP
```json
{
  "command": "npx",
  "args": ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"],
  "transport": "stdio",
  "timeout_seconds": 10
}
```

---

## Встроенные агенты (Пресеты)

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

## Примеры использования

### 1. Быстрый вопрос (simple chat)

```kotlin
// Создать simple чат с агентом assistant
val chatStream = chatService.chatWithAIV2(
    ChatWithAIV2Request.newBuilder()
        .setMessage("Что такое goroutine в Go?")
        .setAgentId("assistant")
        .build()
)

for (response in chatStream) {
    if (response.error.isNotEmpty()) {
        // Handle error
    }
    if (response.token.isNotEmpty()) {
        // Append to UI: textView.append(response.token)
    }
    if (response.finished) {
        // Stream complete
        val chatId = response.agentId  // использовать для следующего сообщения
    }
}
```

### 2. Работа с кодом (developer agent)

```kotlin
val chatStream = chatService.chatWithAIV2(
    ChatWithAIV2Request.newBuilder()
        .setMessage("Напиши функцию на Go для парсинга CSV")
        .setAgentId("developer")
        .build()
)
```

### 3. Агент с инструментами

```kotlin
// Первое сообщение
val stream1 = chatService.chatWithAIV2(
    ChatWithAIV2Request.newBuilder()
        .setSessionId("ai-chat-xxx")
        .setMessage("Найди последние новости про Go 1.22")
        .setAgentId("assistant")
        .build()
)

for (response in stream1) {
    if (response.toolCallsCount > 0) {
        // Агент хочет вызвать инструмент
        for (toolCall in response.toolCallsList) {
            // Выполнить инструмент
            val result = executeTool(toolCall.name, toolCall.arguments)
            
            // Отправить результат обратно
            val followUp = ChatWithAIV2Request.newBuilder()
                .setSessionId("ai-chat-xxx")
                .addToolCalls(ToolCallV2.newBuilder()
                    .setId(toolCall.id)
                    .setName(toolCall.name)
                    .setArguments(toolCall.arguments)
                    .setResult(result)
                    .build())
                .build()
            
            val stream2 = chatService.chatWithAIV2(followUp)
            // Обработать stream2...
        }
    }
}
```

### 4. Создание своего агента

```kotlin
val agent = chatService.createAIAgent(
    CreateAIAgentRequest.newBuilder()
        .setName("My Support Bot")
        .setDescription("Поддержка для моего продукта")
        .setProviderType("openrouter")
        .setProviderConfig("""{"api_key_source": "user"}""")
        .setSystemPrompt("Ты бот поддержки. Отвечай дружелюбно и по делу.")
        .setModel("anthropic/claude-sonnet-4")
        .setMaxTokens(2048)
        .setToolsEnabled(true)
        .build()
)

val agentId = agent.agentId  // использовать в ChatWithAIV2
```

### 5. Получить список агентов

```kotlin
val agents = chatService.listAIAgents(
    ListAIAgentsRequest.newBuilder()
        .setIncludePublic(true)
        .build()
)

for (agent in agents.agentsList) {
    Log.d("Agent", "${agent.name} (${agent.providerType}) - tools: ${agent.toolsEnabled}")
}
```

### 6. Получить список инструментов

```kotlin
val tools = chatService.listAITools(ListAIToolsRequest.newBuilder().build())

for (tool in tools.toolsList) {
    Log.d("Tool", "${tool.name}: ${tool.description}")
}
```

---

## Миграция с v1

### Удалённые методы (больше не существуют)
- `ChatWithOWL` → используйте `ChatWithAIV2` с `agent_id="assistant"`
- `ChatWithOrchestrator` → используйте `ChatWithAIV2` с `chat_type="agent"`
- `ChatWithPipeline` → используйте `ChatWithAIV2` с `chat_type="pipeline"`
- `ChatWithAI` (v1) → используйте `ChatWithAIV2`
- `CreateAgent`, `UpdateAgent`, `DeleteAgent`, `ListAgents` (старые) → используйте `CreateAIAgent`, `UpdateAIAgent`, `DeleteAIAgent`, `ListAIAgents`
- `CreateHermesSession`, `DeleteHermesSession` → sessions создаются автоматически
- `GetOwlSettings`, `UpdateOwlSettings` → настройки в `provider_config` агента

### Таблица миграции

| v1 метод | v2替代 | Изменения |
|----------|--------|-----------|
| `ChatWithOWL` | `ChatWithAIV2` | session_id вместо chat_id, agent_id вместо model |
| `ChatWithOrchestrator` | `ChatWithAIV2` | session_id вместо chat_id |
| `ChatWithPipeline` | `ChatWithAIV2` | session_id вместо chat_id |
| `GetAIChats` | `ListAIAgents` + `ChatWithAIV2` | Чаты создаются через ChatWithAIV2 |
| `CreateAgent` | `CreateAIAgent` | provider_type + provider_config вместо preset_id |
| `ListAgents` | `ListAIAgents` | include_public вместо user_id |

---

## Границы и лимиты

| Параметр | Значение |
|----------|----------|
| Макс. сообщений в истории | 50 |
| Макс. итераций tool calling | 10 |
| Rate limit (custom key) | 10 req/min |
| Rate limit (free tier) | 20 req/hour |
| Макс. изображений за запрос | 5 |
| Макс. размер изображения | 10MB |

---

## Marketplace + Usage Stats (NEW)

### RateAIAgent — Оценка агента
```protobuf
rpc RateAIAgent(RateAIAgentRequest) returns (RateAIAgentResponse);
```
Запрос: `{agent_id, rating: 1-5, review: "optional text"}`
Ответ: `{success, avg_rating, review_count}`

### ListMarketplaceAgents — Каталог публичных агентов
```protobuf
rpc ListMarketplaceAgents(ListMarketplaceAgentsRequest) returns (ListMarketplaceAgentsResponse);
```
Запрос: `{query: "search text", limit, offset}`
Ответ: `{agents: AgentInfoV2[], total}`

### ShareAIAgent — Генерация ссылки
```protobuf
rpc ShareAIAgent(ShareAIAgentRequest) returns (ShareAIAgentResponse);
```
Запрос: `{agent_id}`
Ответ: `{success, share_code}`

### InstallAIAgent — Установка по share code
```protobuf
rpc InstallAIAgent(InstallAIAgentRequest) returns (InstallAIAgentResponse);
```
Запрос: `{share_code, new_name: "optional rename"}`
Ответ: `{success, agent_id}`

### GetAIUsageStats — Статистика использования
```protobuf
rpc GetAIUsageStats(GetAIUsageStatsRequest) returns (GetAIUsageStatsResponse);
```
Запрос: `{}` (берёт текущего пользователя из JWT)
Ответ: `{stats: [{agent_id, agent_name, total_tokens, request_count, period_start}], total_tokens, total_requests}`

### GetAIAgentStats — Статистика агента
```protobuf
rpc GetAIAgentStats(GetAIAgentStatsRequest) returns (GetAIAgentStatsResponse);
```
Запрос: `{agent_id}`
Ответ: `{install_count, avg_rating, review_count}`

### GetAIAgentReviews — Отзывы
```protobuf
rpc GetAIAgentReviews(GetAIAgentReviewsRequest) returns (GetAIAgentReviewsResponse);
```
Запрос: `{agent_id, limit}`
Ответ: `{reviews: [{user_id, rating, review, created_at}], avg_rating, review_count}`

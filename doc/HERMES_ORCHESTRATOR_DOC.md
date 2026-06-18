# Hermes Multi-Agent Orchestrator — Документация реализации

## Обзор

Hermes Orchestrator — это центральный компонент AI-системы мессенджера Лава. Он принимает запросы пользователей через gRPC, анализирует их, маршрутизирует к подходящим AI-агентам и стримит ответы обратно.

## Архитектура

```
┌─────────────────────────────────────────────────────────────────┐│                        Android Client                          ││                                                                 ││  HermesChatActivity ←─── Unified Chat Widget ←─── ChatMessage ││                                            │                    ││                                     ChatMessageAdapter          │└─────────────────────────┬───────────────────────────────────────┘                          │ gRPC                          │┌─────────────────────────▼───────────────────────────────────────┐│                        Dev Server (50052)                       ││                                                                 ││  ┌─────────────────────────────────────────────────────────┐   ││  │                   ChatService (gRPC)                     │   ││  │                                                         │   ││  │  ChatWithPipeline ────► Orchestrator.ProcessWithPipeline │   ││  │  Orchestrate ─────────► Orchestrator.Orchestrate        │   ││  │  HermesAgentService ◄── hermes-agent daemon             │   ││  └─────────────────────────┬───────────────────────────────┘   ││                              │                                   ││  ┌─────────────────────────▼───────────────────────────────┐   ││  │                  Hermes Orchestrator                     │   ││  │                                                         │   ││  │  1. analyzeRequest() ──► LLM выбирает агента(ов)       │   ││  │  2. route to agent(s)                                   │   ││  │     ├─ single: один агент                               │   ││  │     ├─ parallel: несколько параллельно                  │   ││  │     └─ pipeline: цепочка (output N → input N+1)        │   ││  │  3. stream response → Android                           │   ││  └─────────────────────────────────────────────────────────┘   ││                              │                                   ││         ┌────────────────────┼────────────────────┐              ││         ▼                    ▼                    ▼              ││  ┌──────────────┐   ┌──────────────┐   ┌──────────────────┐    ││  │ LLM Router   │   │  RAG Pipeline │   │  Tool Executor   │    ││  │              │   │              │   │                  │    ││  │ OpenRouter   │   │ TF-IDF       │   │ search_messages  │    ││  │ Hermes local │   │ (384 dim)    │   │ search_users     │    ││  └──────────────┘   └──────────────┘   │ web_search       │    ││                                          │ get_chat_info    │    ││                                          └──────────────────┘    ││                                                                     ││  ┌──────────────────────────────────────────────────────────────┐  ││  │                    Agent Registry                             │  ││  │                                                              │  ││  │  8 preset agents:                   Custom agents (from DB): │  ││  │  ├─ hermes-developer (💻)           ├─ custom-{user}-{preset}│  ││  │  ├─ hermes-devops (🔧)              └─ ...                   │  ││  │  ├─ hermes-architect (🏗️)           │                        │  ││  │  ├─ hermes-support (🎧)             │                        │  ││  │  ├─ hermes-qa (🧪)                  │                        │  ││  │  ├─ hermes-analyst (📊)             │                        │  ││  │  ├─ hermes-security (🔒)            │                        │  ││  │  └─ hermes-owl (🦉) fallback        │                        │  ││  └──────────────────────────────────────────────────────────────┘  ││                                                                     ││  ┌──────────────────────────────────────────────────────────────┐  ││  │                 Remote Agent Manager                          │  ││  │                                                              │  ││  │  SendTask() ──► gRPC stream ──► hermes-agent daemon         │  ││  │  WaitForResult() ◄── callback ◄── task result               │  ││  │  healthCheckLoop() ──► 30s heartbeat check                  │  ││  └──────────────────────────────────────────────────────────────┘  │└─────────────────────────────────────────────────────────────────────┘
```

## Ключевые компоненты

### 1. Orchestrator (`hermes_orchestrator.go`)

Центральный оркестратор. Главный метод: `Orchestrate()`.

**Flow:**
```
User Request
    │
    ▼
getOrCreateSession() ──► загрузить/создать сессию в памяти + БД
    │
    ▼
analyzeRequest() ──► LLM анализирует запрос, выбирает агента(ов)
    │                   Ответ: {"mode": "single", "agents": ["developer"], "reason": "..."}
    │
    ▼
Route decision:
    ├─ agent in local registry → runSingleAgent / runParallelAgents / runPipelineAgents
    ├─ agent in remote registry → runRemoteAgent → SendTask()
    └─ no agents found → fallback to hermes-owl
```

**Маршрутизация:**
- `analyzeRequest()` отправляет промпт в OpenRouter со списком доступных агентов
- LLM возвращает JSON с выбранными агентами и режимом (single/parallel/pipeline)
- `parseRoutingDecision()` валидирует ответ, фильтрует несуществующих агентов

**Режимы выполнения:**
- `runSingleAgent()` — один агент, стриминг ответа через callback
- `runParallelAgents()` — N агентов параллельно (goroutine + WaitGroup), агрегация результатов
- `runPipelineAgents()` — цепочка: output агента N → input агента N+1

### 2. Agent Registry (`hermes_agents.go`)

Реестр всех доступных AI-агентов.

**Preset agents (8):**
| ID | Name | Emoji | Описание |
|---|---|---|---|
| hermes-developer | Developer | 💻 | Пишет, рефакторит, отлаживает код |
| hermes-devops | DevOps | 🔧 | Сервер, деплой, мониторинг |
| hermes-architect | Architect | 🏗️ | Проектирование систем |
| hermes-support | Support | 🎧 | Помощь пользователям |
| hermes-qa | QA Engineer | 🧪 | Тестирование, поиск багов |
| hermes-analyst | Analyst | 📊 | Анализ данных, метрики |
| hermes-security | Security | 🔒 | Security review |
| hermes-owl | OWL AI | 🦶 | Универсальный fallback |

**Custom agents:**
- Хранятся в таблице `hermes_custom_agents`
- Создаются из пресетов с кастомными промптами
- CRUD: `CreateCustomAgent()`, `UpdateCustomAgent()`, `DeleteCustomAgent()`

### 3. LLM Router (`core/llm/`)

Маршрутизация между LLM-провайдерами.

**Провайдеры:**
- `OpenRouter` (default) — SSE streaming, tool calls, multimodal images
- `Hermes local` (prefix=local/) — `hermes chat -q --quiet`, stateless

**Маршрутизация:**
```gollmRouter.Register(llm.RouteRule{
    ModelPrefix: "openrouter/",  // priority=10
    Provider: openRouterProvider,
})
llmRouter.Register(llm.RouteRule{
    ModelPrefix: "local/",       // priority=20
    Provider: hermesProvider,
})
```

### 4. RAG Pipeline (`core/rag/`)

Векторный поиск контекста.

**Интерфейсы:**
- `EmbeddingService` — генерация эмбеддингов
- `VectorSearch` — поиск похожих векторов
- `RAGPipeline` — полный пайплайн: embedding → search → augmented prompt

**Текущая реализация:**
- In-memory TF-IDF (384 dim), cosine similarity
- Unit тесты: 4 теста, все PASS

**Production план:**
- Qdrant + CLIP для мультимодальных эмбеддингов

### 5. Tool Executor (`core/tools/`)

Выполнение tool calls на стороне Go.

**Инструменты:**
- `search_messages` — ILIKE по messages таблице
- `search_users` — поиск по username/display_name/phone
- `web_search` — DuckDuckGo Instant Answer API
- `get_chat_info` — имя чата, тип, количество участников

### 6. Pipeline (`core/pipeline/`)

Полный пайплайн: RAG → LLM → Tool Calling.

**Flow:**
```
User Message
    │
    ▼
RAG BuildContext() ──► найти релевантные чанки
    │
    ▼
Augmented Prompt = user_message + rag_context
    │
    ▼
LLM Stream ──► Tool Calling Loop (max 10 iter)
    │
    ▼
Final Response
```

**Tool Calling Loop:**
- Адаптивный: продолжается пока LLM вызывает tools
- Max 10 итераций (страховка от бесконечного цикла)
- Останавливается когда LLM даёт финальный ответ без tool calls

### 7. Remote Agent Manager (`hermes_remote_manager.go`)

Управление удалёнными агентами (hermes-agent daemon).

**Компоненты:**
- `RemoteAgent` — информация о подключённом агенте
- `RemoteTask` — задача для удалённого агента
- `RemoteTaskResult` — результат выполнения

**SendTask flow:**
```
SendTask(task)
    │
    ▼
Validate agent (exists, connected, not busy)
    │
    ▼
Register in pendingTasks
    │
    ▼
Get gRPC stream for agent
    │
    ▼
Marshal task → protobuf → stream.Send()
    │
    ▼
Increment ActiveTasks
    │
    ▼
WaitForResult() ──► select { task.Done || timeout }
```

**HealthCheck:**
- Тикер каждые 30 секунд
- Если LastHeartbeat > 90s → UnregisterAgent()

### 8. HermesAgentService (`hermes_agent_service.go`)

Bidirectional gRPC stream для hermes-agent daemon.

**Методы:**
- `Connect(stream)` — регистрация агента, обработка входящих сообщений
- `handleRegister()` — регистрация нового агента
- `handleTaskResult()` — обработка результата от агента

## Базовая таблица данных

```
hermes_sessions ────► id, user_id, name, active_agent_id, agent_mode, created_at, updated_at
hermes_messages ────► id, session_id, user_id, role, agent_id, content, is_streaming, created_at
hermes_agent_runs ──► id, session_id, user_id, agent_id, agent_mode, routing_reason, status, error_text
hermes_custom_agents ► id, name, role, description, system_prompt, model, max_tokens, created_by, is_active
hermes_remote_agents ► id, name, status, capabilities, metadata, last_heartbeat
hermes_remote_tasks ─► id, agent_id, task_type, params, status, stdout, stderr, exit_code
```

## gRPC API

```
service ChatService {
    rpc ChatWithPipeline(PipelineRequest) returns (stream PipelineResponse);
    rpc Orchestrate(OrchestrateRequest) returns (stream OrchestrateResponse);
    rpc HermesAgentService(stream AgentMessage) returns (stream OrchestratorMessage);
}

service HermesSessionService {
    rpc CreateHermesSession(CreateSessionRequest) returns (CreateSessionResponse);
    rpc DeleteHermesSession(DeleteSessionRequest) returns (DeleteSessionResponse);
    rpc ListUserAgents(ListAgentsRequest) returns (ListAgentsResponse);
    rpc CreateCustomAgent(CreateAgentRequest) returns (CreateAgentResponse);
    rpc UpdateCustomAgent(UpdateAgentRequest) returns (UpdateAgentResponse);
    rpc DeleteCustomAgent(DeleteAgentRequest) returns (DeleteAgentResponse);
}
```

## Конфигурация

**Переменные окружения (.env):**
```
OPENROUTER_API_KEY=sk-or-v1-...
OPENROUTER_MODEL=openrouter/auto
```

## Деплой

```bash
cd /root/msg && export PATH=$PATH:/usr/local/go/bin:~/go/bin
go build -o /tmp/lavender-server-dev .
systemctl stop lavender-server-dev
cp /tmp/lavender-server-dev /root/LavenderMessenger/run/lavender-server-dev
systemctl start lavender-server-dev
```

## Известные проблемы

1. **RAG** — in-memory TF-IDF, нужен Qdrant + CLIP для production
2. **Auth токены** — нет генерации JWT для удалённых агентов
3. **Graceful reconnect** — нет переподключения при разрыве gRPC stream

## Тестирование

```bash
cd /root/msg && go test ./core/rag/... -v
```

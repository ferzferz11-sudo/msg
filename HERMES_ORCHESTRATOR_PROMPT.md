# Hermes Multi-Agent Orchestrator — Промпт для новой сессии

## КТО ТЫ
Ты — ведущий архитектор и Senior Go/Kotlin разработчик проекта Lavender Messenger.
gRPC-мессенджер с E2EE (AES-256) и AI оркестратором.

## ПРОЕКТ

**Корень сервера:** `/root/msg/`
**Корень Android:** `/root/msg.client.android/`
**Dev сервер:** port 50052, DB `chat_db_dev`
**Prod сервер:** port 50051, DB `chat_db`

**Сборка dev:**
```bash
cd /root/msg && export PATH=$PATH:/usr/local/go/bin:~/go/bin
go build -o /tmp/lavender-server-dev .
systemctl stop lavender-server-dev && cp /tmp/lavender-server-dev /root/LavenderMessenger/run/lavender-server-dev && systemctl start lavender-server-dev
```

**Сборка Android:**
```bash
cd /root/msg.client.android && ./gradlew assembleDebug
```

**Proto gen:**
```bash
cd /root/msg && protoc --go_out=./gen --go_opt=paths=source_relative --go-grpc_out=./gen --go-grpc_opt=paths=source_relative messenger.proto
```
⚠️ НЕ использовать `--go_out=.` (генерирует в корень, ломает сборку)

## ТЕКУЩЕЕ СОСТОЯНИЕ

### ✅ Работает на dev сервере (v1.1.0.15):

**Ядро (Ports & Adapters):**
1. **LLM Router** (`core/llm/`) — маршрутизация между провайдерами:
   - `OpenRouter` (default, prefix=openrouter/, priority=10) — SSE streaming, tool calls, multimodal images
   - `Hermes local` (prefix=local/, priority=20) — `hermes chat -q --quiet`, stateless, session через --resume
2. **RAG Pipeline** (`core/rag/`) — векторный поиск контекста:
   - Интерфейсы: `EmbeddingService`, `VectorSearch`, `RAGPipeline`
   - Реализация: `in-memory` с TF-IDF эмбеддингами (384 dim), cosine similarity
   - Unit тесты: `core/rag/memory/memory_test.go` (4 теста, все PASS)
3. **Pipeline** (`core/pipeline/`) — RAG → LLM → Tool Calling loop (max 3 iter)
4. **Tool Executor** (`core/tools/`) — 4 инструмента:
   - `search_messages` — ILIKE по messages таблице
   - `search_users` — поиск по username/display_name/phone
   - `web_search` — DuckDuckGo Instant Answer API
   - `get_chat_info` — имя чата, тип, количество участников

**gRPC API:**
- `ChatWithPipeline(PipelineRequest) → stream PipelineResponse` — полный пайплайн с картинками
- `PipelineRequest`: user_id, session_id, message, images (repeated bytes), model_hint
- `PipelineResponse`: token, finished, error, has_rag_context

**Hermes Orchestrator** (`hermes_orchestrator.go`):
- Маршрутизация к агентам, 3 режима (single/parallel/pipeline), streaming
- LLM Router + RAG Pipeline + AI Pipeline
- `ProcessWithPipeline(ctx, userID, message, images, onChunk)`

**Agent Registry** (`hermes_agents.go`):
- 8 агентов (7 пресетов + hermes-owl fallback)

**Database** (`db_hermes.go`):
- hermes_messages, hermes_sessions, hermes_agent_runs, hermes_custom_agents, hermes_remote_agents, hermes_remote_tasks

### ✅ Android клиент — v1.1.0.10+:
- HermesChatActivity (чат с оркестратором)
- AgentListActivity (список агентов)
- AgentSettingsActivity (настройка агентов)
- Все Hermes методы в HermesGrpc.kt / GrpcClient.kt

### ❌ Не работает / не доделано:
1. **Tool calling loop** — max iterations (3) при активном function calling — нужна доработка pipeline
2. **HermesAgentService** — оркестратор НЕ принимает подключения от hermes-agent daemon
3. **Agent↔Orchestrator** — RemoteAgentManager.SendTask() заглушка
4. **Auth токены** — не генерируются для удалённых агентов
5. **Qdrant + CLIP** — запланировано для production RAG

## АРХИТЕКТУРА

```
ChatService (gRPC)
  │
  ├─→ ChatWithPipeline → Orchestrator.ProcessWithPipeline()
  │                         │
  │                         ├─→ RAG Pipeline (core/rag/)
  │                         │     ├─ EmbeddingService (TF-IDF → Qdrant+CLIP)
  │                         │     └─ VectorSearch (in-memory → Qdrant)
  │                         │
  │                         ├─ LLM Router (core/llm/)
  │                         │     ├─ OpenRouter (default)
  │                         │     └─ Hermes local (prefix=local/)
  │                         │
  │                         └─ Tool Executor (core/tools/)
  │                               ├─ search_messages
  │                               ├─ search_users
  │                               ├─ web_search
  │                               └─ get_chat_info
  │
  ├─→ Orchestrate → Orchestrator.Orchestrate()
  │                   ├─ analyzeRequest (LLM routing)
  │                   ├─ runSingleAgent
  │                   ├─ runParallelAgents
  │                   └─ runPipelineAgents
  │
  └─→ HermesAgentService ←─ hermes-agent daemon (bidirectional stream, НЕ РЕАЛИЗОВАНО)
```

## ПРАВИЛА

- Go: идиоматичный, stdlib + grpc + lib/pq
- НЕ копировать поверх работающего процесса!
- Всегда: stop → cp → start
- НЕ редактировать gen/ файлы
- Proto gen: `--go_out=./gen --go_opt=paths=source_relative` (НЕ `--go_out=.`)
- CRUD: hermes_custom_agents table

## КРИТИЧЕСКИЕ PITFALLS

1. goroutine leak: все channel sends через select с ctx.Done()
2. SQL column duplication: дублирование в SELECT смещает Scan
3. Nil pointer: проверять все указатели (ListUserAgents!)
4. Tool calling loop: max 3 итерации — при активном function calling pipeline может зациклиться
5. Hermes local provider: использует `hermes chat -q --quiet` (НЕ JSON-RPC)

## ФАЙЛЫ ДЛЯ ЧТЕНИЯ

1. `hermes_orchestrator.go` — оркестратор, LLM Router, RAG, Pipeline init
2. `core/llm/provider.go` — LLMProvider, LLMRouter, SimpleRouter interfaces
3. `core/llm/openrouter/provider.go` — OpenRouter SSE provider
4. `core/llm/hermes/provider.go` — Hermes local provider (CLI wrapper)
5. `core/rag/interfaces.go` — EmbeddingService, VectorSearch, RAGPipeline
6. `core/rag/memory/memory.go` — in-memory RAG implementation
7. `core/pipeline/pipeline.go` — RAG → LLM → Tool Calling loop
8. `core/tools/executor.go` — DefaultToolExecutor
9. `server.go` (ChatWithPipeline handler) — gRPC endpoint
10. `messenger.proto` — gRPC определения

# Lavender Messenger — Архитектурный анализ

**Дата:** 2026-06-04
**Версия сервера:** 1.1.0.15
**Версия клиента:** 1.1.0.10+

---

## 1. Общая архитектура

```
┌─────────────┐     gRPC (bidirectional)    ┌──────────────┐
│   Android   │◄────────────────────────────►│              │
│   Client    │     gRPC (unary)             │   Go Server  │
│  (Kotlin)   │◄────────────────────────────►│   (main.go)  │
└─────────────┘                              └──────┬───────┘
                                                    │
┌─────────────┐     gRPC (bidirectional)           │
│    iOS      │◄──────────────────────────────────►│
│   Client    │                                    │
│   (Swift)   │                              ┌─────┴─────┐
└─────────────┘                              │ PostgreSQL │
                                             │   (DB)     │
┌─────────────┐                              └───────────┘
│   macOS     │     gRPC
│   Client    │◄────────────────────────────►│
│   (Swift)   │                              │
└─────────────┘                    ┌─────────┴─────────┐
                                   │       FCM         │
                                   │  (Push Notif.)    │
                                   └───────────────────┘
```

## 2. Порты

| Сервис | Порт | Протокол |
|--------|------|----------|
| gRPC (prod) | 50051 | gRPC |
| gRPC (dev) | 50052 | gRPC |
| HTTP (prod) | 8081-8082 | HTTP |
| HTTP (dev) | 8083 | HTTP |

## 3. Hermes Multi-Agent Orchestrator (v1.1.0.15)

### Архитектура

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
  │                         │     ├─ OpenRouter (default, priority=10)
  │                         │     └─ Hermes local (prefix=local/, priority=20)
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
  └─→ HermesAgentService ←─ hermes-agent daemon (НЕ РЕАЛИЗОВАНО)
```

### LLM Router
- **OpenRouter** (default) — SSE streaming, tool calls, multimodal images
- **Hermes local** (prefix=local/) — `hermes chat -q --quiet`, stateless, --resume для сессий

### RAG Pipeline
- Интерфейсы: `EmbeddingService`, `VectorSearch`, `RAGPipeline`
- Текущая реализация: in-memory TF-IDF (384 dim, cosine similarity)
- Production план: Qdrant + CLIP

### Tool Executor
- `search_messages` — ILIKE по messages таблице
- `search_users` — поиск по username/display_name/phone
- `web_search` — DuckDuckGo Instant Answer API
- `get_chat_info` — имя чата, тип, количество участников

### Pipeline (core/pipeline/)
- Адаптивный tool calling loop: max 10 итераций (страховка от бесконечного цикла)
- Цикл продолжается пока LLM вызывает tools, останавливается когда ответ финальный

## 4. Технический стек

### Сервер
- Go 1.26, gRPC, PostgreSQL, Firebase Cloud Messaging
- AES-256-GCM, bcrypt, keepalive 20s/20s
- systemd сервис, .env конфигурация
- Hermes Agent v0.14.0 (Python 3.11.15) — локальный LLM провайдер

### Клиент (Android)
- Kotlin, gRPC (protobuf-lite manual), Room, Firebase, WebRTC
- minSdk 29, compileSdk 37, targetSdk 35
- MVVM + StateFlow + ViewBinding
- Material Design 3

## 5. Ключевые файлы

| Файл | Назначение |
|------|------------|
| `main.go` | gRPC + HTTP серверы, точка входа |
| `server.go` | Основные gRPC хендлеры (~3550 строк) |
| `hermes_orchestrator.go` | Оркестратор, LLM Router + RAG + Pipeline init |
| `hermes_agents.go` | Реестр агентов (8 агентов) |
| `core/pipeline/pipeline.go` | RAG → LLM → Tool Calling loop |
| `core/llm/` | LLM Router + провайдеры |
| `core/rag/` | RAG интерфейсы + in-memory реализация |
| `core/tools/` | Tool Executor |
| `db.go` | PostgreSQL, миграции |
| `db_hermes.go` | миграции Hermes таблиц |
| `messenger.proto` | gRPC определения |

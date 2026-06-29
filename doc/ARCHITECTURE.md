# Lavender Messenger — Архитектура

**Дата:** 2026-06-29
**Версия сервера:** 1.3.0.36
**Модуль:** `LavenderMessenger` (Go 1.26)

---

## 1. Порты

| Сервис | Порт | Протокол | Описание |
|--------|------|----------|----------|
| gRPC (prod) | 50051 | gRPC | Основной сервер |
| gRPC (dev) | 50052 | gRPC | Dev сервер |
| HTTP (prod) | 8082 | HTTP | Файлы, загрузки, TURN, health |
| HTTP (dev) | 8083 | HTTP | Dev HTTP |
| Log Monitor (prod) | 8090 | HTTP | Логи prod |
| Log Monitor (dev) | 8091 | HTTP | Логи dev |

---

## 2. Структура сервера (Go)

### Core

| Файл | Назначение |
|------|------------|
| `main.go` | Точка входа: env, Firebase, DB, gRPC+HTTP, Hermes, graceful shutdown |
| `server.go` | `server` struct (ChatService), версии сервисов, helpers |
| `hub.go` | Менеджер streams: clients, typing, call, conference, online status, grace period |
| `logger.go` | Structured logging (logrus) |

### Database

| Файл | Назначение |
|------|------------|
| `db.go` | Core DB: PostgreSQL, schema, migrations, ~80+ CRUD |
| `db_messages.go` | Message операции |
| `db_users.go` | User операции |
| `db_chats.go` | Chat операции |
| `db_chatlist_v2.go` | ChatList v2: pin/unpin/archive/search, pinned messages, unread counts |
| `db_ai_v2.go` | AI v2: agents, chats, messages, usage stats, reviews |
| `db_auth_devices.go` | Device CRUD: upsert, revoke, validate refresh tokens |
| `db_auth_migrations.go` | Миграции: user_devices, device_auth_log, user_settings |
| `db_hermes.go` | Hermes DB: sessions, messages, agent runs, tokens |

### gRPC Handlers

| Файл | Назначение | Сервис |
|------|------------|--------|
| `server_chat.go` | Chat/Typing/CallSession streams (WebRTC signaling) | ChatService |
| `server_chatlist_v2.go` | **GetChatsV2** (основной), Pin/UnPin chats, Search, Archive, Pin messages | ChatService |
| `server_chats.go` | Chat CRUD: GetAllChats, Create, Delete, Update | ChatService |
| `server_messages.go` | History, reactions, deletion, editing | ChatService |
| `server_users.go` | Profiles: list, update, get profile, get avatar | ChatService |
| `server_push.go` | FCM push, call push, conference push, online broadcast | ChatService |
| `server_contacts.go` | Contact list, chat list version | ChatService |
| `server_themes.go` | Custom themes | ChatService |
| `server_drafts.go` | Draft messages | ChatService |
| `server_muted.go` | Muted chats | ChatService |
| `server_favorites.go` | Favorites, device mgmt, password reset, user ID | ChatService |
| `server_profile.go` | Profile: username, password, mark read, avatar, delete | ChatService |
| `server_profile_v2.go` | ProfileService: JWT-only profile management | ProfileService |
| `server_management.go` | Admin: list, add, update, delete servers | ServerService |
| `server_remote.go` | Remote agent: list, status, deploy (unary + streaming) | ChatService |
| `server_ai_v2.go` | AI v2: ChatWithAIV2, Agent CRUD, Marketplace, Usage Stats (15 RPCs) | ChatService |

### AI Services v2

| Файл | Назначение |
|------|------------|
| `ai_v2.go` | AI Gateway: session management, streaming, chat flow, usage recording, RAG |
| `ai_router.go` | Hybrid router (keyword + LLM fallback) |
| `ai_agent_executor.go` | Agent execution + tool calling loop (max 10 iterations) |
| `ai_provider.go` | AgentProvider interface + StreamUsage |
| `ai_provider_registry.go` | Provider factory registry (8 types) |
| `ai_provider_openrouter.go` | OpenRouter provider (SSE streaming + usage parsing) |
| `ai_provider_mimo.go` | MiMo provider (HTTP + deep integration) |
| `ai_provider_local.go` | Local Hermes provider (subprocess, one-shot) |
| `ai_provider_hermes_acp.go` | Hermes ACP provider (JSON-RPC 2.0, persistent sessions, sync.Map) |
| `ai_provider_webhook.go` | Webhook provider (HTTP POST) |
| `ai_provider_websocket.go` | WebSocket provider (gorilla/websocket) |
| `ai_provider_subprocess.go` | Subprocess provider (stdin/stdout) |
| `ai_provider_mcp.go` | MCP provider (stdio, JSON-RPC 2.0) |
| `ai_provider_reve.go` | Reve image generation provider |
| `ai_tool.go` | Tool interface |
| `ai_tool_registry.go` | Tool registry + 6 built-in tools |
| `ai_tool_search_messages.go` | Search messages tool |
| `ai_tool_search_users.go` | Search users tool |
| `ai_tool_web_search.go` | Web search tool (DuckDuckGo) |
| `ai_tool_web_fetch.go` | URL fetch tool |
| `ai_tool_get_chat_info.go` | Chat info tool |
| `ai_tool_query_db.go` | DB query tool (SELECT only, admin) |
| `rate_limiter.go` | Rate limiter + callOpenRouterContext |
| `redis_rate_limiter.go` | Redis-backed rate limiter (fallback: in-memory) |
| `core/rag/qdrant/qdrant.go` | Qdrant REST API client (VectorSearch interface) |
| `core/rag/qdrant/embedding.go` | OpenAI embedding service (text-embedding-3-small) |
| `core/rag/memory/memory.go` | In-memory RAG: TF-IDF, cosine similarity (fallback) |

### Hermes Stubs

| Файл | Назначение |
|------|------------|
| `hermes_stubs.go` | Stub types (Orchestrator, OrchestratorMessage, AIChatSettings) for hermes_agent_service.go |
| `hermes_agent_service.go` | gRPC service: hermes-agent daemon connections, tokens |
| `hermes_remote_manager.go` | Remote agent management: connections, tasks, health |

### Infrastructure

| Файл | Назначение |
|------|------------|
| `http_server.go` | HTTP: uploads (avatar/image/file/background/audio), TURN, health, info |
| `email.go` | SMTP: password reset emails |
| `crypto.go` | AES-256-GCM encryption, bcrypt hashing, reset tokens |
| `secret_chat.go` | E2EE: secret chat creation, public key exchange |
| `bot_commands.go` | Bot: /status, /deploy, /restart, /ai, notifications pub/sub |

### Core (LLM/RAG Pipeline)

| Файл | Назначение |
|------|------------|
| `core/llm/provider.go` | LLMProvider interface, Message, ToolDef, ToolCall types |
| `core/llm/openrouter/provider.go` | OpenRouter LLM provider |
| `core/llm/hermes/provider.go` | Hermes LLM provider |
| `core/rag/interfaces.go` | VectorSearch, EmbeddingService, RAGPipeline interfaces |
| `core/rag/qdrant/qdrant.go` | Qdrant REST API client |
| `core/rag/qdrant/embedding.go` | OpenAI embedding service |
| `core/rag/memory/memory.go` | In-memory RAG: TF-IDF, cosine similarity (fallback) |
| `core/pipeline/pipeline.go` | Pipeline orchestrator: RAG → LLM → Tool Calls |

### Auth

| Файл | Назначение |
|------|------------|
| `auth_service.go` | Auth database interface definition |
| `auth_service_v2.go` | AuthService v2: JWT, device management, refresh, sign-out |
| `auth_interceptor.go` | gRPC interceptors: JWT validation |
| `auth_jwt.go` | JWT generation/validation (HMAC-SHA256) |
| `auth/jwt.go` | Agent tokens для hermes-agent daemon |

### Generated

| Файл | Назначение |
|------|------------|
| `gen/messenger.pb.go` | Protobuf: messenger messages |
| `gen/messenger_grpc.pb.go` | Protobuf: gRPC service stubs |
| `gen/server.pb.go` | Protobuf: server messages |
| `gen/server_grpc.pb.go` | Protobuf: server gRPC stubs |
| `gen/hermes_agent/*.pb.go` | Protobuf: hermes agent protocol |

---

## 3. Auth Architecture

```
Client ──gRPC──► AuthInterceptor
                    │
                    ├── Bearer token? → ValidateToken() → ctx (userID, username, deviceID)
                    │
                    └── No token? → error UNAUTHENTICATED
```

JWT Workflow:
1. `SignInV2` → access_token (15 min) + refresh_token (30 days)
2. Every gRPC call: `metadata["authorization"] = "Bearer <access_token>"`
3. Access expired → `RefreshToken(refresh_token)` → new tokens
4. Refresh token rotation: each refresh → new refresh, old invalidated

---

## 4. Graceful Shutdown

```
SIGTERM received
  ├── Broadcast SERVER_SHUTTINGDOWN to all Chat streams (2s grace)
  ├── HTTP server: httpSrv.Shutdown(ctx, 5s timeout)
  ├── gRPC server: s.GracefulStop() → s.Stop() (30s timeout)
  ├── DB: defer db.Close()
  └── Background goroutines: context cancel
```

Health endpoint returns 503 `{"status":"shutting_down"}` during shutdown window.

---

## 5. AI Services v2 Architecture

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

**3 chat types:** simple (direct LLM), agent (multi-agent routing), pipeline (RAG + tools)
**9 providers:** openrouter, local, mimo, hermes_acp, webhook, websocket, subprocess, mcp, reve
**6 tools:** search_messages, search_users, web_search, web_fetch, get_chat_info, query_database
**11 presets:** mimo, assistant, developer, devops, architect, writer, analyst, translator, vision, reve, hermes

---

## 6. Tech Stack

- Go 1.26, gRPC + Protocol Buffers
- PostgreSQL (database/sql + lib/pq)
- Redis 6.0.16 (rate limiting, all limiters wired)
- Firebase Cloud Messaging (push)
- JWT (golang-jwt/v5, HMAC-SHA256)
- logrus (structured logging)
- bcrypt (password hashing)
- AES-256-GCM (E2EE encryption)
- gorilla/websocket v1.5.3 (WebSocket provider)
- Qdrant + OpenAI text-embedding-3-small (production RAG)
- systemd, .env конфигурация

---

## 7. Тесты

| Команда | Описание |
|---------|----------|
| `go test ./...` | Все тесты (~88) |
| `go test -race -count=1 .` | С race detector |

Тесты: `auth_jwt_test.go`, `auth_service_test.go`, `owl_test.go`, `bot_commands_test.go`, `server_push_test.go`, `server_remote_test.go`, `server_stability_test.go`, `chatv2_test.go`, `messages_v2_test.go`, `core/rag/memory/memory_test.go`

---

## 8. Деплой

| Команда | Описание |
|---------|----------|
| `./scripts/deploy-dev-local.sh` | Dev: cross-compile + SCP + restart |
| `./scripts/deploy-prod-local.sh` | Prod: cross-compile + SCP + backup + restart |

---

## 9. Безопасность

- JWT tokens (SHA-256, access+refresh pair, device-bound)
- Grace period 30s для переподключения
- E2EE для секретных чатов (AES-256-GCM)
- Rate limiting: Redis-backed (owl 10/min, free 20/hr, bot 30/min, per-agent configurable)
- Admin-only: bot commands /deploy, /restart, /logs, query_database tool
- Auth interceptor на всех RPC (кроме AuthService)
- Panic recovery в stream handlers (Chat, Typing, CallSession)
- FCM push: batch (500 tokens), exponential backoff, auto-cleanup invalid tokens
- Firebase credentials validated at startup (log CRITICAL on failure)

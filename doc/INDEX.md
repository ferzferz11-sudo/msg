# Лава — Документация

Индекс всех документов проекта. Читать при каждом старте новой сессии.

**Модуль:** `LavenderMessenger` | **Go:** 1.26 | **Сервер:** v1.3.0.19

---

## Быстрый старт

1. **PROMPT.md** — текущий контекст сессии (ветка, статус, команды)
2. **INTEGRATION_SESSION.md** — интеграционная сессия (статус, правила)
3. **CLIENT_INTEGRATION.md** — **полная интеграция клиентов** (все gRPC методы, HTTP endpoints, AI v2, marketplace)

---

## Структура серверного кода

### Core

| Файл | Описание |
|------|----------|
| `main.go` | Точка входа: env, Firebase, DB, gRPC, HTTP, Hermes, graceful shutdown |
| `server.go` | Основной `server` struct (ChatService), версии сервисов, helpers |
| `hub.go` | Менеджер активных gRPC stream'ов, typing, call, conference, online status |
| `logger.go` | Глобальный structured logger (logrus) |

### Auth

| Файл | Описание |
|------|----------|
| `auth_service_v2.go` | AuthService: JWT, device management, token refresh, sign-out |
| `auth_interceptor.go` | gRPC interceptors (JWT validation) |
| `auth_jwt.go` | JWT token generation/validation (HMAC-SHA256) |
| `auth/jwt.go` | Agent token management для hermes-agent daemon |

### Database

| Файл | Описание |
|------|----------|
| `db.go` | Core DB layer: PostgreSQL, schema, migrations |
| `db_messages.go` | Message операции |
| `db_users.go` | User операции |
| `db_chats.go` | Chat операции (GetUserChats v1) |
| `db_chatlist_v2.go` | ChatList v2: pin/unpin/archive/search, pinned messages, unread counts |
| `db_ai_v2.go` | AI v2 DB: agents, chats, messages, usage stats, reviews |
| `db_auth_devices.go` | Device management CRUD |
| `db_auth_migrations.go` | Миграции auth v2 |
| `db_hermes.go` | Hermes DB: sessions, messages, agent runs, tokens |

### gRPC Handlers

| Файл | Описание |
|------|----------|
| `server_chat.go` | Bidirectional streams: Chat, Typing, CallSession |
| `server_chatlist_v2.go` | **GetChatsV2** (основной), Pin/UnPin chats, Search, Archive, Pin messages |
| `server_chats.go` | Chat CRUD: GetAllChats, Create, Delete, Update |
| `server_messages.go` | Message history, reactions, deletion, editing |
| `server_users.go` | User profiles: list, update, get profile, get avatar |
| `server_push.go` | Push notifications (FCM), call pushes, online broadcast |
| `server_contacts.go` | Contact list, chat list version |
| `server_themes.go` | Custom theme management |
| `server_drafts.go` | Draft messages |
| `server_muted.go` | Muted chats |
| `server_favorites.go` | Favorites, device management, password reset, user ID lookup |
| `server_profile.go` | Profile: username/password update, mark read, avatar, delete |
| `server_profile_v2.go` | ProfileService: JWT-only profile management |
| `server_management.go` | Server management (admin) |
| `server_remote.go` | Remote agent RPC: list, status, deploy tasks |
| `server_ai_v2.go` | AI v2: ChatWithAIV2, Agent CRUD, Marketplace, Usage Stats (15 RPCs) |

### AI Services v2

| Файл | Описание |
|------|----------|
| `ai_v2.go` | AI Gateway: session management, streaming, chat flow, RAG |
| `ai_router.go` | Hybrid router (keyword + LLM fallback) |
| `ai_agent_executor.go` | Agent execution + tool calling loop |
| `ai_provider.go` | AgentProvider interface + StreamUsage |
| `ai_provider_registry.go` | Provider factory registry (7 types) |
| `ai_provider_openrouter.go` | OpenRouter (SSE streaming + usage parsing) |
| `ai_provider_mimo.go` | MiMo (HTTP + deep integration) |
| `ai_provider_local.go` | Local Hermes (subprocess) |
| `ai_provider_webhook.go` | Webhook (HTTP POST) |
| `ai_provider_websocket.go` | WebSocket (gorilla/websocket) |
| `ai_provider_subprocess.go` | Subprocess (stdin/stdout) |
| `ai_provider_mcp.go` | MCP (stdio, JSON-RPC 2.0) |
| `ai_tool.go` | Tool interface |
| `ai_tool_registry.go` | Tool registry + 6 built-in tools |
| `ai_tool_cache.go` | LRU cache for AI tool results (1min TTL) |
| `ai_tool_search_messages.go` | Search messages tool |
| `ai_tool_search_users.go` | Search users tool |
| `ai_tool_get_chat_info.go` | Get chat info tool |
| `ai_tool_query_db.go` | Database query tool (admin, read-only) |
| `ai_tool_web_search.go` | Web search tool (DuckDuckGo) |
| `ai_tool_web_fetch.go` | Web fetch tool (SSRF-protected) |
| `rate_limiter.go` | Rate limiter + callOpenRouterContext |
| `redis_rate_limiter.go` | Redis-backed rate limiter (fallback: in-memory) |
| `core/rag/qdrant/` | Qdrant vector DB client + OpenAI embeddings |

### Infrastructure

| Файл | Описание |
|------|----------|
| `http_server.go` | HTTP: file uploads (JWT auth), TURN, health, info |
| `email.go` | SMTP: password reset emails |
| `crypto.go` | AES-256-GCM encryption, bcrypt hashing |
| `secret_chat.go` | E2EE secret chat: creation, public key exchange |
| `bot_commands.go` | Bot commands (/status, /deploy, /restart, /ai), notifications |

### Generated / Tests

| Файл | Описание |
|------|----------|
| `gen/messenger.pb.go` | Protobuf: messenger messages |
| `gen/messenger_grpc.pb.go` | Protobuf: gRPC service stubs |
| `*_test.go` | Unit tests (~88 tests) |

---

## Файлы документации

| Файл | Назначение |
|------|-----------|
| `PROMPT.md` | Промпт для серверных сессий: статус, правила, команды |
| `INTEGRATION_SESSION.md` | Интеграционная сессия: статус, правила |
| `CLIENT_INTEGRATION.md` | **Единый документ интеграции клиентов** — все gRPC методы, HTTP endpoints, auth, AI v2, marketplace |
| `ARCHITECTURE.md` | Архитектура сервера |
| `AI_SERVICES.md` | AI-сервисы: провайдеры, пресеты, инструменты |
| `TASKS.md` | Таск-трекер |
| `PITFALLS.md` | Подводные камни и известные проблемы |
| `TESTING.md` | Модульные тесты |
| `RELEASE.md` | Процесс релиза |
| `LOG_MONITOR.md` | Log Monitor: сборка, деплой, API |
| `MESSAGES_V2.md` | Messages v2: план, чеклист, дизайн |

---

## Правила

- При старте сессии: читать `PROMPT.md` → `INTEGRATION_SESSION.md`
- При интеграции нового клиента: `CLIENT_INTEGRATION.md` (все методы)
- При работе с AI: читать `AI_SERVICES.md`
- При деплое: читать `RELEASE.md`
- Версия сервера в `server.go`
- Android: `/root/msg.client.android/doc/` — документация клиента
- ⚠️ Android собирается ТОЛЬКО локально (нет памяти на сервере)

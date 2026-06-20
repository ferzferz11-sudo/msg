# Лава — Документация

Индекс всех документов проекта. Читать при каждом старте новой сессии.

**Модуль:** `LavenderMessenger` | **Go:** 1.26 | **Сервер:** v1.3.0.8

---

## Быстрый старт

1. **PROMPT.md** — текущий контекст сессии (ветка, статус, команды)
2. **INTEGRATION_SESSION.md** — интеграционная сессия (коммиты, статус)
3. **TASKS.md** — таск-трекер

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
| `auth_service.go` | ⚠️ **Deprecated** v1 AuthService (SignIn/SignUp без JWT) |
| `auth_service_v2.go` | v2 AuthService: JWT, device management, token refresh, sign-out |
| `auth_interceptor.go` | gRPC unary + stream interceptors (JWT validation, v1 fallback) |
| `auth_jwt.go` | JWT token generation/validation (HMAC-SHA256) |
| `auth/jwt.go` | Agent token management для hermes-agent daemon (отдельный пакет) |

### Database

| Файл | Описание |
|------|----------|
| `db.go` | Core DB layer: PostgreSQL connection, schema, ~80+ CRUD методов |
| `db_chatlist_v2.go` | ChatList v2: pin/unpin/archive/search, pinned messages |
| `db_hermes.go` | Hermes Orchestrator DB: sessions, messages, agent runs, tokens |
| `db_auth_migrations.go` | Миграции auth v2: user_devices, device_auth_log, user_settings |
| `db_auth_devices.go` | Device management CRUD: upsert, revoke, validate refresh tokens |

### gRPC Handlers

| Файл | Описание |
|------|----------|
| `server_chat.go` | Bidirectional streams: Chat, Typing, CallSession (WebRTC signaling) |
| `server_chats.go` | Chat CRUD: GetAllChats, GetChats ⚠️ **Deprecated**, Create, Delete, Update |
| `server_chatlist_v2.go` | ChatList v2 RPC: **GetChatsV2** (основной), pin/unpin chats, search, archive, pin messages |
| `server_messages.go` | Message history, reactions, deletion, editing |
| `server_users.go` | User profiles: list, update, get profile, get avatar |
| `server_push.go` | Push notifications (FCM), call pushes, conference pushes, online broadcast |
| `server_contacts.go` | Contact list, chat list version |
| `server_themes.go` | Custom theme management |
| `server_drafts.go` | Draft messages, FCM logs |
| `server_muted.go` | Muted chats |
| `server_favorites.go` | Favorites, device management, password reset, user ID lookup |
| `server_profile.go` | v1 profile: username/password update, mark read, avatar, delete |
| `server_profile_v2.go` | v2 ProfileService (dev only): JWT-only profile management |
| `server_management.go` | Server management (admin): list, add, update, delete servers |
| `server_remote.go` | Remote agent RPC: list, status, deploy tasks (unary + streaming) |
| `server_ai.go` | AI chat: ChatWithAI (streaming), history, settings, free models |

### AI Services

| Файл | Описание |
|------|----------|
| `owl.go` | OpenRouter API: HTTP client, streaming, rate limiter, session manager |
| `ai_chat_manager.go` | Unified AI chat manager: sessions, messages, settings CRUD |
| `hermes_orchestrator.go` | Multi-agent orchestrator: routing, RAG, tool calling, pipeline |
| `hermes_agents.go` | Agent definitions, 8 presets (Dev, DevOps, Architect...), registry |
| `hermes_agent_service.go` | gRPC service для hermes-agent daemon connections |
| `hermes_remote_manager.go` | Remote agent management: connections, tasks, health checks |
| `core/llm/provider.go` | LLM abstraction: interfaces, router, Message/ToolDef types |
| `core/llm/openrouter/provider.go` | OpenRouter LLM provider with streaming + function calling |
| `core/llm/hermes/provider.go` | Local Hermes agent LLM provider (hermes chat subprocess) |
| `core/pipeline/pipeline.go` | AI pipeline: RAG context → LLM streaming → tool-calling loop |
| `core/tools/executor.go` | Default tool executor: search_messages, search_users, web_search, get_chat_info |
| `core/rag/interfaces.go` | RAG interfaces: vector search, embedding, pipeline |
| `core/rag/memory/memory.go` | In-memory RAG: TF-IDF embeddings, cosine similarity, full pipeline |

### Infrastructure

| Файл | Описание |
|------|----------|
| `http_server.go` | HTTP: file uploads (avatar/image/file/background/audio), TURN, health, info |
| `email.go` | SMTP: password reset emails |
| `crypto.go` | AES-256-GCM encryption, bcrypt hashing, reset token generation |
| `secret_chat.go` | E2EE secret chat: creation, public key exchange |
| `bot_commands.go` | Bot commands (/status, /deploy, /restart, /ai...), notification service |

### Generated / Tests

| Файл | Описание |
|------|----------|
| `gen/messenger.pb.go` | Protobuf generated: messenger messages |
| `gen/messenger_grpc.pb.go` | Protobuf generated: gRPC service stubs |
| `gen/server.pb.go` | Protobuf generated: server messages |
| `gen/server_grpc.pb.go` | Protobuf generated: server gRPC stubs |
| `gen/hermes_agent/*.pb.go` | Protobuf generated: hermes agent protocol |
| `*_test.go` | Unit tests (auth_jwt, auth_service, owl, bot_commands, push, remote, memory) |

---

## Deprecated v1 compat — УДАЛЕНО в v1.3.0.8

| Что | Статус |
|-----|--------|
| `authServer` v1 SignIn/SignUp | ✅ Удалён |
| `extractUsernameFromMetadata()` | ✅ Удалён |
| `AuthInterceptor` v1 fallback | ✅ Удалён |
| `AuthStreamInterceptor` bypass | ✅ Удалён |
| `ResolveUserID()` + cache | ✅ Удалён |
| `resolveUserId()` / `resolveUsername()` | ✅ Удалены |
| `GetChats()` v1 endpoint | ✅ Удалён |

---

## Файлы документации (10 файлов)

| Файл | Назначение |
|------|-----------|
| `PROMPT.md` | Промпт для серверных сессий: статус, правила, команды |
| `INTEGRATION_SESSION.md` | Интеграционная сессия: коммиты, статус, deprecated таблица |
| `TASKS.md` | Таск-трекер: сделано/не сделано |
| `ARCHITECTURE.md` | Общая архитектура сервера |
| `CLIENT_INTEGRATION.md` | **Интеграция клиента** — все gRPC методы, HTTP endpoints, auth workflow |
| `AI_SERVICES.md` | AI-сервисы: AI Gateway v2, провайдеры, маркетплейс |
| `MARKETPLACE_AGENTS_SETUP.md` | **Агенты и пресеты** — quickstart, создание, marketplace, tool calling |
| `ANDROID_RATE_LIMIT_PROMPT.md` | **Android rate limiting** — кэширование лимитов, UX при превышении |
| `PITFALLS.md` | Подводные камни и известные проблемы (сервер) |
| `LOG_MONITOR.md` | Log Monitor: сборка, деплой, API |
| `TESTING.md` | Модульные тесты |
| `RELEASE.md` | Процесс релиза |

---

## Правила

- При старте сессии: читать `PROMPT.md` → `INTEGRATION_SESSION.md`
- При интеграции нового клиента: `CLIENT_INTEGRATION.md` (все методы)
- При работе с AI: читать `AI_SERVICES.md`
- При деплое: читать `RELEASE.md`
- После изменений: обновлять `INTEGRATION_SESSION.md` + `TASKS.md`
- Версия сервера в `server.go:ServerVersion`
- CHANGELOG.md в `/root/msg/CHANGELOG.md`
- Deprecated методы будут удалены в v1.3 — не добавлять новых вызовов
- Android: `/root/msg.client.android/doc/` — вся документация клиента там
- ⚠️ Android собирается ТОЛЬКО локально (нет памяти на сервере)

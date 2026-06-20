# Lavender Messenger — Архитектура

**Дата:** 2026-06-19
**Версия сервера:** 1.2.0.9
**Модуль:** `LavenderMessenger` (Go 1.26)

---

## 2. Порты

| Сервис | Порт | Протокол | Описание |
|--------|------|----------|----------|
| gRPC (prod) | 50051 | gRPC | Основной сервер |
| gRPC (dev) | 50052 | gRPC | Dev сервер |
| HTTP (prod) | 8082 | HTTP | Файлы, загрузки, TURN, health |
| HTTP (dev) | 8083 | HTTP | Dev HTTP |
| Log Monitor | 8090 | HTTP | Логи (dev only) |

## 3. Структура сервера (Go)

### Core

| Файл | Назначение |
|------|------------|
| `main.go` | Точка входа: env, Firebase, DB, gRPC+HTTP, Hermes, graceful shutdown |
| `server.go` | `server` struct (ChatService), версии сервисов, helpers |
| `hub.go` | Менеджер streams: clients, typing, call, conference, online status, grace period |
| `logger.go` | Structured logging (logrus) |

### Auth

| Файл | Назначение | Статус |
|------|------------|--------|
| `auth_service.go` | v1 AuthService: SignIn/SignUp без JWT | ⚠️ Deprecated (v1.3) |
| `auth_service_v2.go` | v2 AuthService: JWT, device management, refresh, sign-out | ✅ Current |
| `auth_interceptor.go` | gRPC interceptors: JWT validation + v1 fallback | ⚠️ v1 fallback deprecated |
| `auth_jwt.go` | JWT generation/validation (HMAC-SHA256) | ✅ Current |
| `auth/jwt.go` | Agent tokens для hermes-agent daemon | ✅ Current |

### Database

| Файл | Назначение |
|------|------------|
| `db.go` | Core DB: PostgreSQL, schema, ~80+ CRUD методов |
| `db_chatlist_v2.go` | ChatList v2: pin/unpin/archive/search, pinned messages |
| `db_hermes.go` | Hermes DB: sessions, messages, agent runs, tokens |
| `db_auth_migrations.go` | Миграции: user_devices, device_auth_log, user_settings |
| `db_auth_devices.go` | Device CRUD: upsert, revoke, validate refresh tokens |

### gRPC Handlers

| Файл | Назначение | Сервис |
|------|------------|--------|
| `server_chat.go` | Chat/Typing/CallSession streams (WebRTC signaling) | ChatService |
| `server_chats.go` | Chat CRUD: GetAllChats, GetChats ⚠️, Create, Delete, Update | ChatService |
| `server_chatlist_v2.go` | **GetChatsV2** (основной), Pin/UnPin chats, Search, Archive, Pin messages | ChatService |
| `server_messages.go` | History, reactions, deletion, editing | ChatService |
| `server_users.go` | Profiles: list, update, get profile, get avatar | ChatService |
| `server_push.go` | FCM push, call push, conference push, online broadcast | ChatService |
| `server_contacts.go` | Contact list, chat list version | ChatService |
| `server_themes.go` | Custom themes | ChatService |
| `server_drafts.go` | Draft messages, FCM logs | ChatService |
| `server_muted.go` | Muted chats | ChatService |
| `server_favorites.go` | Favorites, device mgmt, password reset, user ID | ChatService |
| `server_profile.go` | v1 profile: username, password, mark read, avatar, delete | ChatService |
| `server_profile_v2.go` | v2 ProfileService (dev only): JWT-only | ProfileService |
| `server_management.go` | Admin: list, add, update, delete servers | ServerService |
| `server_remote.go` | Remote agent: list, status, deploy (unary + streaming) | ChatService |
| `server_ai.go` | AI chat: streaming, history, settings, free models | ChatService |

### AI Services

| Файл | Назначение |
|------|------------|
| `owl.go` | OpenRouter API: HTTP client, streaming, rate limiter, session manager |
| `ai_chat_manager.go` | Unified AI chat: sessions, messages, settings CRUD |
| `hermes_orchestrator.go` | Multi-agent orchestrator: routing, RAG, tool calling, pipeline |
| `hermes_agents.go` | Agent definitions, 8 presets, registry, custom agents |
| `hermes_agent_service.go` | gRPC service: hermes-agent daemon connections, tokens |
| `hermes_remote_manager.go` | Remote agent management: connections, tasks, health |

### Core (AI Pipeline)

| Файл | Назначение |
|------|------------|
| `core/llm/provider.go` | LLM abstraction: interfaces, router, Message/ToolDef types |
| `core/llm/openrouter/provider.go` | OpenRouter provider: streaming + function calling |
| `core/llm/hermes/provider.go` | Local Hermes provider: hermes chat subprocess |
| `core/pipeline/pipeline.go` | AI pipeline: RAG → LLM streaming → tool-calling loop |
| `core/tools/executor.go` | Tool executor: search_messages, search_users, web_search, get_chat_info |
| `core/rag/interfaces.go` | RAG interfaces: vector search, embedding, pipeline |
| `core/rag/memory/memory.go` | In-memory RAG: TF-IDF, cosine similarity, full pipeline |

### Infrastructure

| Файл | Назначение |
|------|------------|
| `http_server.go` | HTTP: uploads (avatar/image/file/background/audio), TURN, health, info |
| `email.go` | SMTP: password reset emails |
| `crypto.go` | AES-256-GCM encryption, bcrypt hashing, reset tokens |
| `secret_chat.go` | E2EE: secret chat creation, public key exchange |
| `bot_commands.go` | Bot: /status, /deploy, /restart, /ai, notifications pub/sub |

### Generated

| Файл | Назначение |
|------|------------|
| `gen/messenger.pb.go` | Protobuf: messenger messages |
| `gen/messenger_grpc.pb.go` | Protobuf: gRPC service stubs |
| `gen/server.pb.go` | Protobuf: server messages |
| `gen/server_grpc.pb.go` | Protobuf: server gRPC stubs |
| `gen/hermes_agent/*.pb.go` | Protobuf: hermes agent protocol |

## 4. Auth Architecture

```
Client ──gRPC──► AuthInterceptor
                    │
                    ├── Bearer token? → ValidateToken() → ctx (userID, username, deviceID)
                    │
                    └── No token? → extractUsernameFromMetadata() [DEPRECATED v1.3]
                                      └── ctx (username)

Chat Stream ──► AuthStreamInterceptor
                    │
                    ├── Chat/Typing/CallSession? → bypass (v1 legacy) [DEPRECATED v1.3]
                    │
                    └── Other streams → ValidateToken()
```

## 5. Graceful Shutdown

```
SIGTERM received
  ├── HTTP server: httpSrv.Shutdown(ctx, 5s timeout)
  ├── gRPC server: s.GracefulStop() (drains streams)
  ├── DB: defer db.Close()
  └── Background goroutines: online broadcast, hermes agents (context cancel)
```

## 6. Deprecated v1 Compat (удаление в v1.3)

| Что | Файл | Замена |
|-----|------|--------|
| `GetChats()` v1 endpoint | `server_chats.go` | **`GetChatsV2()`** — фильтры, пагинация, v2 поля |
| `authServer` v1 SignIn/SignUp | `auth_service.go` | `authServerV2` |
| `extractUsernameFromMetadata()` | `auth_interceptor.go` | JWT Bearer token |
| `AuthInterceptor` v1 fallback | `auth_interceptor.go` | v2 JWT path |
| `AuthStreamInterceptor` bypass | `auth_interceptor.go` | JWT в metadata |
| `ResolveUserID()` | `auth_interceptor.go` | `GetUserID(ctx)` |
| `resolveUserId()` / `resolveUsername()` | `server.go` | UUID идентификаторы |
| `GetChats()` v1 | `server_chats.go` | `GetChatsV2()` |
| Chat stream v1 password auth | `server_chat.go` | JWT в первом сообщении |
| `IsUserOnline()` username fallback | `hub.go` | UUID-only |
| `BroadcastCall()` username matching | `hub.go` | UUID-only |
| `user_chat_metadata.username` | `db_chatlist_v2.go` | `user_id` (UUID PK) |

## 7. Технический стек

- Go 1.26, gRPC + Protocol Buffers
- PostgreSQL (database/sql + lib/pq)
- Firebase Cloud Messaging (push)
- JWT (golang-jwt/v5, HMAC-SHA256)
- logrus (structured logging)
- bcrypt (password hashing)
- AES-256-GCM (E2EE encryption)
- systemd, .env конфигурация

**Android:** `/root/msg.client.android` — Kotlin, gRPC, Room, Firebase, WebRTC. Сборка ТОЛЬКО локально.

## 8. Тесты

| Команда | Описание |
|---------|----------|
| `go test ./...` | Все тесты |
| `./scripts/run-unit-tests.sh` | Unit-тесты |
| `./scripts/run-streaming-tests.sh` | Тесты стриминга |

Тесты: `auth_jwt_test.go`, `auth_service_test.go`, `owl_test.go`, `bot_commands_test.go`, `server_push_test.go`, `server_remote_test.go`, `core/rag/memory/memory_test.go`

## 9. Деплой

| Команда | Описание |
|---------|----------|
| `./scripts/deploy-dev.sh` | Dev: build + restart (systemd) |
| `./scripts/release.sh <ver> --deploy` | Release: build + deploy + restart |
| `./scripts/release.sh <ver> --deploy --remote` | Remote deploy via SSH |

## 10. Безопасность

- JWT tokens (SHA-256, access+refresh pair, device-bound)
- Grace period 30s для переподключения
- E2EE для секретных чатов (AES-256-GCM)
- Rate limiting: OWL 10 req/min, Hermes 5 req/min, Bot 5 cmd/min
- Admin-only: bot commands /deploy, /restart, /logs
- Auth interceptor на всех RPC (кроме AuthService)
- Panic recovery в stream handlers (Chat, Typing, CallSession)
- Agent marketplace: rate limiting на отзывы (1 отзыв на пользователя на агента)

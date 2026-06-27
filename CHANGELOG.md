# Лава — Server Changelog

## [1.3.0.27] - 2026-06-27

### Cleanup: v1 Messages Removed
- **Dropped tables**: `messages` and `reactions` tables removed from database
- **Removed v1 DB functions**: SaveMessage, GetMessages, GetMessageByUUID, DeleteMessageByUUID, DeleteMessageByID, GetMessagesByUserAndTime, UpdateMessageText, GetMessageImageURL, CleanupEmptyMessages, GetChatMessages, SetReaction, SetReactionByUserID, RemoveReactionByUserID
- **Removed v1 migrations**: CREATE TABLE messages, ALTER TABLE messages, CREATE TABLE reactions from db.go
- **Cleaned up references**: db_users.go (delete user), db_chats.go (delete chat) no longer reference v1 tables
- All message operations now use messages_v2 exclusively

---

## [1.3.0.26] - 2026-06-27

### Backward Compatibility
- **v1 RPCs rewritten to use v2 internally** — GetHistory, SetReaction, DeleteMessages, EditMessage now read/write messages_v2 and convert to v1 proto format. Old clients continue to work.
- **v1 RPCs marked as DEPRECATED** in proto — new clients should use v2 equivalents.
- **GetReactionsForMessage** reads from messages_v2.reactions JSONB instead of reactions table.
- **GetChatMessagesImageURLs** reads from messages_v2 instead of messages.

---

## [1.3.0.25] - 2026-06-27

### Migration: Messages v1 → v2 (complete)
- **SQL migration script** — `scripts/migrate_v1_to_v2.sql` converts all v1 messages to v2 (idempotent, ON CONFLICT DO NOTHING). Handles orphaned users with `[deleted]` system user.
- **DB queries switched to v2** — unread counts (`db_chats.go`, `db_chatlist_v2.go`), pinned messages, favorites, delete chat all use `messages_v2` table.
- **Dual-write removed** — `server_chat.go` no longer writes to v1 `messages` table. All message writes go to `messages_v2` only.
- **AI tool updated** — `ai_tool_search_messages.go` now queries `messages_v2` instead of `messages`.
- **SearchMessages v2-only** — `db_messages_v2.go` SearchMessages removed UNION ALL with v1, queries only `messages_v2`.

### New Features
- **SearchMessages RPC** — new gRPC endpoint for message search (single chat or cross-chat). Uses cursor-based results from `messages_v2`.

### Client Migration Prompts
- `doc/PROMPT_MIGRATE_V2.md` — Android client migration guide (RPC map, proto definitions, implementation steps)
- Web client migration prompt created in separate repo

---

## [1.3.0.24] - 2026-06-27

### Features
- **AgentInfoV2 provider_config** — `AgentInfoV2` now returns `provider_config` (JSON string) in all RPCs (Create, Get, List, Clone, Marketplace). Clients can read API keys, model overrides, and other provider-specific config.

### Improvements
- **DB maintenance script** — updated `db_maintenance.sh` with `messages_v2`, AI v2 tables (`ai_messages_v2`, `ai_chats_v2`, `agents_v2`, `agent_tokens`, `agent_reviews`, `ai_rate_limits`, `ai_usage_stats`), `pinned_messages`, `chat_list_v2`, `user_settings`, `hermes_remote_agents/tasks`. Added empty text message cleanup for `messages_v2`.

---

## [1.3.0.23] - 2026-06-23

### Bug Fixes
- **ListAIAgents UUID fix** — replaced `[]string` with `pq.StringArray` for scanning PostgreSQL text arrays (`tool_whitelist`, `tags`). Fixes `pq: invalid input syntax for type uuid` errors.
- **COALESCE cast** — `COALESCE(created_by::text,'')` prevents UUID cast errors when `created_by` is NULL.

---

## [1.3.0.22] - 2026-06-22

### Bug Fixes
- **ListAIAgents invalid UUID** — added UUID validation to all AI v2 handlers to prevent PostgreSQL crash when `userID` is empty or not a valid UUID. All handlers now use `requireValidAIV2UserID()` which validates UUID format before any SQL query.
- **AI v2 auth hardened** — all 13 AI v2 handlers now validate `userID` is a proper UUID before database access.

---

## [1.3.0.21] - 2026-06-22

### Bug Fixes
- **ChatV2 last_seen_at** — `user_chat_metadata.last_seen_at` and `users.last_seen_at` now update correctly when clients use ChatV2 stream (previously only worked with v1 Chat stream).
- **ChatV2 last_client_version** — `users.last_client_version` now updates on ChatV2 connect and every message (added `client_version` field to `ChatV2Message` proto).

---

## [1.3.0.20] - 2026-06-22

### Features
- **AI Chat v2 History** — new `GetAIV2ChatHistory` RPC returns chat messages with `agent_id`, `token_count`, `model_used` per message.
- **AI Chats List** — new `ListAIV2Chats` RPC returns all user's AI v2 chats with type, agent_id, timestamps.
- **Agent ID in streaming** — `ChatWithAIV2Response` now includes `agent_id` and `agent_name` in every streamed token (fixed missing fields).
- **Multi-agent chat support** — server-side streaming now correctly identifies which agent produced each token, enabling client-side multi-agent routing.

### Improvements
- `StreamFn` signature updated to pass agent metadata through the streaming pipeline
- New proto types: `AIV2ChatMessage`, `AIV2ChatInfo`, `GetAIV2ChatHistoryRequest/Response`, `ListAIV2ChatsRequest/Response`

---

## [1.3.0.19] - 2026-06-21

### Features
- **Messages v2** — lightweight message system with `MessageV2` proto (12 fields vs 26 in v1), `oneof content` (text/media/reply), JSONB reactions, cursor-based pagination, sender_id (UUID).
- **ChatV2 stream** — new bidirectional gRPC stream `ChatV2(stream ChatV2Message)` with `oneof payload` (message/typing/system). JWT auth via first message.
- **6 new RPCs** — `GetHistoryV2`, `SendMessageV2`, `EditMessageV2`, `DeleteMessageV2`, `SetReactionV2`, `ChatV2`.
- **messages_v2 table** — new PostgreSQL table with cursor pagination index, reply index, sender index. Reactions stored as JSONB.
- **Dual-write** — old Chat stream writes to both `messages` and `messages_v2` for gradual migration.

### Improvements
- Removed denormalized fields (avatar_url, is_super_admin, replied_to_text, replied_to_user)
- Auth fields (jwt_token, device_id, etc.) moved out of Message proto
- Reactions: batch read (JSONB) instead of N+1 queries
- Cursor pagination: O(log n) instead of OFFSET

### Backward Compatibility
- All existing v1 RPCs continue to work unchanged
- Old `Message` proto and `Chat` stream untouched
- Dual-write ensures messages_v2 stays in sync during migration

---

## [1.3.0.18] - 2026-06-20

### Features
- **Reve Image Generation** — new `reve` provider type for AI image generation via Reve API. Supports text-to-image, edit, and remix workflows. Returns `image_url` in `ChatWithAIV2Response` (field 10).
- **Image URL in ChatWithAIV2Response** — proto field `image_url = 10` added. When an agent produces an image (e.g. Reve), the final response includes the image URL for display in the client.
- **Reve preset agent** — pre-configured agent `reve` using `reve-2.0` model. Requires `REVE_API_KEY` env var or `api_key` in agent's `provider_config`.

---

## [1.3.0.17] - 2026-06-20

### Features
- **Free AI models via server key** — all preset agents now use free OpenRouter models (`:free` suffix). Clients use the server's `OPENROUTER_API_KEY` by default. Users can set their own key to unlock paid models.
- **AI Chat Settings RPCs** — `GetAIChatSettings` and `UpdateAIChatSettings` allow per-session API key and model override. Users set their own OpenRouter key to use paid models; otherwise defaults to free server key.
- **Model override** — users can override the model for a specific AI chat session via `UpdateAIChatSettings.model`. The override applies to the agent's provider config at execution time.
- **New preset: Vision** — image analysis agent using `google/gemma-4-26b-a4b-it:free` (supports images + tools).
- **Preset auto-update** — `seedPresetAgents` now uses `ON CONFLICT DO UPDATE` instead of `DO NOTHING`, so presets are updated on every server restart.

### Preset Models (all free)

| Agent | Model | Tools | RAG |
|-------|-------|-------|-----|
| MiMo | `mimo-auto` | ✅ | ✅ |
| Assistant | `meta-llama/llama-3.3-70b-instruct:free` | ✅ | ✅ |
| Developer | `qwen/qwen3-coder:free` | ✅ | ❌ |
| DevOps | `meta-llama/llama-3.3-70b-instruct:free` | ✅ | ❌ |
| Architect | `nvidia/nemotron-3-super-120b-a12b:free` | ❌ | ❌ |
| Writer | `meta-llama/llama-3.3-70b-instruct:free` | ❌ | ❌ |
| Analyst | `qwen/qwen3-next-80b-a3b-instruct:free` | ✅ | ✅ |
| Translator | `meta-llama/llama-3.3-70b-instruct:free` | ❌ | ❌ |
| Vision | `google/gemma-4-26b-a4b-it:free` | ✅ | ❌ |

---

## [1.3.0.16] - 2026-06-20

### Bug Fixes
- **last_seen_at / last_client_version** — now updated on every chat stream message, not just at connection time. Clients with newer versions are detected on subsequent messages.
- **JWT_SECRET rotated** — all existing tokens invalidated, users will re-login

### Security Fixes
- **File extension validation** — upload handlers now reject disallowed extensions. Image: .jpg/.jpeg/.png/.gif/.webp. File: .pdf/.doc/.docx/.xls/.xlsx/.csv/.json/.zip etc. Audio: .m4a/.aac/.ogg/.mp3/.wav. Full avatar now validates its own extension.
- **DeleteProfile cascade** — now cleans up AI tables (ai_chats_v2, ai_usage_stats, agent_reviews, agents_v2), user data (themes, pins, favorites, chat_list, calls, secret keys), and Hermes/AI session data before deleting user.

### Features
- **RAG message indexing** — AI chat messages (user + assistant) are now embedded and indexed into Qdrant vector DB for semantic search. Async indexing doesn't block chat responses.

### Code Quality
- **Dead code removed** — `StartHTTPServer`, `StartAPKServer`, `logInfo`/`logWarn`/`logError`/`logFatal`, `backfillLastMessageText`
- **go mod tidy** — cleaned up dependencies

---

## [1.3.0.15] - 2026-06-20

### Security Fixes
- **Plaintext message logging removed** — messages now logged as truncated 40-char preview only
- **LIKE injection fix** — `SearchChats` now escapes `%` and `_` wildcards in user input
- **Bcrypt cost increased** — password hashing cost 10→12 for stronger brute-force resistance

---

## [1.3.0.14] - 2026-06-20

### Security Fixes
- **Firebase key removed from git** — removed tracked service account JSON, will rotate key
- **`.env.example` sanitized** — replaced real credentials with placeholders
- **HTTP upload auth** — added JWT Bearer middleware to all upload endpoints + TURN credentials
- **Bot `/logs` admin check** — restricted to admin-only access
- **`query_database` hardened** — read-only transaction, expanded blocklist, sensitive table blocklist
- **DROP TABLE removed** — removed destructive `DROP TABLE IF EXISTS` from hermes migrations
- **Chat stream auth re-enabled** — uncommented unauthenticated message rejection
- **JWT_SECRET startup validation** — server fails fast if JWT_SECRET or CHAT_SECRET_KEY missing/short
- **SSRF protection** — `web_fetch` tool now blocks private IPs, localhost, cloud metadata endpoints
- **JSON injection fix** — `CreateGroupChat` uses `json.Marshal` instead of manual string concatenation

### Bug Fixes
- **HashPassword error propagation** — `UpdatePassword` now returns bcrypt errors
- **N+1 batch query** — `DeleteChat` uses single batch UPDATE instead of per-participant loop
- **Goroutine lifecycle** — `RemoteAgentManager` goroutines now respect context cancellation
- **Firebase context timeouts** — all push notification calls now have 10-30s timeouts

### Code Quality
- **`rows.Err()` checks** — added to key database iteration loops
- **`go mod tidy`** — cleaned up dependencies
- **Russian→English error messages** — admin-only responses now use English

---

## [1.3.0.13] - 2026-06-20

### Performance Optimizations
- **Cursor-based pagination** — `GetChatsV2` now supports keyset pagination via `cursor`/`next_cursor`/`has_more` fields. Legacy offset preserved for backward compatibility. O(log n) instead of O(n) for deep pages.
- **DB connection pool tuning** — `MaxOpenConns` 25→50, `MaxIdleConns` 15→25
- **AI session deduplication** — per-user mutex prevents race conditions when creating AI sessions with empty `session_id`
- **AI tool result caching** — LRU cache (1min TTL, 500 entries) for `search_messages`, `search_users`, `get_chat_info` tools
- **Database indexes** — added composite index for unread CTE, GIN index for `participant_ids` array containment, index for `last_message_time` ordering

---

## [1.3.0.12] - 2026-06-20

### Performance
- **Unread count optimization** — `GetUserChatsV2`, `GetUserChats`, `GetUserChatsByUserID`, `SearchChats` теперь используют `user_chat_metadata.last_read_at` вместо подсчёта всех непрочитанных сообщений. N+1 → O(1) для unread.

---

## [1.3.0.11] - 2026-06-20

### Features
- **ProfileService v2 enabled on prod** — previously dev-only, now all servers expose `messenger.ProfileService/*` (GetProfile, UpdateProfile, UpdateAvatar, DeleteProfile, GetUserSettings, UpdateUserSettings)
- **Unread count fix** — `GetUserChatsV2` and `SearchChats` now compute unread counts via CTE (was missing, client always saw 0)

### Code Quality
- Applied `goimports` formatting across 40 files (alignment, struct field spacing)
- Marked legacy profile methods in `server_profile.go` as deprecated (UpdateUsername, UpdateAvatar, DeleteProfile)
- Updated `server.go` file comment to note ProfileService v2

### Documentation
- Consolidated client integration docs: deleted 3 redundant files (AI_V2_CLIENT_INTEGRATION.md, ANDROID_AI_BILLING_INTEGRATION.md, MARKETPLACE_AGENTS_SETUP.md)
- Updated ARCHITECTURE.md with current AI v2 file structure
- CLIENT_INTEGRATION.md now single comprehensive guide for all client types
- Cleaned up INTEGRATION_SESSION.md, TASKS.md, PROMPT.md, PITFALLS.md, TESTING.md, INDEX.md

---

## [1.3.0.10] - 2026-06-20

### Production RAG — Qdrant + OpenAI Embeddings
- **core/rag/qdrant/qdrant.go** — Qdrant REST API client implementing `VectorSearch` interface
- **core/rag/qdrant/embedding.go** — OpenAI embedding service (`text-embedding-3-small`, 1536 dim)
- **ai_v2.go** — RAG pipeline wired into AI Gateway: Qdrant + OpenAI → in-memory fallback
- RAG augmentation: when agent has `rag_enabled=true`, user query is enriched with relevant context
- Configuration: `QDRANT_URL` + `OPENAI_API_KEY` env vars (set both for production mode)

### Architecture
- Qdrant as binary (~100-200MB RAM), not Docker — memory-efficient for 1.9GB server
- OpenAI text-embedding-3-small: $0.00002/1K tokens (nearly free)
- Fallback chain: Qdrant+OpenAI → in-memory TF-IDF (no external dependencies)

---

## [1.3.0.9] - 2026-06-20

### Redis Rate Limiter — Wired In
- **rate_limiter.go** — `owlRateLimiter` и `freeTierRateLimiter` заменены на `RedisRateLimiter` (prefix `rl:owl:`, `rl:free:`)
- **bot_commands.go** — `botCmdRateLimiter` заменён на `RedisRateLimiter` (prefix `rl:bot:`), удалён `botRateLimiter` struct
- **ai_v2.go** — per-agent rate limiter заменён на `RedisRateLimiter` (prefix `rl:ai:<id>:`)
- **redis_rate_limiter.go** — методы приведены к lowercase API (`allow`/`cancel`/`remaining`/`cleanup`)
- Все limiters: fallback на in-memory если Redis недоступен

### Cleanup
- **db_ai_v2.go** — удалена функция `DropOldAIV1()` (v1 таблицы уже удалены)
- **main.go** — удалён вызов `DropOldAIV1(db.DB)` при старте

### Configuration
- `.env` / `.env.dev` — добавлен `REDIS_ADDR=localhost:6379`

---

## [1.3.0.8] - 2026-06-20

### v1 Compat Removal
- **auth_interceptor.go** — удалены v1 fallback, `extractUsernameFromMetadata`, `ResolveUserID` + кэш
- **auth_service.go** — удалены `authServer` struct, `SignIn`/`SignUp` v1
- **auth_service_v2.go** — `authServerV2` больше не embedит `*authServer`
- **server_chats.go** — удалён `GetChats` v1 endpoint
- **server.go** — удалены `resolveUserId`/`resolveUsername`

### Bug Fixes
- **server_ai_v2.go** — `getAIV2UserID`: исправлен баг с typed context key (всегда возвращал "")
- **server_push.go** — FCM push > 4KB: добавлен `truncateForFCM()` для data payload
- **server_push.go** — `isInvalidTokenError`: добавлена "Requested entity was not found"
- **server_chat.go** — Call stream: gRPC `context.Canceled` больше не логируется как error

### Logging
- **server_ai_v2.go** — `[AI]` логи для всех AI v2 хендлеров
- **server_chat.go** — объединены "Auth success" + "Device registered"
- **server_chat.go** — удалён неинформативный "Stream for %s closed"

### Documentation
- **doc/MARKETPLACE_AGENTS_SETUP.md** — quickstart, пресеты, marketplace, tool calling
- **doc/ANDROID_RATE_LIMIT_PROMPT.md** — rate limiting для Android клиента

### Cleanup
- Удалена `client.android/` из репозитория

---

## [1.3.0.7] - 2026-06-20

### Redis Rate Limiter
- **redis_rate_limiter.go** — Redis-backed sliding window rate limiter (go-redis/v9)
- Автоматический fallback на in-memory если Redis недоступен
- Ключи: `rl:{prefix}:{userID}`, sorted sets с TTL
- Конфигурация через `REDIS_ADDR` env (по умолчанию localhost:6379)

### Documentation
- **doc/ANDROID_AI_BILLING_INTEGRATION.md** — Android integration guide для Usage Stats UI
- **doc/CLIENT_INTEGRATION.md** — исправлены ListMarketplaceAgents и GetAIUsageStats запросы

---

## [1.3.0.4] - 2026-06-20

### Graceful Shutdown
- **hub.go** — `BroadcastShutdown()`: отправляет `SERVER_SHUTTINGDOWN` всем подключённым клиентам перед остановкой
- **main.go** — при SIGTERM: ставит `isShuttingDown` флаг, broadcast shutdown, пауза 2с, затем GracefulStop
- **http_server.go** — health endpoint возвращает 503 `{"status":"shutting_down"}` во время остановки
- **server.go** — добавлен `isShuttingDown atomic.Bool` в server struct

Клиент может:
1. Ловить `SERVER_SHUTTINGDOWN` из Chat стрима и показывать "Переподключение..."
2. Проверять health endpoint перед реконнектом — если 503, ждать перед retry

---

## [1.3.0.3] - 2026-06-20

### Bug Fixes
- **server_profile.go** — MarkRead: use `ResolveUserID(ctx, s.db)` вместо `GetUserID(ctx)` для корректной работы v1 клиентов (username → UUID fallback). Исправляет `pq: null value in column "user_id"` constraint violation.
- **db_chats.go** — `GetUserChats` (v1): добавлен фильтр `'ai'` в `WHERE c.type NOT IN (...)`. Исправляет ghost AI чаты в списке чатов для v1 клиентов.
- **db_chats.go** — `backfillLastMessageText`: добавлен фильтр `'ai'` в `WHERE c.type NOT IN (...)`.

### Documentation
- **doc/CLIENT_INTEGRATION.md** — обновлен до v1.3.0.2: AI v2 RPC (8 основных + 7 marketplace), Capability Negotiation ai >= 2.0

---

## [1.3.0.2] - 2026-06-20

### AI Services v2 — Usage Stats + Marketplace
- **ai_provider.go** — `StreamChunk` добавлен `Usage *StreamUsage` (prompt_tokens, completion_tokens, total_tokens)
- **ai_provider_openrouter.go** — парсинг `usage` из SSE response (финальный чанк)
- **ai_agent_executor.go** — `Execute()` возвращает `ExecutionResult{ModelUsed, TokenCount}`, трекинг токенов
- **ai_v2.go** — `recordUsage()` записывает агрегированную статистику в `ai_usage_stats` (per user/agent/hour)
- **ai_v2.go** — `saveAssistantMessage()` сохраняет реальный `token_count` + `model_used`
- **ai_v2.go** — `GetAIUsageStats()` + `GetAIUsageStatsSummary()` — статистика использования
- **db_ai_v2.go** — новая таблица `ai_usage_stats` (user_id, agent_id, total_tokens, request_count, period_start)
- **db_ai_v2.go** — новая таблица `agent_reviews` (agent_id, user_id, rating 1-5, review)
- **db_ai_v2.go** — `agents_v2` расширена: install_count, avg_rating, review_count, tags, original_agent_id, version, share_code
- **db_ai_v2.go** — CRUD для отзывов: AddAgentReview, GetAgentReviews, DeleteAgentReview
- **db_ai_v2.go** — Marketplace: ListMarketplaceAgents, GetAgentByShareCode, SetAgentShareCode, IncrementInstallCount
- **server_ai_v2.go** — 7 новых RPC: RateAIAgent, GetAIAgentReviews, ListMarketplaceAgents, GetAIAgentStats, ShareAIAgent, InstallAIAgent, GetAIUsageStats
- **messenger.proto** — 7 новых RPC + 10 новых message типов для marketplace и usage stats
- **ai_provider_websocket.go** — fix: non-constant format string в fmt.Errorf

---

## [1.2.0.11] - 2026-06-19

### Оптимизация
- **db.go** — добавлен индекс `idx_messages_username_time ON messages(username, created_at)` для ускорения запросов по пользователю и времени

### Безопасность
- **messenger.proto** — добавлены `reserved 6, 19; reserved "password", "register"` в Message для предотвращения повторного использования удалённых deprecated полей

---

## [1.2.0.10] - 2026-06-19

### Исправления (E2EE)
- **server_messages.go** — GetHistory: `e2ee_payload` возвращается как ONE base64 слой (fix double-encode)
- **server_messages.go** — EditMessage: добавлена проверка `IsE2EE` при декодировании и broadcast
- **server_favorites.go** — GetFavorites: добавлена проверка `IsE2EE` — E2EE сообщения возвращают `e2ee_payload` вместо расшифрованного текста
- **db.go** — backfillLastMessageText: исключены секретные чаты (`is_secret=TRUE`) — предотвращает утечку расшифрованного текста
- **server_chat.go** — Логирование: добавлены client version и device_id

### Производительность (P1 + P2 оптимизации)
- **server_push.go** — FCM batching: `sendBatchPushNotifications()` через `SendEachForMulticast` (до 500 токенов за вызов)
- **server_push.go** — Exponential backoff retry (до 3 попыток) для UNAVAILABLE/RESOURCE_EXHAUSTED
- **server_push.go** — Автоудаление невалидных FCM токенов (UNREGISTERED/INVALID_ARGUMENT)
- **db.go** — `MessageRow` struct: вынесен один раз вместо 10 анонимных копий
- **db.go** — `SaveMessage`: INSERT + version increment + last_message update объединены в транзакцию
- **db.go** — `backfillLastMessageText`: оптимизирован — JOIN LATERAL вместо N+1 запросов, batch UPDATE в транзакции
- **db.go** — Добавлен индекс `idx_messages_username_time ON messages(username, created_at)`
- **db_chatlist_v2.go** — `GetPinnedMessages`: добавлена пагинация (limit/offset)
- **messenger.proto** — `GetPinnedMessagesRequest`: добавлены поля `limit` (field 3) и `offset` (field 4)
- **server_chatlist_v2.go** — `GetChatsV2` лог понижен до Debug (убран спам в логах)

---

## [1.2.0.9] - 2026-06-19

### Производительность (P1 + P2 оптимизации)
- **server_ai.go** — `getAIChatManager()` thread-safe lazy init через `sync.Once`
- **db.go** — `backfillLastMessageText` SQL fix: скобки для корректного приоритета операторов (AND > OR)
- **auth_interceptor.go** — Stream interceptor: `usernameKey` + `deviceIDKey` добавлены в stream context
- **db_auth_devices.go** — `CleanupDeviceAuthLog()`: DELETE >90 дней + deactivate expired devices, cron каждые 24ч
- **db.go** — `IncrementParticipantsChatListVersion` → UUID[]: `unnest(participant_ids)` вместо JSON
- **owl.go** — Rate limiter `cleanup()` + periodic goroutine каждые 10мин (prevents memory leak)
- **auth_interceptor.go** — ResolveUserID cache: in-memory cache TTL 5мин для username→UUID
- **hub.go** — `IsUserOnline` O(1): reverse-lookup sets `userIdSet` + `usernameSet` вместо O(N) scan
- **db.go** — DB pool tuning: `MaxIdleConns=15`, `ConnMaxIdleTime=5min`
- **owl.go** — Context cancellation check в SSE read loop
- **main.go** — Goroutine leak fix: `context.WithCancel` + ticker для всех periodic goroutines

### Исправления
- **server_messages.go** — E2EE payload: возвращаем `encrypted_text` как есть для E2EE (fix double-encode)

### Инфраструктура
- **scripts/db_maintenance.sh** — Оптимизирован: исправлен баг delete_orphans, batch файловая очистка, VACUUM/ANALYZE
- **scripts/run-db-maintenance.sh** — Новый скрипт для удалённого запуска maintenance

### Документация
- Обновлены все doc файлы до v1.2.0.9

---

## [1.2.0.8] - 2026-06-19

### Производительность (P0 оптимизации)
- **hub.go** — Broadcast/BroadcastGlobal/BroadcastTyping/BroadcastCall/BroadcastConference: snapshot streams под локом, отправка без лока (fix deadlock с медленными клиентами)
- **db_chatlist_v2.go** — `getMutedRoomsSet()` batch метод: один SELECT вместо N запросов `isChatMuted` в SearchChats/GetUserChatsV2
- **server_chat.go** — Push notification: `GetChat` вынесен до цикла, `participantSet` для O(1) lookup, `senderNotifiesOthers` проверяется рано
- **hermes_orchestrator.go** — Cleanup goroutine: TTL-очистка сессий >30мин неактивности + лимит 50 сообщений на сессию
- **server_chat.go** — `cleanupRecentMsgs()`: периодическая очистка dedup cache (>10s)
- **auth_jwt.go** — JWT secret кэшируется с проверкой env changes (было `os.Getenv` на каждый запрос)
- **owl.go** — `io.LimitReader(10MB)` для OpenRouter ответов (prevents OOM)

### Исправления
- **server_ai.go** — OWL assistant response теперь сохраняется в БД (было TODO, ломало историю переписки)
- **main.go** — gRPC `GracefulStop()` с 30s timeout (fix deploy hang при зависшем стриме)

### Документация
- **OPTIMIZATION_PLAN.md** — 35 оптимизаций P0-P3
- Обновлены все doc файлы до v1.2.0.8

---

## [1.2.0.7] - 2026-06-19

### Новое
- **UserInfo** — добавлены поля `user_id` (UUID) и `is_super_admin` (bool) в `GetAllUsers` ответ
- **deploy-dev-local.sh** — скрипт деплоя dev сервера с локальной машины (cross-compile + SCP)

---

## [1.2.0.6] - 2026-06-18

### ChatList V2 Last Message Optimization
- DB миграция: `last_message_username`, `last_message_has_image` в `chats`
- SaveMessage обновляет `chats.last_message_*` при отправке
- CTE `WITH last_messages` удалён из GetUserChatsV2/GetUserChats/GetAllChats

---

## [1.2.0.5] - 2026-06-18

### Новое
- **GetChatsV2** — основной эндпоинт чат-листа: фильтры (pinned/archived/muted), пагинация (limit/offset), v2 поля (is_pinned, is_muted, is_archived, pinned_at)

### Исправления
- **pinned_messages** — `room_id` тип изменён с `UUID` на `VARCHAR(255)` (реальные chat ID вида `Ebiker_ferz_direct_1781341380`, не UUID)
- **pinned_messages** — миграция `ALTER COLUMN room_id TYPE VARCHAR(255) USING room_id::text` для существующих данных
- **GetPinnedMessages** — JOIN исправлен: `m.id = pm.message_id` → `m.message_id = pm.message_id` (`messages.id` — integer PK, `messages.message_id` — varchar)
- **PinMessage** — проверка существования сообщения по `message_id` вместо `id` (соответствует client UUID)
- **PinMessage/UnPinMessage/GetPinnedMessages/IsMessagePinned** — убран `::uuid` каст для `room_id`

### Стабильность (critical fixes)
- **main.go** — сервер останавливается при падении БД (ранее работал с nil DB → паника)
- **server_chat.go** — type assertion `data["start_time"].(float64)` заменён на safe check с fallback → нет паники при кривом JSON от клиента
- **db.go** — `UpdateUsername` транзакция: все `tx.Exec()` проверяют ошибки → нет partial updates при сбое
- **main.go + http_server.go** — HTTP сервер шатдаунится при SIGTERM (added `StartHTTPServerAndReturn` + `httpSrv.Shutdown`)
- **server_chat.go** — `defer recover()` добавлен в Chat, Typing, CallSession → паники в stream handlers больше не крашат горутины

### Deprecated (v1 compat, будет удалено в 1.3)
- `authServer` (v1 SignIn/SignUp) — заменён на `authServerV2` (AuthServiceV2)
- `extractUsernameFromMetadata` — v1 fallback по username из gRPC metadata
- `AuthInterceptor` v1 fallback — извлечение username/user_id из metadata при отсутствии JWT
- `AuthStreamInterceptor` bypass для legacy streams (Chat/Typing/CallSession) — v1 аутентификация по паролю внутри потока
- `ResolveUserID` — username→UUID fallback через DB
- `resolveUserId` / `resolveUsername` — нормализаторы идентификаторов v1→v2
- `GetChats` — v1 эндпоинт чат-листа, клиенты должны использовать `GetChatsV2`
- `Hub.IsUserOnline` username fallback — проверка по username для v1 клиентов
- `Hub.BroadcastCall` username matching — маршрутизация вызовов по username для v1
- `user_chat_metadata.username` — nullable колонка, будет удалена после миграции всех данных на `user_id`

---

## [1.2.0.4] - 2026-06-18

### Безопасность (второй раунд фиксов)
- **ChatList v2** — все 9 handlers используют `GetUserID(ctx)` вместо `req.GetUserId()` (auth bypass)
- **Bot commands** — `ProcessBotCommand` перезаписывает `req.UserId`/`req.Username` из auth context
- **Notifications** — 4 handlers (`SubscribeNotifications`, `GetNotificationHistory`, `MarkNotificationsRead`, `GetUnreadCount`) используют `GetUserID(ctx)`
- **DeleteProfile** — пароль обязателен (был необязателен при пустой строке)
- **Agent tokens** — `GenerateAgentToken`/`RevokeAgentToken`/`ListAgentTokens` используют `GetUserID(ctx)` вместо `req.AdminUserId`
- **Secret chat** — `ExchangeSecretKey`/`GetSecretChatKey` проверяют участие в чате (было: любой auth пользователь мог прочитать ключи)
- **Orchestrator** — `getOrCreateSession` освобождает write lock перед DB I/O (double-check pattern)

### Совместимость
- **AuthInterceptor v1 fallback** — если Bearer token отсутствует, извлекает username из gRPC metadata (для v1 клиентов)
- **GetChatsV2** — fallback на `req.Username` / ctx username для v1 клиентов
- **ResolveUserID helper** — резолвит username→UUID через DB для handlers

### Производительность
- **owl.go** — shared `http.Client` с connection pooling (было новый клиент на каждый запрос)
- **db.go** — 5 новых индексов: `chats(type, creator_id)`, `owl_messages(chat_id)`, `messages(room_id)`, `contacts(username)`, `user_tokens(username)`
- **isChatParticipant** — `json.Unmarshal` вместо `strings.Contains` (fix false positive)

---

## [1.2.0.3] - 2026-06-18

### Безопасность (критические фиксы)
- **Path traversal fix** — `http_server.go:serveFileHandler` защищён от `../` атак через `filepath.Clean` + prefix check
- **JWT fallback secret** — `auth/jwt.go:getTokenSecret()` теперь `log.Fatal` если `JWT_SECRET` не задан или < 32 байт (убран хардкод)
- **TURN credentials** — `http_server.go` теперь читает `TURN_SERVER_HOST` и `TURN_SHARED_SECRET` из env (было в коде)
- **Hardcoded admin** — `db.go:ConnectDB` теперь one-time: проверяет `is_super_admin` перед UPDATE (было каждый старт)
- **MarkReadAndCheck** — исправлена UPSERT логика: UPDATE first → INSERT если нет строк (ON CONFLICT ломался из-за отсутствия unique constraint)
- **User impersonation** — все 25+ handlers теперь используют `GetUserID(ctx)` вместо `req.UserId` из запроса
- **Secret chat caller** — `getCallerUsernameSecret` теперь использует `GetUserID(ctx)` вместо перебора hub clients
- **Secret chat membership** — `ExchangeSecretKey`/`GetSecretChatKey` проверяют участие в чате
- **Agent token RPC** — `GenerateAgentToken`/`Revoke`/`List` используют `GetUserID(ctx)` вместо `req.AdminUserId`
- **Firebase credentials** — удалены из git tracking (`.gitignore` уже содержит `*-firebase-adminsdk-*.json`)
- **auth/user_jwt.go** — удалён неиспользуемый файл (дублировал `auth_jwt.go`)
- **DeleteProfile** — пароль обязателен (был необязателен при пустой строке)

### Совместимость
- **AuthInterceptor v1 fallback** — если Bearer token отсутствует, извлекает username из gRPC metadata
- **GetChatsV2** — fallback на `req.Username` / ctx username для v1 клиентов

### Исправлено
- **db_chatlist_v2.go** — миграция проверяет наличие PK перед `DROP NOT NULL` на username

### Производительность
- **owl.go** — shared `http.Client` с connection pooling (было новый клиент на каждый запрос)
- **db.go** — 5 новых индексов (chats type+creator, owl_messages chat, messages room, contacts, user_tokens)
- **hermes_orchestrator.go** — `getOrCreateSession` освобождает lock перед DB I/O (double-check pattern)

### Исправлено
- **db_chatlist_v2.go** — миграция `user_chat_metadata` проверяет наличие PK перед `DROP NOT NULL` на username (fix: column "username" is in a primary key)

---

## [1.2.0.2] - 2026-06-16

### Новое: FCM Push Notifications uplevel
- **server_push.go**: `AndroidConfig.Priority = "high"` + `AndroidNotification.PriorityHigh` — push проходит через Doze
- **server_push.go**: `sendPushNotification(userId, username, ...)` — новая сигнатура с userId
- **server_push.go**: `CollapseKey = roomID` — заменяет предыдущий push для того же чата
- **server_push.go**: `TTL = 5 min` — не хранит старые push
- **hub.go**: `IsUserOnline(userId, username)` — проверка онлайн-статуса по userId (v2) с fallback на username (v1)
- **hub.go**: `SetUserId()` + `clientUserIds` map — хранение userId по stream
- **hub.go**: `Unregister()` теперь очищает `clientUserIds`
- **server_chat.go**: вызов `SetUserId()` при v2 JWT аутентификации
- **db.go**: `GetAllUsers()` теперь возвращает `UserId` (UUID)
- **server_push_test.go**: 7 тестов для `IsUserOnline` (все проходят)

### Исправлено
- **db_chatlist_v2.go**: исправлена миграция `user_chat_metadata` — разделена на шаги, обработка NULL user_id и UUID-as-username

---

## [1.2.0.1] - 2026-06-15

### Новое: Pin Message
- **messenger.proto**: добавлены RPC методы PinMessage, UnPinMessage, GetPinnedMessages
- **messenger.proto**: добавлены сообщения PinMessageRequest/Response, UnPinMessageRequest/Response, GetPinnedMessagesRequest/Response
- **server_chatlist_v2.go**: реализация PinMessage/UnPinMessage/GetPinnedMessages RPC handlers
- **db_chatlist_v2.go**: таблица pinned_messages, PinnedMessageRow struct, CRUD методы
- Все RPC используют только userId (без username)
- Валидация: пользователь должен быть участником чата, сообщение должно существовать
- protoc генерация выполнена (сессия 17)

### Новое: ChatStream v2 (JWT auth)
- **messenger.proto**: добавлен `jwt_token` (field 26) в Message для ChatStream v2 auth
- **server_chat.go**: Chat stream поддерживает `jwt_token` (v2) + `password` (v1 fallback)
- При JWT auth: извлекает user_id и username из claims, валидирует токен
- При password auth: полная обратная совместимость с v1
- ChatServiceVersion = "2.0"

### Новое: ChatList v2
- **messenger.proto**: добавлены RPC методы PinChat, UnPinChat, SearchChats, ArchiveChat, UnarchiveChat
- **messenger.proto**: добавлены `is_pinned`, `is_muted`, `is_archived`, `pinned_at` в ChatInfo
- **messenger.proto**: добавлены `limit`, `offset`, `filter` в GetChatsRequest (пагинация)
- **server_chatlist_v2.go**: реализация PinChat/UnPinChat/SearchChats/ArchiveChat/UnarchiveChat
- **db_chatlist_v2.go**: миграции (user_chat_metadata: pinned/pinned_at/archived), методы DB
- user_chat_metadata: персальные настройки чатов (pinned, archived) для каждого пользователя

### Backward compatibility
- v1 клиенты работают без изменений
- v2 клиенты определяют версию через /info endpoint

---

## [1.2.1.0] - 2026-06-14

### Новое: ProfileService v2 (dev only)
- Отдельный gRPC сервис для управления профилем (JWT Bearer auth)
- Методы: GetProfile, UpdateProfile, UpdateAvatar, DeleteProfile, GetUserSettings, UpdateUserSettings
- Данные: аватар, bio, status, locale (en/ru), isSuperAdmin, theme, push settings
- Регистрируется только на dev сервере (APP_ENV=dev)
- ProfileServiceVersion = "2.0" в /info endpoint

### Новое: user_settings таблица
- Хранение настроек пользователя: locale, theme_id, push_enabled, custom JSONB
- Миграция через db_auth_migrations.go

### Исправлено: AuthStreamInterceptor whitelist
- Добавлены Typing и CallSession streams в whitelist (v1 compat)
- v1 клиенты теперь могут вызывать Typing/CallSession без JWT

---

## [1.2.0.1] - 2026-06-14

### Новое: Server info endpoint
- **GET /info** — возвращает версии сервисов для client capability negotiation
- `services.auth >= "2.0"` → клиент использует JWT workflow
- `services.auth < "2.0"` или endpoint недоступен → legacy workflow

### Новое: APP_ENV support
- Загрузка `.env.<APP_ENV>` (например `.env.dev`) вместо `.env`
- Systemd: только `Environment=APP_ENV=dev`, без дублирования переменных

### Исправлено
- Panic после `failed to listen` — добавлен `return` после ошибки `net.Listen`
- Systemd dev unit упрощён

---

## [1.2.0.0] - 2026-06-14

### Новое: AuthService v2 (JWT) — основной метод аутентификации
- **SignInV2/SignUpV2** — JWT access (15min) + refresh (30 days) tokens
- **RefreshToken** — ротация refresh token с обнаружением reuse
- **SignOut/RevokeDevice/GetDevices** — управление сессиями
- **gRPC Bearer token interceptor** — валидация JWT на каждом вызове
- **Device management** — user_devices, device_auth_log таблицы
- **Auth audit log** — логирование всех auth событий

### Deprecated: AuthService v1 (Chat stream auth)
- v1 продолжает работать для совместимости со старыми клиентами
- При входе по v1 сервер отправляет warning:
  `DEPRECATED: AuthService v1 is deprecated. Please upgrade to v2 (JWT).`
- Все функции v1 работают без ограничений

---

## [1.1.3.10] - 2026-06-14

### Исправлено
- Онлайн-статус: очистка истекших grace period (30с) в `GetOnlineUsers()`
  Раньше пользователи оставались "онлайн" навсегда после отключения

---

## [1.1.3.9] - 2026-06-13
- ServerVersion обновлён до 1.1.3.9

## [1.1.3.8] - 2026-06-13

### Новое: DeployAgentTaskStream
- Один финальный `done=True` с полными данными (stdout, stderr, exit_code, duration_ms)
- 6 unit-тестов

### Рефакторинг
- Remote Agent RPC вынесен в `server_remote.go`
- Graceful degradation + stale detection

## [1.1.3.7] - 2026-06-13

### Новое: Streaming RPC
- `DeployAgentTaskStream` — server-side streaming для real-time вывода задач
- `HandleTaskStream` + `RemoteTaskStreamUpdate` callback

## [1.1.3.5] - 2026-06-13
- Remote Agent: foreground service + singleton manager

## [1.1.3.4] - 2026-06-12
- Hermes Gateway (SSH туннель), 40 unit tests

## [1.1.3.3] - 2026-06-12
- Reconnect + token filtering + task results

## [1.1.3.2] - 2026-06-12
- Health check, graceful shutdown, agent process management

## [1.1.3.1] - 2026-06-12
- Token UX fixes, rate limiting

## [1.1.3.0] - 2026-06-12
- Agent Token RPCs без IsSuperAdmin, Platform Adapter

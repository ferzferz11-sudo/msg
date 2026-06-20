# Лава — Server Changelog

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

# Лава — Server Changelog

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

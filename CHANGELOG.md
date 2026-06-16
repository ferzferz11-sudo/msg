# Лава — Server Changelog

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

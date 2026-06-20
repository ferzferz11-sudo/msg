# Lavender Messenger — Client Integration Guide

**Сервер:** v1.3.0.9 | **Протокол:** gRPC + Protocol Buffers | **Дата:** 2026-06-20

Единый документ для интеграции нового клиента с сервером Lavender Messenger.

---

## Серверы

| | Prod | Dev |
|---|---|---|
| gRPC | `13.140.25.249:50051` | `13.140.25.249:50052` |
| HTTP | `13.140.25.249:8082` | `13.140.25.249:8083` |
| Logs | — | `http://13.140.25.249/server-logs-dev` |

---

## Аутентификация

### AuthService (messenger.proto)

| Метод | Запрос | Ответ | Описание |
|-------|--------|-------|----------|
| `SignInV2` | `SignInRequestV2 { username, password, device: DeviceInfo { device_id, device_name, platform } }` | `AuthResponseV2 { success, message, user_id, username, access_token, refresh_token, expires_at }` | Вход (JWT) |
| `SignUpV2` | `SignUpRequestV2 { username, password, email, device: DeviceInfo }` | `AuthResponseV2` | Регистрация (JWT) |
| `RefreshToken` | `RefreshTokenRequest { refresh_token }` | `RefreshTokenResponse { success, access_token, refresh_token, expires_at }` | Обновление access token |
| `SignOut` | `SignOutRequest { user_id, all_devices }` | `AuthResponse` | Выход |
| `RevokeDevice` | `RevokeDeviceRequest { user_id, device_id }` | `AuthResponse` | Отзыв устройства |

### JWT Workflow

```
1. SignInV2 → access_token (15 мин) + refresh_token (30 дней)
2. Каждый gRPC запрос: metadata["authorization"] = "Bearer <access_token>"
3. Access истёк → RefreshToken(refresh_token) → новые токены
4. Refresh token rotation: каждый refresh → новый refresh, старый инвалидируется
```

### Interceptor

Сервер использует `AuthInterceptor` — извлекает `user_id` из JWT в контекст.
Для неавторизованных вызовов: только `AuthService` методы (`SignIn`, `SignUp`, `SignInV2`, `SignUpV2`).

---

## gRPC Services

### 1. ChatService (messenger.proto) — 130+ метода

#### Bidirectional Streams

| Метод | Тип | Описание |
|-------|-----|----------|
| `Chat` | `stream Message ↔ stream Message` | Основной чат-стрим. Первое сообщение: `{ room_id, user_id }` для идентификации |
| `Typing` | `stream TypingRequest ↔ stream TypingSignal` | Индикатор набора текста |
| `CallSession` | `stream CallMessage ↔ stream CallMessage` | WebRTC signaling (SDP, ICE, accept, reject, hangup) |

**Chat stream — JWT auth:**
Первое сообщение в стриме должно содержать `user_id` из JWT (не из request fields — сервер валидирует через context).

#### Chat Management

| Метод | Запрос | Ответ | Описание |
|-------|--------|-------|----------|
| `GetChatsV2` | `GetChatsRequest { user_id, limit, offset, filter: pinned/archived/muted/all }` | `GetChatsResponse { chats: ChatInfo[] }` | **Основной** эндпоинт чат-листа |
| `GetAllChats` | `GetAllChatsRequest` | `GetAllChatsResponse { chats: ChatInfo[] }` | Все чаты (admin) |
| `CreateDirectChat` | `CreateDirectChatRequest { user_id, other_user_id }` | `CreateDirectChatResponse { chat_id }` | 1-on-1 чат |
| `CreateGroupChat` | `CreateGroupChatRequest { user_id, name, participant_ids[] }` | `CreateGroupChatResponse { chat_id }` | Групповой чат |
| `DeleteChat` | `DeleteChatRequest { chat_id, requester_user_id }` | `DeleteDeleteChatResponse` | Удаление чата |
| `UpdateChatName` | `UpdateChatNameRequest { chat_id, user_id, new_name }` | `UpdateChatNameResponse` | Переименование |
| `UpdateChatAvatar` | `UpdateChatAvatarRequest { chat_id, user_id, avatar_url }` | `UpdateChatAvatarResponse` | Аватар чата |
| `UpdateChatSettings` | `UpdateChatSettingsRequest { ... }` | `UpdateChatSettingsResponse` | Настройки чата |
| `AddParticipant` | `AddParticipantRequest { chat_id, user_id, new_participant_id }` | `AddParticipantResponse` | Добавить участника |
| `RemoveParticipant` | `RemoveParticipantRequest { chat_id, user_id, participant_id }` | `RemoveParticipantResponse` | Удалить участника |

#### ChatList v2

| Метод | Запрос | Ответ | Описание |
|-------|--------|-------|----------|
| `PinChat` | `PinChatRequest { chat_id, user_id }` | `PinChatResponse` | Закрепить чат |
| `UnPinChat` | `UnPinChatRequest { chat_id, user_id }` | `UnPinChatResponse` | Открепить |
| `ArchiveChat` | `ArchiveChatRequest { chat_id, user_id }` | `ArchiveChatResponse` | Архивировать |
| `UnarchiveChat` | `UnarchiveChatRequest { chat_id, user_id }` | `UnarchiveChatResponse` | Разархивировать |
| `SearchChats` | `SearchChatsRequest { user_id, query, limit, offset }` | `SearchChatsResponse` | Поиск чатов |
| `GetChatListVersion` | `GetChatListVersionRequest { user_id }` | `GetChatListVersionResponse { version }` | Версия для кэширования |

#### Messages

| Метод | Запрос | Ответ | Описание |
|-------|--------|-------|----------|
| `GetHistory` | `GetHistoryRequest { room_id, limit }` | `GetHistoryResponse { messages: Message[] }` | История сообщений |
| `SetReaction` | `ReactionRequest { message_id, room_id, user_id, emoji }` | `ReactionResponse` | Реакция |
| `DeleteMessages` | `DeleteMessagesRequest { message_ids[], room_id, user_id }` | `DeleteMessagesResponse` | Удаление |
| `EditMessage` | `EditMessageRequest { message_id, room_id, user_id, new_text }` | `EditMessageResponse` | Редактирование |
| `MarkRead` | `MarkReadRequest { room_id, user_id, message_id }` | `MarkReadResponse` | Прочитано |

#### Pin Messages

| Метод | Запрос | Ответ | Описание |
|-------|--------|-------|----------|
| `PinMessage` | `PinMessageRequest { message_id, chat_id, user_id }` | `PinMessageResponse` | Закрепить сообщение |
| `UnPinMessage` | `UnPinMessageRequest { message_id, chat_id, user_id }` | `UnPinMessageResponse` | Открепить |
| `GetPinnedMessages` | `GetPinnedMessagesRequest { chat_id, user_id }` | `GetPinnedMessagesResponse` | Список закреплённых |

#### Secret Chats (E2EE)

| Метод | Запрос | Ответ | Описание |
|-------|--------|-------|----------|
| `CreateSecretChat` | `CreateSecretChatRequest { user_id, other_user_id }` | `CreateSecretChatResponse { chat_id }` | Создать секретный чат |
| `ExchangeSecretKey` | `ExchangeSecretKeyRequest { chat_id, user_id, public_key }` | `ExchangeSecretKeyResponse` | Обмен ключами |
| `GetSecretChatKey` | `GetSecretChatKeyRequest { chat_id, user_id }` | `GetSecretChatKeyResponse { public_key }` | Получить ключ |

#### Drafts

| Метод | Запрос | Ответ | Описание |
|-------|--------|-------|----------|
| `SaveDraft` | `SaveDraftRequest { user_id, room_id, text }` | `SaveDraftResponse` | Сохранить черновик |
| `GetDraft` | `GetDraftRequest { user_id, room_id }` | `GetDraftResponse { text }` | Получить черновик |
| `DeleteDraft` | `DeleteDraftRequest { user_id, room_id }` | `DeleteDraftResponse` | Удалить черновик |

#### Favorites

| Метод | Запрос | Ответ | Описание |
|-------|--------|-------|----------|
| `AddFavorite` | `AddFavoriteRequest { user_id, message_id }` | `AddFavoriteResponse` | В избранное |
| `RemoveFavorite` | `RemoveFavoriteRequest { user_id, message_id }` | `RemoveFavoriteResponse` | Убрать из избранного |
| `GetFavorites` | `GetFavoritesRequest { user_id }` | `GetFavoritesResponse` | Список избранных |

#### Muted

| Метод | Запрос | Ответ | Описание |
|-------|--------|-------|----------|
| `GetMutedChats` | `GetMutedChatsRequest { user_id }` | `GetMutedChatsResponse { room_ids[] }` | Заглушенные чаты |
| `SetMutedChat` | `SetMutedChatRequest { user_id, room_id, muted }` | `SetMutedChatResponse` | Заглушить/включить |

#### Users & Profile (v1 — через ChatService)

| Метод | Запрос | Ответ | Описание |
|-------|--------|-------|----------|
| `GetAllUsers` | `GetAllUsersRequest` | `GetAllUsersResponse { users: UserInfo[] }` | Все пользователи |
| `GetUserProfile` | `GetUserProfileRequest { user_id OR username }` | `GetUserProfileResponse` | Профиль пользователя |
| `GetUserAvatar` | `GetUserAvatarRequest { user_id OR username }` | `GetUserAvatarResponse { avatar_url, full_avatar_url }` | Аватар |
| `GetUserId` | `GetUserIdRequest { username }` | `GetUserIdResponse { user_id }` | username → UUID |
| `UpdateProfile` | `UpdateProfileRequest { user_id, bio, status }` | `UpdateProfileResponse` | Обновить профиль |
| `UpdateUsername` | `UpdateUsernameRequest { user_id, new_username }` | `UpdateUsernameResponse` | Сменить username |
| `UpdatePassword` | `UpdatePasswordRequest { user_id, old_password, new_password }` | `UpdatePasswordResponse` | Сменить пароль |
| `AdminUpdatePassword` | `AdminUpdatePasswordRequest { admin_user_id, target_user_id, new_password }` | `AdminUpdatePasswordResponse` | Админ: сменить пароль |
| `DeleteProfile` | `DeleteProfileRequest { user_id, password }` | `DeleteProfileResponse` | Удалить аккаунт (пароль обязателен) |

**UserInfo (GetAllUsers):**
```protobuf
message UserInfo {
  string username = 1;
  string avatar_url = 2;
  string last_client_version = 3;
  Timestamp last_seen_at = 4;
  string email = 5;
  string user_id = 6;       // v1.2.0.7
  bool is_super_admin = 7;  // v1.2.0.7
}
```

#### Contacts

| Метод | Запрос | Ответ | Описание |
|-------|--------|-------|----------|
| `AddContact` | `AddContactRequest { user_id, contact_user_id }` | `AddContactResponse` | Добавить контакт |
| `RemoveContact` | `RemoveContactRequest { user_id, contact_user_id }` | `RemoveContactResponse` | Удалить контакт |
| `GetContacts` | `GetContactsRequest { user_id }` | `GetContactsResponse` | Список контактов |

#### Themes

| Метод | Запрос | Ответ | Описание |
|-------|--------|-------|----------|
| `GetThemes` | `GetThemesRequest { user_id }` | `GetThemesResponse` | Темы пользователя |
| `SaveTheme` | `SaveThemeRequest { user_id, name, colors }` | `SaveThemeResponse` | Сохранить тему |
| `SetCurrentTheme` | `SetCurrentThemeRequest { user_id, theme_id }` | `SetCurrentThemeResponse` | Выбрать тему |
| `DeleteTheme` | `DeleteThemeRequest { user_id, theme_id }` | `DeleteThemeResponse` | Удалить тему |

#### Push Notifications

| Метод | Запрос | Ответ | Описание |
|-------|--------|-------|----------|
| `RegisterToken` | `TokenRequest { user_id, token, platform, device_id }` | `TokenResponse` | Зарегистрировать FCM token |
| `GetDevices` | `GetDevicesRequest { user_id }` | `GetDevicesResponse` | Устройства |
| `DeleteDevice` | `DeleteDeviceRequest { user_id, device_id }` | `DeleteDeviceResponse` | Удалить устройство |
| `DeleteOtherDevices` | `DeleteDeviceRequest { user_id, device_id }` | `DeleteDeviceResponse` | Удалить остальные |

#### Password Reset

| Метод | Запрос | Ответ | Описание |
|-------|--------|-------|----------|
| `RequestPasswordReset` | `RequestPasswordResetRequest { email }` | `RequestPasswordResetResponse` | Отправить email |
| `ResetPassword` | `ResetPasswordRequest { token, new_password }` | `ResetPasswordResponse` | Сбросить пароль |

#### AI Services v2 (с v1.3.0.0)

| Метод | Запрос | Ответ | Описание |
|-------|--------|-------|----------|
| `ChatWithAIV2` | `ChatWithAIV2Request { session_id, message, images[], agent_id, tool_calls[] }` | `stream ChatWithAIV2Response` | Единый AI чат (simple/agent/pipeline) |
| `CreateAIAgent` | `CreateAIAgentRequest { name, provider_type, model, ... }` | `CreateAIAgentResponse { agent_id }` | Создать агента |
| `UpdateAIAgent` | `UpdateAIAgentRequest { agent_id, ... }` | `UpdateAIAgentResponse` | Обновить агента |
| `DeleteAIAgent` | `DeleteAIAgentRequest { agent_id }` | `DeleteAIAgentResponse` | Удалить агента |
| `GetAIAgent` | `GetAIAgentRequest { agent_id }` | `GetAIAgentResponse { agent: AgentInfoV2 }` | Информация об агенте |
| `ListAIAgents` | `ListAIAgentsRequest { include_public }` | `ListAIAgentsResponse { agents[] }` | Список агентов (свои + пресеты + публичные) |
| `CloneAIAgent` | `CloneAIAgentRequest { agent_id, new_name }` | `CloneAIAgentResponse { agent_id }` | Клонировать агента |
| `ListAITools` | `ListAIToolsRequest {}` | `ListAIToolsResponse { tools[] }` | Доступные инструменты |

**AI Marketplace** (с v1.3.0.2):

| Метод | Запрос | Ответ | Описание |
|-------|--------|-------|----------|
| `RateAIAgent` | `RateAIAgentRequest { agent_id, rating, review }` | `RateAIAgentResponse` | Оценить агента |
| `GetAIAgentReviews` | `GetAIAgentReviewsRequest { agent_id }` | `GetAIAgentReviewsResponse { reviews[] }` | Отзывы на агента |
| `ListMarketplaceAgents` | `ListMarketplaceAgentsRequest { query, limit, offset }` | `ListMarketplaceAgentsResponse { agents[], total }` | Маркетплейс агентов с поиском и пагинацией |
| `GetAIAgentStats` | `GetAIAgentStatsRequest { agent_id }` | `GetAIAgentStatsResponse` | Статистика агента |
| `ShareAIAgent` | `ShareAIAgentRequest { agent_id }` | `ShareAIAgentResponse { share_code }` | Поделиться агентом |
| `InstallAIAgent` | `InstallAIAgentRequest { share_code }` | `InstallAIAgentResponse { agent_id }` | Установить агента по коду |
| `GetAIUsageStats` | `GetAIUsageStatsRequest {}` | `GetAIUsageStatsResponse { stats[], total_tokens, total_requests }` | Статистика использования AI (токены, запросы per-agent) |

**Типы AI чатов:**
- `simple` — прямой LLM (как ChatGPT)
- `agent` — multi-agent с роутингом
- `pipeline` — RAG + tools chain

**Встроенные инструменты:** search_messages, search_users, web_search, web_fetch, get_chat_info, query_database

**Провайдеры:** openrouter, local, mimo, webhook, websocket, subprocess, mcp

**Пресеты:** mimo, assistant, developer, devops, architect, writer, analyst, translator

> Полное описание см. в `doc/AI_V2_CLIENT_INTEGRATION.md`

#### Notifications

| Метод | Запрос | Ответ | Описание |
|-------|--------|-------|----------|
| `SubscribeNotifications` | `SubscribeNotificationsRequest { user_id }` | `stream ServerNotification` | Подписка на уведомления |
| `GetNotificationHistory` | `GetNotificationHistoryRequest { user_id, limit }` | `GetNotificationHistoryResponse` | История уведомлений |
| `MarkNotificationsRead` | `MarkNotificationReadRequest { user_id, notification_ids[] }` | `MarkNotificationReadResponse` | Прочитано |
| `GetUnreadCount` | `GetUnreadCountRequest { user_id }` | `GetUnreadCountResponse { count }` | Непрочитанные |

#### Bot Commands

| Метод | Запрос | Ответ | Описание |
|-------|--------|-------|----------|
| `ProcessBotCommand` | `BotCommandRequest { user_id, command, args }` | `BotCommandResponse { response }` | Выполнить команду (/status, /help, /deploy...) |
| `GetBotCommands` | `GetBotCommandsRequest` | `GetBotCommandsResponse` | Список команд |

#### Free Models (admin)

| Метод | Запрос | Ответ | Описание |
|-------|--------|-------|----------|
| `GetFreeModels` | `GetFreeModelsRequest` | `GetFreeModelsResponse { models[] }` | Бесплатные модели |
| `SetFreeModel` | `SetFreeModelRequest { admin_user_id, model_id, name }` | `SetFreeModelResponse` | Добавить модель |
| `RemoveFreeModel` | `RemoveFreeModelRequest { admin_user_id, model_id }` | `RemoveFreeModelResponse` | Удалить модель |

---

### 2. ProfileService (messenger.proto) — JWT-only (dev)

| Метод | Запрос | Ответ | Описание |
|-------|--------|-------|----------|
| `GetProfile` | `GetProfileRequest` | `GetProfileResponse { user_id, username, display_name, bio, status, avatar_url, is_super_admin }` | Профиль из JWT context |
| `UpdateProfile` | `UpdateProfileV2Request { username, bio, status, locale }` | `UpdateProfileV2Response { ... }` | Обновить (возвращает обновлённый профиль) |
| `UpdateAvatar` | `UpdateAvatarV2Request { avatar_url, full_avatar_url }` | `UpdateAvatarV2Response` | Аватар |
| `DeleteProfile` | `DeleteProfileV2Request { password }` | `DeleteProfileV2Response` | Удалить аккаунт |
| `GetUserSettings` | `GetUserSettingsRequest` | `GetUserSettingsResponse { locale, theme_id, push_enabled, settings{} }` | Настройки |
| `UpdateUserSettings` | `UpdateUserSettingsRequest { locale, theme_id, push_enabled, settings{} }` | `UpdateUserSettingsResponse` | Обновить настройки |

---

### 3. ServerService (server.proto) — public + admin

| Метод | Запрос | Ответ | Описание |
|-------|--------|-------|----------|
| `ListServers` | `ListServersRequest` | `ListServersResponse { servers[] }` | Список серверов (public) |
| `GetDefaultServer` | `GetDefaultServerRequest` | `GetDefaultServerResponse` | Сервер по умолчанию (public) |
| `AddServer` | `AddServerRequest { auth: AdminAuth, name, address, ... }` | `AddServerResponse` | Добавить (admin) |
| `UpdateServer` | `UpdateServerRequest { auth: AdminAuth, ... }` | `UpdateServerResponse` | Обновить (admin) |
| `DeleteServer` | `DeleteServerRequest { auth: AdminAuth, server_id }` | `DeleteServerResponse` | Удалить (admin) |
| `SetDefaultServer` | `SetDefaultServerRequest { auth: AdminAuth, server_id }` | `SetDefaultServerResponse` | По умолчанию (admin) |

---

### 4. HermesAgentService (hermes_agent.proto) — agent daemon

| Метод | Тип | Описание |
|-------|-----|----------|
| `Connect` | `stream AgentMessage ↔ stream OrchestratorMessage` | Bidirectional stream для hermes-agent daemon |
| `GenerateAgentToken` | `AgentTokenRequest → AgentTokenResponse` | Создать токен (admin) |
| `RevokeAgentToken` | `RevokeTokenRequest → RevokeTokenResponse` | Отозвать токен (admin) |
| `ListAgentTokens` | `ListTokensRequest → ListTokensResponse` | Список токенов (admin) |

---

## HTTP Endpoints

| Метод | Путь | Описание |
|-------|------|----------|
| GET | `/health` | Health check |
| GET | `/info` | Версии сервисов (для capability negotiation) |
| POST | `/upload/avatar` | Загрузка аватара |
| POST | `/upload/image` | Загрузка изображения |
| POST | `/upload/file` | Загрузка файла |
| POST | `/upload/background` | Загрузка фона |
| POST | `/upload/audio` | Загрузка аудио |
| GET | `/files/<path>` | Просмотр файлов |
| GET | `/turn` | TURN credentials (WebRTC) |

---

## Capability Negotiation

При старте клиент запрашивает `GET /info`:
```json
{
  "services": {
    "auth": "2.0",
    "chat": "2.0",
    "profile": "2.0",
    "ai": "2.0"
  }
}
```

- `auth >= "2.0"` → использовать `SignInV2` + JWT workflow
- `chat >= "2.0"` → использовать `GetChatsV2`, JWT в Chat stream
- `ai >= "2.0"` → использовать `ChatWithAIV2` вместо старого `ChatWithAI`

---

## Graceful Shutdown и Reconnection (v1.3.0.4+)

Сервер поддерживает graceful shutdown — клиенты получают предупреждение перед отключением.

### Поведение сервера при остановке

1. Сервер получает SIGTERM (рестарт, деплой)
2. Отправляет `SERVER_SHUTTINGDOWN` всем подключённым клиентам через Chat стрим
3. Ждёт 2 секунды (время на получение клиентом)
4. Вызывает `GracefulStop()` — закрывает новые подключения, ждёт завершения активных RPC (до 30с)

### Health Endpoint

```
GET /health
```

| Статус | Ответ | Когда |
|--------|-------|-------|
| `200 OK` | `{"status":"ok","version":"1.3.0.4"}` | Сервер работает |
| `503 Service Unavailable` | `{"status":"shutting_down","version":"1.3.0.4"}` | Сервер останавливается |

### Рекомендуемое поведение клиента

**1. Обработка `SERVER_SHUTTINGDOWN` в Chat стриме:**
```kotlin
// В обработчике сообщений Chat стрима
if (message.user == "SYSTEM" && message.text == "SERVER_SHUTTINGDOWN") {
    // Показать индикатор "Переподключение..."
    showReconnecting()
}
```

**2. Обработка `UNAVAILABLE` ошибки:**
```kotlin
// При получении StatusRuntimeException.UNAVAILABLE
catch (e: StatusRuntimeException) {
    if (e.status.code == Status.Code.UNAVAILABLE) {
        // Не очищать список чатов!
        // Показать "Сервер недоступен, повторное подключение..."
        showReconnecting()
        scheduleReconnect()
    }
}
```

**3. Retry с exponential backoff:**
```kotlin
fun scheduleReconnect() {
    // Сначала проверить health endpoint
    scope.launch {
        val health = checkHealth() // GET /health
        if (health.status == "shutting_down") {
            delay(5000) // Ждать 5с если сервер останавливается
            scheduleReconnect()
            return@launch
        }
        // Сервер доступен — реконнект
        delay(reconnectDelay)
        reconnectDelay = minOf(reconnectDelay * 2, 30_000L) // max 30s
        reconnectChatStream()
    }
}
```

**4. Кэширование данных:**
- Кэшировать список чатов локально (Room DB / SQLite)
- При `SERVER_SHUTTINGDOWN` или `UNAVAILABLE` — показывать кэшированные данные
- После реконнекта — обновить данные с сервера

### Порядок действий при деплое сервера

```
1. Клиент получает SERVER_SHUTTINGDOWN → показывает "Переподключение..."
2. Сервер закрывает соединения (GracefulStop)
3. Клиент получает UNAVAILABLE → НЕ очищает UI, показывает кэш
4. Клиент poll /health → 503 → ждёт
5. Новый сервер запускается → /health возвращает 200
6. Клиент переподключается → обновляет данные
```

---

## Message (основной тип)

```protobuf
message Message {
  string message_id = 1;     // UUID
  string room_id = 2;        // chat ID
  string user_id = 3;        // UUID отправителя
  string username = 4;       // отображаемое имя (deprecated, используй user_id)
  bytes encrypted = 5;       // зашифрованное содержимое
  string text = 6;           // расшифрованный текст
  int64 timestamp = 7;
  string reply_to = 8;
  repeated Reaction reactions = 9;
  bool is_read = 10;
  bool has_image = 11;
  // ... (см. полное определение в messenger.proto)
}
```

---

## Proto Files

| Файл | Назначение |
|------|------------|
| `messenger.proto` | ChatService, AuthService, ProfileService, все основные RPC |
| `server.proto` | ServerService (admin) |
| `hermes_agent.proto` | HermesAgentService (agent daemon) |

---

## Команды для разработки

```bash
# Генерация proto
protoc --go_out=./gen --go_opt=paths=source_relative \
  --go-grpc_out=./gen --go-grpc_opt=paths=source_relative messenger.proto

# Деплой dev (с сервера)
./scripts/deploy-dev.sh

# Деплой dev (с локальной машины)
./scripts/deploy-dev-local.sh

# Деплой prod (с локальной машины)
./scripts/release.sh <version> --deploy --remote

# Тесты
go test ./...
```

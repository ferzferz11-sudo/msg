# Лава — Server Changelog

## [1.1.2.10] - 2026-06-11
- **Рефакторинг server.go** — разбит на 12 файлов по доменам (server_*.go)
- server_chat.go — Chat, Typing, CallSession, GetClients
- server_users.go — GetAllUsers, UpdateProfile, GetUserProfile, GetUserAvatar
- server_chats.go — GetAllChats, GetChats, CreateDirectChat, CreateGroupChat, DeleteChat, etc.
- server_messages.go — GetHistory, SetReaction, DeleteMessages, EditMessage
- server_profile.go — UpdateUsername, UpdatePassword, AdminUpdatePassword, MarkRead, UpdateAvatar, DeleteProfile
- server_push.go — RegisterToken, sendPushNotification, broadcastOnlineUsers, etc.
- server_contacts.go — AddContact, RemoveContact, GetContacts, GetChatListVersion
- server_themes.go — GetThemes, SaveTheme, SetCurrentTheme, DeleteTheme
- server_drafts.go — GetFCMLogs, SaveDraft, GetDraft, DeleteDraft
- server_muted.go — GetMutedChats, SetMutedChat
- server_favorites.go — GetUserId, AddFavorite, RemoveFavorite, GetFavorites, etc.
- server_ai.go — ChatWithOWL, ChatWithAI, ChatWithOrchestrator, Hermes sessions, etc.
- server_management.go — ServerServiceServer (восстановлен)
- Dev сервер обновлён и работает

## [1.1.2.9] - 2026-06-11
- **AuthService — отдельный gRPC сервис для аутентификации**
- Новый proto-сервис `AuthService` с методами `SignIn` и `SignUp`
- `SignIn` — проверка username/password, возврат токена (UUID) и User
- `SignUp` — регистрация нового пользователя с проверкой уникальности username/email
- Новые proto-сообщения: `User`, `SignInRequest`, `SignUpRequest`, `AuthResponse`
- Реализация: `auth_service.go` — `authServer` с методами `SignIn`, `SignUp`
- Регистрация в `main.go`: `gen.RegisterAuthServiceServer(s, authServer)`
- Метод `SaveUserWithEmail` в `db.go` для создания пользователя с email
- Dev сервер обновлён и работает

## [1.1.2.8] - 2026-06-11
- **Auth токены для удалённых агентов (JWT)**
- HS256 JWT с подписью, expiration, agent_id в claims
- Таблица `agent_tokens` в БД (хранится только SHA-256 хеш, не сам токен)
- 3 новых admin RPC: `GenerateAgentToken`, `RevokeAgentToken`, `ListAgentTokens`
- `validateToken()` — полная проверка: подпись, expiration, agent_id match, revoked в БД
- Секрет из `JWT_SECRET` env (32+ байта)
- Dev и prod обновлены

## [1.1.2.7] - 2026-06-11
- **Android: Splash улучшения, онбординг удалён, чекбокс чата**
- Серверных изменений нет, v1.1.2.4

## [1.1.2.6] - 2026-06-10
- **Android: ChangelogActivity — bundled changelog + GitHub links**
- **Bundled changelog**: `app/src/main/assets/changelog_bundled.txt` — встроен в APK, показывается мгновенно
- **Новая логика загрузки**: bundled → GitHub API → server fallback
- **Ссылки на CHANGELOG.md**: кнопки «Ченджлог сервера» и «Ченджлог клиента» на GitHub
- **changelog.txt удалён** из проекта и из деплоя на сервер
- **deploy_android.sh обновлён**: убрана загрузка changelog.txt
- Серверных изменений нет, v1.1.2.4

## [1.1.2.5] - 2026-06-10
- **Android: ChangelogActivity — кастомные темы** — добавлен ThemeUi.bind для применения кастомных тем
- Серверных изменений нет, v1.1.2.4

## [1.1.2.4] - 2026-06-10
- **Bugfix: Hermes история не загружалась** — ChatWithOrchestrator и GetOrchestratorHistory
  использовали hermesDB (hermes_messages таблица), но она была дропнута в v1.1.2.3
- **Исправлено:** все вызовы переведены на AIChatManager (ai_chat_messages таблица)
- ChatWithOrchestrator: save → manager.AddMessage()
- GetOrchestratorHistory: load → manager.GetHistory() + проверка владельца
- **Bugfix: Rate limiter — failed requests refund** — cancel(userID) добавлен во все failure paths
- **Bugfix: Avatar delete — новый файл удалялся при совпадении хеша** — UpdateAvatar теперь сравнивает имена файлов
- ServerVersion: 1.1.2.4
- Dev и prod обновлены

## [1.1.2.3] - 2026-06-09
- **AI Chat Refactor:** единый менеджер ai_chat_manager.go
- **Новые таблицы:** ai_chat_sessions, ai_chat_messages, ai_chat_settings с FK CASCADE
- **Дропнуты:** owl_messages, owl_chat_settings, hermes_messages, hermes_sessions, hermes_chat_settings
- **Proto:** AIChatRequest, AIChatResponse, AIChatMessage, AIChatSettings
- **Новые RPC:** ChatWithAI (streaming), GetAIChatHistory, GetAIChatSettings, UpdateAIChatSettings
- **ChatWithAI:** маршрутизация owl→OpenRouter, hermes→Orchestrator
- **Deprecated:** ChatWithOWL, ChatWithOrchestrator
- **Android:** AiChatGrpc.kt, AIChat*Proto классы, GrpcClient facade

## [1.1.2.2] - 2026-06-09
- **Bugfix: DeleteChat не удалял hermes_sessions** — добавлено каскадное удаление из hermes_sessions + hermes_messages для hermes-чатов, owl_messages + owl_chat_settings для owl-чатов
- **Cleanup:** полная очистка всех AI-чатов на dev и prod (orphaned записи)

## [1.1.2.1] - 2026-06-09
- **Bugfix: Hermes история из БД** — `GetOrchestratorHistory` теперь загружает из `hermes_messages` через `HermesDB.GetOrchestratorHistory()` вместо in-memory `session.Messages`. История сохраняется после рестарта сервера.
- **Security: проверка владельца в GetOwlHistory** — добавлена проверка `creator_id` по `req.UserId`
- **Rate limiter: remaining count** — добавлен метод `remaining(userID)` в `rateLimiter`, возвращает количество оставшихся запросов в текущем окне
- **GetOwlSettings / GetHermesSettings** — теперь возвращают `remaining`, `limit`, `window_seconds` в ответе
- **Proto:** добавлены поля `remaining`, `limit`, `window_seconds` в `GetOwlSettingsResponse` и `GetHermesSettingsResponse`

## [1.1.2.0] - 2026-06-09
- **Prod релиз:** все фичи v1.1.1.x задеплоены на prod
- **Bugfix: Hermes permission denied** — `ALTER TABLE hermes_sessions OWNER TO lavender` на prod DB
- **Bugfix: HermesGrpc proto mapping** — исправлены номера полей в CreateHermesSessionResponse, CreateAgentResponse, AgentInfo
- **Bugfix: last_message_text пустой для Hermes** — добавлен `UPDATE chats SET last_message_text` после ответа оркестратора
- **Bugfix: дубли чатов в UI** — GetAIChats берёт оба типа из chats таблицы
- **Bugfix: getOrCreateSession создаёт дубли** — ищет существующую сессию по user_id вместо создания новой
- **Bugfix: Log-monitor JS split escape** — исправлено экранирование `\n` в Go raw string для prod log-monitor
- **Bugfix: Log-monitor показывал старые логи** — убран `--since "24 hours ago"` из journalctl
- **Docs:** добавлена документация LOG_MONITOR.md, обновлён INDEX.md

## [1.1.1.15] - 2026-06-09
- **Free OpenRouter Models:** новая таблица `free_openrouter_models` — управляемый список бесплатных моделей
- **RPC GetFreeModels:** получение списка бесплатных моделей (model_id, display_name, sort_order)
- **RPC SetFreeModel / RemoveFreeModel:** админ-управление списком бесплатных моделей
- **GetOwlSettings:** теперь возвращает `free_models` — список бесплатных моделей в ответе
- **Proto:** добавлены `FreeModelInfo`, `GetFreeModelsRequest/Response`, `SetFreeModelRequest/Response`, `RemoveFreeModelRequest/Response`
- **Favorites flickering fix:** startSync() теперь включает Favorites в setChats(), updateAvatarCache() корректно смещает позиции
- Dev сервер обновлён и работает

## [1.1.1.14] - 2026-06-09
- **Дизайн + полировка UI** (Android):
  - Анимации появления сообщений (fade-in + slide) в ChatMessageAdapter
  - Улучшенный typing indicator — анимированные точки вместо статичных
  - Полировка AIBottomSheet — иконки команд (OWL/Hermes), hover-эффекты, единый стиль
  - Полировка CommandBottomSheet — иконки команд, hover-эффекты, скругления
  - StandardBottomSheet — обёрнут в MaterialCardView (тени, скругления 28dp)
  - Splash screen — анимация загрузки (fade-in логотипа + названия)
  - Статус бар — цвет под тему для AI экранов
  - Тёмная тема — проверены и обновлены все AI-экраны
- Серверные изменения отсутствуют, все фичи v1.1.1.13 работают

## [1.1.1.13] - 2026-07-18
- Полное тестирование всех фич v1.1.1.x: AI чаты (OWL + Hermes), бот-команды, rate limiting, per-chat settings, reconnect, notifications
- Документация: обновлены INTEGRATION_SESSION.md, TASKS.md
- Подготовка к деплою на prod → v1.1.2.0

## [1.1.1.12] - 2026-06-09
- **Нет серверных изменений** — все фичи предыдущих версий работают
- Dev сервер обновлён и работает

## [1.1.1.11] - 2026-06-08
- **Key/model info banner:** показ источника ключа и модели в шапке AI-чатов (toolbarInfo в ChatWidget)
- **Robot icon:** ic_ai.xml заменён на robot vector drawable

## [1.1.1.10] - 2026-06-08
- **Hermes per-chat settings:** таблица `hermes_chat_settings`, RPCs `GetHermesSettings`/`UpdateHermesSettings` — per-session API key + model
- **Rate limiting:** свой ключ = 10 req/min, бесплатный тариф = 20 req/hour (`freeTierRateLimiter`)
- **GetOwlSettings:** добавлено поле `is_using_custom_key`
- **GetAIChats:** возвращает `is_using_custom_key` + `model` для всех типов чатов
- **Proto:** новые сообщения `GetHermesSettingsRequest/Response`, `UpdateHermesSettingsRequest/Response`, поля добавлены к `AIChatInfo` и `GetOwlSettingsResponse`

## [1.1.1.9] - 2026-06-08
- **Graceful reconnect (сервер):** добавлен grace period (30s) в hub — при разрыве соединения пользователь не сразу считается offline, а переходит в состояние "reconnecting"
- **Grace period API:** `StartGracePeriod()`, `IsInGracePeriod()`, `ClearGracePeriod()`, `GetGracePeriodRemaining()` методы в hub
- **GetOnlineUsers:** пользователи в grace period по-прежнему отображаются как online
- **ClearGracePeriod:** вызывается при успешной ре-аутентификации в Chat handler
- **Keepalive:** серверные параметры без изменений (MinTime=5s, Time=20s, Timeout=20s)

## [1.1.1.8] - 2026-06-08
- **Исправлен невалидный JSON в participants:** заменена конкатенация на `json.Marshal([]string{userId})`. Теперь хранится UUID вместо username — не зависит от символов в имени.
- **GetUserChats исключает AI-чаты:** `WHERE c.type NOT IN ('owl', 'hermes')` — убран jsonb-каст для AI-типов
- **GetAllChats не включает AI-чаты:** OWL/Hermes полностью убраны из основного списка, отдельный RPC GetAIChats
- **Новый RPC GetAIChats:** возвращает все AI-чаты пользователя (OWL + Hermes)
- **Новый RPC RenameAIChat:** переименование AI-чата с проверкой владельца
- **DeleteChat:** убрано уведомление участников для AI-чатов
- **Proto:** добавлены GetAIChatsRequest/Response, RenameAIChatRequest/Response, AIChatInfo

## [1.1.1.7] - 2026-07-18
- **Notification badge (серверная часть):** добавлен per-user read tracking для серверных уведомлений
- **notificationService:** добавлено поле `readStates` — map[userID]map[notificationID]bool для отслеживания прочитанных
- **MarkNotificationsRead:** теперь реально отмечает уведомления как прочитанные для конкретного пользователя
- **GetNotificationHistory:** возвращает уведомления с флагом `is_read` для текущего пользователя
- **GetUnreadCount RPC:** новый RPC — возвращает количество непрочитанных уведомлений для пользователя
- **Proto:** добавлено поле `is_read` (field 7) в `ServerNotification`
- **Proto:** добавлены `GetUnreadCountRequest` и `GetUnreadCountResponse` сообщения

## [1.1.1.6] - 2026-07-18
- **Multiple OWL/Hermes chats with numbering:** каждый новый чат уникален с порядковым номером
- **CreateOwlChat:** генерирует UUID-based chatID + имя с номером (#1, #2, ...), добавлен `name` в ответ
- **CreateHermesSession:** генерирует UUID-based sessionID + имя с номером (#1, #2, ...), добавлен `name` в ответ
- **getNextChatNumber():** SQL MAX(existing)+1 — номера не переиспользуются при удалении
- **generateChatName():** локализованные имена по умолчанию (русский: "Лава ИИ #N", "Оркестратор #N")
- **Proto:** добавлено поле `name` в `CreateOwlChatResponse` (field 4) и `CreateHermesSessionResponse` (field 4)
- **Proto:** добавлены `CreateOwlChatRequest` и `CreateOwlChatResponse` сообщения

## [1.1.1.5] - 2026-07-18
- **HermesSession → chats:** при создании HermesSession добавляется запись в таблицу `chats` (type="hermes") для корректного удаления и отображения в списке чатов
- **DeleteChat для Hermes:** добавлен fallback — если чат не найден в `chats`, проверяется `hermes_sessions` и удаляется оттуда (исправляет `sql: no rows in result set`)
- **GetOwlSettings RPC:** добавлен новый RPC для получения per-chat настроек (api_key, model) из `owl_chat_settings`
- **UpdateOwlSettings fix:** исправлена проверка владельца — теперь по `creator_id` (UUID) вместо `creator_username`
- **creator_id миграция:** добавлена колонка `creator_id` в таблицу `chats` для надёжной идентификации владельца по UUID
- **Все проверки владельца** (DeleteOwlChat, UpdateOwlSettings, GetOwlSettings) теперь используют `creator_id` (UUID) вместо `creator_username`
- **HermesDB:** добавлены методы `GetSessionID()` и `DeleteSession()`
- **ChatWithOWL INSERT:** добавлен `creator_id` при создании OWL чата
- **CreateOwlChat INSERT:** добавлен `creator_id` при создании OWL чата через RPC
- **GetAllChats OWL SELECT:** поиск OWL чатов теперь через `creator_id` с subquery по `users.username`
- **Multiple chats naming (подготовка):** chatID остаётся UUID-based, name будет генерироваться с номером (#1, #2...) — реализация в v1.1.1.6

## [1.1.1.4] - 2026-07-17
- **OWL FK fix:** авто-создание OWL чата в таблице `chats` при первом сообщении (исправляет `violates foreign key constraint "owl_messages_chat_id_fkey"`)
- **HermesSession username→UUID:** добавлен резолвинг username в UUID в `CreateHermesSession` для совместимости со старыми клиентами

## [1.1.1.3] - 2026-07-17
- **Bot Commands:** исправлен fmt.Sprintf в /logs handler
- **Unit Tests:** добавлены модульные тесты для bot_commands.go (rate limiter, command handlers, dispatcher, notification service, utility functions)

## [1.1.1.2] - 2026-07-17
- **SendServerNotification:** добавлены уведомления в `/deploy` и `/restart` handlers (start, success, error)
- **/ai команда:** улучшен системный промпт с именем пользователя

## [1.1.1.1] - 2026-07-17
- **Bot Commands:** добавлен Bot Command Processor (`bot_commands.go`) с командами: `/status`, `/deploy`, `/logs`, `/restart`, `/ai`, `/help`, `/version`
- **Bot Commands:** rate limiting 30 cmd/min per user, AI rate limit 10 req/min
- **Bot Commands:** интеграция в Chat stream — сообщения начинающиеся с `/` автоматически обрабатываются сервером
- **OWL Status:** добавлен `GetOWLStatus` RPC для проверки доступности AI
- **Server Notifications:** добавлен `NotificationService` с broadcast и history (100 max)
- **Server Notifications:** добавлены `SubscribeNotifications`, `GetNotificationHistory`, `MarkNotificationsRead` RPCs
- **Proto:** добавлены `BotCommandRequest/Response/Info`, `OWLStatusRequest/Response`, `ServerNotification` сообщения
- **Proto:** добавлены `ProcessBotCommand`, `GetBotCommands`, `GetOWLStatus` RPCs в ChatService

## [1.1.0.16] - 2026-06-07
- Удалена серверная инъекция Favorites из GetChats (клиент сам создаёт placeholder)

## [1.1.0.15] - 2026-06-05
- **LLM Router + RAG Pipeline:** добавлены интерфейсы и реализации
- **Hermes local provider:** in-memory RAG + Tool Executor
- **HermesAgentService:** подключение к remote agent routing
- **IsSuperAdmin:** проверка по user_id с fallback на username
- **Log-monitor:** добавлен source code, docs, deploy guide
- **Rebrand:** Hermes → Lava AI (Лава ИИ) в server logs и session names

## [1.1.0.14] - 2026-06-05
- **Hermes sessions in chat list:** `GetChats` теперь возвращает hermes_sessions как `type="hermes"`
- **DB migration:** добавлены колонки `user_id`, `preset_id` в `hermes_custom_agents`

## [1.1.0.13] - 2026-06-05
- **SSE parser fix:** `json.NewDecoder` → `bufio.Reader` + построчный разбор
- **Agent Management gRPC:** добавлены ListAgentPresets, ListAgents, CreateAgent, UpdateAgent, DeleteAgent
- **Hermes sessions:** CreateHermesSession создаёт запись и в hermes_sessions, и в chats
- **IsSuperAdmin:** проверка по userId (UUID)

## [1.1.0.12] - 2026-06-04
- **GRANT permissions:** для lavender user на hermes-таблицы

## [1.1.0.11] - 2026-06-04
- Hermes Orchestrator работает

## [1.1.0.10] - 2026-06-04
- Hermes: CreateHermesSession резолвит username → userId
- DB maintenance script: integrity check, orphaned records
- OWL init log показывает provider и model

## [1.1.0.9] и ранее
- Предыдущие версии...

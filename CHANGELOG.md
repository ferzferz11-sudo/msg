# Lavender Messenger — Server Changelog

## [1.1.1.4] - 2026-07-17
- Версия обновлена до 1.1.1.4
- **OWL FK fix:** авто-создание OWL чата в таблице `chats` при первом сообщении (исправляет `violates foreign key constraint "owl_messages_chat_id_fkey"`)
- **HermesSession username→UUID:** добавлен резолвинг username в UUID в `CreateHermesSession` для совместимости со старыми клиентами

## [1.1.1.3] - 2026-07-17
- Версия обновлена до 1.1.1.3
- **Bot Commands:** исправлен fmt.Sprintf в /logs handler
- **Unit Tests:** добавлены модульные тесты для bot_commands.go (rate limiter, command handlers, dispatcher, notification service, utility functions)

## [1.1.1.2] - 2026-07-17
- Версия обновлена до 1.1.1.2
- **SendServerNotification:** добавлены уведомления в `/deploy` и `/restart` handlers (start, success, error)
- **/ai команда:** улучшен системный промпт с именем пользователя

## [1.1.1.1] - 2026-07-17
- Версия обновлена до 1.1.1.1
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

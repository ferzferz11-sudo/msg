# Lavender Messenger - Changelog

**Author:** Pavel Davodov (ferz)

## [1.1.1.2] - 2026-07-17
- **Server:**
  - Версия обновлена до 1.1.1.2
  - **SendServerNotification:** добавлены уведомления в `/deploy` и `/restart` handlers (start, success, error)
  - **/ai команда:** улучшен системный промпт с именем пользователя
- **Android:**
  - Версия обновлена до 1.1.1.2
  - **OWL streaming:** добавлены отдельные `owlTyping`/`owlResponses` SharedFlows (не переиспользуют Hermes flows)
  - **ChatWithOWL:** добавлен `chatWithOwl()` метод — реальный gRPC вызов вместо пустого stub
  - **OwlChatViewModel:** собирает OWL ответы из отдельного потока, аккумулирует streaming chunks
  - **OwlRequestProto/OwlResponseProto:** новые data classes для OWL gRPC

## [1.1.1.1] - 2026-07-17
- **Server:**
  - Версия обновлена до 1.1.1.1
  - **Bot Commands:** добавлен Bot Command Processor (`bot_commands.go`) с командами: `/status`, `/deploy`, `/logs`, `/restart`, `/ai`, `/help`, `/version`
  - **Bot Commands:** rate limiting 30 cmd/min per user, AI rate limit 10 req/min
  - **Bot Commands:** интеграция в Chat stream — сообщения начинающиеся с `/` автоматически обрабатываются сервером
  - **OWL Status:** добавлен `GetOWLStatus` RPC для проверки доступности AI
  - **Server Notifications:** добавлен `NotificationService` с broadcast и history (100 max)
  - **Server Notifications:** добавлены `SubscribeNotifications`, `GetNotificationHistory`, `MarkNotificationsRead` RPCs
  - **Proto:** добавлены `BotCommandRequest/Response/Info`, `OWLStatusRequest/Response`, `ServerNotification` сообщения
  - **Proto:** добавлены `ProcessBotCommand`, `GetBotCommands`, `GetOWLStatus` RPCs в ChatService
- **Android:**
  - Версия обновлена до 1.1.1.1
  - **OwlChatActivity:** новый экран чата с OWL AI ассистентом
  - **OwlChatViewModel:** ViewModel для управления состоянием OWL чата
  - **OWL AI кнопка:** добавлена в bottom sheet меню ChatListActivity
  - **Slash command detection:** при вводе `/` в поле ввода показываются подсказки команд
  - **Bot Commands UI:** диалог со списком доступных команд
  - **gRPC:** добавлены `processBotCommand`, `getBotCommands`, `getOWLStatus` методы
  - **Proto:** добавлены `BotCommandRequest/Response/Info`, `OWLStatusRequest/Response`, `ServerNotification` классы
  - **OwlMessage:** новая data class для сообщений OWL чата

## [1.1.0.16] - 2026-06-07
- **Android:**
  - **Favorites flicker fix:** Favorites вынесен из RecyclerView в статический view выше списка
  - **Favorites:** использование ImageView вместо ShapeableImageView, AppCompatImageView с srcCompat
  - **Favorites:** стилизация выровнена с элементами списка чатов (margins, corner radius, theme colors)
  - **Server:** удалена серверная инъекция Favorites из GetChats (клиент сам создаёт placeholder)
  - **Server:** добавлена серверная инъекция Favorites как synthetic entry (временно, потом убрана)

## [1.1.0.15] - 2026-06-05
- **Server:**
  - **LLM Router + RAG Pipeline:** добавлены интерфейсы и реализации
  - **Hermes local provider:** in-memory RAG + Tool Executor
  - **HermesAgentService:** подключение к remote agent routing
  - **IsSuperAdmin:** проверка по user_id с fallback на username
  - **Log-monitor:** добавлен source code, docs, deploy guide
  - **Rebrand:** Hermes → Lava AI (Лава ИИ) в server logs и session names
- **Android:**
  - **Force reconnect fix:** `connect(force=true)` больше не убивает активные стримы
  - **Registration crash fix:** `startActivity+finish` → `recreate()` (focus race)
  - **Cache cleanup:** очистка кэша в logout()/deleteProfile(), не при login
  - **Logout:** `FLAG_ACTIVITY_NEW_TASK|CLEAR_TASK` + синхронная очистка Room DB

## [1.1.0.14] - 2026-06-05
- **Server:**
  - **Hermes sessions in chat list:** `GetChats` теперь возвращает hermes_sessions как `type="hermes"`
  - **DB migration:** добавлены колонки `user_id`, `preset_id` в `hermes_custom_agents`
- **Android:**
  - **Hermes sessions in chat list:** Room DB version 8→9 (activeAgentId, agentMode)
  - **Navigation:** тап по hermes чату → HermesChatActivity с CHAT_ID, ACTIVE_AGENT_ID, AGENT_MODE

## [1.1.0.13] - 2026-06-05
- **Android:**
  - **Mention system:** `@` в поле ввода → popup со списком агентов
  - **HermesChatActivity:** переписан на ChatWidget
  - **UI polish:** активный агент выделен, typing indicator с именем агента
  - **Два MentionAdapter:** agents/emojis и users/avatars — не объединять
- **Server:**
  - **SSE parser fix:** `json.NewDecoder` → `bufio.Reader` + построчный разбор
  - **Agent Management gRPC:** добавлены ListAgentPresets, ListAgents, CreateAgent, UpdateAgent, DeleteAgent
  - **Hermes sessions:** CreateHermesSession создаёт запись и в hermes_sessions, и в chats
  - **IsSuperAdmin:** проверка по userId (UUID)

## [1.1.0.12] - 2026-06-04
- **Android:**
  - **Unified Chat Widget:** widget_chat.xml, item_chat_message.xml, ChatMessageAdapter, ChatWidget.kt
  - **HermesChatActivity:** агенты как участники группового чата
  - **OWL removed:** -2425 строк (OwlActivity, OwlGrpc, OWL layouts)
  - **Bottom Sheet:** "Hermes AI" + "Агенты" вместо "Чат с AI"
- **Server:**
  - **GRANT permissions:** для lavender user на hermes-таблицы

## [1.1.0.11] - 2026-06-04
- **Android + Server:** Hermes Orchestrator работает
- **Android:** Proto mismatch fix в CreateHermesSession response

## [1.1.0.10] - 2026-06-04
- **Server:**
  - Hermes: CreateHermesSession резолвит username → userId
  - DB maintenance script: integrity check, orphaned records
  - OWL init log показывает provider и model
- **Android:** CreateHermesSession response marshaller fix

## [1.1.0.9] и ранее
- Предыдущие версии...

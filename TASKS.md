# Hermes Orchestrator — Задачи

**Версия:** v1.1.1.1
**Обновлено:** 2026-07-17
**Статус:** ✅ Фаза 1 завершена — Bot Commands, OWL Bot UI, Server Notifications

---

## ✅ Сделано (v1.1.1.1)

### Bot Commands (сервер)
- Bot Command Processor (`bot_commands.go`) с 7 командами: /status, /deploy, /logs, /restart, /ai, /help, /version
- Rate limiting: 30 cmd/min для бота, 10 req/min для AI
- Bot command detection в Chat stream (сообщения начинающиеся с /)
- Super admin проверка для /deploy и /restart
- /ai команда вызывает OpenRouter напрямую

### OWL Status (сервер)
- GetOWLStatus RPC для проверки доступности AI
- Возвращает available, model, status

### Server Notifications (сервер)
- NotificationService с broadcast и history (100 max)
- SubscribeNotifications, GetNotificationHistory, MarkNotificationsRead RPCs
- SendServerNotification хелпер

### Proto (сервер)
- BotCommandRequest/Response/Info, GetBotCommandsRequest/Response
- OWLStatusRequest/Response
- ServerNotification, SubscribeNotificationsRequest, GetNotificationHistoryRequest/Response, MarkNotificationReadRequest/Response
- ProcessBotCommand, GetBotCommands, GetOWLStatus RPCs в ChatService

### OWL Bot UI (Android)
- OwlChatActivity — экран чата с OWL AI
- OwlChatViewModel — ViewModel для OWL чата
- Slash command detection (/) в поле ввода
- Bot Commands UI (dialog со списком команд)
- OWL AI кнопка в bottom sheet меню
- processBotCommand, getBotCommands, getOWLStatus gRPC методы
- BotCommand*, OWLStatus*, ServerNotification* proto классы
- OwlMessage data class
- activity_owl_chat.xml, ic_owl.xml
- compileDebugKotlin passes

---

## ✅ Сделано (v1.1.0.16)

### Favorites
- Favorites вынесен из серверного GetChats (клиент создаёт placeholder)
- Favorites добавлен в начало списка чатов как статический элемент
- Серверная инъекция Favorites удалена

---

## ✅ Сделано (ранее)

### v1.1.0.15 — Force reconnect + Registration fix
- Force reconnect не убивает активные стримы
- Registration crash fix (recreate vs startActivity+finish)
- Cache clearing только в logout()/deleteProfile()

### v1.1.0.14 — Hermes sessions in chat list
- GetChats возвращает hermes_sessions как type="hermes"
- Room DB version 8→9 (activeAgentId, agentMode)

### v1.1.0.13 — ChatWidget + Mention system
- HermesChatActivity переписан на ChatWidget
- Mention system (@ в поле ввода → popup)
- Agent chips с активным состоянием

### v1.1.0.12 — Unified Chat Widget
- widget_chat.xml, ChatMessageAdapter, ChatWidget.kt
- HermesChatActivity: агенты как участники
- OWL полностью удалён (-2425 строк)

### v1.1.0.11 — Hermes Orchestrator
- Оркестратор отвечает на Android
- CreateHermesSession, ChatWithOrchestrator работают

### v1.1.0.10 — Agent Management gRPC
- ListAgentPresets, ListAgents, CreateAgent, UpdateAgent, DeleteAgent
- 8 пресет-агентов

---

## ⏳ Не начато (по приоритету)

### Высокий приоритет
1. **Тестирование** — собрать APK, проверить бот-команды, OWL чат, уведомления
2. **OWL streaming** — отдельный gRPC поток для OWL на Android (сейчас переиспользует Hermes flows)
3. **Deploy интеграция** — подключить deploy-dev.sh / deploy-prod.sh к бот-командам

### Средний приоритет
4. **Server Notifications UI** — SubscribeNotifications на Android
5. **Rate limiting для уведомлений**
6. **Модульные тесты** для бот-команд

### Низкий приоритет
7. **Auth токены для удалённых агентов** — JWT при регистрации
8. **Qdrant + CLIP** — production RAG
9. **Graceful reconnect** при keepalive failed
10. **NewChatActivity** — миграция на ChatWidget (рефакторинг)

---

## Известные проблемы

- `OwlChatViewModel` переиспользует `hermesTyping/hermesResponses` SharedFlows (не идеально)
- Server migration warnings: `role "lavender" does not exist` (не критично)
- `/ai` команда вызывает OpenRouter напрямую (не через оркестратор)
- `SendServerNotification` не используется в deploy/restart handlers

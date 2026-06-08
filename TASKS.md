# Lavender Messenger — Задачи

**Версия:** v1.1.1.3
**Обновлено:** 2026-07-17
**Статус:** ✅ v1.1.1.3 — Server Notifications UI + тесты

---

## Архитектура Android gRPC

```
data/grpc/
├── OwlGrpc.kt          — OWL AI: chatWithOwl, processBotCommand, getBotCommands, getOWLStatus, notifications
├── HermesGrpc.kt       — Hermes Orchestrator: chatWithOrchestrator, agent management
├── GrpcClient.kt       — единая точка доступа (делегирует на OwlGrpc + HermesGrpc)
├── RealGrpcClient.kt   — gRPC канал, подключение
└── SecretChatGrpc.kt   — E2EE секретные чаты
```

Принцип: каждый AI-сервис в своём файле, свои SharedFlows, свои marshallers.
Баг в одном сервисе не влияет на другой.

---

## ✅ Сделано (v1.1.1.3)

### Server Notifications UI (Android)
- **NotificationActivity** — экран просмотра серверных уведомлений с историей и real-time
- **NotificationAdapter** — адаптер с иконками по типу (🚀 deploy, ✅ deploy_done, ❌ deploy_error, 🔄 restart, ⚠️ warning, ℹ️ info)
- **OwlGrpc.kt** — subscribeNotifications (server streaming), getNotificationHistory, markNotificationsRead
- **OwlGrpc.kt** — serverNotifications SharedFlow
- **GrpcClient** — публичные методы для уведомлений
- **ChatListActivity** — кнопка "Уведомления" в ActionBottomSheet
- **Layouts:** activity_notification.xml, item_notification.xml
- **Drawables:** ic_notifications.xml, ic_arrow_back.xml, circle_background.xml

### Server
- Исправлен fmt.Sprintf в /logs handler
- Unit tests: bot_commands_test.go (rate limiter, handlers, dispatcher, notification service, utilities)
- ServerVersion: 1.1.1.2 → 1.1.1.3

### Сборка
- go build ✅
- compileDebugKotlin ✅
- go test (bot_commands_test.go) ✅

---

## ✅ Сделано (v1.1.1.2)

### Сервер: исправления
- SendServerNotification в /deploy handler (старт, успех, ошибка)
- SendServerNotification в /restart handler
- /ai команда: улучшен промпт с именем пользователя
- ServerVersion: 1.1.1.1 → 1.1.1.2

### Android: архитектурное разделение
- **OwlGrpc.kt** — новый файл, полная изоляция OWL от Hermes:
  - `chatWithOwl()` — ChatWithOWL server streaming gRPC
  - `processBotCommand()` — отправка бот-команд на сервер
  - `getBotCommands()` — список доступных команд
  - `getOWLStatus()` — проверка доступности OWL AI
  - `OwlRequestMarshaller/OwlResponseMarshaller` — protobuf сериализация
  - `owlTyping/owlResponses` — отдельные SharedFlows
- **HermesGrpc.kt** — очищен от OWL-кода, только orchestrator + agent management
- **MessengerProto.kt** — добавлены `OwlRequestProto`, `OwlResponseProto`
- **OwlChatViewModel** — использует `owlTyping/owlResponses`, аккумулирует streaming chunks
- **GrpcClient** — `owlResponses/owlTyping` свойства
- compileDebugKotlin passes

---

## ✅ Сделано (v1.1.1.1)

### Bot Commands (сервер)
- Bot Command Processor (bot_commands.go) с 7 командами: /status, /deploy, /logs, /restart, /ai, /help, /version
- Rate limiting: 30 cmd/min для бота, 10 req/min для AI
- Bot command detection в Chat stream (сообщения начинающиеся с /)
- Super admin проверка для /deploy и /restart

### OWL Status (сервер)
- GetOWLStatus RPC для проверки доступности AI

### Server Notifications (сервер)
- NotificationService с broadcast и history (100 max)
- SubscribeNotifications, GetNotificationHistory, MarkNotificationsRead RPCs
- SendServerNotification хелпер — используется в deploy/restart

### Proto (сервер)
- BotCommand*, OWLStatus*, ServerNotification* сообщения
- ProcessBotCommand, GetBotCommands, GetOWLStatus RPCs в ChatService

### OWL Bot UI (Android)
- OwlChatActivity — экран чата с OWL AI
- OwlChatViewModel — ViewModel для OWL чата
- Slash command detection (/) в поле ввода
- Bot Commands UI (dialog со списком команд)
- OWL AI кнопка в bottom sheet меню
- OwlMessage data class
- activity_owl_chat.xml, ic_owl.xml

---

## ✅ Сделано (v1.1.0.16)

### Favorites
- Favorites вынесен из серверного GetChats (клиент создаёт placeholder)
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

### v1.1.0.12 — Unified Chat Widget
- widget_chat.xml, ChatMessageAdapter, ChatWidget.kt
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
1. **Тестирование на dev** — проверить бот-команды, OWL чат, ChatWithOWL streaming, уведомления
2. **Deploy интеграция** — проверить deploy-dev.sh / deploy-prod.sh (уже подключены к /deploy)
3. **Деплой на prod** — проверить на dev, потом деплой

### Средний приоритет
4. **Server Notifications UI** — SubscribeNotifications на Android (показ уведомлений)
5. **Rate limiting для уведомлений** — защита от спама
6. **Модульные тесты** для бот-команд
7. **APK сборка и тест на устройстве** — полный цикл

### Низкий приоритет
8. **Auth токены для удалённых агентов** — JWT при регистрации
9. **Qdrant + CLIP** — production RAG
10. **Graceful reconnect** при keepalive failed
11. **NewChatActivity** — миграция на ChatWidget (рефакторинг)

---

## Известные проблемы

- Server migration warnings: `role "lavender" does not exist` (не критично, сервер работает)

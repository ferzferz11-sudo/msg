# Lavender Messenger — Задачи

**Версия:** v1.1.1.3
**Обновлено:** 2026-07-17
**Статус:** ✅ v1.1.1.3 — Server Notifications UI + тесты + тема

---

## Архитектура Android gRPC

```
data/grpc/
├── OwlGrpc.kt          — OWL AI: chatWithOwl, bot commands, OWL status, notifications
├── HermesGrpc.kt       — Hermes Orchestrator: chatWithOrchestrator, agent management
├── GrpcClient.kt       — единая точка доступа (делегирует на OwlGrpc + HermesGrpc)
├── RealGrpcClient.kt   — gRPC канал, подключение
└── SecretChatGrpc.kt   — E2EE секретные чаты
```

Принцип: каждый AI-сервис в своём файле, свои SharedFlows, свои marshallers.

---

## ✅ Сделано (v1.1.1.3)

### Server Notifications UI (Android)
- **NotificationActivity** — экран уведомлений с историей и real-time подпиской
- **NotificationAdapter** — адаптер с иконками по типу (🚀 deploy, ✅ deploy_done, ❌ deploy_error, 🔄 restart, ⚠️ warning, ℹ️ info)
- **OwlGrpc.kt** — subscribeNotifications (server streaming), getNotificationHistory, markNotificationsRead
- **OwlGrpc.kt** — serverNotifications SharedFlow
- **GrpcClient** — публичные методы для уведомлений
- **ChatListActivity** — кнопка "Уведомления" в ActionBottomSheet
- **Тема:** toolbar_background, colorOnPrimary, colorSurface, colorOnSurface, ThemeUi.bind()
- **Layouts:** activity_notification.xml, item_notification.xml
- **Drawables:** ic_notifications.xml, ic_back_arrow (существующий), circle_background.xml

### Server
- Исправлен fmt.Sprintf в /logs handler
- Unit tests: bot_commands_test.go (18 тестов: rate limiter, handlers, dispatcher, notification service, utilities)
- ServerVersion: 1.1.1.2 → 1.1.1.3

### Сборка и деплой
- go build ✅ | compileDebugKotlin ✅ | go test ✅
- Dev сервер обновлён и работает
- Оба репозитория запушены в feat/1.1.1.x
- Теги v1.1.1.3 на обоих репозиториях

---

## ✅ Сделано (v1.1.1.2)

### Сервер
- SendServerNotification в /deploy и /restart handlers
- /ai команда: улучшен промпт с именем пользователя
- ServerVersion: 1.1.1.1 → 1.1.1.2

### Android: архитектурное разделение
- **OwlGrpc.kt** — отдельный файл, полная изоляция OWL от Hermes
- **HermesGrpc.kt** — очищен от OWL-кода
- OwlChatViewModel — отдельный поток, аккумулирует streaming chunks
- compileDebugKotlin passes

---

## ✅ Сделано (v1.1.1.1)

### Bot Commands (сервер)
- Bot Command Processor (bot_commands.go) с 7 командами: /status, /deploy, /logs, /restart, /ai, /help, /version
- Rate limiting: 30 cmd/min для бота, 10 req/min для AI
- Bot command detection в Chat stream

### Server Notifications (сервер)
- NotificationService с broadcast и history (100 max)
- SubscribeNotifications, GetNotificationHistory, MarkNotificationsRead RPCs

### OWL Bot UI (Android)
- OwlChatActivity, OwlChatViewModel, Slash command detection, Bot Commands UI

---

## ✅ Сделано (ранее)

### v1.1.0.16 — Favorites fix
### v1.1.0.15 — Force reconnect + Registration fix + Cache clearing
### v1.1.0.14 — Hermes sessions in chat list
### v1.1.0.13 — ChatWidget + Mention system
### v1.1.0.12 — Unified Chat Widget
### v1.1.0.11 — Hermes Orchestrator
### v1.1.0.10 — Agent Management gRPC

---

## ⏳ Не начато (по приоритету)

### Высокий приоритет
1. **APK сборка и тест на устройстве** — полный цикл, проверить все функции
2. **Деплой на prod** — проверить на dev, потом деплой
3. **[AI] кнопка в списке чатов (v1.1.1.4)** — см. детальный план ниже

### Средний приоритет
4. **NotificationActivity — badge с количеством непрочитанных** — показывать счётчик на иконке колокольчика
5. **Graceful reconnect** при keepalive failed — переподключение без потери стримов
6. **Модульные тесты для OWL streaming** — тестировать chatWithOwl gRPC

### Низкий приоритет
7. **Auth токены для удалённых агентов** — JWT при регистрации
8. **Qdrant + CLIP** — production RAG
9. **NewChatActivity** — миграция на ChatWidget (рефакторинг)

---

## 📋 План: [AI] кнопка в списке чатов (v1.1.1.4)

### Концепция
Заменить разрозненные AI-кнопки в [+] на единую кнопку [AI] в списке чатов (рядом с [+]).
При нажатии открывается шторка (BottomSheet) аналогичная [+], но с группировкой AI-сервисов.

### Структура шторки [AI]

```
┌─────────────────────────────────────┐
│  🤖 AI Сервисы                      │
├─────────────────────────────────────┤
│  🎼 Lava AI (Оркестратор)           │
│  🤖 Агенты                          │
│  📋 Уведомления                     │
├─────────────────────────────────────┤  ← разделитель
│  🦉 OWL AI (Чат)                    │
│  ⚙️ Настройки OWL                   │
└─────────────────────────────────────┘
```

### Группировка

**Верхняя группа — Оркестратор:**
- Lava AI → HermesChatActivity
- Агенты → AgentListActivity
- Уведомления → NotificationActivity

**Разделитель (divider)**

**Нижняя группа — OWL:**
- OWL AI → OwlChatActivity
- Настройки OWL → (новая активити, если нужно)

### Что сделать

1. **Создать кнопку [AI]** в ChatListActivity layout (рядом с [+])
2. **Создать AIBottomSheet** — аналог ActionBottomSheet с группировкой
3. **Добавить разделитель** между группами Оркестратор/OWL
4. **Перенести существующие AI-пункты** из [+] в [AI] шторку
5. **Убрать дубликаты** из [+] (оставить только обычные чаты)
6. **Иконки:** для каждой кнопки использовать существующие drawable

### Файлы
- `ChatListActivity.kt` — добавить кнопку [AI], обработчик
- `AIBottomSheet.kt` (или `bottom_sheet_ai.xml`) — шторка с группировкой
- `bottom_sheet_add.xml` — убрать AI-пункты из основной шторки

### Принципы
- Полная изоляция OWL и Hermes — разные группы
- Визуальное разделение через divider
- Использовать существующие Activity, не создавать новые
- Тема должна соответствовать приложению

---

## Известные проблемы

- Server migration warnings: `role "lavender" does not exist` (не критично, сервер работает)

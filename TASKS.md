# Lavender Messenger — Задачи

**Версия:** v1.1.1.4
**Обновлено:** 2026-07-17
**Статус:** ✅ v1.1.1.4 — [AI] кнопка в списке чатов

---

## ✅ Сделано (v1.1.1.4)

### [AI] кнопка в списке чатов (Android)
- **Кнопка [AI]** рядом с [+] в ChatListActivity (activity_chat_list.xml)
- **AIBottomSheet** — шторка с двумя группами и разделителем
- **Группа Оркестратор:** Lava AI, Агенты, Уведомления
- **Группа OWL:** OWL AI, Настройки OWL
- **AI-пункты перенесены** из [+] в [AI] шторку
- **Новые файлы:** AIBottomSheet.kt, widget_ai_bottom_sheet.xml, widget_section_header.xml, widget_section_divider.xml
- **Сервер:** версия 1.1.1.3 → 1.1.1.4
- **Сборка:** compileDebugKotlin ✅ | go build ✅ | go test ✅
- **Dev сервер:** обновлён и работает

---

## ✅ Сделано (v1.1.1.3)

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
1. **Исправить пути скриптов деплоя** — /deploy искал скрипты в /root/scripts/ (не существовало). Исправлено на /root/msg/scripts/. Нужно проверить что /deploy dev и /prod работают корректно.
2. **APK сборка и тест на устройстве** — полный цикл, проверить все функции
3. **Деплой на prod** — проверить на dev, потом деплой

### Средний приоритет
4. **NotificationActivity — badge с количеством непрочитанных** — показывать счётчик на иконке колокольчика
5. **Graceful reconnect** при keepalive failed — переподключение без потери стримов
6. **Модульные тесты для OWL streaming** — тестировать chatWithOwl gRPC

### Низкий приоритет
7. **Auth токены для удалённых агентов** — JWT при регистрации
8. **Qdrant + CLIP** — production RAG
9. **NewChatActivity** — миграция на ChatWidget (рефакторинг)

---

## Известные проблемы

- Server migration warnings: `role "lavender" does not exist` (не критично, сервер работает)

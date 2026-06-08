# Lavender Messenger — Задачи

**Версия:** v1.1.1.4
**Обновлено:** 2026-07-17
**Статус:** ✅ v1.1.1.4 — [AI] кнопка, OWL FK fix, HermesSession UUID fix

---

## ✅ Сделано (v1.1.1.4)

### Android
- **[AI] кнопка** рядом с [+] в ChatListActivity (activity_chat_list.xml)
- **AIBottomSheet** — шторка с двумя группами и разделителем
- **Группа Оркестратор:** Lava AI, Агенты, Уведомления
- **Группа OWL:** OWL AI, Настройки OWL
- **AI-пункты перенесены** из [+] в [AI] шторку
- **Новые файлы:** AIBottomSheet.kt, widget_ai_bottom_sheet.xml, widget_section_header.xml, widget_section_divider.xml
- **HermesChatActivity:** использует userId (UUID) из сессии вместо username

### Server
- **OWL FK fix:** авто-создание OWL чата в `chats` при первом сообщении (исправляет FK constraint на `owl_messages`)
- **HermesSession username→UUID:** резолвинг username в UUID в `CreateHermesSession`
- ServerVersion: 1.1.1.3 → 1.1.1.4

### Сборка и деплой
- compileDebugKotlin ✅ | go build ✅ | go test ✅
- Dev сервер обновлён и работает
- Оба репозитория запушены в feat/1.1.1.x
- Теги v1.1.1.4 на обоих репозиториях

---

## ✅ Сделано (v1.1.1.3)

### Android
- **NotificationActivity** — экран уведомлений с историей и real-time подпиской
- **NotificationAdapter** — адаптер с иконками по типу
- **OwlGrpc.kt** — subscribeNotifications, getNotificationHistory, markNotificationsRead
- **OwlGrpc.kt** — serverNotifications SharedFlow
- **ChatListActivity** — кнопка "Уведомления" в ActionBottomSheet

### Server
- Исправлен fmt.Sprintf в /logs handler
- Unit tests: bot_commands_test.go (18 тестов)
- ServerVersion: 1.1.1.2 → 1.1.1.3

---

## ✅ Сделано (v1.1.1.2)

### Server
- SendServerNotification в /deploy и /restart handlers
- /ai команда: улучшен промпт с именем пользователя

### Android
- **OwlGrpc.kt** — отдельный файл, полная изоляция OWL от Hermes
- **HermesGrpc.kt** — очищен от OWL-кода
- OwlChatViewModel — отдельный поток, аккумулирует streaming chunks

---

## ✅ Сделано (v1.1.1.1)

### Server
- Bot Command Processor (bot_commands.go) с 7 командами
- Rate limiting: 30 cmd/min, 10 AI/min
- NotificationService с broadcast и history

### Android
- OwlChatActivity, OwlChatViewModel, Slash command detection, Bot Commands UI

---

## ✅ Сделано (ранее)

- v1.1.0.16 — Favorites fix
- v1.1.0.15 — Force reconnect + Registration fix + Cache clearing
- v1.1.0.14 — Hermes sessions in chat list
- v1.1.0.13 — ChatWidget + Mention system
- v1.1.0.12 — Unified Chat Widget
- v1.1.0.11 — Hermes Orchestrator
- v1.1.0.10 — Agent Management gRPC

---

## ⏳ Не начато (по приоритету)

### Высокий приоритет
1. **OWL Settings** — экран настроек OWL (API key, model selector), кнопка в [AI] шторке ведёт на него
2. **DeleteChat для Hermes** — `sql: no rows in result set` при удалении hermes сессии

### Средний приоритет
3. **HermesSession → chats** — при создании сессии добавлять запись в таблицу `chats` (для корректного удаления и отображения в списке)
4. **NotificationActivity badge** — счётчик непрочитанных на иконке колокольчика
5. **Graceful reconnect** — переподключение без потери стримов

### Низкий приоритет
6. **Auth токены для удалённых агентов** (JWT)
7. **Qdrant + CLIP** (production RAG)
8. **NewChatActivity** — миграция на ChatWidget
9. **Деплой на prod** — только после завершения всех задач интеграции

---

## Известные проблемы

- Server migration warnings: `role "lavender" does not exist` (не критично, сервер работает)

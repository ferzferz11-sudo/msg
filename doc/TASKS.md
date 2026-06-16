# Лава — Задачи

**Версия:** v1.2.0.2
**Ветка:** feat/1.2.0.x
**Обновлено:** 2026-06-16 (сессия 26)

---

## ✅ v1.2.0.2 — FCM Push Notifications uplevel (Сессия 26)

### FCM Push
- ✅ Hub.IsUserOnline(userId, username) — проверка онлайн-статуса
- ✅ Hub.SetUserId() + clientUserIds map
- ✅ sendPushNotification — skip online + collapse key + TTL
- ✅ GetAllUsers() возвращает UserId
- ✅ server_push_test.go — 7 тестов
- ✅ Исправлена миграция user_chat_metadata

---

## ✅ v1.2.0.1 — ChatStream v2 + ChatList v2 + Pin Message
(Сессии 11, 15, 17 — завершено)

### Pin Message
- ✅ messenger.proto: PinMessage/UnPinMessage/GetPinnedMessages RPC
- ✅ db_chatlist_v2.go: pinned_messages table, CRUD методы
- ✅ server_chatlist_v2.go: RPC handlers
- ✅ protoc генерация выполнена (сессия 17)
- ✅ Валидация: пользователь — участник чата, сообщение существует

### ChatStream v2
- ✅ messenger.proto: jwt_token (field 26) в Message
- ✅ server_chat.go: JWT auth + password fallback

### ChatList v2
- ✅ messenger.proto: PinChat, SearchChats, ArchiveChat RPC
- ✅ server_chatlist_v2.go: реализация RPC
- ✅ db_chatlist_v2.go: user_chat_metadata таблица

---

## 📋 Активные задачи

### Высокий приоритет
- [ ] **Тестирование Pin Message** — RPC на dev сервере
- [ ] **Тестирование FCM push** — проверка на dev/prod

### Средний приоритет
- [ ] **Read receipts (MarkAsRead)** — если нужно на сервере
- [ ] **Prometheus метрики** — мониторинг

### Отложено
- [ ] Qdrant + CLIP (production RAG) — см. AI_SERVICES.md
- [ ] Редеплой prod сервера — только после выхода Android клиента

---

## 🗄️ Структура файлов

### Сервер (Go)
```
main.go                    — Entry point, gRPC server, graceful shutdown
server.go                  — ServerVersion = "1.2.0.2", service version constants
auth_service.go            — AuthService v1 (deprecated)
auth_service_v2.go         — AuthService v2 (JWT, основной)
auth_interceptor.go        — gRPC Bearer token interceptor
auth_jwt.go                — JWT генерация/валидация
db_auth_devices.go         — CRUD для user_devices + device_auth_log
db_auth_migrations.go      — миграция таблиц
db_chatlist_v2.go          — ChatList v2 + Pin Message DB methods
server_profile_v2.go       — ProfileService v2 (JWT, dev only)
server_chatlist_v2.go      — ChatList v2 + Pin Message RPC handlers
server_chat.go             — Chat stream v2 (JWT + password)
server_remote.go           — Remote Agent RPC
server_push.go             — FCM push notifications (RegisterToken, sendPushNotification)
server_push_test.go        — тесты для IsUserOnline (7 тестов)
hermes_remote_manager.go   — HandleTaskStream
ai_chat_manager.go         — AI чаты
owl.go                     — OWL AI
hermes_orchestrator.go     — Hermes Orchestrator
http_server.go             — HTTP (/health, /info)
messenger.proto            — ChatService v2, AuthService v2, ProfileService v2, Pin Message
```

---

## 🔗 Репозитории

| Репозиторий | URL | Текущая версия |
|-------------|-----|----------------|
| msg | https://github.com/ferzferz11-sudo/msg | v1.2.0.2 |
| msg.client.android | https://github.com/ferzferz11-sudo/msg.client.android | v1.1.3.21 |
| msg.remote.agent | https://github.com/ferzferz11-sudo/msg.remote.agent | v1.1.3.4 |

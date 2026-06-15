# Лава — Задачи

**Версия:** v1.2.0.1
**Ветка:** feat/1.2.0.x
**Обновлено:** 2026-06-16 (сессия 13)

---

## ✅ v1.2.0.1 — ChatStream v2 + ChatList v2
(Сессия 11 — завершено)

---

## ✅ v1.1.3.16 — ChatList v2 UI (Android)
(Сессия 12-13 — завершено)

---

## 📋 Активные задачи (Сессия 16)

### Высокий приоритет
- [x] **Pin Message** — серверные RPC PinMessage/UnPinMessage + таблица pinned_messages + клиентская реализация

### Средний приоритет
- [ ] **Тестирование** — v1.1.3.16 на dev и prod серверах
- [ ] **protoc генерация** — перегенерировать Go proto после добавления PinMessage

### Отложено
- [ ] Qdrant + CLIP (production RAG) — см. AI_SERVICES.md
- [ ] Prometheus метрики
- [ ] Read receipts (MarkAsRead)

---

## 🗄️ Структура файлов

### Сервер (Go)
```
main.go                    — Entry point, gRPC server, graceful shutdown
server.go                  — ServerVersion = "1.2.0.1", service version constants
auth_service.go            — AuthService v1 (deprecated)
auth_service_v2.go         — AuthService v2 (JWT, основной)
auth_interceptor.go        — gRPC Bearer token interceptor
auth_jwt.go                — JWT генерация/валидация
db_auth_devices.go         — CRUD для user_devices + device_auth_log
db_auth_migrations.go      — миграция таблиц
db_chatlist_v2.go          — ChatList v2 DB methods
server_profile_v2.go       — ProfileService v2 (JWT, dev only)
server_chatlist_v2.go      — ChatList v2 RPC
server_chat.go             — Chat stream v2 (JWT + password)
server_chats.go            — GetAllChats, CreateDirectChat, etc.
server_remote.go           — Remote Agent RPC
hermes_remote_manager.go   — HandleTaskStream
ai_chat_manager.go         — AI чаты
owl.go                     — OWL AI
hermes_orchestrator.go     — Hermes Orchestrator
http_server.go             — HTTP (/health, /info)
messenger.proto            — ChatService v2, AuthService v2, ProfileService v2
```

---

## 🔗 Репозитории

| Репозиторий | URL | Текущая версия |
|-------------|-----|----------------|
| msg | https://github.com/ferzferz11-sudo/msg | v1.2.0.1 |
| msg.client.android | https://github.com/ferzferz11-sudo/msg.client.android | v1.1.3.16 |
| msg.remote.agent | https://github.com/ferzferz11-sudo/msg.remote.agent | v1.1.3.4 |

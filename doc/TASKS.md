# Лава — Задачи

**Версия:** v1.2.0.1
**Ветка:** feat/1.2.0.x
**Обновлено:** 2026-06-16 (сессия 12)

---

## ✅ v1.2.0.1 — ChatStream v2 + ChatList v2
(Сессия 11 — завершено)

---

## ✅ v1.1.3.16 — ChatList v2 UI (Android)
(Сессия 12 — завершено)

---

## 📋 Активные задачи (Сессия 13)

### Высокий приоритет
- [ ] **Pin Message** — серверные RPC PinMessage/UnPinMessage + таблица pinned_messages + клиентская реализация
- [ ] **TabLayout + ViewPager2** — табы All / AI / Groups в ChatListActivityV2
- **Переключение v1/v2 при старте** — программный выбор Activity

### Средний приоритет
- [ ] **Тестирование** — v1.1.3.16 на dev и prod серверах

### Отложено
- [ ] Qdrant + CLIP (production RAG) — см. AI_SERVICES.md
- [ ] Prometheus метрики
- [ ] Read receipts (MarkAsRead)
- `de3d55d` — docs: update version to v1.2.0.1, branch to feat/1.2.0.x

---

## ✅ v1.2.0.0 — ProfileService v2 + Typing/CallSession compat
(Сессия 9 — завершено)

---

## ✅ v1.2.0.0 — AuthService v2 (JWT) + Server info + UI fixes
(Сессия 6-8 — завершено)

---

## 📋 Активные задачи

### Высокий приоритет
- [ ] **ChatList v2 UI** — ChatListActivity v2 с секциями (Pinned/Favorites/All), табами, search, unread badges
- [ ] **Деплой prod сервера** — после завершения Android клиента v1.1.3.14+

### Средний приоритет
- [ ] **Тесты для ProfileService v2** — unit-тесты (сервер + Android)
- [ ] **Тесты для ChatStream v2** — unit-тесты JWT auth в Chat stream
- [ ] **Тесты для ChatList v2** — unit-тесты PinChat/SearchChats/ArchiveChat

### Отложено
- [ ] Qdrant + CLIP (production RAG) — см. AI_SERVICES.md
- [ ] Prometheus метрики
- [ ] Read receipts (MarkAsRead) — протокол готов, нужно подключить в UI

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
|| msg | https://github.com/ferzferz11-sudo/msg | v1.2.0.1 |
|| msg.client.android | https://github.com/ferzferz11-sudo/msg.client.android | v1.1.3.16 |
|| msg.remote.agent | https://github.com/ferzferz11-sudo/msg.remote.agent | v1.1.3.4 |

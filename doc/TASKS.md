# Лава — Задачи

**Версия:** v1.2.0.8
**Ветка:** feat/1.2.0.x
**Обновлено:** 2026-06-19

---

## ✅ v1.2.0.7 — UserInfo fields + deploy script (Сессия 39)

- ✅ UserInfo: `user_id` (UUID) + `is_super_admin` (bool) в GetAllUsers
- ✅ GetAllUsers SQL: возвращает `is_super_admin`
- ✅ deploy-dev-local.sh — кросс-компиляция с Mac → SCP → рестарт
- ✅ CHANGELOG, doc обновлены

---

## ✅ v1.2.0.6 — ChatList V2 Last Message Optimization (Сессия 38)

- ✅ DB миграция: `last_message_username`, `last_message_has_image` в `chats`
- ✅ SaveMessage обновляет `chats.last_message_*` при отправке
- ✅ CTE `WITH last_messages` удалён из GetUserChatsV2/GetUserChats/GetAllChats
- ✅ Decrypt text в Go (не в SQL) для last_message_text preview
- ✅ Dev (50052): ✅ работает, Prod (50051): ✅ работает

---

## ✅ v1.2.0.5 — userId Migration (Сессия 37)

### DB Migration (Этап 1)
- ✅ UUID-колонки в `reactions`, `contacts`, `user_tokens`, `user_themes`
- ✅ `chats.participant_ids UUID[]` + GIN индекс
- ✅ SQL миграция: `migrations/001_userid_migration.sql`

### UUID-based DB Methods (Этап 2)
- ✅ ByUserID варианты для каждого DB-метода

### Handler Migration (Этап 3)
- ✅ Все handlers переключены на `GetUserID(ctx)` + `resolveDisplayName()`

---

## ✅ v1.2.0.2 — FCM Push Notifications uplevel (Сессия 26)

- ✅ Hub.IsUserOnline(userId, username) — проверка онлайн-статуса
- ✅ Hub.SetUserId() + clientUserIds map
- ✅ sendPushNotification — skip online + collapse key + TTL
- ✅ server_push_test.go — 7 тестов

---

## ✅ v1.2.0.1 — ChatStream v2 + ChatList v2 + Pin Message

- ✅ messenger.proto: PinMessage/UnPinMessage/GetPinnedMessages RPC
- ✅ server_chatlist_v2.go: реализация всех RPC
- ✅ db_chatlist_v2.go: user_chat_metadata таблица
- ✅ Chat stream v2: JWT auth + password fallback

---

## ✅ v1.2.0.8 — P0 Performance Optimizations (Сессия 40)

- ✅ Broadcast deadlock fix — snapshot streams, send without lock
- ✅ isChatMuted N+1 — batch getMutedRoomsSet
- ✅ Push N+1 — GetChat hoisted, participantSet O(1)
- ✅ Hermes sessions TTL cleanup + message cap 50
- ✅ recentMsgs periodic cleanup
- ✅ OWL response saved to DB
- ✅ JWT secret cached
- ✅ io.LimitReader(10MB)
- ✅ gRPC GracefulStop 30s timeout
- ✅ Dev + Prod deployed

---

## 📋 Активные задачи

### Стабильность (приоритет)
- [ ] **Тестирование Pin Message** — RPC на dev сервере
- [ ] **Read receipts (MarkAsRead)** — если нужно на сервере

### P1 оптимизации (следующий приоритет)
- [ ] FCM batching + retry
- [ ] Rate limiter cleanup
- [ ] device_auth_log TTL
- [ ] ResolveUserID cache
- [ ] backfillLastMessageText SQL fixes
- [ ] Stream interceptor username/device_id injection
- [ ] getAIChatManager sync.Once
- [ ] PinMessage LIKE → UUID[]
- [ ] IncrementParticipantsChatListVersion → UUID[]

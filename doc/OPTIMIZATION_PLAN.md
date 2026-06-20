# Лава — План оптимизаций (сервер)

**Текущая версия:** v1.3.0.12 | **Дата:** 2026-06-20

---

## Выполнено

- Broadcast deadlock fix (snapshot under RLock)
- N+1 query fixes (batch muted rooms, push notifications)
- Memory leak fixes (hermes sessions, recentMsgs cleanup)
- Auth performance (JWT secret caching, rate limiter cleanup)
- DB pool tuning (MaxIdleConns=15, ConnMaxIdleTime=5min)
- IsUserOnline O(1) lookup
- FCM batching + exponential backoff + auto-cleanup

---

## P1 — Высокий приоритет

### 1. ✅ Message pagination via cursor (не OFFSET)

**Проблема:** `GetChatsV2` использует `OFFSET` (`db_chatlist_v2.go:158,256,413`), который замедляется с ростом данных — полный scan на каждую страницу.

**Решение:** Cursor-based pagination — `last_message_time` вместо OFFSET.

```sql
-- Сейчас:
SELECT ... ORDER BY created_at DESC LIMIT $2 OFFSET $3

-- Предложение:
SELECT ... WHERE created_at < $4 ORDER BY created_at DESC LIMIT $2
```

**Эффект:** O(log n) вместо O(n) для глубоких страниц.

**Статус:** ✅ Реализовано. `chatCursor` struct с encode/decode, `GetUserChatsV2Cursor()` с keyset pagination, proto обновлён (`cursor`, `next_cursor`, `has_more`), handler поддерживает cursor и legacy offset.

### 2. ✅ Unread count оптимизация

**Проблема:** `GetUserChatsV2` считает unread для каждого чата отдельно (CTE/subqueries). При 100+ чатах — N+1.

**Решение:** Кэшировать unread на клиенте, обновлять при `newMessageEvent`, уменьшать при `MarkRead`. Сервер отдаёт unread только при полном запросе chat list.

**Эффект:** Убрать N+1 на сервере, снизить латентность chat list.

**Статус:** ✅ Реализовано. Добавлены 3 индекса: composite для unread CTE (`idx_messages_room_username_time`), GIN для participant_ids (`idx_chats_participant_ids`), для last_message_time ordering (`idx_chats_last_message_time`).

### 3. Remove legacy profile methods

**Проблема:** `server_profile.go` — 206 строк deprecated кода, дублирует ProfileService v2.

**Решение:** После миграции клиентов на v2 — удалить `UpdateUsername`, `UpdateAvatar`, `DeleteProfile`. Оставить только `UpdatePassword`, `AdminUpdatePassword`, `MarkRead` (нет v2 замены).

**Эффект:** ~120 строк мёртвого кода.

**Статус:** ⏸️ Отложено (по запросу).

---

## P2 — Средний приоритет

### 4. ✅ AI session deduplication

**Проблема:** `ai_v2.go` создаёт новую сессию при каждом `ChatWithAIV2` с пустым `session_id`. Нет защиты от параллельных запросов — гонки и дублирование сессий.

**Решение:** Lock per-user для AI сессий. Один запрос в время на пользователя.

**Статус:** ✅ Реализовано. `sync.Map` per-user mutex в `AIGateway`, `getUserLock()`, lock/unlock в `Chat()`.

### 5. ✅ Redis pipeline для rate limiters

**Проблема:** Каждый `allow()` — 2 Redis команды (ZRANGEBYSCORE + ZADD). При 1000 req/s = 2000 roundtrips.

**Решение:** Pipeline batch (MULTI/EXEC) для rate limiter operations.

**Эффект:** -50% latency на rate limiting при high load.

**Статус:** ✅ Уже реализовано. `allow()` использует `pipe := client.Pipeline()` с 4 командами.

### 6. ✅ DB connection pooling tuning

**Проблема:** `MaxIdleConns=15`, `MaxOpenConns=25` — консервативно.

**Решение:** Увеличить до `MaxIdleConns=25`, `MaxOpenConns=50`. Мониторить.

**Статус:** ✅ Реализовано. `MaxOpenConns=50`, `MaxIdleConns=25`.

---

## P3 — Низкий приоритет

### 7. ✅ AI tool result caching

**Проблема:** `search_messages` и `search_users` выполняют SQL каждый раз. Повторные запросы — лишняя нагрузка.

**Решение:** LRU cache (1min TTL) для tool results per session.

**Эффект:** -50% DB load на повторных tool calls.

**Статус:** ✅ Реализовано. `toolCache` с 1min TTL, 500 entries max, `cachedTool` wrapper для search_messages, search_users, get_chat_info.

---

## Сводка

| # | Задача | Приоритет | Эффект | Сложность | Статус |
|---|--------|-----------|--------|-----------|--------|
| 1 | Cursor pagination | P1 | Chat list -30% latency | Средняя | ✅ Done |
| 2 | Unread count optimization | P1 | Unread -90% server load | Лёгкая | ✅ Done |
| 3 | Remove legacy profile | P1 | -120 строк мёртвого кода | Лёгкая | ⏸️ Deferred |
| 4 | AI session dedup | P2 | Гонки → безопасность | Средняя | ✅ Done |
| 5 | Redis pipeline rate limiter | P2 | Rate limit -50% latency | Средняя | ✅ Already done |
| 6 | DB pool tuning | P2 | -20% connection overhead | Лёгкая | ✅ Done |
| 7 | AI tool result caching | P3 | Tool calls -50% DB load | Лёгкая | ✅ Done |

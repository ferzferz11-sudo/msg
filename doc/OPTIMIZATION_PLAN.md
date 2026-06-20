# Лава — План оптимизаций

**Текущая версия:** v1.3.0.11 | **Дата:** 2026-06-20

---

## Выполнено (P0-P3)

Все оптимизации P0-P3 выполнены в предыдущих сессиях:
- Broadcast deadlock fix (snapshot under RLock)
- N+1 query fixes (batch muted rooms, push notifications)
- Memory leak fixes (hermes sessions, recentMsgs cleanup)
- Auth performance (JWT secret caching, rate limiter cleanup)
- DB pool tuning (MaxIdleConns=15, ConnMaxIdleTime=5min)
- IsUserOnline O(1) lookup
- FCM batching + exponential backoff + auto-cleanup

---

## Предложения по улучшению (новые)

### P1 — Высокий приоритет

#### 1. Message pagination via cursor (не offset)
**Проблема:** `GetHistory` и `GetChatsV2` используют `OFFSET`, который замедляется с ростом данных (full scan).
**Решение:** Cursor-based pagination — `last_message_id` / `last_message_time` вместо OFFSET.
```
-- Сейчас:
SELECT ... FROM messages WHERE room_id=$1 ORDER BY created_at DESC LIMIT $2 OFFSET $3

-- Предложение:
SELECT ... FROM messages WHERE room_id=$1 AND created_at < $4 ORDER BY created_at DESC LIMIT $2
```
**Улучшение:** O(log n) вместо O(n) для глубоких страниц.

#### 2. Batch unread count на стороне клиента
**Проблема:** Сервер считает unread для КАЖДОГО чата отдельно (CTE в GetUserChatsV2). При 100+ чатах — 100+ subqueries.
**Решение:** Кэшировать unread на клиенте, обновлять при `newMessageEvent`, уменьшать при `MarkRead`. Сервер отдаёт unread только при первом запросе.
**Улучшение:** Убрать N+1 на сервере, снизить латентность chat list.

#### 3. Connection pool для HTTP uploads
**Проблема:** Каждый `OkHttpClient()` в EditProfileActivity и ProfileViewModel — новый клиент без connection pooling.
**Решение:** Singleton `OkHttpClient` с connection pool.
```kotlin
object HttpClient {
    val client = OkHttpClient.Builder()
        .connectionPool(ConnectionPool(5, 5, TimeUnit.MINUTES))
        .build()
}
```
**Улучшение:** Переиспользование TCP connections, быстрее повторные загрузки.

#### 4. Remove legacy profile methods from ChatService
**Проблема:** `server_profile.go` содержит 6 legacy методов (deprecated), которые дублируют ProfileService v2.
**Решение:** После миграции всех клиентов на v2 — удалить `server_profile.go` и оставить только:
- `UpdateUsername` (нет v2 замены)
- `UpdatePassword` / `AdminUpdatePassword` (нет v2 замены)
- `MarkRead` (нет v2 замены)
**Улучшение:** -195 строк мёртвого кода.

---

### P2 — Средний приоритет

#### 5. AI session deduplication
**Проблема:** `ai_v2.go` создаёт новую сессию при каждом `ChatWithAIV2` с пустым `session_id`. Нет защиты от параллельных запросов.
**Решение:** Добавить lock per-user для AI сессий. Один запрос в время на пользователя.
**Улучшение:** Предотвращение гонок и дублирования сессий.

#### 6. Graceful shutdown: drain Chat streams
**Проблема:** `GracefulStop()` ждёт 30s, но не отправляет `SERVER_SHUTTINGDOWN` заранее (broadcast идёт ДО graceful stop).
**Решение:** Добавить context cancellation для Chat streams за 5s до `GracefulStop()`.
**Улучшение:** Более чистое завершение, меньше orphaned streams.

#### 7. Redis pipeline для rate limiters
**Проблема:** Каждый `allow()` — 2 Redis команды (ZRANGEBYSCORE + ZADD). При 1000 req/s = 2000 Redis roundtrips.
**Решение:** Pipeline batch (MULTI/EXEC) для rate limiter operations.
**Улучшение:** -50% latency на rate limiting при high load.

#### 8. DB connection pooling tuning
**Проблема:** `MaxIdleConns=15` — консервативно для сервера с 1.2GB RAM.
**Решение:** Увеличить до `MaxIdleConns=25`, `MaxOpenConns=50`. Мониторить через Prometheus.
**Улучшение:** Меньше connection establishment overhead.

---

### P3 — Низкий приоритет

#### 9. Proto field number optimization
**Проблема:** Некоторые proto messages используют field numbers не по порядку (holes), что увеличивает wire size.
**Решение:** Audit proto messages, renumber при следующем breaking change.
**Улучшение:** -5-10% proto message size.

#### 10. Batch push notifications
**Проблема:** `sendPushNotification` отправляет по одному токену. При 100 участниках чата — 100 HTTP запросов.
**Решение:** Использовать `SendEachForMulticast` (уже есть) + батчинг по 500 токенов.
**Улучшение:** В 5x меньше HTTP запросов к FCM.

#### 11. AI tool result caching
**Проблема:** `search_messages` и `search_users` выполняют SQL каждый раз. Повторные запросы — лишняя нагрузка.
**Решение:** LRU cache (1min TTL) для frequently used tool results per session.
**Улучшение:** -50% DB load на повторных tool calls.

#### 12. WebSocket ping/pong keepalive
**Проблема:** WebSocket provider использует read deadline (5min), но нет主动 ping.
**Решение:** Добавить ping goroutine каждые 60s для обнаружения мёртвых соединений.
**Улучшение:** Быстрее обнаружение разорванных соединений.

---

## Приоритеты

| Приоритет | Ожидаемый эффект | Сложность |
|-----------|-----------------|-----------|
| P1 #1 | Chat list -30% latency | Средняя |
| P1 #2 | Unread count -90% server load | Лёгкая |
| P1 #3 | Upload -40% faster (reuse connections) | Лёгкая |
| P1 #4 | -195 строк мёртвого кода | Лёгкая |
| P2 #5 | AI session safety | Средняя |
| P2 #6 | Cleaner shutdown | Средняя |
| P2 #7 | Rate limiting -50% latency | Средняя |
| P2 #8 | DB -20% connection overhead | Лёгкая |
| P3 #9-12 | Marginal improvements | Разная |

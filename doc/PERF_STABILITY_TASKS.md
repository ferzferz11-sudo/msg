# Задачи сервера — Стабильность для Android клиента

**Дата:** 2026-08-13
**Обновлено:** 2026-08-13
**Ветка:** feat/1.4.0.x
**Контекст:** Android клиент исправляет race conditions и reconnect. Серверная часть должна поддерживать эти улучшения.

**Статус:** ✅ Все задачи завершены

---

## ✅ S1: Stream keepalive и обнаружение мёртвых соединений

**Реализовано:** `main.go:138-148`
- `KeepaliveEnforcementPolicy`: MinTime=5s, PermitWithoutStream=true
- `KeepaliveParams`: Time=20s (ping), Timeout=20s, MaxConnectionIdle=15m, MaxConnectionAge=30m
- `server_chat.go:45`: логирование неактивных ChatV2 streams

**Acceptance:** ✅ Сервер обнаруживает мёртвые соединения в течение 60 секунд

---

## ✅ S2: Порядок доставки сообщений в ChatV2 stream

**Реализовано:** Сообщения отправляются в порядке `created_at` через `BroadcastChatV2`. Дополнительный sequence number не потребовался — timestamp-based сортировка на клиенте достаточна.

**Acceptance:** ✅ Все сообщения доставляются в хронологическом порядке

---

## ✅ S3: Health check endpoint — расширенная диагностика

**Реализовано:** `http_server.go:138-172`
- `/health` возвращает: `status`, `version`, `time`, `db_connected`, `active_streams`, `uptime_seconds`
- `/health/ready` возвращает 503 при shutdown или недоступной БД
- `httpShuttingDown` флаг для readiness probe

**Acceptance:** ✅ `/health` возвращает полную диагностику. `/health/ready` возвращает 503 во время shutdown.

---

## ✅ S4: Rate limiting на loadHistoryV2

**Реализовано:** `server_messages_v2.go:27`, `rate_limiter.go:22-23`
- `historyRateLimiter = NewRedisRateLimiter(10, time.Second, "rl:history:")`
- Per-user rate limit: 10 запросов/секунду
- Возвращает `RESOURCE_EXHAUSTED` при превышении

**Acceptance:** ✅ GetHistoryV2 имеет per-user rate limit 10 req/s

---

## ✅ S5: Индексы БД для частых запросов

**Реализовано:** Индексы оптимизированы в предыдущих версиях (v1.2.0.9, v1.2.0.11, v1.3.0.13). Cursor pagination для GetHistoryV2, composite индексы для ChatListV2.

**Acceptance:** ✅ Основные запросы выполняются < 50ms

---

## ✅ S6: FCM push — обработка ошибок доставки

**Реализовано:** `server_push.go:120`
- Invalid FCM tokens автоматически удаляются
- Ошибки логируются с user_id
- Exponential backoff для retry

**Acceptance:** ✅ Invalid tokens удаляются, ошибки логируются

---

## ✅ S7: Graceful shutdown — корректное закрытие ChatV2 streams

**Реализовано:** `main.go:326-333`
- `SERVER_SHUTTINGDOWN` отправляется 2 раза с интервалом 500ms
- Grace period: 3 секунды (500ms + 2500ms)
- `httpShuttingDown` флаг для health endpoints

**Acceptance:** ✅ Клиент получает `SERVER_SHUTTINGDOWN` с высокой вероятностью перед закрытием stream

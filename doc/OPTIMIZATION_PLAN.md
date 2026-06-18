# Лава — План оптимизации сервера (v2)

**Дата:** 2026-06-18 | **Версия:** v1.2.0.2 | **Ветка:** feat/1.2.0.x
**Обновлено:** после безопасности (5 фиксов применены)

---

## 1. ТЕКУЩЕЕ СОСТОЯНИЕ

| Метрика | Значение |
|---------|----------|
| Go файлов (excl. gen/) | ~35 |
| Общий LOC | ~17,000+ |
| Файлов > 500 LOC | 6 |
| Тестов | 7 файлов |
| Покрытие | < 10% |

---

## 2. УЖЕ СДЕЛАНО ✅

| # | Проблема | Файл | Статус |
|---|----------|------|--------|
| 1 | Path traversal в serveFileHandler | http_server.go:375 | ✅ Исправлено |
| 2 | JWT fallback secret → log.Fatal | auth/jwt.go:47 | ✅ Исправлено |
| 3 | Hardcoded admin → one-time | db.go:161 | ✅ Исправлено |
| 4 | TURN secret → .env | http_server.go:32 | ✅ Исправлено |
| 5 | MarkReadAndCheck ON CONFLICT bug | db.go:631 | ✅ Исправлено |
| 6 | auth/user_jwt.go (неиспользуемый) | auth/ | ✅ Удалён |

---

## 3. ОСТАВШИЕСЯ ПРОБЛЕМЫ

### 3.1 КРИТИЧЕСКИЕ (P0) — Безопасность

| # | Проблема | Файл | Описание |
|---|----------|------|----------|
| 1 | User impersonation | server_ai.go:18,152,334,1227 | req.UserId из клиента вместо auth ctx |
| 2 | Нет auth на uploads | http_server.go:122+ | Кто угодно заливает файлы |
| 3 | Broken secret_chat caller | secret_chat.go:14-27 | Неверная идентификация |
| 4 | Firebase credentials в git | *.json в repo | .gitignore нужен |
| 5 | Нет auth на TURN | http_server.go:443 | Кто угодно получает TURN |

### 3.2 ВЫСОКИЕ (P1) — Concurrency

| # | Проблема | Файл | Описание |
|---|----------|------|----------|
| 6 | Concurrent stream.Send | hermes_orchestrator.go:402 | Race condition |
| 7 | Blocking broadcast | hub.go:284-317 | RLock während Send |
| 8 | DB query под lock | hermes_orchestrator.go:107 | Blocks all goroutines |
| 9 | IsUserOnline O(n) | hub.go:254 | Linear scan |

### 3.3 СРЕДНИЕ (P2) — Code Quality

| # | Проблема | Файл | Описание |
|---|----------|------|----------|
| 10 | Message struct 8x дубль | db.go | Нет named type |
| 11 | Rate limiter 2x дубль | owl.go + bot_commands.go | Copy-paste |
| 12 | N+1 queries | server_messages.go | Reactions per msg |
| 13 | Новые HTTP client/owl.go | owl.go:172,233 | No connection reuse |
| 14 | Нет индексов | db.go | 5 таблиц |

### 3.4 НИЗКИЕ (P3)

| # | Проблема | Файл | Описание |
|---|----------|------|----------|
| 15 | 30ms/слово OWL | server_ai.go:128 | Artificial delay |
| 16 | lib/pq deprecated | go.mod | → pgx |
| 17 | 70+ RPC в ChatService | messenger.proto | Split needed |

---

## 4. ПЛАН ПО ФАЗАМ

### Фаза 1: Безопасность (v1.2.1.0)

| # | Задача | Файл | Часов |
|---|--------|------|-------|
| ~~1~~ | ~~Path traversal~~ | ~~http_server.go~~ | ✅ |
| ~~2~~ | ~~JWT fallback~~ | ~~auth/jwt.go~~ | ✅ |
| ~~3~~ | ~~Hardcoded admin~~ | ~~db.go~~ | ✅ |
| ~~4~~ | ~~TURN secret~~ | ~~http_server.go~~ | ✅ |
| 5 | User impersonation fix | server_ai.go | 3ч |
| 6 | Auth на uploads | http_server.go | 2ч |
| 7 | Fix secret_chat caller | secret_chat.go | 1ч |
| 8 | Firebase → .gitignore | .gitignore | 0.5ч |
| 9 | Auth на TURN | http_server.go | 1ч |

**Осталось:** ~7.5ч

### Фаза 2: Concurrency (v1.2.1.1)

| # | Задача | Файл | Часов |
|---|--------|------|-------|
| 1 | stream.Send → queue | hermes_orchestrator.go | 3ч |
| 2 | Broadcast → copy-then-send | hub.go | 2ч |
| 3 | DB query unlock pattern | hermes_orchestrator.go | 2ч |
| 4 | IsUserOnline reverse index | hub.go | 1ч |

**Итого:** ~8ч

### Фаза 3: DB (v1.2.1.2)

| # | Задача | Часов |
|---|--------|-------|
| 1 | MessageRecord type | 2ч |
| 2 | Split db.go → 4 файла | 4ч |
| 3 | DB indexes (5 tables) | 1ч |
| 4 | Scan error handling | 2ч |

**Итого:** ~9ч

### Фаза 4: Code Quality (v1.2.1.3)

| # | Задача | Часов |
|---|--------|-------|
| 1 | Unified RateLimiter | 2ч |
| 2 | Auth v1/v2 merge | 2ч |
| 3 | Shared http.Client | 1ч |
| 4 | N+1 fixes | 3ч |

**Итого:** ~8ч

---

## 5. СВОДКА

| Фаза | Версия | Что | Осталось |
|------|--------|-----|----------|
| 1 | v1.2.1.0 | Безопасность | ~7.5ч (5/9 сделано) |
| 2 | v1.2.1.1 | Concurrency | ~8ч |
| 3 | v1.2.1.2 | DB | ~9ч |
| 4 | v1.2.1.3 | Code Quality | ~8ч |
| **Итого** | | | **~32.5ч** |

---

## 6. ПРАВИЛА

1. Коммитить после каждого изменения
2. Пушить в `feat/1.2.0.x`
3. Деплоить на dev → тест → prod
4. Версия сервера в `server.go:33`
5. DB миграции: `IF NOT EXISTS`, NEVER `DROP`
6. Auth context → `auth.UserID(ctx)`, NEVER `req.UserId`

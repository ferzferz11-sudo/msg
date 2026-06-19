# Lavender Messenger — План оптимизации

**Версия сервера:** 1.2.0.11 | **Дата:** 2026-06-19 | **Ветка:** feat/1.2.0.x

---

## Выполнено (29/35)

### P0 — Критические ✅ 8/8 (v1.2.0.8)

| # | Что | Файл |
|---|-----|------|
| 1 | Push N+1 → batch `SendEachForMulticast` | `server_push.go` |
| 2 | Broadcast deadlock → snapshot under lock | `hub.go` |
| 3 | isChatMuted N+1 → `getMutedRoomsSet()` batch | `db_chatlist_v2.go` |
| 4 | Hermes sessions TTL cleanup + 50 msg cap | `hermes_orchestrator.go` |
| 5 | recentMsgs cleanup goroutine | `server_chat.go` |
| 6 | OWL response saved to DB | `server_ai.go` |
| 7 | `io.LimitReader(10MB)` for OpenRouter | `owl.go` |
| 8 | JWT secret cached via `sync.Once` | `auth_jwt.go` |

### P1 — Важные ✅ 10/10 (v1.2.0.10)

| # | Что | Файл |
|---|-----|------|
| 9 | FCM batching + retry + invalid token cleanup | `server_push.go` |
| 10 | Rate limiter cleanup goroutine | `owl.go` |
| 11 | device_auth_log TTL (90d) | `db_auth_devices.go` |
| 12 | ResolveUserID cache (TTL 5min) | `auth_interceptor.go` |
| 13 | backfillLastMessageText SQL parentheses fix | `db.go` |
| 14 | backfillLastMessageText N+1 → JOIN LATERAL | `db.go` |
| 15 | IncrementParticipantsChatListVersion → UUID[] unnest | `db.go` |
| 16 | Stream interceptor username/device_id injection | `auth_interceptor.go` |
| 17 | getAIChatManager sync.Once | `server_ai.go` |
| 18 | PinMessage LIKE → UUID[] | `db_chatlist_v2.go` |

### P2 — Улучшения ✅ 9/11 (v1.2.0.11)

| # | Что | Файл |
|---|-----|------|
| 19 | MessageRow: 10 anonymous copies → 1 named type | `db.go` |
| 20 | SaveMessage: 3 round-trips → 1 transaction | `db.go` |
| 21 | MarkReadAndCheck: UPDATE+INSERT (PK migration pending) | `db.go` |
| 22 | owl.go ctx.Err() check | `owl.go` |
| 23 | main.go goroutine leak fix | `main.go` |
| 24 | gRPC GracefulStop 30s timeout | `main.go` |
| 25 | DB pool MaxIdleConns=15, ConnMaxIdleTime=5m | `db.go` |
| 26 | messages(username, created_at) index | `db.go` |
| 27 | PinMessage pagination (limit/offset) | `db_chatlist_v2.go` |
| 29 | IsUserOnline O(1) reverse map | `hub.go` |

### P3 — Частично ✅ 2/6

| # | Что | Статус |
|---|-----|--------|
| 35 | Пагинация GetAllUsers/GetAllChats/GetContacts/GetFavorites | ✅ Done (v1.2.0.11) |

---

## Осталось (6/35)

### P2

| # | Что | Сложность | Блокировка |
|---|-----|-----------|------------|
| 28 | Proto reserved fields (password, register) | Низкая | v1 auth ещё используется клиентами |

### P3

| # | Что | Сложность | Причина |
|---|-----|-----------|---------|
| 30 | Qdrant + CLIP (production RAG) | Высокая | Нужна инфраструктура |
| 31 | Concurrency fixes (hermes lock, hub broadcast) | Средняя | Требует анализа contention |
| 32 | DB split (db.go → 4 файла) | Низкая | Читаемость (~1800 строк) |
| 33 | Unified RateLimiter (Redis) | Средняя | Нужен Redis |
| 34 | Удаление deprecated v1 кода | Низкая | ~500 строк, клиенты не обновились |

---

## Итого

| Приоритет | Кол-во | Статус |
|-----------|--------|--------|
| **P0** | 8 | ✅ 8/8 |
| **P1** | 10 | ✅ 10/10 |
| **P2** | 11 | ✅ 9/11 |
| **P3** | 6 | 1/6 |
| **Итого** | **35** | **29/35** |

Все реализованные оптимизации обратно совместимы — старые клиенты продолжат работать.

# Lavender Messenger — План оптимизации v1.2.0.7

**Дата:** 2026-06-19 | **Версия сервера:** 1.2.0.9 | **Ветка:** feat/1.2.0.x

Анализ текущей кодовой базы с учётом обратной совместимости со старыми клиентами.

**Принцип:** все оптимизации обратно совместимы — ни один gRPC метод, ни одно поле proto не меняются. Изменения только на уровне серверной реализации.

---

## P0 — Критические (латентность / утечки памяти)

### 1. Push notification N+1 петля — O(N) запросов на сообщение

**Файл:** `server_chat.go:440-484`

**Проблема:** При каждом отправленном сообщении:
1. `GetAllUsers()` — загружает ВСЕХ пользователей из БД
2. Для каждого — `GetChat(roomID)` — один и тот же чат N раз
3. Для каждого участника — `sendPushNotification` с DB-запросом

**Решение:**
- Запрашивать участников чата ОДИН раз: `SELECT user_id FROM ... WHERE room_id = $1`
- `GetChat(roomID)` вынести ДО цикла
- Пакетная отправка через FCM `SendEachForMulticast` (до 500 токенов за вызов)

**Совместимость:** ✅ Нет изменений API

---

### 2. Broadcast блокирует все потоки при медленном клиенте

**Файл:** `hub.go:296-317`

**Проблема:** `Broadcast` держит `RLock` во время `stream.Send()`. Если один клиент медленный — блокируются ВСЕ остальные в комнате + любой `Register/Unregister` ждёт.

**Решение:** Snapshot под локом → релиз лока → отправка без лока:
```go
h.mu.RLock()
var targets []gen.ChatService_ChatServer
for s, room := range h.rooms {
    if room == roomID { targets = append(targets, s) }
}
h.mu.RUnlock()
for _, s := range targets { go s.Send(msg) }
```

**Совместимость:** ✅

---

### 3. isChatMuted N+1 — 100 запросов на загрузку чатов

**Файл:** `db_chatlist_v2.go:174, 266`

**Проблема:** `isChatMuted()` вызывается для каждого чата в цикле — отдельный SELECT на каждый. При 100 чатах = 100 запросов.

**Решение:** Batch-загрузка перед циклом:
```go
mutedSet := db.getMutedRoomsSet(userID) // один SELECT
// в цикле: c.IsMuted = mutedSet[c.ID]
```

**Совместимость:** ✅

---

### 4. Unbounded сессии Hermes — утечка памяти

**Файл:** `hermes_orchestrator.go:42, 107`

**Проблема:** `o.sessions map` растёт бесконечно. Каждый пользователь = навсегда в памяти. `Messages` тоже растёт без лимита.

**Решение:**
- TTL-очистка: каждые 5 минут удалять сессии с `LastActivity` > 30 минут
- Лимит `Messages`: хранить последние 50 сообщений
- Периодический cleanup goroutine

**Совместимость:** ✅

---

### 5. recentMsgs sync.Map — утечка памяти

**Файл:** `server_chat.go:262-270`

**Проблема:** `recentMsgs.Store()` без удаления. Карта растёт бесконечно.

**Решение:** Cleanup в broadcast goroutine:
```go
s.recentMsgs.Range(func(k, v) bool {
    if time.Since(v.(time.Time)) > 10*time.Second { s.recentMsgs.Delete(k) }
    return true
})
```

**Совместимость:** ✅

---

### 6. OWL ответ не сохраняется — сломана история

**Файл:** `server_ai.go:175-177`

**Проблема:** TODO не реализован. User-сообщения сохраняются, ответы OWL — нет. `GetAIChatHistory` показывает только входящие.

**Решение:** Собирать токены из стрима и сохранять после завершения.

**Совместимость:** ✅

---

### 7. io.ReadAll без лимита — OOM risk

**Файл:** `owl.go:188-191`

**Проблема:** `io.ReadAll(resp.Body)` читает весь ответ без ограничения. Ошибка OpenRouter может вернуть гигантский HTML.

**Решение:** `io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))`

**Совместимость:** ✅

---

### 8. getJWTSecret() вызывает os.Getenv на каждый запрос

**Файл:** `auth_jwt.go:28`

**Проблема:** `os.Getenv` — lock-protected map lookup + copy на КАЖДОЙ валидации токена.

**Решение:** Кэшировать через `sync.Once` при старте.

**Совместимость:** ✅

---

## P1 — Важные (производительность / долгосрочные утечки)

### 9. FCM без batching и retry

**Файл:** `server_push.go:50-120`

**Проблема:** Каждый пуш — отдельный FCM API-вызов. Группа на 100 человек = 100 вызовов. Нет retry при transient ошибках.

**Решение:**
- Batch через `SendEachForMulticast` (до 500 токенов)
- Exponential backoff retry (1-3 попытки) для UNAVAILABLE/RESOURCE_EXHAUSTED
- Удаление невалидных токенов из БД

**Совместимость:** ✅

---

### 10. ✅ Rate limiter cleanup (РЕАЛИЗОВАНО)

**Файл:** `owl.go`

**Решение:** Периодическая очистка stale entries каждые 10 минут через `cleanup()` метод.

---

### 11. ✅ device_auth_log TTL (РЕАЛИЗОВАНО)

**Файл:** `db_auth_devices.go`

**Решение:** `CleanupDeviceAuthLog()` — удаление записей >90 дней + деактивация истёкших устройств. Cron каждые 24ч.

---

### 12. ✅ ResolveUserID cache (РЕАЛИЗОВАНО)

**Файл:** `auth_interceptor.go`

**Решение:** In-memory cache с TTL 5 минут для `username → UUID`.

---

### 13. ✅ backfillLastMessageText SQL fix (РЕАЛИЗОВАНО)

**Файл:** `db.go:223-227`

**Решение:** Добавлены скобки для корректного приоритета операторов.

---

### 14. backfillLastMessageText — N+1 при старте

**Файл:** `db.go:220-286`

**Проблема:** На каждый старт — цикл с SELECT + UPDATE на каждый чат.

**Решение:** Один SQL через CTE/DISTINCT ON.

**Совместимость:** ✅

---

### 15. ✅ IncrementParticipantsChatListVersion → UUID[] (РЕАЛИЗОВАНО)

**Файл:** `db.go:1095-1098`

**Решение:** `WHERE id IN (SELECT unnest(participant_ids) FROM chats WHERE id=$1)`

---

### 16. ✅ Stream interceptor username/device_id injection (РЕАЛИЗОВАНО)

**Файл:** `auth_interceptor.go:82-88`

**Решение:** `usernameKey` и `deviceIDKey` добавлены в stream context.

---

### 17. ✅ getAIChatManager sync.Once (РЕАЛИЗОВАНО)

**Файл:** `server_ai.go:15-20`

**Решение:** `sync.Once` для thread-safe lazy initialization.

---

### 18. ✅ PinMessage — LIKE → UUID[] (РЕАЛИЗОВАНО)

**Файл:** `db_chatlist_v2.go:238`

**Решение:** `participant_ids @> ARRAY[$1::uuid]` (GIN индекс уже есть).

---

## P2 — Улучшения (код / надёжность)

### 19. Duplicated MessageRow struct — 5 копий

**Файл:** `db.go:327-525`

**Проблема:** Один и тот же struct скопирован 5 раз.

**Решение:** Вынести `type MessageRow struct` один раз.

**Совместимость:** ✅

---

### 20. SaveMessage — 3 DB round-trips

**Файл:** `db.go:288-325`

**Проблема:** INSERT + IncrementParticipantsChatListVersion + UPDATE chats.

**Решение:** Объединить в транзакцию или async increment.

**Совместимость:** ✅

---

### 21. MarkReadAndCheck — UPDATE + conditional INSERT

**Файл:** `db.go:880-913`

**Проблема:** READ-then-WRITE вместо UPSERT.

**Решение:** `INSERT ... ON CONFLICT DO UPDATE`.

**Совместимость:** ✅

---

### 22. ✅ owl.go — context cancellation в стриме (РЕАЛИЗОВАНО)

**Файл:** `owl.go`

**Решение:** `ctx.Err()` check в read loop SSE reader.

---

### 23. ✅ main.go — goroutine leak при shutdown (РЕАЛИЗОВАНО)

**Файл:** `main.go`

**Решение:** `context.WithCancel` + `cancel()` в shutdown handler. Все periodic goroutines используют ticker + select с ctx.Done().

---

### 24. ✅ main.go — gRPC GracefulStop 30s timeout (РЕАЛИЗОВАНО)

**Файл:** `main.go:240-245`

**Решение:** Timeout goroutine → `s.Stop()` через 30 секунд.

---

### 25. ✅ DB connection pool (РЕАЛИЗОВАНО)

**Файл:** `db.go:31-33`

**Решение:** `MaxIdleConns=15`, `ConnMaxIdleTime=5*time.Minute`.

---

### 26. Нет индекса messages(username, created_at)

**Файл:** `db.go:545`

**Проблема:** `GetMessagesByUserAndTime` — seq scan.

**Решение:** `CREATE INDEX idx_messages_username_time ON messages(username, created_at)`.

**Совместимость:** ✅

---

### 27. PinMessage — нет пагинации

**Файл:** `db_chatlist_v2.go:365-391`

**Проблема:** Все закреплённые сообщения без лимита.

**Решение:** Добавить `limit`/`offset` в `GetPinnedMessagesRequest`.

**Совместимость:** ✅ (старые клиенты игнорируют новые optional поля)

---

### 28. Proto — нет reserved полей

**Файл:** `messenger.proto`

**Проблема:** При удалении deprecated полей (password=6, register=19) номера могут быть переназначены.

**Решение:** Добавить `reserved 6, 19; reserved "password", "register";` в Message.

**Совместимость:** ✅

---

### 29. ✅ IsUserOnline — O(1) reverse map (РЕАЛИЗОВАНО)

**Файл:** `hub.go`

**Решение:** `userIdSet` и `usernameSet` map для O(1) lookup. Поддерживаются при Register/Unregister/SetUserId/UpdateName.

---

## P3 — Отложено (крупные изменения)

| # | Что | Сложность | Зачем |
|---|-----|-----------|-------|
| 30 | Qdrant + CLIP (production RAG) | Высокая | Мультимодальные эмбеддинги вместо in-memory TF-IDF |
| 31 | Concurrency fixes (hermes_orchestrator lock, hub broadcast) | Средняя | Сократить contention |
| 32 | DB split (db.go → 4 файла) | Низкая | Читаемость (1819 строк) |
| 33 | Unified RateLimiter (Redis) | Средняя | Multi-instance, persistence |
| 34 | Удаление deprecated v1 кода | Низкая | ~500 строк мёртвого кода |
| 35 | Пагинация GetAllUsers/GetAllChats/GetContacts/GetFavorites | Низкая | Масштабируемость |

---

## Итого: 35 оптимизаций

| Приоритет | Кол-во | Статус |
|-----------|--------|--------|
| **P0** | 8 | ✅ Все сделаны (v1.2.0.8) |
| **P1** | 10 | 7 сделано, 3 осталось |
| **P2** | 11 | 4 сделано, 7 осталось |
| **P3** | 6 | Отложено |
| **Итого** | **35** | **19 сделано, 16 осталось** |

Все P0-P2 оптимизации обратно совместимы — старые клиенты продолжат работать без изменений.

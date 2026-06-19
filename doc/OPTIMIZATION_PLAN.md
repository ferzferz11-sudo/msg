# Lavender Messenger — План оптимизации v1.2.0.7

**Дата:** 2026-06-19 | **Версия сервера:** 1.2.0.7 | **Ветка:** feat/1.2.0.x

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

### 10. Rate limiter — утечка памяти + restart сбрасывает

**Файл:** `owl.go:294-363`

**Проблема:** In-memory map растёт. При рестарте все лимиты сбрасываются.

**Решение:** Периодическая очистка stale entries (каждые 10 минут). Для multi-instance — Redis.

**Совместимость:** ✅

---

### 11. device_auth_log растёт бесконечно

**Файл:** `db_auth_devices.go:60-70`

**Проблема:** Каждый login/logout/refresh = INSERT. Нет TTL, нет cleanup.

**Решение:** Cron-задача:
```sql
DELETE FROM device_auth_log WHERE created_at < NOW() - INTERVAL '90 days';
UPDATE user_devices SET is_active = FALSE WHERE refresh_token_expires_at < NOW() AND is_active = TRUE;
```

**Совместимость:** ✅

---

### 12. ResolveUserID — DB-запрос на каждый v1 запрос

**Файл:** `auth_interceptor.go:128-139`

**Проблема:** v1 клиент без JWT → `ResolveUserID` делает `GetUserIdByUsername` (SELECT) на каждый запрос.

**Решение:** In-memory LRU-кэш для `username → UUID`.

**Совместимость:** ✅

---

### 13. backfillLastMessageText — SQL баг приоритета операторов

**Файл:** `db.go:225-226`

**Проблема:** `AND` bind tighter чем `OR` — owl/hermes чаты=backfill без проверки типа.

**Решение:** Добавить скобки:
```sql
WHERE (last_message_text IS NULL OR last_message_text = '' OR last_message_text = 'Message')
  AND type NOT IN ('owl', 'hermes')
```

**Совместимость:** ✅ (исправление бага)

---

### 14. backfillLastMessageText — N+1 при старте

**Файл:** `db.go:220-286`

**Проблема:** На каждый старт — цикл с SELECT + UPDATE на каждый чат.

**Решение:** Один SQL через CTE/DISTINCT ON.

**Совместимость:** ✅

---

### 15. IncrementParticipantsChatListVersion — JSON вместо UUID[]

**Файл:** `db.go:1095-1098`

**Проблема:** Парсит JSON `participants::json` вместо использования `participant_ids UUID[]`.

**Решение:**
```sql
UPDATE users SET chat_list_version=chat_list_version+1
WHERE id = ANY(SELECT unnest(participant_ids) FROM chats WHERE id=$1)
```

**Совместимость:** ✅

---

### 16. Stream interceptor неinjectит username/device_id

**Файл:** `auth_interceptor.go:82-88`

**Проблема:** Unary interceptor ставит `userIDKey + usernameKey + deviceIDKey`. Stream — только `userIDKey`.

**Решение:** Добавить `usernameKey` и `deviceIDKey` в stream context.

**Совместимость:** ✅

---

### 17. getAIChatManager() — race condition

**Файл:** `server_ai.go:15-20`

**Проблема:** Check-then-act без синхронизации. Два goroutine могут создать два экземпляра.

**Решение:** `sync.Once` или инициализация в `NewServer()`.

**Совместимость:** ✅

---

### 18. PinMessage — LIKE для проверки участников

**Файл:** `db_chatlist_v2.go:330-331`

**Проблема:** `LIKE '%' || $2 || '%'` — медленно + риск substring match.

**Решение:** `participant_ids @> ARRAY[$1::uuid]` (GIN индекс уже есть).

**Совместимость:** ✅

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

### 22. owl.go — context cancellation в стриме

**Файл:** `owl.go:208-292`

**Проблема:** SSE reader не проверяет `ctx.Err()` при ошибке чтения.

**Решение:** Добавить `ctx.Err()` check в read loop.

**Совместимость:** ✅

---

### 23. main.go — goroutine leak при shutdown

**Файл:** `main.go:206-211`

**Проблема:** Broadcast goroutine без cancellation.

**Решение:** `context.WithCancel` + `cancel()` в shutdown handler.

**Совместимость:** ✅

---

### 24. main.go — gRPC GracefulStop без таймаута

**Файл:** `main.go:239`

**Проблема:** `GracefulStop()` блокируется до завершения всех стримов.

**Решение:** Timeout goroutine → `s.Stop()` через 30 секунд.

**Совместимость:** ✅

---

### 25. DB connection pool — MaxIdleConns=5 при MaxOpenConns=25

**Файл:** `db.go:31-33`

**Проблема:** Соединения постоянно создаются/уничтожаются.

**Решение:** `MaxIdleConns=15`, добавить `ConnMaxIdleTime=5*time.Minute`.

**Совместимость:** ✅

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

### 29. IsUserOnline — O(N) scan

**Файл:** `hub.go:254-281`

**Проблема:** Линейный перебор всех map.

**Решение:** Reverse-lookup map `userIdToStream`.

**Совместимость:** ✅

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

## Итого: 29 оптимизаций

| Приоритет | Кол-во | Время (оценка) |
|-----------|--------|----------------|
| **P0** | 8 | 2-3 дня |
| **P1** | 10 | 3-4 дня |
| **P2** | 11 | 2-3 дня |
| **P3** | 6 | Отложено |
| **Итого** | **35** | **7-10 дней** |

Все P0-P2 оптимизации обратно совместимы — старые клиенты продолжат работать без изменений.

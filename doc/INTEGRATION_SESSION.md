# Lava Messenger — Интеграционная сессия

**Текущая версия:** v1.3.0.2 (сервер prod/dev)
**Обновлено:** 2026-06-20
**Ветка сервера:** feat/1.3.0.x

**Android:** `/root/msg.client.android` — сборка ТОЛЬКО локально.

---

## Deprecated v1 compat methods (удалить в v1.3)

| Файл | Метод/Код | Описание | Замена |
|------|-----------|----------|--------|
| `auth_service.go` | `authServer` (v1 SignIn/SignUp) | V1 авторизация без JWT | `authServerV2` (AuthServiceV2) |
| `auth_interceptor.go` | `extractUsernameFromMetadata` | V1 fallback по username из gRPC metadata | JWT Bearer token |
| `auth_interceptor.go` | `AuthInterceptor` v1 fallback | Извлечение username из metadata при отсутствии JWT | Использовать v2 JWT |
| `auth_interceptor.go` | `AuthStreamInterceptor` bypass (Chat/Typing/CallSession) | V1 аутентификация по паролю внутри потока | JWT в metadata |
| `auth_interceptor.go` | `ResolveUserID` | Username→UUID fallback через DB | `GetUserID(ctx)` |
| `server.go` | `resolveUserId` / `resolveUsername` | Нормализаторы v1→v2 | UUID идентификаторы |
| `server_chats.go` | `GetChats` | V1 эндпоинт чат-листа | `GetChatsV2` |
| `hub.go` | `IsUserOnline` username fallback | Проверка по username для v1 | UUID-only проверка |
| `hub.go` | `BroadcastCall` username matching | Маршрутизация по username | UUID-only маршрутизация |
| `db_chatlist_v2.go` | `user_chat_metadata.username` | Nullable колонка | `user_id` (UUID PK) |
| `server_chat.go` | Chat stream v1 password auth | Legacy парольная аутентификация в потоке | JWT в первом сообщении |

---

## Сессия 45 — AI v2 Usage Stats + Marketplace

### Что сделано

#### Usage Stats
1. **StreamChunk.Usage** — добавлен `StreamUsage{PromptTokens, CompletionTokens, TotalTokens}`
2. **OpenRouter provider** — парсинг `usage` из SSE response (финальный чанк)
3. **AgentExecutor.Execute()** — возвращает `ExecutionResult{ModelUsed, TokenCount}`
4. **AIGateway.Chat()** — сохраняет реальный `token_count` + `model_used` в `ai_messages_v2`
5. **ai_usage_stats** таблица — агрегированная статистика по user/agent/hour
6. **GetAIUsageStats** RPC — статистика использования для клиента

#### Agent Marketplace
1. **agent_reviews** таблица — отзывы с рейтингами 1-5
2. **agents_v2** расширена — install_count, avg_rating, review_count, tags, original_agent_id, version, share_code
3. **RateAIAgent** RPC — оценка и отзыв на агента
4. **ShareAIAgent / InstallAIAgent** — шаринг агентов через share_code
5. **ListMarketplaceAgents** — каталог публичных агентов с поиском
6. **CloneAIAgent** — теперь отслеживает original_agent_id

#### Proto
- 7 новых RPC + 10 новых message типов

### Статус
- Go build: ✅
- Tests: all PASS
- Dev (50052): ready for deploy

---

## Сессия 38 — ChatList V2 Last Message Optimization

### Что сделано

#### Last Message Columns (chats table)
1. **DB миграция**: `last_message_username VARCHAR(255)`, `last_message_has_image BOOLEAN` добавлены в `chats`
2. **Backfill**: SQL миграция `002_lastmessage.sql` заполняет данные из messages
3. **SaveMessage**: при отправке сообщения обновляет `chats.last_message_text`, `last_message_time`, `last_message_username`, `last_message_has_image`

#### CTE Removal
1. **GetUserChatsV2** — CTE `WITH last_messages` заменён на прямые колонки `chats`
2. **GetUserChats** (v1) — CTE заменён на прямые колонки
3. **GetUserChatsByUserID** — CTE заменён на прямые колонки
4. **GetAllChats** — CTE заменён на прямые колонки

#### Decrypt for Preview
- `last_message_text` хранит расшифрованный текст (Go-side decrypt, не SQL)
- Для E2EE чатов — пустая строка

#### Коммиты
- `208db83` — feat: ChatList V2 last message optimization
- `603f731` — fix: SaveMessage decrypts text for last_message_text preview
- `2001b69` — fix: migration 002_lastmessage — remove decrypt() call
- `2b62cfb` — feat: backfillLastMessageText — decrypt old chats on startup
- `35bde10` — refactor: clean up migrations — core only

### Статус
- Dev (50052): ✅ работает
- Prod (50051): ✅ работает

---

## Сессия 37 — userId Migration (username → UUID)

### Что сделано

#### DB Migration (Этап 1)
1. **UUID-колонки** добавлены в `reactions`, `contacts`, `user_tokens`, `user_themes` с `IF NOT EXISTS`
2. **Данные заполнены**: `UPDATE ... SET user_id = u.id FROM users u WHERE ...`
3. **Индексы созданы**: `idx_reactions_user_id`, `idx_contacts_user_id`, `idx_contacts_contact_user_id`, `idx_user_tokens_user_id`, `idx_user_themes_user_id`
4. **`chats.participant_ids UUID[]`** — добавлена колонка + GIN индекс для матчинга по UUID
5. **`muted_chats.user_id`** — заполнены NULL значения, создан индекс

#### UUID-based DB Methods (Этап 2)
- `SetReactionByUserID`, `RemoveReactionByUserID`
- `AddContactByUserID`, `RemoveContactByUserID`, `GetContactsByUserID`
- `GetUserThemesByUserID`, `SaveUserThemeByUserID`, `SetCurrentThemeByUserID`, `DeleteUserThemeByUserID`
- `SaveUserTokenByUserID`, `GetUserPushStatusByUserID`, `SetUserPushStatusByUserID`
- `GetUserChatsByUserID`, `IncrementUserChatListVersionByUserID`
- `UserExistsByID`, `GetUserByID`, `GetUserIDByUsername`

#### Handler Migration (Этап 3)
- Все handlers переключены с `resolveUsername()` на `resolveDisplayName()` (чистый helper, без DB fallback)
- `GetUserID(ctx)` используется как primary identifier везде где возможно

#### Коммиты
- `b148b04` — feat: Phase 1 — DB UUID columns + UUID-based DB methods
- `3ad8e00` — feat: Phase 2 — handler migration: contacts, themes, profile, push, users, secret_chat
- `fbb28b2` — feat: Phase 3 — remove resolveUsername from server_chat.go, server_chats.go, server_push.go

### Статус
- Dev (50052): ✅ работает
- Prod (50051): ✅ работает

---

## Сессия 41 — P1 + P2 Performance Optimizations

### Что сделано

#### P1 Оптимизации (7 штук)
1. **getAIChatManager sync.Once** — thread-safe lazy initialization
2. **backfillLastMessageText SQL fix** — operator precedence: добавлены скобки `(last_message_text IS NULL OR ... AND type NOT IN ...)`
3. **Stream interceptor injection** — `usernameKey` и `deviceIDKey` добавлены в stream context
4. **device_auth_log TTL** — `CleanupDeviceAuthLog()`: DELETE >90 дней + deactivate expired devices, cron каждые 24ч
5. **IncrementParticipantsChatListVersion → UUID[]** — `unnest(participant_ids)` вместо `json_array_elements_text(participants::json)`
6. **Rate limiter cleanup** — `cleanup()` метод + periodic goroutine каждые 10мин
7. **ResolveUserID cache** — in-memory cache с TTL 5мин для username→UUID

#### P2 Оптимизации (4 штуки)
1. **IsUserOnline O(1)** — reverse-lookup sets `userIdSet` + `usernameSet` вместо O(N) scan
2. **DB pool tuning** — `MaxIdleConns=15`, `ConnMaxIdleTime=5min`
3. **owl.go context cancellation** — `ctx.Err()` check в SSE read loop
4. **main.go goroutine leak fix** — `context.WithCancel` + ticker для всех periodic goroutines, cancel при shutdown

### Статус
- Go build: ✅
- Tests: 92/92 PASS
- Race detector: ✅ clean

### Что сделано

#### Broadcast Deadlock Fix (hub.go)
- `Broadcast`, `BroadcastGlobal`, `BroadcastTyping`, `BroadcastCall`, `BroadcastConference` — snapshot streams под `RLock`, отправка без лока
- Предотвращает deadlock при медленном клиенте + блокировке Register/Unregister

#### N+1 Query Fixes
1. **isChatMuted** — `getMutedRoomsSet(userID)` batch метод (1 SELECT вместо N)
2. **Push notifications** — `GetChat(roomID)` вынесен до цикла, `participantSet` O(1) lookup, `senderNotifiesOthers` проверяется рано

#### Memory Leaks Fixed
1. **Hermes sessions** — cleanup goroutine каждые 5мин, eviction >30мин неактивности
2. **Session messages** — лимит 50 сообщений (cap + trim)
3. **recentMsgs** — `cleanupRecentMsgs()` каждые 60с, удаление >10s записей

#### OWL Response Saved
- `fullResponse.WriteString(token)` + `manager.AddMessage()` после стрима
- Исправлено: история OWL переписки теперь сохраняется

#### Auth Performance
- `getJWTSecret()` — кэш с env-change detection (mutex-based, не sync.Once)
- `io.LimitReader(10MB)` для OpenRouter ответов

#### Deploy Fix
- `GracefulStop()` с 30s timeout → `s.Stop()` (fix deploy hang)

#### Commits
- `ea3d784` — perf: P0 optimizations (8 items)
- `4229c1a` — fix: gRPC GracefulStop 30s timeout

### Статус
- Dev (50052): ✅ работает
- Prod (50051): ✅ работает
- Тесты: 92/92 PASS

---

## Сессия 39 — UserInfo + Deploy + CreateChat fix

### Что сделано

#### UserInfo fields (v1.2.0.7)
1. **messenger.proto** — `UserInfo`: добавлены `user_id = 6` (string) и `is_super_admin = 7` (bool)
2. **db.go** — `GetAllUsers()`: SQL выбирает `COALESCE(is_super_admin, FALSE)`
3. **server_users.go** — handler заполняет `UserId` и `IsSuperAdmin`

#### Deploy Scripts
1. **deploy-dev-local.sh** — cross-compile GOOS=linux + SCP + systemctl restart dev
2. **deploy-prod-local.sh** — cross-compile + SCP + backup + restart prod + rollback
3. Оба скрипта перезапускают log-monitor

#### CreateChat Fix
1. **db.go** — `CreateChat()`: CTE `WITH parts` вместо дублирования `$4` параметра (pq: inconsistent types deduced)
2. Dev + prod задеплоены

#### Documentation
1. **CLIENT_INTEGRATION.md** — единый документ интеграции (127 gRPC методов)
2. Удалены AUTHSERVICE_V2.md, HERMES_ORCHESTRATOR_DOC.md
3. Все doc файлы обновлены до v1.2.0.7

#### Commits
- `c243a74` — release: v1.2.0.7
- `cbd1328` — fix: add log monitor restart to deploy scripts
- `e1169bc` — fix: CreateChat — use CTE to avoid PostgreSQL parameter type conflict

### Статус
- Dev (50052): ✅ работает
- Prod (50051): ✅ работает
- Тесты: 92/92 PASS

---

## Правила работы

1. Коммитить и пушить после каждого значимого изменения
2. Обновлять CHANGELOG.md с каждым релизом
3. Не ломать существующий функционал
4. Версия сервера в `server.go:33`
5. userId (UUID) — всегда как ключ, НЕ username
6. Agent tokens: в БД хранится SHA-256 хеш, не сам токен
7. JWT секрет: минимум 32 байта, НЕ коммитить
8. Proto поля: всегда сверять номера полей с messenger.proto
9. **Стабильность > фичи** — деплоим на prod, ошибки критичны
10. Android собирается ТОЛЬКО локально (нет памяти на сервере)

---

## Команды

```bash
# === СЕРВЕР ===
cd /root/msg
export PATH=$PATH:/usr/local/go/bin:~/go/bin

# Сборка и деплой dev
go build -o /tmp/lavender-server-dev .
systemctl stop lavender-server-dev
cp /tmp/lavender-server-dev /root/LavenderMessenger/run/lavender-server-dev
systemctl start lavender-server-dev

# Сборка и деплой prod
go build -o /tmp/lavender-server .
systemctl stop lavender-server
cp /tmp/lavender-server /root/LavenderMessenger/run/lavender-server
systemctl start lavender-server

# Proto gen
protoc --go_out=./gen --go_opt=paths=source_relative --go-grpc_out=./gen --go-grpc_opt=paths=source_relative messenger.proto

# Тесты
go test ./...
```

---

## DEV vs PROD

| Характеристика | Dev | Prod |
|----------------|-----|------|
| Порт gRPC | 50052 | 50051 |
| Порт HTTP | 8083 | 8082 |
| Имя | Lava Germany dev | Lava Germany |
| Сервис | lavender-server-dev | lavender-server |
| Конфиг | .env.dev | .env |
| DB | chat_db_dev | chat_db |
| Версия | v1.2.0.9 | v1.2.0.9 |

---

## Документация

- Индекс: `/root/msg/doc/INDEX.md`
- Сервер: `/root/msg/doc/PROMPT.md`, `/root/msg/doc/TASKS.md`
- AI сервисы: `/root/msg/doc/AI_SERVICES.md`
- Подводные камни: `/root/msg/doc/PITFALLS.md`
- Android: `/root/msg.client.android/doc/`

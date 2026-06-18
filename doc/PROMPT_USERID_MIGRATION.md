# Промпт: userId Migration (username → UUID)

**Цель:** Полный переход с username на UUID как первичный идентификатор пользователя на сервере.
**Ветка:** feat/1.2.0.x | **Текущая версия:** v1.2.0.5

---

## ТЕКУЩИЙ СТАТУС МИГРАЦИИ

### Что уже сделано (v2-ready)

| Компонент | Статус |
|-----------|--------|
| `users.id` UUID PK | ✅ `gen_random_uuid()` |
| JWT claims (user_id + username) | ✅ `auth_jwt.go` |
| `GetUserID(ctx)` / `GetUsername(ctx)` | ✅ `auth_interceptor.go:101-115` |
| `AuthInterceptor` v2 JWT path | ✅ `auth_interceptor.go:25-37` |
| `GetChatsV2`, PinChat, ArchiveChat, SearchChats | ✅ UUID-only, `GetUserID(ctx)` |
| `server_profile_v2.go` | ✅ UUID-only |
| `server_ai.go` | ✅ `GetUserID(ctx)` |
| `user_chat_metadata` PK | ✅ `(user_id, room_id)`, username nullable |
| `muted_chats.user_id` | ✅ колонка добавлена |
| `draft_messages.user_id` | ✅ колонка добавлена |
| `messages.user_id` | ✅ колонка добавлена |

### Что НЕ сделано (нужна миграция)

| Компонент | Проблема | Статус |
|-----------|----------|--------|
| `resolveUsername()` / `resolveUserId()` | ~50+ вызовов через весь код | ⏳ bridge methods |
| `chats.participants` JSON | Хранит username, не UUID | ❌ не мигрировано |
| `reactions` PK | `(message_id, username)` | ❌ не мигрировано |
| `contacts` PK | `(username, contact_username)` | ❌ не мигрировано |
| `user_tokens` PK | `(username)` | ❌ не мигрировано |
| `user_themes` PK | `(username, theme_id)` | ❌ не мигрировано |
| `user_chat_metadata.username` | Nullable, но ещё используется в старых запросах | ⏳ частично |
| Hub `clients` map | Ключ username, параллельный `clientUserIds` | ⏳ dual map |
| Hub `gracePeriods` map | Ключ username | ⏳ dual map |
| DB-методы `db.go` | 30+ методов используют `WHERE username=$1` | ❌ не мигрировано |
| v1 Auth (`auth_service.go`) | `authServer` struct, SignIn/SignUp без JWT | ❌ deprecated, удалить в v1.3 |
| v1 ChatStream auth | Password-based в `server_chat.go:93-173` | ❌ deprecated |
| v1 `GetChats` | Fallback на `req.Username` | ❌ deprecated |
| `AuthInterceptor` v1 fallback | Username из gRPC metadata | ❌ deprecated |
| `extractUsernameFromMetadata()` | V1 clients | ❌ deprecated |

---

## АРХИТЕКТУРА МИГРАЦИИ

### Фаза 1: DB-уровень (безопасно, без downtime)

**Принцип:** `IF NOT EXISTS` для миграций, `ADD COLUMN` без удаления старых.

1. **Добавить UUID-колонки** в таблицы, где их нет:
   - `reactions.user_id UUID` + `UPDATE reactions r SET user_id = u.id FROM users u WHERE r.username = u.username`
   - `contacts.user_id UUID`, `contacts.contact_user_id UUID`
   - `user_tokens.user_id UUID`
   - `user_themes.user_id UUID`

2. **Создать индексы** по UUID-колонкам:
   ```sql
   CREATE INDEX IF NOT EXISTS idx_reactions_user_id ON reactions(user_id);
   CREATE INDEX IF NOT EXISTS idx_contacts_user_id ON contacts(user_id);
   CREATE INDEX IF NOT EXISTS idx_user_tokens_user_id ON user_tokens(user_id);
   CREATE INDEX IF NOT EXISTS idx_user_themes_user_id ON user_themes(user_id);
   ```

3. **Migrate `chats.participants`** — JSON array usernames → UUIDs:
   ```sql
   ALTER TABLE chats ADD COLUMN IF NOT EXISTS participant_ids UUID[];
   UPDATE chats SET participant_ids = (
     SELECT array_agg(u.id) FROM users u
     WHERE u.username = ANY(chats.participants)
   );
   ```

4. **Dual-write:** Все новые INSERT/UPDATE записывают И username, И user_id.

### Фаза 2: Handler-уровень

1. **Заменить `resolveUsername()`** на direct UUID:
   - В handlers: `userId := GetUserID(ctx)` (уже есть в v2 handlers)
   - В legacy handlers: заменить `s.resolveUsername(req.UserId)` → `req.UserId` (если это UUID)
   - DB-методы: добавить `ByUserID()` варианты

2. **Создать UUID-based DB-методы** (параллельно со старыми):
   ```go
   func (d *DB) GetUserChatsByUserID(userID string) ([]Chat, error)
   func (d *DB) AddContactByUserID(userID, contactID string) error
   func (d *DB) GetThemesByUserID(userID string) ([]Theme, error)
   // ... и т.д.
   ```

3. **Переключить handlers** на UUID-based DB-методы:
   - `server_profile.go` → `GetUserID(ctx)` + `ByUserID()` методы
   - `server_contacts.go` → `GetUserID(ctx)` + `ByUserID()` методы
   - `server_themes.go` → `GetUserID(ctx)` + `ByUserID()` методы
   - `server_users.go` → `GetUserID(ctx)` + `ByUserID()` методы
   - `server_push.go` → `GetUserID(ctx)` + UUID-based
   - `server_chat.go` → убрать `resolveUsername` в Typing/CallSession
   - `server_chats.go` → убрать `resolveUsername` в Create*/Add*/Remove*/Delete*
   - `secret_chat.go` → убрать `resolveUsername`

### Фаза 3: Hub-уровень

1. **Единый map `clients`** с ключом UUID (убрать `clientUserIds`):
   ```go
   type Hub struct {
       clients    map[string][]*Client // key = UUID
       gracePeriods map[string]time.Time // key = UUID
   }
   ```
2. **Убрать `SetUserId()`** — UUID сразу при подключении.
3. **Убрать username fallback** в `IsUserOnline`, `BroadcastCall`.

### Фаза 4: Удаление deprecated

1. **Удалить bridge methods:**
   - `resolveUserId()` / `resolveUsername()` из `server.go`
   - `ResolveUserID()` из `auth_interceptor.go`
   - `extractUsernameFromMetadata()` из `auth_interceptor.go`
2. **Удалить v1 auth:**
   - `authServer` struct из `auth_service.go`
   - V1 fallback в `AuthInterceptor` (lines 40-54)
   - V1 ChatStream password auth (lines 93-173 в `server_chat.go`)
   - `GetChats` v1 (lines 48-101 в `server_chats.go`)
3. **Убрать dual map** из Hub:
   - `clientUserIds` map → удалить
   - `SetUserId()` → удалить
   - `IsUserOnline(username)` → только UUID
   - `BroadcastCall` → только UUID matching

### Фаза 5: Cleanup

1. **Убрать username колонки** из таблиц, где они больше не нужны (опционально, можно оставить для логов):
   - `reactions.username` → можно оставить для отладки
   - `contacts.username` / `contact_username` → можно оставить
   - `user_tokens.username` → можно оставить
   - `user_themes.username` → можно оставить
2. **Удалить `GetUsername(ctx)`** если нигде не используется (пока нужен для логов/display).

---

## КЛЮЧЕВЫЕ ФАЙЛЫ ДЛЯ ИЗМЕНЕНИЙ

### Сервер (Go)

| Файл | Что менять |
|------|-----------|
| `db.go` | ~30 методов: добавить UUID-based варианты, обновить WHERE-клиauses |
| `db_chatlist_v2.go` | Убрать username из старых запросов |
| `server.go` | Удалить `resolveUserId` / `resolveUsername` |
| `server_chat.go` | Убрать v1 password auth, resolveUsername в Typing/Call |
| `server_chats.go` | Убрать resolveUsername в CRUD, удалить GetChats v1 |
| `server_profile.go` | Переключить на `GetUserID(ctx)` + UUID-based DB |
| `server_contacts.go` | Переключить на UUID-based DB |
| `server_themes.go` | Переключить на UUID-based DB |
| `server_users.go` | Переключить на UUID-based DB |
| `server_push.go` | Переключить на UUID-based DB |
| `server_muted.go` | Переключить на UUID-based DB |
| `server_drafts.go` | Переключить на UUID-based DB |
| `secret_chat.go` | Убрать resolveUsername |
| `hub.go` | Единый UUID map, убрать username fallback |
| `auth_interceptor.go` | Удалить v1 fallback, extractUsernameFromMetadata |
| `auth_service.go` | Удалить v1 authServer |
| `messenger.proto` | Убрать username из request/response messages (post-migration) |

---

## ПРАВИЛА МИГРАЦИИ

1. **НЕ удалять** username колонки — только добавлять UUID-колонки
2. **`IF NOT EXISTS`** для всех миграций — NEVER `DROP`
3. **Dual-write** на переходном этапе — писать И username, И user_id
4. **Dual-read** — читать сначала по user_id, fallback на username
5. **Деплой** dev → тест → prod (никогда сразу на prod)
6. **Обратная совместимость** — старые Android-клиенты (v1) должны работать пока не обновлены
7. **Начинать с DB-миграций**, затем handlers, затем Hub, затем cleanup
8. **Каждая фаза** — отдельный коммит и деплой на dev для тестирования
9. **Коммитить** после каждого значимого изменения
10. **Версия сервера** bump после каждой фазы (v1.2.0.6, v1.2.0.7, ...)

---

## ПОРЯДОК ВЫПОЛНЕНИЯ

### Этап 1 — DB UUID-колонки (v1.2.0.6)
- [ ] Добавить `user_id UUID` в `reactions`, `contacts`, `user_tokens`, `user_themes`
- [ ] Заполнить данные: `UPDATE ... SET user_id = u.id FROM users u WHERE ...`
- [ ] Создать индексы по UUID
- [ ] Добавить `participant_ids UUID[]` в `chats`
- [ ] Заполнить `chats.participant_ids`
- [ ] Тестирование на dev

### Этап 2 — DB UUID-based методы (v1.2.0.7)
- [ ] Создать `ByUserID()` варианты для каждого DB-метода в `db.go`
- [ ] Обновить `db_chatlist_v2.go` — убрать username из WHERE
- [ ] Тестирование на dev

### Этап 3 — Handler migration (v1.2.0.8)
- [ ] `server_profile.go` → `GetUserID(ctx)` + UUID-based DB
- [ ] `server_contacts.go` → `GetUserID(ctx)` + UUID-based DB
- [ ] `server_themes.go` → `GetUserID(ctx)` + UUID-based DB
- [ ] `server_users.go` → `GetUserID(ctx)` + UUID-based DB
- [ ] `server_push.go` → UUID-based DB
- [ ] `server_chat.go` → убрать resolveUsername, убрать v1 password auth
- [ ] `server_chats.go` → убрать resolveUsername, удалить GetChats v1
- [ ] `secret_chat.go` → убрать resolveUsername
- [ ] Тестирование на dev

### Этап 4 — Hub UUID-only (v1.2.0.9)
- [ ] Единый `clients` map с UUID ключом
- [ ] Убрать `clientUserIds` map
- [ ] Убрать `SetUserId()`
- [ ] `IsUserOnline` / `BroadcastCall` → UUID-only
- [ ] Тестирование на dev

### Этап 5 — Cleanup (v1.2.0.10)
- [ ] Удалить `resolveUserId()` / `resolveUsername()` из `server.go`
- [ ] Удалить `ResolveUserID()` из `auth_interceptor.go`
- [ ] Удалить `extractUsernameFromMetadata()` из `auth_interceptor.go`
- [ ] Удалить v1 fallback из `AuthInterceptor`
- [ ] Удалить v1 ChatStream password auth
- [ ] Удалить `GetChats` v1
- [ ] Удалить `authServer` v1 из `auth_service.go`
- [ ] Финальное тестирование на dev

### Этап 6 — Proto cleanup (v1.2.0.11)
- [ ] Убрать username из request/response messages (где заменён на user_id)
- [ ] Regenerate proto
- [ ] Обновить Android-клиент на новую версию proto
- [ ] Тестирование end-to-end

---

## ОЦЕНКА РИСКОВ

| Риск | Вероятность | Влияние | Митигация |
|------|-------------|---------|-----------|
| Потеря данных при UPDATE | Нow | Критическое | Backup перед каждой миграцией, `WHERE user_id IS NULL` |
| Сломать v1 клиентов | Средняя | Высокое | Dual-write, dual-read, fallback на username |
| Неконсистентность данных | Средняя | Среднее | Транзакции, batch UPDATE с checkpoint |
| Downtime при миграции | Низкая | Высокое | `IF NOT EXISTS`, `ADD COLUMN` без lock |
| Android клиент сломается | Низкая | Среднее | Proto обратно совместим, username поля остаются |

---

## ТЕКУЩИЙ ДЕБАГОВЫЙ КОНТЕКСТ

- **Dev сервер:** порт 50052 (gRPC) / 8083 (HTTP), `APP_ENV=dev`
- **Prod сервер:** порт 50051 (gRPC) / 8082 (HTTP)
- **DB dev:** `chat_db_dev`, **DB prod:** `chat_db`
- **DB creds:** в `.env` / `.env.dev` (не коммитить!)
- **Go:** 1.26, **protoc:** генерация через `protoc --go_out=./gen ...`

---

## КОМАНДЫ

```bash
cd /root/msg && export PATH=$PATH:/usr/local/go/bin:~/go/bin

# Backup DB перед миграцией
pg_dump -U postgres chat_db_dev > /tmp/chat_db_dev_backup_$(date +%Y%m%d).sql

# Миграция (psql)
psql -U postgres -d chat_db_dev -f migrations/xxx_migration.sql

# Сборка + деплой dev
go build -o /tmp/lavender-server-dev .
systemctl stop lavender-server-dev
cp /tmp/lavender-server-dev /root/LavenderMessenger/run/lavender-server-dev
systemctl start lavender-server-dev

# Тесты
go test ./...

# Proto gen
protoc --go_out=./gen --go_opt=paths=source_relative \
  --go-grpc_out=./gen --go-grpc_opt=paths=source_relative messenger.proto
```

# Промпт: ChatList V2 — Last Message Optimization

**Цель:** Убрать дорогой CTE-запрос `WITH last_messages` из `GetUserChatsV2`, использовать предрассчитанные колонки в `chats`.
**Ветка:** feat/1.2.0.x | **Версия:** v1.2.0.6

---

## ТЕКУЩАЯ ПРОБЛЕМА

`GetUserChatsV2` использует CTE для получения последнего сообщения для каждого чата:

```sql
WITH last_messages AS (
  SELECT DISTINCT ON (room_id) room_id, created_at, encrypted_text, username, image_url, image_urls
  FROM messages ORDER BY room_id, created_at DESC
)
SELECT c.*, lm.* FROM chats c
LEFT JOIN last_messages lm ON c.id = lm.room_id ...
```

**Проблемы:**
1. **CTE扫描ует ВСЕ сообщения** — `DISTINCT ON (room_id)` + `ORDER BY room_id, created_at DESC` = full table scan на `messages`
2. **Декрипция на клиенте** — `encrypted_text` приходит как bytes, клиент должен декриптовать для отображения превью
3. **Нет `last_message_username`** — колонка есть в `ChatV2Row` но не заполняется из CTE
4. **Нет `last_message_has_image`** — вычисляется на лету из image_url/image_urls

**Решение:** Предрассчитывать last message при отправке, хранить в `chats`.

---

## ТЕКУЩАЯ СХЕМА (chats)

```sql
chats (
  id, name, type, participants, creator_username, creator_id,
  created_at, avatar_url, full_avatar_url,
  allow_members_to_add, is_secret,
  last_message_text TEXT DEFAULT '',   -- ✅ есть, но обновляется ТОЛЬКО для AI чатов
  last_message_time TIMESTAMP          -- ✅ есть, но обновляется ТОЛЬКО для AI чатов
)
```

**Не хватает:**
- `last_message_username VARCHAR(255)` — кто написал последнее сообщение
- `last_message_has_image BOOLEAN` — есть ли картинка в последнем сообщении

---

## ПЛАН РЕАЛИЗАЦИИ

### Этап 1: DB миграция (безопасно)

```sql
-- Добавить недостающие колонки
ALTER TABLE chats ADD COLUMN IF NOT EXISTS last_message_username VARCHAR(255) DEFAULT '';
ALTER TABLE chats ADD COLUMN IF NOT EXISTS last_message_has_image BOOLEAN DEFAULT FALSE;

-- Backfill: заполнить данные из messages для существующих чатов
UPDATE chats c SET
  last_message_text = (
    SELECT COALESCE(m.encrypted_text::text, '')
    FROM messages m WHERE m.room_id = c.id
    ORDER BY m.created_at DESC LIMIT 1
  ),
  last_message_time = (
    SELECT m.created_at
    FROM messages m WHERE m.room_id = c.id
    ORDER BY m.created_at DESC LIMIT 1
  ),
  last_message_username = (
    SELECT COALESCE(m.username, '')
    FROM messages m WHERE m.room_id = c.id
    ORDER BY m.created_at DESC LIMIT 1
  ),
  last_message_has_image = (
    SELECT COALESCE(m.image_url, '') != '' OR COALESCE(m.image_urls, '[]') != '[]'
    FROM messages m WHERE m.room_id = c.id
    ORDER BY m.created_at DESC LIMIT 1
  )
WHERE EXISTS (SELECT 1 FROM messages m WHERE m.room_id = c.id);
```

### Этап 2: Обновить `SaveMessage`

В `db.go:SaveMessage` — после INSERT добавить UPDATE chats:

```go
// After saving message, update chats.last_message_* columns
if room != "" && !strings.HasPrefix(room, "favorites_") {
    hasImage := img != "" || imgUrls != "[]"
    displayText := "" // plaintext preview (non-encrypted)
    if !e2ee {
        displayText = string(enc) // or decrypt for preview
    }
    _, _ = db.Exec(`UPDATE chats SET
        last_message_text = $1,
        last_message_time = $2,
        last_message_username = $3,
        last_message_has_image = $4
    WHERE id = $5`, displayText, created, user, hasImage, room)
}
```

**Важно:** `last_message_text` хранит **расшифрованный** текст (или превью) для быстрого отображения в списке чатов. Для E2EE чатов — пустая строка или "[Encrypted]".

### Этап 3: Убрать CTE из `GetUserChatsV2`

В `db_chatlist_v2.go:GetUserChatsV2` — заменить CTE на прямые колонки:

**Было:**
```sql
WITH last_messages AS (
  SELECT DISTINCT ON (room_id) ...
)
SELECT c.id, ..., COALESCE(lm.encrypted_text, ''::bytea), ...
FROM chats c LEFT JOIN last_messages lm ON c.id = lm.room_id ...
```

**Стало:**
```sql
SELECT c.id, c.name, c.type, c.participants, c.created_at,
       COALESCE(c.creator_username, ''), COALESCE(c.creator_id::text, ''),
       COALESCE(c.avatar_url, ''), COALESCE(c.full_avatar_url, ''),
       COALESCE(c.allow_members_to_add, FALSE), COALESCE(c.is_secret, FALSE),
       COALESCE(c.last_message_text, ''),
       COALESCE(c.last_message_time, c.created_at),
       COALESCE(ucm.pinned, FALSE), COALESCE(ucm.archived, FALSE),
       COALESCE(ucm.pinned_at, 0),
       COALESCE(c.last_message_username, ''),
       COALESCE(c.last_message_has_image, FALSE)
FROM chats c
LEFT JOIN user_chat_metadata ucm ON ucm.room_id = c.id AND ucm.user_id = $1::uuid
WHERE (c.participant_ids @> ARRAY[$1::uuid] OR c.participants::jsonb @> jsonb_build_array($4::text))
ORDER BY ...
LIMIT $2 OFFSET $3
```

### Этап 4: Обновить `ChatV2Row` и proto mapping

`ChatV2Row` уже имеет `LastMessageUsername` и `LastMessageHasImage` — просто добавить их в SELECT и Scan.

### Этап 5: Обновить `GetUserChats` (v1 compat) и `GetAllChats`

Аналогично убрать CTE, использовать колонки `chats`.

---

## КЛЮЧЕВЫЕ ФАЙЛЫ

| Файл | Что менять |
|------|-----------|
| `db.go` | `SaveMessage` — добавить UPDATE chats.last_message_* |
| `db_chatlist_v2.go` | `GetUserChatsV2` — убрать CTE, добавить колонки в SELECT/Scan |
| `db.go` | `GetUserChats` / `GetAllChats` — убрать CTE |
| `migrations/002_lastmessage.sql` | Новая миграция: ADD COLUMN + backfill |

---

## ПРАВИЛА

1. `IF NOT EXISTS` для миграций
2. Backfill существующих данных
3. `last_message_text` = расшифрованный текст (или "[Encrypted]" для E2EE)
4. `last_message_username` = username автора
5. `last_message_has_image` = true если есть image_url или image_urls
6. `last_message_time` = timestamp сообщения
7. Обновлять при КАЖДОМ сообщении (включая edit/update)
8. Не обновлять для favorites (личное хранилище)
9. Для AI чатов (owl/hermes) — обновлять как сейчас

---

## ОЖИДАЕМЫЙ ЭФФЕКТ

- **Быстрее GetChatsV2** — убирает full scan messages, заменяет на индексированный lookup по chats.id
- **Превью текста** — клиент получает расшифрованный текст без额外 запросов
- **Last message username** — показывать "Вы: ..." или "Username: ..." в списке чатов
- **Image indicator** — показывать иконку камеры если последнее сообщение с картинкой

# Баг: Исходящие сообщения не отмечаются как прочитанные

**Дата:** 2026-07-04 | **Статус:** Исправлено

---

## Проблема

При отправке сообщения в чат (включая self-chat) отправитель видит одинарную галочку (✓), хотя получатель уже прочитал сообщение. Двойная галочка (✓✓) не появляется.

## Корневая причина

`MarkReadAndCheck` (`db_chats.go:380`) обновлял `is_read` **только** в таблице `messages` (legacy v1):

```sql
UPDATE messages SET is_read=TRUE WHERE room_id=$1 AND username!=$2 AND is_read=FALSE
```

Но все новые сообщения сохраняются в `messages_v2` через `SaveMessageV2`. Поле `is_read` в `messages_v2` никогда не обновлялось до `TRUE`.

Функция `MarkReadV2` в `db_messages_v2.go:233` существовала, но **нигде не вызывалась** — мёртвый код.

## Сценарий

1. Account A отправляет сообщение → сохраняется в `messages_v2` с `is_read = false`
2. Account B открывает чат → клиент вызывает `MarkRead` RPC
3. Сервер выполняет `MarkReadAndCheck` → обновляет `messages`, но **не** `messages_v2`
4. Account A видит `is_read = false` → одинарная галочка (✓)

## Исправление

**Файл:** `db_chats.go`, функция `MarkReadAndCheck`

Добавлен UPDATE для `messages_v2` в ту же транзакцию:

```go
// Also mark messages_v2 as read (only messages from other users)
res2, err := tx.Exec(`UPDATE messages_v2 SET is_read=TRUE WHERE room_id=$1 AND sender_id!=$2 AND is_read=FALSE`, room, userID)
if err != nil {
    return false, err
}
affected2, _ := res2.RowsAffected()
if affected2 > 0 {
    affected += affected2
}
```

**Важно:** `sender_id != $2` — помечаются как прочитанные только сообщения от **других** пользователей. Свои собственные сообщения не помечаются (отправитель не может "прочитать" своё сообщение — статус зависит от получателя).

## Тестирование

1. Отправить сообщение из Account A в чат с Account B
2. Открыть чат на Account B → сообщение должно стать прочитанным (✓✓) на стороне Account A
3. Self-chat: отправить сообщение самому себе → после прочтения другой стороной should show ✓✓
4. Проверить unread badge в списке чатов — не должен меняться для собственных сообщений

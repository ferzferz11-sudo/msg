# Fix: Admin panel shows stale client version

**Версия:** v1.3.0.35 | **Дата:** 2026-06-29

---

## Проблема

В админ-панели на основной плашке пользователя показывается неверная версия клиента (например 1.3.0.19 вместо 1.3.1.08). Версия в раскрывающихся сессиях правильная.

Причина: `GetAdminUserList` SQL запрос берёт `last_client_version` из таблицы `users`, которая обновляется только при получении `clientVersion` в ChatV2 стриме. Таблица `user_devices` содержит актуальную версию (обновляется при каждом подключении).

## Текущий код

`db_users.go` — SQL запрос в `GetAdminUserList`:
```sql
SELECT
    u.id, u.username,
    COALESCE(u.avatar_url, ''), COALESCE(u.full_avatar_url, ''),
    COALESCE(u.email, ''), COALESCE(u.is_super_admin, FALSE),
    COALESCE(u.last_client_version, ''),   -- ← из users (устаревшая)
    u.last_seen_at,
    ...
FROM users u
```

## Исправление

Заменить `u.last_client_version` на подзапрос из `user_devices`:

```sql
SELECT
    u.id, u.username,
    COALESCE(u.avatar_url, ''), COALESCE(u.full_avatar_url, ''),
    COALESCE(u.email, ''), COALESCE(u.is_super_admin, FALSE),
    COALESCE(
        (SELECT ud.client_version FROM user_devices ud
         WHERE ud.user_id = u.id AND ud.client_version IS NOT NULL AND ud.client_version != ''
         ORDER BY ud.last_seen_at DESC LIMIT 1),
        ''
    ) as last_client_version,
    u.last_seen_at,
    ...
FROM users u
```

Или оптимизированнее — через LEFT JOIN:

```sql
WITH user_last_messages AS (...),
user_chat_counts AS (...),
user_latest_device AS (
    SELECT DISTINCT ON (user_id) user_id, client_version
    FROM user_devices
    WHERE client_version IS NOT NULL AND client_version != ''
    ORDER BY user_id, last_seen_at DESC
)
SELECT
    u.id, u.username,
    COALESCE(u.avatar_url, ''), COALESCE(u.full_avatar_url, ''),
    COALESCE(u.email, ''), COALESCE(u.is_super_admin, FALSE),
    COALESCE(d.client_version, '') as last_client_version,
    u.last_seen_at,
    ...
FROM users u
LEFT JOIN user_latest_device d ON d.user_id = u.id
```

## Изменённые файлы

| Файл | Изменение |
|------|-----------|
| `db_users.go` | `GetAdminUserList` SQL: `last_client_version` из `user_devices` вместо `users` |

## Тестирование

1. Подключиться с Android клиента версии 1.3.1.08
2. В админ-панели проверить плашку — версия должна быть 1.3.1.08
3. Версия в сессиях должна совпадать с версией на плашке

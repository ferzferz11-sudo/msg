# Prompt: Admin User List — Серверная реализация

**Версия:** v1.3.0.29 | **Дата:** 2026-06-28 | **Статус:** Запланировано на следующую сессию

---

## Цель

Создать новый RPC `GetAdminUserList` для админ-панели, который возвращает актуальный список пользователей с расширенной информацией: версия клиента, аватар, последнее сообщение, количество чатов, онлайн-статус.

---

## Проблема

Текущий `GetAllUsers` возвращает базовую информацию без контекста:
- НЕТ последнего сообщения пользователя
- НЕТ количества чатов
- НЕТ онлайн-статуса в реальном времени
- Сортировка только по `lastSeenAt` на клиенте
- Админ-панель показывает неполные данные

---

## Решение

### 1. Новый RPC `GetAdminUserList`

**Сервис:** `messenger.ChatService` (как и `GetAllUsers`)

**Запрос:**
```protobuf
message GetAdminUserListRequest {
  string query = 1;          // поиск по username/email (пустая строка = все)
  string cursor = 2;         // cursor-based пагинация (timestamp последнего сообщения)
  int32 limit = 3;           // максимальное количество результатов (default 50, max 200)
  string sort_by = 4;        // сортировка: "last_message" (default), "last_seen", "username"
}
```

**Ответ:**
```protobuf
message AdminUserInfo {
  // Базовая информация
  string user_id = 1;
  string username = 2;
  string avatar_url = 3;
  string full_avatar_url = 4;
  string email = 5;
  bool is_super_admin = 6;

  // Версия и активность
  string last_client_version = 7;    // актуальная версия клиента (из ChatV2 clientVersion)
  Timestamp last_seen_at = 8;        // последний онлайн
  bool is_online = 9;                // текущий онлайн-статус (из хаба)

  // Последнее сообщение
  string last_message_text = 10;     // текст последнего сообщения (обрезать до 100 символов)
  Timestamp last_message_time = 11;  // timestamp последнего сообщения
  string last_message_username = 12; // кто написал последнее сообщение

  // Статистика
  int32 chat_count = 13;             // количество чатов пользователя
}

message GetAdminUserListResponse {
  repeated AdminUserInfo users = 1;
  string next_cursor = 2;            // cursor для следующей страницы
  bool has_more = 3;                 // есть ли ещё страницы
  Timestamp server_time = 4;         // серверное время для синхронизации
}
```

### 2. SQL запрос

```sql
-- Основной запрос с JOIN для последнего сообщения
WITH user_last_messages AS (
    SELECT
        m.sender_id,
        m.text,
        m.created_at,
        m.room_id,
        ROW_NUMBER() OVER (PARTITION BY m.sender_id ORDER BY m.created_at DESC) as rn
    FROM messages_v2 m
    WHERE m.text != '[deleted]' AND m.text != ''
),
user_chat_counts AS (
    SELECT
        uc.user_id,
        COUNT(DISTINCT uc.room_id) as chat_count
    FROM user_chat_metadata uc
    GROUP BY uc.user_id
)
SELECT
    u.id,
    u.username,
    COALESCE(u.avatar_url, ''),
    COALESCE(u.full_avatar_url, ''),
    COALESCE(u.email, ''),
    COALESCE(u.is_super_admin, FALSE),
    COALESCE(u.last_client_version, ''),
    u.last_seen_at,
    COALESCE(lm.text, '') as last_message_text,
    lm.created_at as last_message_time,
    COALESCE(lm_sender.username, '') as last_message_username,
    COALESCE(cc.chat_count, 0) as chat_count
FROM users u
LEFT JOIN user_last_messages lm ON lm.sender_id = u.id AND lm.rn = 1
LEFT JOIN users lm_sender ON lm_sender.id = lm.sender_id
LEFT JOIN user_chat_counts cc ON cc.user_id = u.id
WHERE ($1::text = '' OR u.username ILIKE '%' || $1 || '%' OR u.email ILIKE '%' || $1 || '%')
ORDER BY lm.created_at DESC NULLS LAST, u.username ASC
LIMIT $2 OFFSET $3
```

### 3. Онлайн-статус

**Реализация:**
```go
// В server.go или server_users.go
func (s *server) GetAdminUserList(ctx context.Context, req *gen.GetAdminUserListRequest) (*gen.GetAdminUserListResponse, error) {
    // 1. Получаем пользователей из БД
    users, err := s.db.GetAdminUserList(req.Query, req.Limit, req.Offset)
    if err != nil {
        return nil, status.Error(codes.Internal, err.Error())
    }

    // 2. Добавляем онлайн-статус из хаба
    onlineUsers := s.hub.GetOnlineUserSet() // map[string]bool
    for _, u := range users {
        u.IsOnline = onlineUsers[u.Username]
    }

    // 3. Сортировка (если нужна серверная)
    switch req.SortBy {
    case "last_seen":
        sort.Slice(users, func(i, j int) bool {
            if users[i].LastSeenAt == nil { return false }
            if users[j].LastSeenAt == nil { return true }
            return users[i].LastSeenAt.AsTime().After(users[j].LastSeenAt.AsTime())
        })
    case "username":
        sort.Slice(users, func(i, j int) bool {
            return users[i].Username < users[j].Username
        })
    // default: "last_message" — уже отсортировано SQL
    }

    return &gen.GetAdminUserListResponse{
        Users:      users,
        NextCursor: nextCursor,
        HasMore:    len(users) == int(req.Limit),
        ServerTime: timestamppb.Now(),
    }, nil
}
```

**Hub метод:**
```go
// В hub.go
func (h *Hub) GetOnlineUserSet() map[string]bool {
    h.mu.RLock()
    defer h.mu.RUnlock()
    online := make(map[string]bool)
    for _, conn := range h.connections {
        if conn.username != "" {
            online[conn.username] = true
        }
    }
    return online
}
```

### 4. Cursor-based пагинация

**Cursor формат:** Base64-encoded JSON `{"last_message_time": "2026-06-29T12:00:00Z", "username": "alice"}`

**Пагинация SQL:**
```sql
-- Для следующей страницы:
WHERE ($1::text = '' OR u.username ILIKE '%' || $1 || '%')
  AND (
    lm.created_at < $4::timestamp
    OR (lm.created_at = $4::timestamp AND u.username > $5::text)
    OR lm.created_at IS NULL
  )
ORDER BY lm.created_at DESC NULLS LAST, u.username ASC
LIMIT $2
```

### 5. Обновление клиента

**Изменения в Android клиенте:**

1. **Proto:** Добавить `AdminUserInfoProto` и `GetAdminUserListRequest/ResponseProto`
2. **Marshallers:** `AdminUserInfoMarshaller`, `GetAdminUserListRequestMarshaller`, `GetAdminUserListResponseMarshaller`
3. **GrpcAIv2Client или новый GrpcAdminClient:** Метод `getAdminUserList(query, cursor, limit, sortBy)`
4. **SuperAdminActivity:** Использовать новый RPC вместо `loadAllUsers()` + `getAllChats()`
5. **SuperAdminAdapter:** Обновить `UserViewHolder` для отображения `lastMessageText`, `lastMessageTime`, `chatCount`, `isOnline`

**UI обновления:**
```
UserViewHolder (обновлённый):
├── avatarView — CircleImageView
├── nameText — username
├── versionText — "v1.3.1.05"
├── timeAgoText — "5 мин назад" (последнее сообщение)
├── lastMessageText — "Привет! Как дела?" (обрезано до 50 символов)
├── chatCountText — "12 чатов"
├── statusDot — 🟢/🔴 (онлайн/оффлайн)
└── adminBadge — 👑 (если isSuperAdmin)
```

---

## Требования

1. **Обратная совместимость:** Существующий `GetAllUsers` НЕ удалять — он используется для резолва username→UUID
2. **Производительность:** SQL запрос должен использовать индексы:
   - `messages_v2(sender_id, created_at)` — для последнего сообщения
   - `user_chat_metadata(user_id, room_id)` — для подсчёта чатов
   - `users(username)` — для поиска
3. **Безопасность:** RPC доступен только для `is_super_admin` (проверка JWT context)
4. **Пагинация:** Cursor-based (как в `GetChatsV2`), не OFFSET
5. **Обрезка текста:** `last_message_text` — максимум 100 символов + "..."
6. **Онлайн-статус:** Из хаба (реальное время), НЕ из БД (`last_seen_at` может быть устаревшим)

---

## План реализации (сервер)

1. **Прото:** Добавить `AdminUserInfo`, `GetAdminUserListRequest/Response` в `messenger.proto`
2. **Генерация:** `protoc --go_out=. --go-grpc_out=. messenger.proto`
3. **DB:** Добавить `GetAdminUserList()` в `db_users.go`
4. **Server:** Добавить `GetAdminUserList()` в `server_users.go`
5. **Hub:** Добавить `GetOnlineUserSet()` в `hub.go`
6. **Тесты:** Юнит-тесты для SQL запроса и сортировки
7. **Деплой:** Только после проверки на dev сервере

---

## План реализации (клиент)

1. **Proto:** Добавить `AdminUserInfoProto`, `GetAdminUserListRequest/ResponseProto` в `MessengerProto.kt`
2. **Marshallers:** Добавить marshallers в `GrpcMarshallers.kt`
3. **Client:** Добавить `getAdminUserList()` в `GrpcChatClient.kt` или новый `GrpcAdminClient.kt`
4. **GrpcClient facade:** Экспозиция метода
5. **SuperAdminActivity:** Заменить `loadAllUsers()` + `getAllChats()` на `getAdminUserList()`
6. **SuperAdminAdapter:** Обновить layout и ViewHolder
7. **Layout:** Обновить `item_user.xml` — добавить lastMessageText, chatCountText
8. **Strings:** Добавить строки для нового UI

---

## Схема взаимодействия

```
Android App                    Server
    |                            |
    |-- GetAdminUserList -------->|
    |   (query, cursor, limit)   |
    |                            |-- SQL: users + last message + chat count
    |                            |-- Hub: isOnline check
    |                            |-- Sort + paginate
    |<-- AdminUserInfo[] ---------|
    |   (users, next_cursor)     |
    |                            |
    |-- GetAdminUserList -------->|
    |   (cursor=next_cursor)     |
    |<-- AdminUserInfo[] ---------|
    |   (has_more=false)         |
```

---

## Тестовые сценарии

1. **Пустой список:** Нет пользователей → пустой ответ
2. **Поиск:** query="alice" → только пользователи с "alice" в username/email
3. **Пагинация:** limit=2 → 2 пользователя, next_cursor, has_more=true
4. **Сортировка:** sort_by="last_message" → активные пользователи наверх
5. **Онлайн:** Пользователь подключён → is_online=true
6. **Последнее сообщение:** Пользователь писал → last_message_text/time заполнены
7. **Без сообщений:** Пользователь никогда не писал → last_message_text=""
8. **Админ:** isSuperAdmin=true → adminBadge отображается
9. **Производительность:** 1000+ пользователей → ответ < 200ms

---

## Зависимости

- **Индекс `messages_v2(sender_id, created_at)`** — для быстрого поиска последнего сообщения
- **Индекс `user_chat_metadata(user_id, room_id)`** — для подсчёта чатов
- **Hub `GetOnlineUserSet()`** — новый метод (или адаптация существующего `GetOnlineUsers()`)
- **Прото генерация** — `protoc` с Go плагинами

---

## Риски

1. **Производительность SQL:** JOIN с `messages_v2` для 1000+ пользователей может быть медленным
   - Решение:.materialized view или кэширование в `last_message_*` полях в `users` таблице
2. **Онлайн-статус:** Хаб может не знать о всех подключённых пользователях
   - Решение: проверить `hub.go` — как хранятся соединения
3. **Cursor пагинация:** Сложная реализация с `NULLS LAST`
   - Решение: использовать простой `created_at < $cursor` без сложных OR условий

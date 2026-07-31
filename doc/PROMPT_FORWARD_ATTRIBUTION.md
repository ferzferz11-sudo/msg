# Промпт: Forward Message Attribution — Серверная часть

**Дата:** 2026-07-31 | **Ветка:** feat/1.3.0.x
**Контекст:** Android клиент v1.3.4.6 реализовал forward attribution через кодирование в тексте (`\u200B\u2709username\u200B\n` prefix). Теперь нужна серверная поддержка через proto поле.

---

## Задача

Добавить поле `forwarded_from` в proto `MessageV2` и `SendMessageV2Request`, чтобы сервер хранил и пересылал информацию об оригинальном отправителе при пересылке сообщений.

---

## Изменения

### 1. Proto — `messenger.proto`

**`SendMessageV2Request`** — добавить поле:
```protobuf
message SendMessageV2Request {
  string room_id = 1;
  oneof content {
    string text = 2;
    MessageMedia media = 3;
  }
  string reply_to_id = 4;
  bool is_e2ee = 5;
  string e2ee_payload = 6;
  repeated string mentions = 7;
  string forwarded_from = 8;  // ← НОВОЕ: username оригинального отправителя
}
```

**`MessageV2`** — добавить поле:
```protobuf
message MessageV2 {
  // ... существующие поля ...
  repeated string mentions = 40;
  string forwarded_from = 50;  // ← НОВОЕ: username оригинального отправителя
}
```

После изменения proto — перегенерировать:
```bash
PATH=$PATH:~/go/bin protoc --go_out=gen --go_opt=paths=source_relative \
  --go-grpc_out=gen --go-grpc_opt=paths=source_relative messenger.proto
```

### 2. DB Migration — `db_migrations.go`

Добавить миграцию (в `RunMigrations` или аналог):
```sql
ALTER TABLE messages_v2 ADD COLUMN IF NOT EXISTS forwarded_from TEXT DEFAULT '';
```

### 3. MessageRowV2 — `db_messages_v2.go`

Добавить поле в struct:
```go
type MessageRowV2 struct {
    // ... существующие поля ...
    ForwardedFrom string  // ← НОВОЕ
}
```

### 4. SaveMessageV2 — `db_messages_v2.go`

Добавить `forwarded_from` в INSERT запрос. Текущий INSERT:
```sql
INSERT INTO messages_v2 (id, room_id, sender_id, content_type, text, media_url, media_urls, duration, reply_to_id, reply_preview, reply_sender_id, edited, is_read, is_e2ee, e2ee_payload, reactions, mentions, created_at)
```
Нужно добавить `forwarded_from` в конец:
```sql
INSERT INTO messages_v2 (id, room_id, sender_id, content_type, text, media_url, media_urls, duration, reply_to_id, reply_preview, reply_sender_id, edited, is_read, is_e2ee, e2ee_payload, reactions, mentions, created_at, forwarded_from)
VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8, $9, $10, $11, $12, $13, $14, $15, $16::jsonb, $17, $18, $19)
```

Аналогично в `SaveMessageV2Tx` если есть.

### 5. GetMessageV2ByUUID / GetHistoryV2 — `db_messages_v2.go`

В SELECT запросах добавить `forwarded_from` и сканировать в `MessageRowV2.ForwardedFrom`.

### 6. rowToProtoV2 — `server_messages_v2.go`

Добавить в конвертацию:
```go
if r.ForwardedFrom != "" {
    m.ForwardedFrom = r.ForwardedFrom
}
```

### 7. SendMessageV2 handler — `server_messages_v2.go`

Читать `forwarded_from` из запроса:
```go
if req.ForwardedFrom != "" {
    row.ForwardedFrom = req.ForwardedFrom
}
```

---

## Порядок работы

1. Изменить `messenger.proto` → перегенерировать Go код
2. Добавить миграцию в `db_migrations.go`
3. Обновить `MessageRowV2` struct
4. Обновить `SaveMessageV2` INSERT
5. Обновить SELECT запросы (GetMessageV2ByUUID, GetHistoryV2)
6. Обновить `rowToProtoV2`
7. Обновить `SendMessageV2` handler
8. `go test ./...`
9. Деплой на dev через `./scripts/deploy-dev-local.sh`

---

## Тестирование

1. Отправить обычное сообщение — `forwarded_from` должен быть пустым
2. Переслать сообщение через Android клиент — `forwarded_from` должен содержать username
3. Проверить что `GetHistoryV2` возвращает `forwarded_from` в proto
4. Проверить что push notification не ломается

---

## Связанные клиентские изменения (v1.3.4.6)

Клиент уже реализовал:
- `Message` модель: `isForwarded`, `forwardedFrom` поля
- `ChatSelectionDelegate`: кодирует `\u200B\u2709username\u200B\n` в текст при пересылке
- `ProtoUtils`: парсит префикс из текста
- `MessageAdapter`: показывает "Переслано от: Username"
- `ChatAdapter`: `stripForwardPrefix()` для превью

После серверного изменения клиент нужно будет обновить:
- `ProtoUtils.createMessageFromV2Proto()` — читать `forwarded_from` из proto вместо парсинга текста
- `ProtoUtils.createMessageV2Proto()` — передавать `forwardedFrom` в proto
- `ChatSelectionDelegate` — убрать текстовый префикс, использовать proto поле

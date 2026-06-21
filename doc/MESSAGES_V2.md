# Messages v2 — План реализации

**Дата:** 2026-06-21 | **Версия сервера:** v1.3.0.16
**Статус:** ✅ Реализовано

---

## Цели

1. Разгрузить Message от ненужных данных (avatar, is_super_admin, auth поля)
2. Полная изоляция AI и普通ных сообщений
3. Cursor-based pagination для истории
4. JSONB reactions (batch вместо N+1)
5. Убрать denormalized replied_to_text/user

---

## Чеклист

- [x] **1. Proto** — MessageV2 + MessageMedia + MessageReply + новые RPC
- [x] **2. Regen proto** — protoc
- [x] **3. DB миграция** — messages_v2 таблица
- [x] **4. DB CRUD** — db_messages_v2.go
- [x] **5. Handlers** — server_messages_v2.go
- [x] **6. ChatV2 stream** — server_chat.go
- [x] **7. Dual-write** — запись в обе таблицы при старом стриме
- [x] **8. Tests** — unit tests (8 tests, all pass)
- [x] **9. Документация** — обновить CLIENT_INTEGRATION.md

---

## 1. Proto

### MessageV2 (core)
```protobuf
message MessageV2 {
  string id = 1;
  string room_id = 2;
  string sender_id = 3;           // UUID (вместо username)
  oneof content {
    string text = 10;
    MessageMedia media = 11;
    MessageReply reply = 12;
  }
  bool edited = 20;
  bool is_read = 21;
  google.protobuf.Timestamp created_at = 22;
  bytes reactions = 23;            // JSON: {"uuid":"emoji",...}
  bool is_e2ee = 30;
  string e2ee_payload = 31;
}
```

### MessageMedia
```protobuf
message MessageMedia {
  string type = 1;       // "image" | "voice" | "file"
  string url = 2;
  repeated string urls = 3;
  int32 duration = 4;
}
```

### MessageReply
```protobuf
message MessageReply {
  string message_id = 1;
  string preview = 2;
}
```

### Новые RPC
```protobuf
rpc GetHistoryV2(GetHistoryV2Request) returns (GetHistoryV2Response);
rpc SendMessageV2(SendMessageV2Request) returns (SendMessageV2Response);
rpc EditMessageV2(EditMessageV2Request) returns (EditMessageV2Response);
rpc DeleteMessageV2(DeleteMessageV2Request) returns (DeleteMessageV2Response);
rpc SetReactionV2(SetReactionV2Request) returns (SetReactionV2Response);
```

---

## 2. DB Schema

```sql
CREATE TABLE IF NOT EXISTS messages_v2 (
    id              VARCHAR(255) PRIMARY KEY,
    room_id         VARCHAR(255) NOT NULL,
    sender_id       UUID NOT NULL REFERENCES users(id),
    content_type    VARCHAR(20) NOT NULL,
    text            TEXT DEFAULT '',
    media_url       VARCHAR(512) DEFAULT '',
    media_urls      JSONB DEFAULT '[]',
    duration        INT DEFAULT 0,
    reply_to_id     VARCHAR(255) DEFAULT NULL,
    edited          BOOLEAN DEFAULT FALSE,
    is_read         BOOLEAN DEFAULT FALSE,
    is_e2ee         BOOLEAN DEFAULT FALSE,
    e2ee_payload    BYTEA DEFAULT NULL,
    reactions       JSONB DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_messages_v2_room_cursor
    ON messages_v2(room_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_messages_v2_reply_to
    ON messages_v2(reply_to_id) WHERE reply_to_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_messages_v2_sender
    ON messages_v2(sender_id);
```

---

## 3. Что убрано vs старый Message

| Поле | Причина удаления |
|------|-----------------|
| `user` (username) | Заменено на `sender_id` (UUID) |
| `avatar_url` | Есть в `users.avatar_url` |
| `is_super_admin` | Есть в `users.is_super_admin` |
| `replied_to_user` | JOIN из messages_v2 |
| `replied_to_text` | JOIN из messages_v2 |
| `jwt_token` | Auth в interceptor'е |
| `password` | Deprecated v1 auth |
| `device_id`, `device_name` | Auth metadata |
| `client_version` | Auth metadata |
| `register` | Deprecated v1 flag |
| `image_url`, `image_urls` | В MessageMedia |
| `voice_url`, `duration` | В MessageMedia |
| `is_read` | Оставлен (нужен для delivery receipt) |

---

## 4. AI Messages (без изменений)

AI сообщения остаются в `ai_messages_v2`:
- Таблица: `ai_messages_v2` (role, agent_id, tool_calls, token_count, model_used)
- Proto: `ChatWithAIV2Request/Response` (streaming + tool calling)
- Полная изоляция от普通ных сообщений

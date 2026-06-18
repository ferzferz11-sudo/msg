# Промт для сервера: Fix pinned_messages room_id type

## Ошибка

```
pq: operator does not exist: integer = character varying at position 4:27
```

В `GetPinnedMessages`, `PinMessage`, `UnPinMessage`, `IsMessagePinned`.

## Корень проблемы

Таблица `pinned_messages` создана с `room_id UUID NOT NULL`, но реальные chat ID в приложении — строки типа `Ebiker_ferz_direct_1781341380` (не UUID). PostgreSQL не может сравнить UUID с VARCHAR.

## Файл: db_chatlist_v2.go

### 1. Миграция — изменить тип room_id на VARCHAR

В `MigratePinnedMessages` добавить миграцию:

```go
func MigratePinnedMessages(db *sql.DB) {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS pinned_messages (
			user_id UUID NOT NULL,
			room_id VARCHAR(255) NOT NULL,
			message_id VARCHAR(255) NOT NULL,
			pinned_at BIGINT NOT NULL DEFAULT 0,
			PRIMARY KEY (user_id, room_id, message_id)
		)`,
		`DO $$ BEGIN ALTER TABLE pinned_messages ALTER COLUMN room_id TYPE VARCHAR(255) USING room_id::text; EXCEPTION WHEN duplicate_column THEN NULL; END $$`,
		`CREATE INDEX IF NOT EXISTS idx_pinned_messages_room ON pinned_messages(user_id, room_id)`,
	}

	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			if !strings.Contains(err.Error(), "already exists") && !strings.Contains(err.Error(), "already has type") {
				logger.Errorf("PinnedMessages migration error: %v", err)
			}
		}
	}
}
```

### 2. Убрать `::uuid` каст для room_id во всех запросах

**PinMessage** (line ~342):
```go
// БЫЛО:
VALUES ($1::uuid, $2::uuid, $3, $4)
// СТАЛО:
VALUES ($1::uuid, $2, $3, $4)
```

**UnPinMessage** (line ~353):
```go
// БЫЛО:
WHERE user_id = $1::uuid AND room_id = $2::uuid AND message_id = $3
// СТАЛО:
WHERE user_id = $1::uuid AND room_id = $2 AND message_id = $3
```

**GetPinnedMessages** (line ~365):
```go
// БЫЛО:
WHERE pm.user_id = $1::uuid AND pm.room_id = $2::uuid
// СТАЛО:
WHERE pm.user_id = $1::uuid AND pm.room_id = $2
```

**IsMessagePinned** (line ~392):
```go
// БЫЛО:
WHERE user_id = $1::uuid AND room_id = $2::uuid AND message_id = $3
// СТАЛО:
WHERE user_id = $1::uuid AND room_id = $2 AND message_id = $3
```

После изменений — пересобрать и задеплоить сервер.

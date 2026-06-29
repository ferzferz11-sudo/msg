# Prompt: Call Signal Routing — UUID Fix

**Дата:** 2026-06-29 | **Приоритет:** P0 | **Статус:** ✅ Исправлено

---

## Проблема

Звонки не доставляются. `BroadcastCall` на сервере не находит получателя в `callStreams` because client sends **username** as `ReceiverId`, but server stores **UUID** in `callStreams`.

**Серверный лог:**
```
[CALL] Signal: INITIATE | From: 70ebb6c0-56ca-4bb1-839e-514bab92833e (BaRon) | To: ferz (ferz) | CallID:
[CALL] INITIATE from 70ebb6c0-56ca-4bb1-839e-514bab92833e to ferz delivered: false
```

`ReceiverId = "ferz"` (username) → `callStreams` хранит UUID → нет совпадения.

---

## Анализ

### Текущий flow

1. Клиент: `initiateCall(receiverId)` → `receiverId` = username из `participantsJson`
2. Сервер: `CallSession()` → `msg.ReceiverId` = username
3. Сервер: `resolveDisplayName(db, msg.ReceiverId)` → `ReceiverName` = username (т.к. это уже username)
4. Сервер: `BroadcastCall(msg)` → ищет `callStreams` по `signal.ReceiverId` или `signal.ReceiverName`
5. `callStreams[stream]` = UUID (из `UpdateCallName(stream, currentUserId)`)
6. `"ferz" != UUID` → **не найдено** → `delivered: false`

### Что должно быть

`msg.ReceiverId` должен быть **UUID**. Тогда `BroadcastCall` найдёт совпадение с `callStreams`.

---

## Исправление (1 файл)

### `server_push.go` — sendCallPushNotification

**Строка 541-548:**

Текущий код:
```go
senderUsername := resolveDisplayName(s.db, senderName)

message := &messaging.Message{
    Token: token,
    Data: map[string]string{
        "type":      "VOIP_CALL",
        "call_id":   callId,
        "sender_id": senderUsername,
    },
```

**Проблема:** `senderName` передаётся как `sender_id` в FCM push. Если `senderName` = UUID — ок. Но в `server_chat.go:544`:

```go
s.sendCallPushNotification(msg.ReceiverId, msg.SenderName, msg.CallId)
```

`msg.SenderName` = результат `resolveDisplayName(db, msg.SenderId)` = **username** (не UUID).

**Исправление:** В FCM push `sender_id` должен быть `msg.SenderId` (UUID), а не `senderName` (username).

#### Вариант 1 (рекомендуемый): Изменить вызов в server_chat.go

```go
// server_chat.go:544 — передавать UUID, а не displayName
s.sendCallPushNotification(msg.ReceiverId, msg.SenderId, msg.CallId)
```

И в `server_push.go` убрать `resolveDisplayName`:
```go
func (s *server) sendCallPushNotification(receiverId, senderId, callId string) {
    // ... (без изменений до sender_id)
    message := &messaging.Message{
        Token: token,
        Data: map[string]string{
            "type":      "VOIP_CALL",
            "call_id":   callId,
            "sender_id": senderId,  // UUID, не username
        },
    }
```

#### Вариант 2: Добавить sender_id в push отдельно

Если `sendCallPushNotification` используется и для display name, добавить отдельный параметр:
```go
s.sendCallPushNotification(msg.ReceiverId, msg.SenderId, msg.SenderName, msg.CallId)
```

И в push data:
```go
"sender_id":   senderId,     // UUID для routing
"sender_name": senderName,   // username для отображения
```

---

## Дополнительно: клиент也需要 исправление

Клиент также отправляет username как `receiverId` в `initiateCall()`. Это основная причина `delivered: false`. Клиентское исправление в `CallManager.kt`:
- `initiateCall()`: резолвить username → UUID через `allUsers`
- Все conference методы: `getCurrentUsername()` → `getUserId()`

Но серверное исправление FCM push также необходимо — даже когда стрим-сигнал доставлен, FCM fallback отправляет username.

---

## Файлы

| Файл | Изменение |
|------|-----------|
| `server_chat.go:544` | `msg.SenderName` → `msg.SenderId` (UUID) в вызов `sendCallPushNotification` |
| `server_push.go:541-548` | Убрать `resolveDisplayName`, использовать `senderId` параметр напрямую |

---

## Тестирование

1. BaRon initiates call to ferz
2. Серверный лог: `[CALL] INITIATE from ... to ... delivered: true`
3. Ferz получает стрим-сигнал и открывает CallActivity
4. Ferz получает FCM push с `sender_id` = UUID (не username)

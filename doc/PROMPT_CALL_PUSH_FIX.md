# Prompt: Исправление FCM push для звонков

**Проблема:** При входящем звонке клиент (Xiaomi/MIUI) не получает уведомление о звонке. Сервер отправляет `[FCM SUCCESS] Call Push sent`, но device не показывает notification.

**Причина:** `sendCallPushNotification()` и `sendCallEndedPushNotification()` отправляют **data-only** FCM push — без `Notification` payload и без `AndroidNotification.ChannelID`.

Сравнение:
- **Обычные сообщения** (`sendPushNotification`, строка 92-113): `Notification{Title, Body}` + `Data` + `AndroidNotification{ChannelID: "lavender_messages", Priority: high, Sound: default}` → system notification показывается даже когда приложение убито
- **Звонки** (`sendCallPushNotification`, строка 549-559): только `Data` + `Android.Priority: high` → **без system notification**, `onMessageReceived` может не вызваться на Xiaomi

На Xiaomi MIUI data-only FCM messages без notification payload не wake'app и не показывают system notification.

## Исправление

### 1. `sendCallPushNotification()` (server_push.go:549-559)

Добавить `Notification` payload и `AndroidNotification` с channel:

```go
message := &messaging.Message{
    Token: token,
    Notification: &messaging.Notification{
        Title: "Входящий звонок",
        Body:  senderName + " звонит вам",
    },
    Data: map[string]string{
        "type":        "VOIP_CALL",
        "call_id":     callId,
        "sender_id":   senderId,
        "sender_name": senderName,
    },
    Android: &messaging.AndroidConfig{
        Priority: "high",
        Notification: &messaging.AndroidNotification{
            ChannelId: "lavender_calls",
            Priority:  "max",
            Sound:     "default",
        },
    },
}
```

### 2. `sendCallEndedPushNotification()` (server_push.go:584-594)

Аналогично — добавить `Notification` payload:

```go
message := &messaging.Message{
    Token: token,
    Notification: &messaging.Notification{
        Title: "Звонок завершён",
        Body:  "Звонок окончен",
    },
    Data: map[string]string{
        "type":      "CALL_ENDED",
        "call_id":   callId,
        "sender_id": senderId,
    },
    Android: &messaging.AndroidConfig{
        Priority: "high",
        Notification: &messaging.AndroidNotification{
            ChannelId: "lavender_calls",
            Priority:  "max",
            Sound:     "default",
        },
    },
}
```

### Важно

- Channel ID `lavender_calls` должен совпадать с тем, что создаёт клиент в `LavenderMessagingService.showCallNotification()` (канал `lavender_calls` с `IMPORTANCE_HIGH`)
- С `Notification` payload Firebase показывает system notification автоматически (даже когда приложение убито)
- `Data` payload остаётся — клиентский `onMessageReceived` по-прежнему вызывается когда приложение в foreground
- Это стандартный подход для VOIP push: notification payload для system visibility + data payload для client-side logic

### Серверная интеграция

См. `CLIENT_INTEGRATION.md` — FCM push для звонков теперь включает notification payload.

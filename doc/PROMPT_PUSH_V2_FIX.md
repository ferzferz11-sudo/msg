# Промпт: Исправление ChannelID для push-уведомлений

**Дата:** 2026-07-05 | **Приоритет:** Высокий

---

## Проблема

Push-уведомления для сообщений не отображаются на Android 8+ устройствах.

**Симптомы:**
1. Push для VOIP-звонков работает ✅ (`ChannelID: "lavender_calls"`)
2. Push для сообщений НЕ работает ❌

---

## Корень проблемы

Несоответствие ChannelID между сервером и клиентом:

| | Сервер | Клиент |
|--|--------|--------|
| Channel ID | `lavender_messages` | `lavender_messages_v2` |

**Цепочка:**
1. Android клиент создаёт notification channel с ID `lavender_messages_v2` (`LavenderMessagingService.kt:161`)
2. Android клиент удаляет старый канал `lavender_messages` (`LavenderMessagingService.kt:166-167`)
3. Сервер отправляет push с `ChannelID: "lavender_messages"` (`server_push.go:108,208`)
4. Android 8+ игнорирует уведомления для несуществующего канала

---

## Исправление

Заменить `ChannelID: "lavender_messages"` на `ChannelID: "lavender_messages_v2"` в:

- `server_push.go:108` — `sendPushNotification()`
- `server_push.go:208` — `sendMulticastWithRetry()`

---

## Тесты

```bash
go test ./... -run "Push"
```

---

## Связанные файлы

- `server_push.go:108,208` — ChannelID в sendPushNotification и sendMulticastWithRetry
- Android: `LavenderMessagingService.kt:161` — channelId = "lavender_messages_v2"

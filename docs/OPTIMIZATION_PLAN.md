# Lavender Messenger — План оптимизации и стабилизации

**Дата:** 2026-06-01
**Приоритеты:** P0 (критический) → P1 (важный) → P2 (желательный)

---

## P0 — Критичные исправления (блокируют релиз)

### 1. FCM: исправить push notifications
- [ ] Сгенерировать новый Firebase Admin SDK key в Firebase Console
- [ ] Закинуть на сервер в `/root/LavenderMessenger/run/`
- [ ] Перезапустить `systemctl restart lavender-server`
- [ ] Протестировать push через сообщение

*Примечание: FCM ключ создаётся пользователем, ожидание.*

### 2. Сервер: добавить HANGUP при abrupt disconnect
- **Файл:** `server.go`, функция `handleAbruptDisconnect`
- **Проблема:** когда клиент теряет соединение, собеседник не получает HANGUP
- **Решение:** отправить HANGUP всем активным звонкам с этим user

### 3. Сервер: добавить TURN сервер для WebRTC
- **Проблема:** только Google STUN, NAT traversal не работает
- **Решение:** добавить coturn на сервер или использовать публичный TURN

### 4. Android: исправить receiverId/username в call signaling
- **Файл:** `CallActivity.kt`, `CallManager.kt`
- **Проблема:** путаница между username и UUID в call signals
- **Решение:** всегда использовать UUID для senderId/receiverId

---

## P1 — Важные исправления (стабильность)

### 5. Android: добавить WebRTC connection timeout
- **Файл:** `CallActivity.kt`, `WebRtcClient.kt`
- **Решение:** 30s timeout → auto hangup

### 6. Android: ICE connection state handling
- **Файл:** `WebRtcClient.kt`
- **Решение:** `DISCONNECTED` → показать ошибку, `FAILED` → retry

### 7. Android: VOIP_CALL FCM handling улучшение
- **Файл:** `LavenderMessagingService.kt`
- **Решение:** high priority FCM + ensure gRPC соединение

### 8. Сервер: reconnect после keepalive failure
- **Файл:** `RealGrpcClient.kt` (Android), `server.go`
- **Решение:** silent reconnect + state restore

---

## P2 — Желательные улучшения

### 9. Сервер: рефакторинг server.go → пакеты
### 10. Сервер: rate limiting
### 11. Сервер: graceful shutdown
### 12. Сервер: structured logging
### 13. UI: OWL chat keyboard fix
### 14. UI: дублирование сообщений (проверить)

---

## Порядок выполнения

| # | Задача | Файл(ы) | Сложность |
|---|--------|---------|-----------|
| 1 | FCM key update | server | Зависит от пользователя |
| 2 | HANGUP при disconnect | server.go | Низкая |
| 3 | TURN сервер | server + WebRTC | Высокая |
| 4 | receiverId fix | CallActivity.kt | Средняя |
| 5 | WebRTC timeout | CallActivity.kt | Низкая |
| 6 | ICE state handling | WebRtcClient.kt | Средняя |
| 7 | VOIP FCM | LavenderMessagingService.kt | Средняя |
| 8 | Reconnect fix | RealGrpcClient.kt | Высокая |

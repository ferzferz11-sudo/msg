# Lavender Messenger — Журнал изменений

**Дата:** 2026-06-02
**Ветка:** feat/1.1.0.x
**Версия:** 1.1.0.9

---

## Сервер (Go)

### Добавлено

#### TURN сервер (coturn) для WebRTC
- Установлен и настроен coturn на сервере
- Порт 3478 (TCP/UDP), relay порты 10000-20000 (UDP)
- HMAC-based временные креденшалы черезTURN REST API
- Endpoint `/turn-credentials` на HTTP порту — выдаёт iceServers с username/password
- Конфиг: `/etc/turnserver.conf`

#### HTTP server — /turn-credentials endpoint
- GET `/turn-credentials` возвращает `{ iceServers: [{ urls, username, credential }] }`
- Username = timestamp + TTL (24h), password = HMAC-SHA1(username, secret)
- **Файл:** `http_server.go`

### Исправлено

#### OpenRouter API ключ
- Обновлён ключ в `.env` — исправлена ошибка 401 (User not found)
- OWL AI снова работает

---

## Android клиент (Kotlin)

### Исправлено

#### WebRtcClient — убран дубликат PeerConnection.Observer
- **Было:** Дубликат `PeerConnection.Observer` в `initPeerConnection` — код вне метода
- **Стало:** Убраны 24 строки дублирования
- **Результат:** Исправлены ошибки компиляции (Unresolved reference: iceCandidateQueue, peerConnection)

#### CallActivity — получение TURN креденшалов
- Добавлен `fetchTurnCredentials()` — асинхронный GET запрос к `/turn-credentials`
- `initPeerConnection()` теперь принимает динамические ICE серверы (TURN + STUN)
- Fallback на STUN при недоступности TURN
- **Файл:** `CallActivity.kt`

---

## Известные проблемы (остаются)

1. ~~**FCM сломан**~~ → ✅ Исправлен (обновлён firebase key)
2. ~~**Нет TURN сервера**~~ → ✅ Установлен coturn
3. **ChangelogActivity белый экран** — требует logcat для диагностики

---

## Предыдущие изменения (2026-06-01)

### Исправлено

#### handleAbruptDisconnect — теперь отправляет HANGUP
- **Было:** функция была пустой — только логирование
- **Стало:** при разрыве соединения (context canceled / transport closing):
  - Находит все активные/ожидающие звонки пользователя в БД
  - Отправляет HANGUP сигнал собеседнику через call stream
  - Сохраняет системное сообщение "Соединение потеряно" в чат
  - Обновляет статус звонка на "completed"
- **Файлы:** `server.go`, `db.go`

#### GetActiveCallsByUser — новый метод в DB
- Возвращает все активные/ожидающие звонки для пользователя
- Используется в handleAbruptDisconnect
- **Файл:** `db.go`

#### BroadcastCall — fallback по username
- **Было:** поиск call stream только по ReceiverId (UUID)
- **Стало:** поиск по ReceiverId ИЛИ ReceiverName (username)
- Обеспечивает совместимость если клиент отправляет username вместо UUID
- **Файл:** `hub.go`

---

## Android клиент (Kotlin)

### Исправлено

#### CallManager.sendWebRtcSignal — UUID для senderId
- **Было:** `GrpcClient.getCurrentUsername()` — использовался username
- **Стало:** `GrpcClient.getUserId()` с fallback на username
- Сервер теперь получает корректный UUID для routing
- **Файл:** `CallManager.kt`

#### CallActivity — WebRTC connection timeout
- **Было:** звонок висел бесконечно если партнёр не ответил
- **Стало:** 30 секунд timeout для исходящих звонков
  - Показывается Toast "Не удалось соединиться"
  - Автоматический hangup и finish
  - Timeout отменяется при успешном соединении
- **Файл:** `CallActivity.kt`

#### CallActivity — ICE connection state monitoring
- **Было:** `onIceConnectionChange` только логировался
- **Стало:** полная обработка состояний:
  - `CONNECTED/COMPLETED` → отмена timeout, обновление статуса
  - `FAILED` → ошибка + автоматический hangup
  - `DISCONNECTED` → логирование (может восстановиться)
- **Файлы:** `CallActivity.kt`, `WebRtcClient.kt`

#### WebRtcClient — onIceConnectionStateChange callback
- Добавлен callback для внешней обработки ICE состояний
- **Файл:** `WebRtcClient.kt`

---

## Известные проблемы (остаются)

1. **FCM сломан** — Invalid JWT Signature. Требуется новый Firebase key.
2. **Нет TURN сервера** — WebRTC не работает через NAT (мобильные сети)
3. **Сервер версия** — нужно обновить ServerVersion до 1.1.0.8
4. **iOS клиент** — многие методы stubs (не завершён)

---

## Что нужно сделать дальше

1. [ ] FCM: получить новый Firebase key от пользователя
2. [ ] TURN сервер: установить coturn или использовать публичный
3. [ ] Тестирование звонков с двумя устройствами
4. [ ] Обновить сервер на production (scp + restart)

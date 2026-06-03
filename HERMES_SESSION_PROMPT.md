# Hermes Orchestrator — Следующая сессия

## Цели (в порядке приоритета)

### 1. Диагностика и исправление gRPC соединения Android → Dev Server

**Симптомы:**
- `UNAVAILABLE: Channel shutdownNow invoked` на Android
- `connect() called: addr=13.140.25.249:50052 force=true status=FAILED`
- `getChats: onClose error: UNAVAILABLE` — повторяется каждые 5 секунд
- `Chat stream already active for ferz21 in ferz_ferz21_direct_1780517430, skipping restart` — Android думает что стрим активен, но соединение мертво

**Что проверить:**
1. Может ли телефон достучаться до сервера: `adb shell ping -c 3 13.140.25.249`
2. Работает ли HTTP с телефона: `adb shell curl -v http://13.140.25.249:8083/`
3. Возможно проблема с HTTP/2 — gRPC требует HTTP/2, а обычный HTTP/1.1 прокси может блокировать
4. Проверить нет ли прокси/VPN на телефоне который блокирует gRPC
5. Проверить что `usePlaintext()` вызывается ДО `build()` в RealGrpcClient
6. Возможно нужно добавить keepalive параметры в ManagedChannelBuilder

**Файлы:**
- Android: `RealGrpcClient.kt` — создание gRPC канала
- Сервер: `server.go` — `grpc.NewServer()` без TLS

### 2. Управление агентами — шторка настроек (AgentSettingsBottomSheet)

**Требование:** Тап по тулбару с аватаркой Hermes → открывается шортлист/шторка с настройками агента.

**Что сделать:**
1. Найти где в коде обрабатывается тап по тулбару HermesChatActivity
2. Проверить что `AgentSettingsBottomSheet.kt` существует и правильно реализован
3. Привязать тап по аватарке/тулбару к показу AgentSettingsBottomSheet
4. Убедиться что шторка содержит:
   - Список агентов с возможностью выбора
   - Настройки выбранного агента (имя, модель, системный промпт, температура и т.д.)
   - Кнопку сохранения

**Файлы:**
- `HermesChatActivity.kt` или `HermesChatFragment.kt` — обработка тапа по тулбару
- `AgentSettingsBottomSheet.kt` — реализация шторки
- Layout файлы для шторки

### 3. Welcome Message

**Требование:** При создании сессии Hermes показывает welcome message с инструкциями.

**Текущее состояние:** `buildWelcomeMessage()` написана в `server.go`, но не протестирована.

**Что проверить:**
1. Вызывается ли `buildWelcomeMessage()` в `ChatWithOrchestrator`
2. Отправляется ли welcome message как первое сообщение в стриме
3. Правильно ли Android отображает входящие сообщения от сервера

## Контекст

### Сервер
- Dev: `lavender-server-dev.service`, порт 50052 (gRPC), 8083 (HTTP)
- Бинарник: `/root/msg/server/lavender-server`
- Ветка: `main`
- База: `chat_db_dev`
- `go build` проходит успешно

### Android
- Проект: `/root/msg.client.android`
- Ветка: `master`
- APK: debug, собран локально (НЕ на сервере — OOM)
- Пользователь на dev: `ferz21`
- Адрес сервера: `13.140.25.249:50052`

### Известные проблемы
- Proto mismatch (возможно уже исправлено): Android отправлял `agentId` как tag 2, сервер ожидает `name` как tag 2 в `CreateHermesSessionRequest`
- `name` колонка добавлена в `hermes_sessions` таблицу
- `streamOpenRouter` мигрирован на callback API

### Что НЕ делать
- НЕ собирать release APK на сервере (OOM kill)
- НЕ перезапускать prod сервер
- НЕ менять prod базу данных

## Подход

1. **Сначала диагностика** — понять почему gRPC не соединяется (ping, curl с телефона)
2. **Затем исправление** — в зависимости от результата диагностики
3. **Потом UI** — управление агентами через шторку
4. **В конце** — welcome message

Не торопиться, делать по одному шагу за раз, проверять результат каждого шага.

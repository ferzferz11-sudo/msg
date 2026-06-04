# Hermes Orchestrator — Ночная сессия (03-04 июня 2026)

## Статус на начало сессии

### Что сделано (вечер 03.06)
- Dev сервер v1.1.0.13-dev собран и деплоен (753c1a5, 22b7b8f)
- Welcome message: +Finished:true
- CreateHermesSession: fallback name="Hermes"
- getOrCreateSession: персистит в hermes_sessions
- gRPC keepalive настроен на сервере (20s/20s)
- Push origin main OK
- Prod сервер НЕ тронут

### Открытые проблемы
1. **gRPC соединение Android → Dev Server** — телефон рвёт соединение (UNAVAILABLE, context canceled)
2. **AgentSettingsBottomSheet** — есть в коде, но не привязан к тапу по тулбару/аватарке Hermes
3. **Proto mismatch** — Android createHermesSession сериализует agentId как tag 2 вместо name

## Задачи на ночь (в порядке приоритета)

### 1. Диагностика gRPC соединения Android → Dev Server

**Симптомы:**
- `UNAVAILABLE: Channel shutdownNow invoked`
- `connect() called: addr=13.140.25.249:50052 force=true status=FAILED`
- `getChats: onClose error: UNAVAILABLE` — повторяется каждые 5 секунд
- `Chat stream already active for ferz21` — Android думает что стрим активен, но соединение мертво

**Шаги диагностики:**
1. Проверить что dev сервер работает: `systemctl status lavender-server-dev`
2. Проверить порты: `ss -tlnp | grep -E "50052|8083"`
3. Проверить firewall: `iptables -L INPUT -n | grep 50052`
4. Проверить логи: `journalctl -u lavender-server-dev --since "10 min ago" --no-pager`
5. Если есть доступ к телефону через adb:
   - `adb shell ping -c 3 13.140.25.249`
   - `adb shell curl -v http://13.140.25.249:8083/`

**Возможные причины и решения:**
- HTTP/2 прокси/VPN на телефоне блокирует gRPC → попробовать без VPN
- `usePlaintext()` не вызывается до `build()` в RealGrpcClient.kt → исправить порядок
- Нет keepalive на клиенте → добавить `.keepAliveTime(30, TimeUnit.SECONDS)` в ManagedChannelBuilder
- Таймаут подключения слишком короткий → увеличить

**Файлы для проверки (Android — локально у пользователя):**
- `RealGrpcClient.kt` — создание gRPC канала
- `HermesChatActivity.kt` или `HermesChatFragment.kt` — вызов ChatWithOrchestrator

### 2. Управление агентами — AgentSettingsBottomSheet

**Требование:** Тап по тулбару с аватаркой Hermes → открывается шторка с настройками агента.

**Что сделать:**
1. Найти где обрабатывается тап по тулбару HermesChatActivity/HermesChatFragment
2. Проверить что AgentSettingsBottomSheet.kt существует и правильно реализован
3. Привязать тап по аватарке/тулбару к показу AgentSettingsBottomSheet
4. Шторка должна содержать:
   - Список агентов с возможностью выбора
   - Настройки выбранного агента (имя, модель, системный промпт, температура)
   - Кнопку сохранения

**Файлы (Android — локально):**
- `HermesChatActivity.kt` или `HermesChatFragment.kt`
- `AgentSettingsBottomSheet.kt`
- Layout файлы для шторки

### 3. Welcome Message — проверка

**Текущее состояние:** `buildWelcomeMessage()` вызывается в `ChatWithOrchestrator`, отправляется с `Finished: true`.

**Что проверить:**
1. После исправления gRPC — убедиться что welcome message приходит на Android
2. Проверить что Android корректно отображает markdown (жирный текст, списки)
3. Если markdown не поддерживается — упростить формат

### 4. Улучшение оркестратора (если время позволяет)

**Идеи:**
- Добавить сохранение истории сообщений оркестратора в БД (сейчас только в памяти)
- Добавить endpoint для получения списка агентов с их статусом
- Добавить возможность переключения агента через команду `@agentname`
- Улучшить промпт оркестратора — более точная маршрутизация

## Контекст

### Сервер
- Dev: `lavender-server-dev.service`, порт 50052 (gRPC), 8083 (HTTP)
- Бинарник: `/root/LavenderMessenger/run/lavender-server-dev`
- Исходники: `/root/msg/`
- Ветка: `main`
- База: `chat_db_dev`
- Deploy: `cd /root/msg && go build -o /tmp/lavender-server-dev . && systemctl stop lavender-server-dev && cp /tmp/lavender-server-dev /root/LavenderMessenger/run/lavender-server-dev && systemctl start lavender-server-dev`

### Android
- Проект: локально у пользователя (не на сервере!)
- Ветка: `master`
- APK: debug, собирать локально (НЕ на сервере — OOM)
- Пользователь на dev: `ferz21`
- Адрес сервера: `13.140.25.249:50052`

### Что НЕ делать
- НЕ собирать release APK на сервере (OOM kill)
- НЕ перезапускать prod сервер
- НЕ менять prod базу данных
- НЕ пушить в main без тестирования на dev

## Подход

1. Сначала диагностика gRPC — понять почему не соединяется
- Если проблема на стороне сервера — исправить, пересобрать, деплоить
- Если проблема на стороне Android — написать пользователю инструкцию
2. Затем улучшения оркестратора
3. В конце — welcome message и UI

Не торопиться, делать по одному шагу за раз.

# Лава — Pitfalls

Подводные камни и известные проблемы. Читать перед началом работы!

**Обновлено:** 2026-06-14

---

## Android

### Server switch — двойной вход (v1.1.3.11 — исправлено)
- **Анти-pattern:** `CredentialStore.setServerAddress()` до `SessionManager.login()` в ServersActivity
- **Симптом:** 3 входа подряд при смене сервера (лог: ferz11→dev, ferz→dev, ferz→prod)
- **Причина:** преждевременное сохранение serverAddress → ChatListActivity видит новый сервер → auto-login → onResume reconnect
- **Правило:** сохранять serverAddress ТОЛЬКО после успешного входа (SUCCESS callback)
- **Правило:** НЕ делать auto-login в serversActivityLauncher — пользователь уже вошёл
- **Правило:** использовать флаг `justReturnedFromServersActivity` для пропуска reconnect в onResume

### SplashActivity
- `UserSession`, не `Session` (`data/session/UserSession.kt`)

### StandardBottomSheet
- `root` — `MaterialCardView`, не `LinearLayout`

### Bottom sheet items
- `bg_action_item_hover.xml` заменяет `selectableItemBackground`

### AccelerateDecelerateInterpolator
- `android.view.animation`, НЕ `androidx`

### ValueAnimator
- Всегда вызывайте `cancel()` перед запуском нового (TypingHolder leak)

### Favorites — отображение при пустом списке чатов
- **Статус:** исправлено в v1.1.2.8
- Favorites показывается сразу в onCreate(), до сетевых запросов
- Fallback при ошибке загрузки чатов — Favorites всё равно отображается
- Паттерн: статический first item должен быть добавлен ДО загрузки данных с сервера

### Favorites flickering
- **Статус:** исправлено в v1.1.2.8 (c873fbc), подтверждено в v1.1.3.7
- `sendSync()` передавал list без favoritesItem, вызывая remove/insert каждые 5с
- Паттерн: статический first item в RecyclerView должен быть ВКЛЮЧЁН во все background updates
- Решение: Favorites вынесен как отдельный `favoritesItem` в ChatAdapter, не участвует в DiffUtil
- DiffUtil работает только с actualChats, все notify* вызовы смещены на +1 для Favorites
- `startSync()` при сравнении изменений использует offset `i + 1` для Favorites

### ChangelogActivity — bundled changelog
- **changelog.txt УДАЛЁН** из проекта и из деплоя на сервер (v1.1.2.6)
- Вместо него: `app/src/main/assets/changelog_bundled.txt` — встроен в APK, показывается мгновенно
- При каждом релизе: обновлять `assets/changelog_bundled.txt` вместе с `CHANGELOG.md`
- Формат: emoji-заголовки, буллеты `—`, секции по версиям
- Если bundled не обновлён — пользователь увидит устаревший ченджлог из APK
- **Цвета в fallback**: устанавливаются программно из `ThemeStore` (не через XML-атрибуты)

### ChangelogAdapter — цвета на кастомных темах
- **Статус:** исправлено в v1.1.2.8
- Использует `ThemeStore` цвета вместо `resolveColorAttr`

### HermesGrpc proto mapping
- `CreateHermesSessionResponse`: field 1=success(bool), field 2=session_id(string) — НЕ наоборот!
- `CreateAgentResponse`: field 1=success(bool), field 2=agent_id(string)
- `AgentInfo`: field 4=is_preset(bool), 5=system_prompt(string), 6=model(string)

---

## Server

### Структура файлов (v1.1.2.10+)
- server.go — структура server, общие методы (logErrorOnce, logFCM, resolveUserId, resolveUsername)
- server_*.go — методы по доменам (chat, users, chats, messages, profile, push, contacts, themes, drafts, muted, favorites, ai)
- При добавлении новых методов — класть в соответствующий server_*.go файл
- Не добавлять методы напрямую в server.go (только структура и общие утилиты)

### hermes_sessions owner
- Таблица должна принадлежать `lavender`, не `postgres`
- Исправление: `cd /tmp && sudo -u postgres psql -d chat_db -c "ALTER TABLE hermes_sessions OWNER TO lavender;"`

### getOrCreateSession создаёт дубли
- Старое поведение: создавал сессию с `id = "hermes-" + userID` каждый раз
- Исправлено: ищет существующую сессию по `user_id`

### JSON в SQL
- Никогда не собирайте JSON через конкатенацию: `"["+username+"]"` → невалидный JSON
- Всегда `json.Marshal`

### DeleteChat и AI чаты
- DeleteChat удаляет из chats, но НЕ из hermes_sessions (orphaned sessions копятся)
- Исправлено в v1.1.2.2: каскадное удаление из hermes_sessions + hermes_messages
- Для OWL: FK CASCADE на owl_messages срабатывает, но owl_chat_settings — нет, нужно явное удаление

### Prod vs Dev
- Dev: порт 50052, DB `chat_db_dev`, config `.env.dev`
- Prod: порт 50051, DB `chat_db`, config `.env`
- Версия сервера в `server.go:33`

### Hermes история — всегда из БД
- `GetOrchestratorHistory` должен загружать из `hermes_messages` через `HermesDB.GetOrchestratorHistory()`
- НЕ использовать `session.Messages` (in-memory) — пропадает после рестарта сервера
- `getOrCreateSession` создаёт пустую сессию без загрузки истории из БД

### Rate limiter — refund on failure
- `allow()` добавляет timestamp ДО выполнения запроса
- При ошибке (OpenRouter, orchestrator) timestamp остаётся — слот потерян
- **Правило:** всегда вызывать `cancel(userID)` в failure path после успешного `allow()`
- `remaining()` возвращает `limit - len(valid)` — корректно отражает оставшиеся запросы

### /dev/null сломан после OOM
- Если `/dev/null` стал файлом вместо device node: `rm /dev/null && mknod /dev/null c 1 3 && chmod 666 /dev/null`
- Без этого `go build` падает с "open /dev/null: no such file or directory"

---

## Android — паттерны и анти-patterns

### ChatAdapter filter() с Favorites
- **Анти-pattern:** `notifyItemRangeChanged(1, filtered.size)` — не обновляет размер списка → crash при уменьшении
- **Правило:** Использовать `diffResult.dispatchUpdatesTo()` с `ListUpdateCallback` и offset +1 для Favorites
- Паттерн аналогичен `setChats()` — см. ChatAdapter.kt

### ChatAdapter empty chat text (v1.1.3.9)
- **Анти-pattern:** показывать `favorites_description` для всех пустых чатов
- **Правило:** `favorites_description` только для `chat.type == "favorites"`, остальные пустые чаты → `no_messages` ("No messages" / "Нет сообщений")
- Пустой `lastMessageText` ≠ Favorites, всегда проверяйте `chat.type`

### ChatAdapter Favorites offset
- Favorites всегда на position 0, не участвует в DiffUtil
- Все notify* вызовы смещены на +1 для Favorites
- `getItemCount()` = displayedChats.size + 1 (если Favorites есть)

### ChatWidget reuse
- Обязательно: TextWatcher для send button visibility, commandButton listener, hide internal toolbar, auto-scroll
- Без TextWatcher send button не появляется/исчезает при вводе

### Error Handling
- Все Toast ошибки дублировать в AppLog.error()
- CancellationException → AppLog.info() (не ERROR), re-throw, НЕ показывать toast
- "Job was cancelled" — CancellationException в ViewModelScope, обрабатывать отдельно

### Темы
- НЕ использовать `?attr/` в XML для текста на кастомных тёмных темах
- Цвета устанавливать программно через ThemeUtils.parseSafeColor()
- Новые FAB добавлять в ThemeApplier

### Сборка
- НЕ компилировать на сервере (OOM kill)
- assembleRelease — ТОЛЬКО локально

---

## Пути к репозиториям

### Android — отдельный репозиторий
- **Верный путь:** `/root/msg.client.android/` (отдельный git-репозиторий)
- **НЕВЕРНО:** `/root/msg/client.android/` — такой папки нет, не использовать!
- Android-репозиторий клонирован отдельно от серверного, не является подмодулем

---

## JWT Agent Auth

### Секретный ключ
- `JWT_SECRET` — минимум 32 байта, хранится в `.env` / `.env.dev`
- Никогда не коммитить в git!
- При компрометации — немедленно перегенерировать все токены

### Валидация токена
- `validateToken()` проверяет: HMAC подпись, expiration, agent_id match, revoked в БД
- Пустой токен = отклонение (нет backward compat с неавторизованными агентами)
- Для тестирования без токена — нужно явно создать токен через `GenerateAgentToken`

### Хранение
- В БД хранится только SHA-256 хеш токена, не сам токен
- Токен показывается клиенту только один раз при генерации
- `RevokeAgentToken` — помечает `revoked = TRUE`, существующие подключения продолжают работать до реконнекта

### Admin-only
- `GenerateAgentToken`, `RevokeAgentToken`, `ListAgentTokens` — требуют `IsSuperAdmin()`
- `admin_user_id` в запросе должен совпадать с супер-админом в БД

---

## Remote Agent (v1.1.3.0)

### DeployAgentTaskStream — стриминг результатов (v1.1.3.10 — WORKING)

**Поток данных:**
```
Агент → AGENT_TASK_STREAM_UPDATE(done=False) → onStream → streamCh → клиент
Агент → AGENT_TASK_STREAM_UPDATE(done=True)  → streamDone flag, continue (НЕ отправляем клиенту)
Агент → AGENT_TASK_RESULT                    → onResult → close(streamCh)
Сервер → клиент: один done=True с полными Stdout/Stderr/ExitCode/DurationMs
```

**Правила:**
- При `done=True` от агента — НЕ отправлять `done=True` клиенту сразу
- Ставить флаг `streamDone = true`, `continue`
- После закрытия `streamCh` (от `onResult`) — отправить **один** `done=True` с полными данными
- НЕ использовать таймаут ожидания TaskResult — ждать через закрытие канала

**Анти-pattern (v1.1.3.7):**
- Двойной `done=True` — первый пустой, второй полный → клиент видит мерцание

### Token RPC маршрутизация

- `GenerateAgentToken`, `RevokeAgentToken`, `ListAgentTokens` — методы `HermesAgentService` (hermes_remote.proto), НЕ `ChatService`
- Полное имя: `hermes_agent.HermesAgentService/GenerateAgentToken`
- В Android HermesGrpc.kt используйте правильный сервис

### IsSuperAdmin убран (v1.1.3.0)

- Начиная с v1.1.3.0, token RPC доступны любому авторизованному пользователю
- Remote agents запускаются на сервере пользователя, не на нашем
- Пользователь сам управляет своими токенами

### Токен не появляется в списке

**Симптом:** После генерации токена список остаётся пустым
**Причины:**
1. `JobCancellationException` — корутина отменяется до завершения gRPC вызова
2. `ListAgentTokens` возвращает пустой список (токен не сохраняется в БД)
3. `hermesDB.SaveAgentToken()` возвращает ошибка

**Отладка:**
- Android: `adb logcat -s "RemoteAgentSettings" "HermesGrpc"`
- Сервер: `journalctl -u lavender-server-dev -f | grep "HermesAgentService"`

### CancellationException в lifecycleScope

- `lifecycleScope.launch` отменяется при уничтожении Activity
- `CancellationException` НЕ должен ловиться как обычный `Exception`
- Всегда используйте отдельный `catch (e: CancellationException)` с `throw e`

### writeRawVarint32 deprecated

- `CodedOutputStream.writeRawVarint32()` deprecated в protobuf 4.x
- Замените на `writeUInt32NoTag()` для length-delimited полей
- Для tag+value: `writeUInt32(fieldNumber, value)`

---

## ThemeApplier

### FAB кнопки
- Новые FAB кнопки добавлять в список в `ThemeApplier.kt`:
  `listOf(R.id.aiFab, R.id.addChatFab, R.id.addContactFab, R.id.addThemeFab)`
- ThemeApplier устанавливает `backgroundTintList=customPrimary` и `imageTintList=customOnPrimary`
- Без этого FAB остаётся default `colorSecondaryContainer`

---

## Dev Server Management

### Systemd service
- **Файл:** `/etc/systemd/system/lavender-server-dev.service`
- **НЕ редактировать напрямую** — использовать `sudo tee`:
  ```bash
  sudo tee /etc/systemd/system/lavender-server-dev.service > /dev/null << 'EOF'
  [Unit]
  Description=Lavender Messenger Server — DEV
  After=network.target postgresql.service
  Wants=postgresql.service

  [Service]
  Type=simple
  WorkingDirectory=/root/LavenderMessenger/run
  ExecStart=/root/LavenderMessenger/run/lavender-server-dev
  Restart=always
  RestartSec=5
  Environment=APP_ENV=dev

  [Install]
  WantedBy=multi-user.target
  EOF
  ```
- После изменения: `sudo systemctl daemon-reload && sudo systemctl restart lavender-server-dev`

### Environment files
- **Dev config:** `/root/LavenderMessenger/run/.env.dev` — загружается автоматически при `APP_ENV=dev`
- **Prod config:** `/root/LavenderMessenger/run/.env`
- **НЕ коммитить** .env файлы — содержат секреты (JWT_SECRET, DB credentials, API keys)
- Формат: `KEY=value` (без кавычек, без пробелов вокруг `=`)

### Common issues
- **`missing port in address`** — значит `SERVER_ADDRESS` не загрузился из .env. Проверить:
  1. `APP_ENV=dev` установлен в systemd service
  2. `.env.dev` существует и содержит `SERVER_ADDRESS=0.0.0.0:50052`
  3. Нет старого `Environment=SERVER_ADDRESS=***` в systemd service
- **Panic после `failed to listen`** — баг в main.go: нет `return` после ошибки `net.Listen`. Исправлено в v1.2.0.1+
- **Text file busy** при копировании бинарника — сначала `systemstctl stop`, потом `kill -9 <PID>`, потом копировать

### Server info endpoint
- `GET http://host:8082/info` — возвращает версии сервисов
- Используется Android клиентом для capability negotiation
- `services.auth >= "2.0"` → JWT workflow, иначе legacy

---

## Сессия 8 — новые подводные камни

### BearerTokenInterceptor — совместимость с v1 серверами
- Интерцептор является no-op если `AuthManager.getBearerToken() == null`
- При подключении к v1 серверу (без JWT) — токен не подставляется, запросы идут без Bearer
- Chat stream пропускается интерцептором (legacy auth с password)
- **Анти-pattern**: не добавлять Bearer token в Chat stream — сервер не ожидает его

### UNIQUE constraint на user_devices
- `CREATE TABLE IF NOT EXISTS` НЕ добавляет UNIQUE constraint к существующей таблице
- На prod сервере таблица `user_devices` может существовать без UNIQUE constraint
- Ошибка 42P10: `pq: there is no unique or exclusion constraint matching the ON CONFLICT specification`
- **Решение**: миграция через `ALTER TABLE ... ADD CONSTRAINT ... UNIQUE (user_id, device_id)`
- Добавлено в db_auth_devices.go, но требует ручного выполнения на prod БД

### getChats() callback при ошибке
- Если канал мёртв — `getChats()` возвращается без вызова callback → корутина зависает
- Исправлено: `onClose` вызывает `callback(emptyList())` при ошибке
- `loadChats()` обёрнут в `withTimeoutOrNull(10с)` для предотвращения зависания

### EncryptedSharedPreferences.edit()
- `EncryptedSharedPreferences.edit()` НЕ принимает лямбду (в отличие от `androidx.core.content.edit`)
- **Правило**: использовать `edit().putXxx().apply()` или `edit().remove().apply()`
- **Анти-pattern**: `edit { putString(...) }` — не компилируется

### Первый вход на prod — только Favorites
- Проблема в локальном кеше Android — после очистки всё ОК
- Не является багом нового кода
- `loadChats()` может не вызваться если `connectionStatus` не успел перейти в READY

---

## Сессия 11 — новые подводные камни

### ChatStream v2 — JWT auth в Chat stream
- На v2 серверах (chat >= "2.0") Chat stream использует `jwt_token` вместо `password`
- Если у клиента нет JWT токена — fallback на password auth (полная совместимость)
- BearerTokenInterceptor НЕ пропускает Chat stream на v2 серверах (token в первом сообщении)
- **Анти-pattern**: не проверять `isChatV2Supported()` перед отправкой jwt_token

### ChatList v2 — per-user chat metadata
- user_chat_metadata таблица хранит pinned/archived статус для каждого пользователя
- `pinned_at` — timestamp для сортировки закреплённых чатов
- **Анти-pattern**: не использовать `chats.is_pinned` напрямую — это per-user поле в user_chat_metadata

### Kotlin 2.3.21 — CancellableContinuation.resume()
- `CancellableContinuation.resume(value)` требует `onCancellation = {}` параметр
- **Правило**: всегда `cont.resume(value, onCancellation = {})`
- **Анти-pattern**: `cont.resume(value)` — deprecated warning, может не компилироваться

### fetchServerInfo fallback
- Если /info недоступен — все версии пустые → v1 fallback для ВСЕХ сервисов
- **Правило**: проверять `isXxxSupported()` перед использованием v2 API
- **Анти-pattern**: использовать v2 API без проверки версии сервера

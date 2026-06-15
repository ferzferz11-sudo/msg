# Lava Messenger — Интеграционная сессия

**Текущая версия:** v1.2.1.0 (сервер dev) / v1.1.3.13 (Android)
**Обновлено:** 2026-06-15 (сессия 10)
**Тег:** v1.1.3.10 (stable prod)
**Ветка:** feat/1.1.3.x

---

## Сессия 10 — Документация + ProfileClient fixes

### Что сделано

1. **ProfileClient fixes (Android)** — исправлены все проблемы после первоначального PR
   - `unaryCall()` — единообразное использование
   - Inline Marshaller objects (deprecation fix)
   - Недостающие imports для ProfileV2 proto classes
   - ProtoMarshaller сделан internal

2. **Документация актуализирована** — TASKS.md, PROMPT.md, PROMPT_ANDROID.md, SESSION_NOTES.md
   - Версии: сервер v1.2.1.0, Android v1.1.3.13
   - Индексы обновлены

### Коммиты (Android)
- `7782993` — fix: ProfileClient — use unaryCall consistently
- `73da2e1` — fix: use inline Marshaller objects
- `d707fa8` — fix: add missing imports for ProfileV2 proto classes
- `1a73dee` — fix: suppress newInstance deprecation warning

---

## Сессия 9 — ProfileService v2 + Typing/CallSession compat

### Что сделано

1. **ProfileService v2 (сервер)** — отдельный gRPC сервис для управления профилем с JWT Bearer auth
   - Методы: GetProfile, UpdateProfile, UpdateAvatar, DeleteProfile, GetUserSettings, UpdateUserSettings
   - Данные: аватар, bio, status, locale (en/ru), isSuperAdmin, theme, push settings
   - Регистрируется ТОЛЬКО на dev сервере (APP_ENV=dev)
   - ProfileServiceVersion = "2.0" в /info endpoint
2. **user_settings таблица** — новая таблица для хранения настроек пользователя (locale, theme_id, push_enabled, custom JSONB)
3. **Typing/CallSession whitelist** — добавлены в AuthStreamInterceptor как legacy streams (v1 compat)
   - Теперь v1 клиенты могут вызывать Typing и CallSession без JWT
4. **ProfileClient (Android)** — клиент для ProfileService v2 с автоопределением версии сервера через /info
   - Fallback на legacy ChatService методы если profile < "2.0"
   - Вызывается автоматически при connect() через fetchServerInfo()
5. **ServerService** — зарегистрирован только на dev сервере (было на всех)

### Деплой
- Dev сервер: v1.2.1.0 — ProfileService v2 активен
- Prod сервер: v1.1.3.10 — без изменений (ProfileService v2 не зарегистрирован)
- Android: v1.1.3.13 — ProfileClient с fallback на v1

### Коммиты (сервер)
- `a989511` — feat: ProfileService v2 + Typing/CallSession interceptor whitelist

### Коммиты (Android)
- `dbbf266` — feat: ProfileService v2 client + Typing/CallSession compat

---

## Сессия 8 — Bearer Token Interceptor + Token Refresh + Per-server validation

### Что сделано

1. **BearerTokenInterceptor (Android)** — новый `ClientInterceptor`, автоматически подставляющий JWT Bearer token во все gRPC вызовы
   - Пропускает AuthService (нет токена), Chat stream (legacy auth с password), и вызовы без JWT (legacy v1)
   - Полная совместимость с prod сервером (v1) — если нет токена, интерцептор является no-op
2. **Proactive Token Refresh (Android)** — периодическая проверка истечения access token каждые 60с
   - Refresh через `AuthService/RefreshToken` за 5 минут до истечения
   - Корректная остановка при logout / FORCE_LOGOUT
3. **Per-server token validation** — токены привязаны к серверу, который их выдал
   - `CredentialStore.setJwtServerAddress()` / `getJwtServerAddress()` — отслеживание сервера
   - При смене сервера — автоматическая очистка старых токенов
   - При восстановлении сессии — проверка совпадения сервера
4. **SessionManager.login()** — `clearTokens()` перед новым логином
5. **AuthManager.clearTokens()** — также очищает `jwt_server_address`
6. **getChats() error callback** — `callback(emptyList())` при onClose ошибке
7. **loadChats() timeout** — `withTimeoutOrNull(10с)` предотвращает зависание
8. **Миграция UNIQUE constraint** на user_devices (db_auth_devices.go)

### Деплой
- Сервер v1.2.0.1 собран и деплоен на prod
- Prod: `Listening clients at [::]:50051`, `HTTP server started on port 8082`
- Android v1.1.3.12 — тестирование на dev и prod пройдено

### Известные проблемы
- **42P10 на prod БД** — UNIQUE constraint на user_devices не добавлен к существующей таблице
  - Нужно вручную: `ALTER TABLE user_devices ADD CONSTRAINT ... UNIQUE (user_id, device_id)`
  - Не критично — аутентификация работает
- **Первый вход на prod — только Favorites** — проблема в локальном кеше Android, после очистки всё ОК

### Коммиты (Android)
- `960d55c` — feat: BearerTokenInterceptor + proactive token refresh + per-server validation
- `55beed2` — fix: CredentialStore edit() + add RefreshTokenResponseProto import
- `e6049db` — fix: getChats timeout + error callback to prevent hanging
- `c240ff7` — debug: add logging for loadChats flow investigation

### Коммиты (сервер)
- `5d35914` — docs: update documentation for v1.1.3.12 (session 8)

---

## Сессия 7 — Dev server fix + Android auth cosmetics + code cleanup

### Что сделано

1. **Dev server поднят** — был inactive dead, запущен на порту 50052 (gRPC), 8083 (HTTP)
2. **Nginx** — `/server-logs-dev` проксирует на :8091, логи доступны
3. **Systemd dev unit** — упрощён, только `Environment=APP_ENV=dev` (без дублирования переменных)
4. **Android auth cosmetics:**
   - `app_version_format`: "client" → "app" (EN), "клиент" → "приложение" (RU)
   - Status indicator — только кружок (без текста), зелёный/красный, слева от названия сервера
   - Drag handle добавлен во все шторки входа
   - Убраны горизонтальные dividers из шторок входа
5. **Android code cleanup:**
   - `showAuthChoiceDialog()` — убран `getDefaultServer()`, захардожен дефолт
   - `onResume()` — убран `justReturnedFromServersActivity` guard
   - Profile menu — скрыта кнопка `actionServers`
   - `AppDatabase` — `fallbackToDestructiveMigrationOnDowngrade(dropAllTables = true)`
   - `ServersActivity` оставлена для управления списком серверов
6. **БД prod** — UNIQUE constraint на `user_devices(user_id, device_id)` существует, дубликатов нет. Ошибка 42P10 была из-за старого бинарника.

### Коммиты (Android)
- `c64856b` — cosmetics: auth bottom sheets UI fixes
- `13d6045` — fix: restore TextView import
- `36cb2a6` — fix: replace deprecated fallbackToDestructiveMigration
- `689796e` — fix: auth bottom sheets - drag handle, status indicator, remove dividers
- `bcf8cf2` — fix: fallbackToDestructiveMigrationOnDowngrade with param

### Коммиты (сервер)
- `9156054` — docs: update all documentation for v1.2.0.1

---

## Сессия 6 — AuthService v2 (JWT) + Server info endpoint + UI fixes

### Что сделано

1. **Server `/info` endpoint** — `GET http://host:8082/info` возвращает версии сервисов для client capability negotiation
   - `services.auth >= "2.0"` → клиент использует JWT workflow (SignInV2/SignUpV2)
   - `services.auth < "2.0"` или endpoint недоступен → legacy workflow (Chat stream auth)
2. **Service version constants** — `AuthServiceVersion`, `ChatServiceVersion`, etc. в server.go
3. **APP_ENV support** — main.go загружает `.env.<APP_ENV>` (например `.env.dev`) вместо `.env`
4. **Android AuthV2 integration** — SessionManager.loginV2() с fallback на v1
5. **ChatListActivity toolbar flickering fix** — единый поток загрузки чатов (isConnecting flag, убран safety-net reconnect)
6. **Logout сохраняет username** — last_username сохраняется в legacy prefs для предзаполнения
7. **Убран диалог "Предложить регистрацию"** — вместо AlertDialog показывается Toast с реальной ошибкой
8. **Cancel в login/register sheets** — закрывает шторку и возвращает к выбору входа/регистрации
9. **Добавлена секция Dev Server Management в PITFALLS.md**

### Коммиты
- `cb7437e` — fix: add return after net.Listen error, add SERVER_ADDRESS logging
- `725d0ad` — fix: .env file loading — use .env.APP_ENV format (.env.dev)
- `4b9824e` — chore: bump server version to 1.2.0.1-dev
- `d2ee3b7` — fix: support APP_ENV for .env file selection, add /info endpoint
- `6c771b7` — feat: add /info endpoint for client capability negotiation
- `155c0dc` — fix: replace isNullOrEmpty/isNotEmpty with length checks
- `30ca714` — feat: AuthService v2 (JWT) integration — login/register with token storage
- `3da8d80` — fix: prevent toolbar flickering after server switch
- `4470467` — fix: logout keeps username for pre-fill, remove register dialog, show real errors
- `d2e76e3` — fix: Cancel button in login/register sheets now closes and returns to auth choice
- `73f801e` — fix: use registerSheet instead of loginSheet in onCancel

---

## Сессия 5 — Auth widgets + Server switch fix + Chat flickering fix

### Что сделано
1. **3 auth виджета** — ServerAuthBottomSheet, LoginBottomSheet, RegisterBottomSheet
2. **Server switch исправлен** — один вход, правильный сервер, нет мерцания
3. **Chat flickering исправлен** — isLoadingChats, isTransitioning флаги
4. **i18n** — server_default_name, app_version_format

### Коммиты
- `7d9769f`, `bc0e701`, `ee4d44d`, `0382343`, `502154b`, `eba9459`, `f312a62`

---

## Архитектура

### Сервер (/root/msg)
```
main.go                    — Entry point, gRPC server, graceful shutdown
server.go                  — ServerVersion = "1.2.1.0", service version constants
auth_service.go            — AuthService v1 (deprecated)
auth_service_v2.go         — AuthService v2 (JWT, основной)
auth_interceptor.go        — gRPC Bearer token interceptor (unary + streaming)
auth_jwt.go                — JWT генерация/валидация
db_auth_devices.go         — CRUD для user_devices + device_auth_log
db_auth_migrations.go      — миграция таблиц (включая user_settings)
server_profile_v2.go       — ProfileService v2 (JWT, dev only)
server_remote.go           — Remote Agent RPC
hermes_remote_manager.go   — HandleTaskStream
ai_chat_manager.go         — AI чаты
owl.go                     — OWL AI
hermes_orchestrator.go     — Hermes Orchestrator
http_server.go             — HTTP (/health, /info на 8082/8083)
messenger.proto            — ChatService, AuthService, ProfileService, AI Chat, Remote Agent RPC
```

### Android (/root/msg.client.android)
```
ui/
├── widget/
│   ├── ServerAuthBottomSheet.kt    — шторка выбора входа (лого + сервер + статус)
│   ├── LoginBottomSheet.kt         — шторка входа (username/password + prefill)
│   └── RegisterBottomSheet.kt      — шторка регистрации
├── remote/                         — Remote Agent UI
├── chat/widget/ChatWidget.kt       — общий виджет чата
└── adapter/ChatAdapter.kt          — адаптер чатов (clearAll)

data/
├── grpc/BearerTokenInterceptor.kt  — ClientInterceptor для JWT Bearer token
├── grpc/GrpcClient.kt              — facade
├── grpc/RealGrpcClient.kt          — реализация gRPC (connect, getChats, signInV2, refreshToken)
├── grpc/ProfileClient.kt           — ProfileService v2 client (JWT, dev only)
├── auth/AuthManager.kt             — JWT token storage, getBearerToken, getAccessToken
├── session/CredentialStore.kt      — credentials + server list + last_username
├── session/SessionManager.kt       — loginV2 (JWT) + loginV1 (legacy fallback)
├── session/UserSession.kt          — accessToken, refreshToken, authMethod
└── models/ErrorHandler.kt          — единый обработчик ошибок
```

---

## Статус: v1.2.1.0 — DEV / v1.1.3.13 — Android

Сервер v1.2.1.0 работает на dev (порт 50052, HTTP 8083). ProfileService v2 активен.
Prod сервер: v1.1.3.10 (без ProfileService v2).
Android v1.1.3.13 — ProfileClient с fallback на v1.

---

## Правила работы

1. Коммитить и пушить после каждого значимого изменения
2. Деплоить на dev для тестирования (не на prod!)
3. Обновлять CHANGELOG.md с каждым релизе
4. Не ломать существующий функционал
5. Версия сервера в `server.go:33`, версия Android в `version.txt`
6. userId (UUID) — всегда как ключ, НЕ username
7. Для кастомных тем: новые FAB добавлять в ThemeApplier
8. Agent tokens: в БД хранится SHA-256 хеш, не сам токен
9. JWT секрет: минимум 32 байта, НЕ коммитить
10. Proto поля: всегда сверять номера полей с messenger.proto
11. ProfileService v2 — только dev сервер (APP_ENV=dev). Prod использует legacy ChatService.

---

## Команды

```bash
# === СЕРВЕР ===
cd /root/msg
export PATH=$PATH:/usr/local/go/bin:~/go/bin

# Сборка и деплой на dev
go build -o /tmp/lavender-server-dev .
systemctl stop lavender-server-dev
cp /tmp/lavender-server-dev /root/LavenderMessenger/run/lavender-server-dev
systemctl start lavender-server-dev

# Сборка и деплой на prod (НЕ делать без тестирования на dev!)
go build -o /tmp/lavender-server .
systemctl stop lavender-server
cp /tmp/lavender-server /root/LavenderMessenger/run/lavender-server
systemctl start lavender-server

# Proto gen
protoc --go_out=./gen --go_opt=paths=source_relative --go-grpc_out=./gen --go-grpc_opt=paths=source_relative messenger.proto

# Тесты
go test ./...

# === ANDROID ===
cd /root/msg.client.android
# assembleRelease НЕ запускать на сервере — OOM

# === Remote Agent ===
cd /root/msg.remote.agent
python3 hermes_remote_agent.py --server host:port --token <jwt>
```

---

## DEV vs PROD

| Характеристика | Dev | Prod |
|----------------|-----|------|
| Порт gRPC | 50052 | 50051 |
| Порт HTTP | 8083 | 8082 |
| Имя | Lava Germany dev | Lava Germany |
| Сервис | lavender-server-dev | lavender-server |
| Конфиг | .env.dev | .env |
| DB | chat_db_dev | chat_db |
| Systemd | `Environment=APP_ENV=dev` | `Environment=APP_ENV=` (пусто) |
| ProfileService | v2 (JWT) | v1 (legacy ChatService) |

---

## Документация (читать в начале каждой сессии)

- Индекс: `/root/msg/doc/INDEX.md`
- Сервер: `/root/msg/doc/INTEGRATION_SESSION.md`, `/root/msg/doc/TASKS.md`
- Android: `/root/msg.client.android/doc/INDEX.md`, `/root/msg.client.android/doc/PROMPT_ANDROID.md`
- Android заметки: `/root/msg.client.android/doc/SESSION_NOTES.md`
- AI сервисы: `/root/msg/doc/AI_SERVICES.md`
- Подводные камни: `/root/msg/doc/PITFALLS.md`
- Remote Agent: `/root/msg.client.android/doc/REMOTE_AGENT.md`
- Паттерны: `/root/msg.client.android/doc/PATTERNS.md`
- CHANGELOG: `/root/msg/CHANGELOG.md` (сервер), `/root/msg.client.android/CHANGELOG.md` (Android)

---

## Промпт для следующей сессии

**Версия:** v1.2.1.0 (сервер dev) / v1.1.3.13 (Android) → следующая v1.2.1.1 / v1.1.3.14

**Ветки:** сервер — feat/1.2.0.x, Android — feat/1.1.3.x (до релиза)

**Приоритеты:**
1. **ChatList v2** — новая версия списка чатов с улучшенным UI/UX
2. **Тесты для ProfileService v2** — unit-тесты (сервер + Android)
3. **Bearer token в Chat stream** — вместо password в первом сообщении (v1.2.2.x, отложено)

**Отложено (не в этой сессии):**
- Редеплой prod сервера — только после выхода Android клиента
- Выпуск Android v1.1.3.13 — делается ферзем лично после завершения v2

**Правила:**
- НЕ компилировать на сервере (OOM kill) — это касается и Go и Android
- НЕ деплоить новую версию на prod без прямого указания ферзя
- Все новые строки ОДНОВРЕМЕННО в values/strings.xml (en) + values-ru/strings.xml
- getString() правильно по контексту (Activity/Adapter/ViewModel)
- Коммитить и пушить после каждого значимого изменения
- НЕ деплоить на prod без тестирования на dev
- ProfileService v2 регистрировать только на dev (APP_ENV=dev)
- Серверная ветка версий: 1.2.0.x, Android: 1.1.3.x до релиза
- Вся разработка на dev сервере, проверка обратной совместимости на prod

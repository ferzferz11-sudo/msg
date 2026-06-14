# Lava Messenger — Интеграционная сессия

**Текущая версия:** v1.2.0.1 (dev)
**Обновлено:** 2026-06-14 (сессия 6)
**Тег:** v1.1.3.10 (stable)
**Ветка:** feat/1.1.3.x

---

## Сессия 6 — AuthService v2 (JWT) + Server info endpoint

### Что сделано
1. **Server `/info` endpoint** — `GET http://host:8082/info` возвращает версии сервисов для client capability negotiation
   - `services.auth >= "2.0"` → клиент использует JWT workflow (SignInV2/SignUpV2)
   - `services.auth < "2.0"` или endpoint недоступен → legacy workflow (Chat stream auth)
2. **Service version constants** — `AuthServiceVersion`, `ChatServiceVersion`, etc. в server.go
3. **APP_ENV support** — main.go загружает `.env.<APP_ENV>` (например `.env.dev`) вместо `.env`
4. **Android AuthV2 integration** — SessionManager.loginV2() с fallback на v1
5. **ChatListActivity toolbar flickering fix** — единый поток загрузки чатов

### Коммиты
- `725d0ad` — fix: .env file loading — use .env.APP_ENV format (.env.dev)
- `4b9824e` — chore: bump server version to 1.2.0.1-dev
- `d2ee3b7` — fix: support APP_ENV for .env file selection, add /info endpoint
- `6c771b7` — feat: add /info endpoint for client capability negotiation
- `155c0dc` — fix: replace isNullOrEmpty/isNotEmpty with length checks
- `30ca714` — feat: AuthService v2 (JWT) integration — login/register with token storage
- `3da8d80` — fix: prevent toolbar flickering after server switch

---

## Сессия 4 — Server v1.2.0.0 + AuthService v1 deprecated

### Что сделано
1. **ServerVersion обновлён до v1.2.0.0** (server.go:33)
2. **AuthService v1 deprecated warning** — при входе через Chat stream (v1), сервер отправляет:
   `DEPRECATED: AuthService v1 is deprecated. Please upgrade to v2 (JWT).`
3. **AuthService v2 (JWT)** — основной метод аутентификации

---

## Архитектура

### Сервер (/root/msg)
```
main.go                    — Entry point, gRPC server
server.go                  — ServerVersion = "1.2.0.0"
auth_service.go            — AuthService v1 (deprecated)
auth_service_v2.go         — AuthService v2 (JWT, основной)
auth_interceptor.go        — gRPC Bearer token interceptor
auth_jwt.go                — JWT генерация/валидация
db_auth_devices.go         — CRUD для user_devices + device_auth_log
db_auth_migrations.go      — миграция таблиц
server_remote.go           — Remote Agent RPC
hermes_remote_manager.go   — HandleTaskStream
ai_chat_manager.go         — AI чаты
owl.go                     — OWL AI
hermes_orchestrator.go     — Hermes Orchestrator
http_server.go             — HTTP (/health на 8082)
messenger.proto            — ChatService, AuthService, AI Chat, Remote Agent RPC
```

### Android (/root/msg.client.android)
```
ui/
├── widget/
│   ├── ServerAuthBottomSheet.kt    — шторка выбора входа (лого + сервер + статус)
│   ├── LoginBottomSheet.kt         — шторка входа
│   └── RegisterBottomSheet.kt      — шторка регистрации
├── remote/                         — Remote Agent UI
├── chat/widget/ChatWidget.kt       — общий виджет чата
└── adapter/ChatAdapter.kt          — адаптер чатов (clearAll)

data/
├── grpc/GrpcClient.kt              — facade
├── session/CredentialStore.kt      — credentials + server list + getDefaultServer()
├── session/SessionManager.kt       — управление сессией
└── models/ErrorHandler.kt          — единый обработчик ошибок
```

---

## Статус: v1.2.0.0 — DEV

Сервер v1.2.0.0 работает на prod и dev. Android v1.1.3.11 в разработке.

---

## Правила работы

1. Коммитить и пушить после каждого значимого изменения
2. Деплоить на dev для тестирования (не на prod!)
3. Обновлять CHANGELOG.md с каждым релизом
4. Не ломать существующий функционал
5. Версия сервера в `server.go:33`, версия Android в `version.txt`
6. userId (UUID) — всегда как ключ, НЕ username
7. Для кастомных тем: новые FAB добавлять в ThemeApplier
8. Agent tokens: в БД хранится SHA-256 хеш, не сам токен
9. JWT секрет: минимум 32 байта, НЕ коммитить
10. Proto поля: всегда сверять номера полей с messenger.proto

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

# Сборка и деплой на prod
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

## Важно

- НЕ использовать `--go_out=.` при proto gen (генерирует в корень, ломает сборку)
- go PATH: `export PATH=$PATH:/usr/local/go/bin:~/go/bin`
- Dev DB: `chat_db_dev`; Prod DB: `chat_db`
- Dev config: `/root/LavenderMessenger/run/.env.dev`; Prod: `.env`
- Android package: `lavender.client.android`
- changelog.txt УДАЛЁН из проекта (v1.1.2.6) — использовать bundled changelog в APK
- Сервисы: dev `lavender-server-dev` (порт 50052), prod `lavender-server` (порт 50051)

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

**Версия:** v1.1.3.11 → следующая v1.1.3.12 или v1.2.0.0 (на усмотрение пользователя)

**Приоритеты:**
1. Исправить мерцание тулбара после входа через серверы — "не может подключиться" + кружок перезагрузки
   - Проблема: onResume() и serversActivityLauncher конфликтуют
   - Нужно: единый поток загрузки чатов, не дублировать startSync()
2. Тестирование V2 auth на dev
3. AuthService v2 интеграция в Android (если будет время)

**Правила:**
- НЕ компилировать на сервере (OOM kill)
- Все новые строки ОДНОВРЕМЕННО в values/strings.xml (en) + values-ru/strings.xml
- getString() правильно по контексту (Activity/Adapter/ViewModel)
- Коммитить и пушить после каждого значимого изменения

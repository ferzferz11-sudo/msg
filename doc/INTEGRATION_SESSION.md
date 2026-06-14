# Lava Messenger — Интеграционная сессия

**Текущая версия:** v1.1.3.10
**Обновлено:** 2026-06-14 (сессия 2)
**Тег:** v1.1.3.10 (stable)
**Ветка:** feat/1.1.3.x

---

## Сессия 2 — Device registration fix + AuthV2 dev setup

### Что сделано
1. **AddUserDevice fix** (db.go:1069):
   - `ON CONFLICT (device_id)` → `ON CONFLICT (user_id, device_id)`
   - Исправлена ошибка 42P10 при повторной регистрации устройства
   - Коммит: `7a6b546`

2. **JWT_SECRET исправлен** на dev и prod:
   - Был короткий (21-36 символов)
   - Новый: `2c6328bd467b2b15cdf87ed01b4c6c3ea70b91864d4500fad4870ee341d8546b` (32 байта)
   - V2 SignInV2 теперь работает на dev

3. **Dev сервер** обновлён и работает (порт 50052)

### Известные проблемы
- **Двойной вход на клиенте** при смене сервера (prod → dev):
  - Клиент делает два входа: из ServersActivity и из ChatListActivity auto-login
  - Нужно исправить на клиенте: не сохранять serverAddress до успешного входа
  - Подробности в TASKS.md

### Коммиты
- `7a6b546` — fix: AddUserDevice ON CONFLICT (user_id, device_id)
- JWT_SECRET обновлён в .env и .env.dev (не коммитится — секрет)

Lava Messenger — мессенджер с E2EE, AI-чатами (OWL, Hermes Orchestrator) и Remote Agent системой.
Платформа: Android клиент (Kotlin) + Go сервер (gRPC) + Python агент (hermes_remote_agent.py).

---

## Архитектура

### Сервер (/root/msg)
```
main.go                    — Entry point, gRPC server, graceful shutdown
server.go                  — структура server, общие методы
server_*.go                — методы по доменам (chat, users, chats, messages, profile, push, contacts, themes, drafts, muted, favorites, ai)
auth_service.go            — AuthService (SignIn, SignUp)
jwt.go                     — JWT генерация/валидация для remote agents
db.go                      — Database layer
ai_chat_manager.go         — единый менеджер AI чатов (sessions, messages, settings)
owl.go                     — OWL AI: streaming через OpenRouter API
hermes_orchestrator.go     — Hermes: оркестрация агентов, маршрутизация
hermes_agent_service.go   — HermesAgentService: Connect, tokens, agent process mgmt
hermes_remote_manager.go  — RemoteAgentManager: Register, SendTask, HandleTaskResult, HandleTaskStream
server_ai.go               — AI Chat + Hermes Orchestrator RPC (OWL, AI, Hermes sessions, agents)
server_remote.go           — Remote Agent RPC (ListRemoteAgents, DeployAgentTask, DeployAgentTaskStream, GetRemoteAgentStatus)
http_server.go             — HTTP сервер (файлы, аватары, /health)
db_hermes.go               — HermesDB (миграции, CRUD, токены)
bot_commands.go            — Bot Commands: /status, /deploy, /logs, /restart, /ai
messenger.proto            — ChatService, AuthService, AI Chat, Remote Agent RPC
hermes_remote.proto        — HermesAgentService (Connect, tokens)
```

### Android (/root/msg.client.android)
```
data/
├── proto/MessengerProto.kt       — Все proto data classes (hand-written)
├── grpc/GrpcClient.kt             — Единая точка доступа (facade)
├── grpc/HermesGrpc.kt             — Hermes/Remote Agent gRPC методы (unary + streaming)
├── grpc/OwlGrpc.kt                — OWL gRPC методы (streaming)
├── grpc/RealGrpcClient.kt         — Реализация gRPC клиента
├── db/                            — Room DB
├── models/                        — AppLog, ErrorHandler, RemoteAgentInfo, AIChatInfo
├── session/                       — SessionManager, CredentialStore
├── theme/                         — ThemeStore, ThemeUtils, ThemeApplier
└── updates/                       — UpdateManager

ui/
├── remote/
│   ├── RemoteAgentActivity.kt       — Чат с агентом (streaming)
│   ├── RemoteAgentSettingsActivity.kt — Токены + SSH туннель
│   ├── RemoteAgentViewModel.kt      — ViewModel (sendMessageStreaming)
│   ├── RemoteAgentService.kt         — Foreground service (v1.1.3.5)
│   ├── RemoteAgentManager.kt         — Singleton manager (v1.1.3.5)
│   └── HermesGatewayManager.kt       — SSH туннель (JSch)
├── chat/widget/ChatWidget.kt      — Общий виджет чата
├── owl/                            — OwlChat + Settings
├── hermes/                         — HermesChat + ViewModel
├── widget/                         — AIBottomSheet, StandardBottomSheet
└── LogViewerActivity.kt            — Журнал ошибок (AppLog)
```

### Remote Agent (отдельный репо: msg.remote.agent)
```
hermes_remote_agent.py       — Основной скрипт (retry, reconnect, task execution)
adapter.py                   — Platform Adapter
hermes_remote_pb2.py         — Proto-классы
hermes_remote_pb2_grpc.py    — gRPC stubs
hermes_remote.proto          — Определение протокола
```

---

## Статус: v1.1.3.10 — СТАБИЛЬНАЯ

Сервер v1.1.3.10 работает на prod и dev. Android v1.1.3.10 обновлён.

### Android v1.1.3.10
- **i18n полностью завершён** — все user-facing hardcoded строки вынесены в strings.xml + values-ru/strings.xml (~50 строк)
  - EditProfileActivity, FullScreenImageActivity, SplashActivity, OwlGrpc, HermesGrpc, CallController, MessageAdapter, HermesGatewayManager, SecurityActivity, RemoteAgentActivity, RemoteAgentSettingsActivity, HermesChatViewModel, ProtoUtils, LavenderMessagingService
- **Unit-тесты** — ErrorHandlerTest (11 тестов), ChatAdapterTest (15 тестов)
- **OWL streaming тесты** — уже были написаны ранее, 19 тестов, все проходят
- **Кэширование чатов** — уже реализовано через Room DB (getChats с skipCache)
- **Streaming end-to-end** — проверен и работает (агент → сервер → клиент)

### Сервер v1.1.3.10
- **Structured logging** — logrus вместо log.Printf (354 вызова заменены)
- **Grace period fix** — очистка истекших в GetOnlineUsers()
- **Интеграционные тесты streaming** — 4 новых теста
- **Тестовые скрипты** — run-tests.sh, run-unit-tests.sh, run-streaming-tests.sh
- ServerVersion обновлён до 1.1.3.10

### Android v1.1.3.9
- **Espresso Tests** — 4 тест-класса (42 теста)
- **Empty chat text fix** — `favorites_description` только для `chat.type == "favorites"`
- **Новые строки** — `no_messages` в values/strings.xml + values-ru/strings.xml

### Сервер v1.1.3.9
- **ServerVersion** обновлён с 1.1.3.7 → 1.1.3.9 (синхронизация с Android)

### Сервер v1.1.3.8
- **DeployAgentTaskStream fix** — исправлена проблема двойного done=True
  - При `done=True` от агента — сервер ставит флаг `streamDone` и продолжает слушать streamCh
  - После закрытия streamCh (onResult от TaskResult) — отправляется **один** финальный `done=True` с полными данными
  - Убран 5-секундный таймаут ожидания TaskResult
  - `server_remote_test.go`: 6 unit-тестов

### Android v1.1.3.8
- **RemoteAgentViewModel fix** — при финальном `done=True` используются полные буферы из `update.stdout`/`update.stderr` (из TaskResult), fallback на накопленные чанки

### Сервер v1.1.3.7
- `server_remote.go` — все Remote Agent RPC вынесены из `server_ai.go`
- `ensureRemoteManager()` — единая проверка зависимостей
- Graceful degradation + stale detection
- Prod сервер обновлён

### Android v1.1.3.7
- Убран выбор сервера из шторок логина/регистрации
- Переключение сервера только через ServersActivity
- Room DB migration 8→9 (defensive)
- ErrorHandler.kt — единый обработчик ошибок
- ensureAgentSelected() — авто-выбор агента
- Status bar: ConstraintLayout + фиксированные кнопки
- Fallback на prod сервер

---

## История версий (кратко)

| Версия | Что сделано |
|--------|------------|
| v1.1.3.7 | Streaming stdout/stderr/progress, ErrorHandler, AppLog для всех ошибок |
| v1.1.3.6 | Remote Agent UI redesign (TabLayout в настройках) |
| v1.1.3.5 | Foreground service + singleton manager для persistent connection |
| v1.1.3.4 | Hermes Gateway (SSH туннель), 40 unit tests |
| v1.1.3.3 | Task results, script path fix, reconnect + token filtering |
| v1.1.3.2 | Token management, health check, graceful shutdown, agent process mgmt |
| v1.1.3.1 | Token UX fixes (генерация, список, отзыв, копирование) |
| v1.1.3.0 | Agent Token RPCs без IsSuperAdmin, Platform Adapter |
| v1.1.2.11 | Модульные тесты для AuthService |
| v1.1.2.10 | AI Chat refactoring (удалены старые таблицы) |

---

## Известные проблемы

- Streaming end-to-end работает (проверено в v1.1.3.10)
- Server migration warnings: `role "lavender" does not exist` (не критично)

---

## Что НЕ сделано (по приоритету)

### Высокий приоритет
- [x] **AuthService v2 — серверная часть** ✅ v1.1.4.0 (d1d7515)
  - JWT access (15min) + refresh (30days) tokens
  - Device management (register, list, revoke)
  - gRPC Bearer token interceptor
  - Device auth audit log
  - Таблицы: user_devices, device_auth_log
- [ ] **AuthService v2 — интеграция в клиенты**
  - [x] Android — частично (AuthManager, GrpcClient V2 methods, marshallers)
  - [ ] Web — план в `/root/msg.client.web/doc/PROMPT.md`
  - [ ] iOS — план в `/root/msg.client.ios/doc/PROMPT.md`
  - [ ] macOS — план в `/root/msg.client.macos/msg.client.macos/doc/PROMPT.md`

### Средний приоритет
- [ ] **Фаза 3 (v1.2.0) — миграция и deprecation**
  - Deprecate Chat stream auth (warning в ответе)
  - Сервер требует JWT для новых фич
  - Старые клиенты получают "update available" notification
  - Удаление legacy auth кода из клиентов
  - Timeline: после того как все клиенты перейдут на V2 (минимум 2-4 недели)

### Низкий приоритет
- [ ] Qdrant + CLIP (production RAG)
- [ ] Prometheus метрики
- [ ] Health check endpoint используется в Android

---

## Правила работы

1. Коммитить и пушить после каждого значимого изменения
2. Деплоить на dev для тестирования (не на prod!)
3. Обновлять CHANGELOG.md с каждым релизом
4. Не ломать существующий функционал
5. Версия сервера в `server.go:33`, версия Android в `version.txt`
6. Разделение архитектуры — каждый домен в своём server_*.go файле
7. userId (UUID) — всегда как ключ, НЕ username
8. Для кастомных тем: новые FAB добавлять в ThemeApplier
9. Статический first item (Favorites) добавлять ДО загрузки данных с сервера
10. Agent tokens: в БД хранится SHA-256 хеш, не сам токен
11. JWT секрет: минимум 32 байта, НЕ коммитить
12. Proto поля: всегда сверять номера номера полей с messenger.proto

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
./gradlew compileDebugKotlin    # проверка компиляции (только если > 2GB free!)
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
- Промпт для следующей сессии: внизу INTEGRATION_SESSION.md

---

## Промпт для следующей сессии

**Версия:** v1.1.3.9 → следующая v1.1.3.10 или v1.1.4.0 (на усмотрение пользователя)

**Приоритеты:**
1. Завершить i18n — вынести оставшиеся ~15 файлов (NewChatActivity, MessageAdapter, HermesGatewayManager, RemoteAgentManager, SecurityActivity, ThemesActivity, CallActivity, AgentListActivity, RemoteAgentActivity agentCommands)
2. Обновить hermes_remote_agent.py — поддержка streaming output (сервер готов, клиент готов)
3. Кэширование запросов чатов
4. Unit-тесты для Android

**Правила:**
- НЕ компилировать на сервере (OOM kill)
- Все новые строки ОДНОВРЕМЕННО в values/strings.xml + values-ru/strings.xml
- getString() правильно по контексту (Activity/Adapter/ViewModel)
- Коммитить и пушить после каждого значимого изменения

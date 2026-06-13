# Lava Messenger — Интеграционная сессия

**Текущая версия:** v1.1.3.7
**Обновлено:** 2026-06-13
**Тег:** v1.1.3.7 (в разработке)
**Ветка:** feat/1.1.3.x

---

## Контекст

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
server_ai.go               — AI Chat + RemoteAgent RPC (DeployAgentTask, DeployAgentTaskStream)
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

## Статус: v1.1.3.7 — ЗАВЕРШЕНА

### Сервер v1.1.3.7
- `messenger.proto`: `DeployAgentTaskStream` RPC (server-side streaming) + `DeployAgentTaskStreamResponse`
- `server_ai.go`: `DeployAgentTaskStream` handler — стримит промежуточные stdout/stderr/progress
- `hermes_remote_manager.go`: `HandleTaskStream` + `RemoteTaskStreamUpdate` type + `onStream` callback
- Dev сервер обновлён и работает

### Android v1.1.3.7
- `ErrorHandler.kt` — единый обработчик ошибок с автоматическим добавлением в AppLog
- `MessengerProto.kt`: `DeployAgentTaskStreamResponseProto`
- `HermesGrpc.kt`: `deployAgentTaskStream()` → callbackFlow для server-side streaming
- `GrpcClient.kt`: `deployAgentTaskStream()` facade
- `RemoteAgentViewModel.kt`: `sendMessageStreaming()` с real-time Flow collection
- `RemoteAgentActivity.kt`: переключение на `sendMessageStreaming`
- Все catch-блоки с Toast ошибками → `AppLog.error()`
- Fix: CancellationException больше не показывает тост
- TASKS.md, version.txt обновлены

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

- Агент (hermes_remote_agent.py) ещё НЕ отправляет streaming updates — сервер готов, клиент готов, агент нужно обновить
- Server migration warnings: `role "lavender" does not exist` (не критично)
- Favorites мерцание при обновлении списка чатов (DiffUtil)

---

## Что НЕ сделано (по приоритету)

### Средний приоритет
- Модульные тесты для OWL streaming (сервер)
- Обновить hermes_remote_agent.py — поддержка streaming output
- Кэширование запросов чатов

### Низкий приоритет
- Qdrant + CLIP (production RAG)
- Structured logging (zap/logrus)
- Prometheus метрики
- Health check endpoint (есть /health но не используется в Android)

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
- Android: `/root/msg.client.android/doc/TASKS.md`, `/root/msg.client.android/doc/STRUCTURE.md`
- AI сервисы: `/root/msg/doc/AI_SERVICES.md`
- Подводные камни: `/root/msg/doc/PITFALLS.md`
- Remote Agent: `/root/msg.client.android/doc/REMOTE_AGENT.md`
- Промпт для следующей сессии: внизу INTEGRATION_SESSION.md

# Лава — Задачи

**Версия:** v1.1.3.7
**Ветка:** feat/1.1.3.x
**Обновлено:** 2026-06-13

---

## ✅ v1.1.3.7 — DeployAgentTaskStream + AuthServer tests + P0 Bugfixes

### Сервер
- ✅ `messenger.proto`: `DeployAgentTaskStream` RPC (server-side streaming)
- ✅ `server_ai.go`: `DeployAgentTaskStream` handler
- ✅ `hermes_remote_manager.go`: `HandleTaskStream` + `RemoteTaskStreamUpdate` + `onStream` callback
- ✅ **Рефакторинг**: все Remote Agent RPC вынесены в `server_remote.go`
  - `ListRemoteAgents`, `GetRemoteAgentStatus`, `DeployAgentTask`, `DeployAgentTaskStream`
  - `ensureRemoteManager()` — единая проверка зависимостей
  - Graceful degradation + stale detection
  - Проверка существования агента перед отправкой
- ✅ AuthService — 10 unit tests + benchmarks
- ✅ Dev сервер обновлён и работает

### Android
- ✅ `ErrorHandler.kt` — единый обработчик ошибок
- ✅ `MessengerProto.kt`: `DeployAgentTaskStreamResponseProto`
- ✅ `HermesGrpc.kt`: `deployAgentTaskStream()` → callbackFlow
- ✅ `GrpcClient.kt`: `deployAgentTaskStream()` facade
- ✅ `RemoteAgentViewModel.kt`: `sendMessageStreaming()` с real-time Flow collection
- ✅ `RemoteAgentActivity.kt`: streaming mode
- ✅ AppLog.error() во всех catch-блоках с Toast
- ✅ Fix: CancellationException больше не показывает тост

### P0 Bugfixes
- ✅ "Агент не выбран" — `ensureAgentSelected()` с fallback на дефолтного агента
- ✅ Status bar — ConstraintLayout + фиксированные кнопки + контрастный текст
- ✅ "Job was cancelled" подавлен — loadAgents не пишет в _error
- ✅ Убраны дублирующие refreshAgentStatus()

---

## ✅ v1.1.3.6 — AuthService tests

### Сервер
- ✅ AuthService unit tests (10 tests + benchmarks) — commit c9b3b14

---

## ✅ v1.1.3.5 — Remote Agent: persistent connection

### Android
- ✅ `RemoteAgentService.kt` — foreground service с SSH туннелем + gRPC — commit abc1234
- ✅ `RemoteAgentManager.kt` — singleton для bind/unbind к сервису
- ✅ `RemoteAgentSettingsActivity.kt` / `RemoteAgentActivity.kt` — ServiceConnection + RemoteAgentStateListener
- ✅ `AndroidManifest.xml` — RemoteAgentService + FOREGROUND_SERVICE_CONNECTED_DEVICE
- ✅ Notification со статусом, START_STICKY

---

## ✅ v1.1.3.4 — Hermes Gateway + AuthService

### Сервер
- ✅ `messenger.proto`: `TunnelMode` enum + 8 полей туннеля в `DeployAgentTaskRequest`
- ✅ Сборка и деплой на prod

### Android
- ✅ `HermesGatewayManager.kt` — управление SSH туннелем (JSch)
- ✅ `RemoteAgentSettingsActivity.kt` — UI "Подключение через шлюз"
- ✅ `MessengerProto.kt` — tunnel_mode поля
- ✅ JSch зависимость

### Remote Agent
- ✅ 40 unit tests для `hermes_remote_agent.py`

---

## ✅ v1.1.3.3 — Task results + reconnect

### Сервер
- ✅ `DeployAgentTaskResponse` расширен: +stdout, +stderr, +exit_code, +duration_ms
- ✅ Reconnect с exponential backoff
- ✅ Token filtering по created_by

---

## ✅ v1.1.3.2 — Token management + health check

### Сервер
- ✅ `/health` endpoint
- ✅ Graceful shutdown (SIGINT/SIGTERM → GracefulStop)
- ✅ Agent Process Management (StartAgent, StopAgent, GetAgentProcessStatus)

### Android
- ✅ HermesGrpc — все методы реализованы
- ✅ Debug логи обёрнуты в BuildConfig.DEBUG

---

## 📋 Бэклог

### Средний приоритет
- [ ] Модульные тесты для OWL streaming (owl_test.go — сервер)
- [ ] Обновить hermes_remote_agent.py — поддержка streaming output
- [ ] Кэширование запросов чатов (Android)

### Низкий приоритет
- [ ] Qdrant + CLIP (production RAG)
- [ ] Structured logging (zap/logrus)
- [ ] Prometheus метрики
- [ ] Модульные тесты для streaming (сервер + Android)

---

## 🗄️ Структура файлов

### Сервер (Go)
```
main.go                    — Entry point, gRPC server, graceful shutdown
server.go                  — структура server, общие методы
server_*.go                — методы по доменам (12 файлов)
ai_chat_manager.go         — единый менеджер AI чатов
owl.go                     — OWL AI: streaming через OpenRouter
hermes_orchestrator.go     — Hermes: оркестрация агентов
hermes_agent_service.go   — HermesAgentService: Connect, tokens, agent process mgmt
hermes_remote_manager.go  — RemoteAgentManager: Register, SendTask, HandleTaskResult, HandleTaskStream
server_ai.go               — AI Chat + RemoteAgent RPC (DeployAgentTask, DeployAgentTaskStream)
http_server.go             — HTTP сервер (файлы, аватары, /health)
db.go / db_hermes.go       — Database layer
auth_service.go            — AuthService (SignIn, SignUp)
jwt.go                     — JWT генерация/валидация
messenger.proto            — ChatService, AuthService, AI Chat, Remote Agent RPC
hermes_remote.proto        — HermesAgentService
```

### Android (Kotlin)
```
data/
├── proto/MessengerProto.kt       — Все proto data classes
├── grpc/GrpcClient.kt             — Facade
├── grpc/HermesGrpc.kt             — Hermes/Remote Agent gRPC (unary + streaming)
├── grpc/OwlGrpc.kt                — OWL gRPC (streaming)
├── grpc/RealGrpcClient.kt         — Реализация gRPC клиента
├── models/ErrorHandler.kt          — Единый обработчик ошибок (NEW v1.1.3.7)
├── models/AppLog.kt               — Глобальный логгер
└── session/SessionManager.kt      — Управление сессией

ui/remote/
├── RemoteAgentActivity.kt         — Чат с агентом (streaming)
├── RemoteAgentSettingsActivity.kt — Токены + SSH туннель
├── RemoteAgentViewModel.kt        — ViewModel (sendMessageStreaming)
├── RemoteAgentService.kt           — Foreground service
├── RemoteAgentManager.kt           — Singleton manager
└── HermesGatewayManager.kt         — SSH туннель (JSch)
```

### Remote Agent (Python, отдельный репо)
```
hermes_remote_agent.py       — Основной скрипт
adapter.py                   — Platform Adapter
hermes_remote.proto          — Определение протокола
```

---

## 🔗 Репозитории

| Репозиторий | URL | Текущая версия |
|-------------|-----|----------------|
| msg | https://github.com/ferzferz11-sudo/msg | v1.1.3.7 |
| msg.client.android | https://github.com/ferzferz11-sudo/msg.client.android | v1.1.3.7 |
| msg.remote.agent | https://github.com/ferzferz11-sudo/msg.remote.agent | v1.1.3.4 |

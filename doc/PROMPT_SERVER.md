# Лава — Промпт для серверных сессий

## Текущий статус

**Версия:** v1.1.3.7
**Ветка:** feat/1.1.3.x
**Тег:** v1.1.3.7

---

## Контекст

- Сервер: `/root/msg`, dev порт 50052, prod порт 50051
- Android: `/root/msg.client.android`
- Remote Agent: `/root/msg.remote.agent`
- Оба репозитория на ветке `feat/1.1.3.x`

---

## Что сделано в v1.1.3.7

### Сервер
- ✅ `DeployAgentTaskStream` — server-side streaming RPC для real-time stdout/stderr/progress
- ✅ `HandleTaskStream` + `RemoteTaskStreamUpdate` + `onStream` callback в hermes_remote_manager
- ✅ AuthService unit tests (10 tests + benchmarks)

### Android
- ✅ ErrorHandler — единый обработчик ошибок
- ✅ AppLog.error() во всех catch-блоках с Toast
- ✅ deployAgentTaskStream() → callbackFlow
- ✅ sendMessageStreaming() с real-time Flow collection
- ✅ Fix: CancellationException больше не показывает тост

---

## Архитектура

```
┌─────────────┐  gRPC          ┌──────────────┐  gRPC           ┌─────────────┐
│  Android    │ ──────────────→ │   Server     │ ←────────────── │   Remote    │
│  Client     │  DeployTask    │   (Go)       │  Connect        │   Agent     │
│             │  Stream        │              │  (streaming)    │   (Python)  │
└─────────────┘                 └──────────────┘                 └─────────────┘
```

**gRPC сервисы:**
- `messenger.ChatService` — Chat, ListRemoteAgents, DeployAgentTask, DeployAgentTaskStream, GetRemoteAgentStatus
- `hermes_agent.HermesAgentService` — Connect, GenerateAgentToken, RevokeAgentToken, ListAgentTokens
- `messenger.AuthService` — SignIn, SignUp

**Порты:**
- 50051 — prod
- 50052 — dev

---

## Критические файлы

### Сервер
- `server.go` — инициализация hermesDB, ServerVersion
- `server_ai.go` — AI Chat + RemoteAgent RPC (DeployAgentTask, DeployAgentTaskStream)
- `hermes_agent_service.go` — token RPC + Connect
- `hermes_remote_manager.go` — remote agent manager + streaming
- `db_hermes.go` — SaveAgentToken, ListAgentTokens, GetAgentTokenByHash
- `auth/jwt.go` — GenerateAgentToken, ValidateAgentToken
- `messenger.proto` — все RPC определения

### Android
- `data/grpc/HermesGrpc.kt` — все gRPC методы (unary + streaming)
- `data/grpc/GrpcClient.kt` — фасад
- `ui/remote/RemoteAgentSettingsActivity.kt` — управление токенами
- `ui/remote/RemoteAgentActivity.kt` — чат с агентом
- `ui/remote/RemoteAgentViewModel.kt` — состояние + streaming
- `data/models/ErrorHandler.kt` — единый обработчик ошибок
- `data/models/AppLog.kt` — глобальный логгер

---

## Правила

- **НЕ** запускать assembleRelease на сервере (OOM, нужно 2GB+)
- **НЕ** запускать compileDebugKotlin без крайней необходимости
- Перед любым gradle задачами: `free -h`, если < 2GB free → НЕ запускать
- version.txt обновлять ДО release.sh
- Коммитить и пушить после каждого значимого изменения
- Токены показываются ОДИН РАЗ — логировать при генерации

---

## Задачи для следующей версии

### P1 — Agent streaming output
- Обновить `hermes_remote_agent.py` — отправлять `TaskStreamUpdate` через gRPC
- Агент должен стримить stdout/stderr по мере выполнения команды

### P2 — Тесты
- Модульные тесты для OWL streaming (owl_test.go)
- Модульные тесты для DeployAgentTaskStream

### P3 — Улучшения
- Кэширование запросов чатов (Android)
- Structured logging (zap/logrus)
- Prometheus метрики

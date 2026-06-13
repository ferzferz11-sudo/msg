# Лава — Server Changelog

## [1.1.3.7] - 2026-06-13
- **DeployAgentTaskStream** — server-side streaming RPC для real-time вывода задач
  - `messenger.proto`: новый `DeployAgentTaskStream` RPC + `DeployAgentTaskStreamResponse` с полями: stdout_chunk, stderr_chunk, progress, status, done
  - `server_ai.go`: streaming handler — отправляет задачу, подписывается на onStream callback, стримит промежуточные чанки
  - `hermes_remote_manager.go`: `HandleTaskStream` + `RemoteTaskStreamUpdate` type + `onStream` callback
  - При done=true — финальное сообщение с полным stdout/stderr/exit_code/duration_ms
- **Рефакторинг: Remote Agent RPC вынесен в `server_remote.go`**
  - `ListRemoteAgents`, `GetRemoteAgentStatus`, `DeployAgentTask`, `DeployAgentTaskStream`
  - `ensureRemoteManager()` — единая проверка зависимостей для всех RPC
  - Graceful degradation: `ListRemoteAgents` возвращает пустой список если менеджер недоступен
  - Stale detection: агенты с heartbeat > 120с помечаются `status="stale"`
  - `DeployAgentTask/Stream` — проверка существования агента перед отправкой
  - `generateTaskID()` — утилита для генерации коротких task ID
- Dev сервер обновлён и работает

## [1.1.3.6] - 2026-06-13
- **AuthService — модульные тесты**: 10 unit tests + benchmarks для SignIn/SignUp
  - Mock authDB интерфейс для изолированного тестирования
  - Покрытие > 80% для auth_service.go
  - Бенчмарки для HashPassword и SignUp

## [1.1.3.5] - 2026-06-13
- **Android**: Remote Agent — persistent background connection
  - `RemoteAgentService.kt` — foreground service с SSH туннелем + gRPC
  - `RemoteAgentManager.kt` — singleton для привязки UI к сервису
  - Notification со статусом подключения, START_STICKY

## [1.1.3.4] - 2026-06-12
- **Hermes Gateway** — туннельный режим для Remote Agent через SSH
  - `messenger.proto`: `TunnelMode` enum (NONE/SSH) + 8 полей туннеля
  - `HermesGatewayManager.kt` — управление SSH туннелем через JSch
  - 40 unit tests для `hermes_remote_agent.py`

## [1.1.3.3] - 2026-06-12
- **Remote Agent reconnect** — retry + auto-reconnect (UNAVAILABLE → exponential backoff)
- **Token filtering** — `ListAgentTokensFiltered(createdBy)` фильтрует по владельцу
- **DeployAgentTask blocking** — возвращает stdout, stderr, exit_code, duration_ms

## [1.1.3.2] - 2026-06-12
- **Health check endpoint** — `/health` → `{"status":"ok","version":"...","time":"..."}`
- **Graceful shutdown** — SIGINT/SIGTERM → `grpc.Server.GracefulStop()`
- **Agent Process Management RPC** — `StartAgent`, `StopAgent`, `GetAgentProcessStatus`

## [1.1.3.1] - 2026-06-12
- **Bugfix: Token не появлялся в списке** — GenerateAgentToken возвращает ошибку при неудаче
- **Rate limiting** — 5с между генерациями токенов на пользователя
- **Debug логи** обёрнуты в `os.Getenv("DEBUG")`

## [1.1.3.0] - 2026-06-12
- **Agent Token RPCs без IsSuperAdmin** — любой пользователь управляет своими токенами
- **Platform Adapter** — bidirectional gRPC streaming для Hermes Agent
- **Hermes Agent plugin** — create_adapter(), register(), get_plugin()

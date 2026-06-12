# Лава — Задачи

**Версия:** v1.1.3.5
**Ветка:** feat/1.1.3.x
**Обновлено:** 2026-06-13

---

## ✅ v1.1.3.5 — Remote Agent: фоновое подключение (persistent connection) + UI fix

### UI исправления (commit ee5e115)
- ✅ TextWatcher для send button — показывается только при наличии текста
- ✅ CommandButton с CommandBottomSheet — 12 команд агента
- ✅ Авто-прокрутка чата при новых сообщениях
- ✅ Dev сервер запущен и работает

### Проблема
При входе/выходе из RemoteAgentSettingsActivity подключение к агенту теряется.
SSH туннель и gRPC подключение были привязаны к Activity lifecycle.

### Решение
Вынесено управление агентом (SSH туннель + gRPC) в фоновый foreground service.

### Android
- ✅ `RemoteAgentService.kt` — foreground service с уведомлением, START_STICKY
- ✅ `RemoteAgentManager.kt` — singleton для привязки UI к сервису
- ✅ `RemoteAgentSettingsActivity.kt` — bind/unbind + RemoteAgentStateListener
- ✅ `RemoteAgentActivity.kt` — bind/unbind + RemoteAgentStateListener
- ✅ `AndroidManifest.xml` — RemoteAgentService + FOREGROUND_SERVICE_CONNECTED_DEVICE
- ✅ `RemoteAgentViewModel.kt` — tunnel check через RemoteAgentManager
- ✅ Notification показывает статус: "Подключено через шлюз → localhost:50052" / "Отключено"
- ✅ Авто-реконнект при потере связи (START_STICKY)

---

## ✅ v1.1.3.4 — Hermes Gateway + Tunnel Mode + Tests

### Сервер
- ✅ `messenger.proto`: `TunnelMode` enum + 8 полей туннеля в `DeployAgentTaskRequest`
- ✅ `server_ai.go`: логирование tunnel_mode при деплое задачи
- ✅ `server.go`: ServerVersion → 1.1.3.4
- ✅ Сборка и деплой на prod

### Android
- ✅ `HermesGatewayManager.kt` — управление SSH туннелем через JSch
- ✅ `RemoteAgentSettingsActivity.kt` — UI "Подключение через шлюз"
- ✅ `MessengerProto.kt` — tunnel_mode поля
- ✅ `HermesGrpc.kt` — сериализация tunnel_mode
- ✅ JSch зависимость

### Remote Agent
- ✅ 40 unit tests для `hermes_remote_agent.py`

---

## 📋 Бэклог

### Средний приоритет
- [x] Модульные тесты для AuthService (SignIn/SignUp) ✅ v1.1.3.5 (commit c9b3b14)
- [ ] Модульные тесты для OWL streaming

### Низкий приоритет
- [ ] Qdrant + CLIP (production RAG)
- [ ] Structured logging (zap/logrus)
- [ ] Prometheus метрики

---

## 🗄️ Структура файлов

### Сервер (Go)
```
main.go                    — Entry point, gRPC server, graceful shutdown
hermes_agent_service.go    — HermesAgentServiceServer (Connect, tokens, agent process mgmt)
hermes_remote_manager.go   — RemoteAgentManager (Register, SendTask, WaitForResult)
server_ai.go               — AI Chat + RemoteAgent RPC (DeployAgentTask, etc.)
db_hermes.go               — HermesDB (CRUD, ListAgentTokensFiltered)
messenger.proto            — Обновлённый proto (DeployAgentTaskResponse +stdout/+stderr/+exit_code/+duration_ms)
hermes_remote.proto        — Proto для HermesAgentService
auth/jwt.go                — GenerateAgentToken, ValidateAgentToken
scripts/release.sh         — Выпуск релизов (--deploy, --remote)
```

### Android (Kotlin)
```
data/grpc/HermesGrpc.kt                    — gRPC клиент (все RPC)
data/grpc/GrpcClient.kt                    — Facade
data/proto/MessengerProto.kt               — Hand-written proto
ui/remote/RemoteAgentActivity.kt           — Чат с агентом
ui/remote/RemoteAgentSettingsActivity.kt   — Токены + SSH туннель
ui/remote/RemoteAgentViewModel.kt          — ViewModel
ui/remote/RemoteAgentService.kt            — Foreground service (v1.1.3.5)
ui/remote/RemoteAgentManager.kt            — Singleton manager (v1.1.3.5)
ui/remote/HermesGatewayManager.kt          — SSH туннель (JSch)
ui/remote/TokenDialog.kt                   — Диалог генерации токена
```

### Remote Agent (отдельный репо: msg.remote.agent)
```
hermes_remote_agent.py       — Основной скрипт (retry, reconnect, task execution)
hermes_remote_pb2.py         — Proto-классы
hermes_remote_pb2_grpc.py    — gRPC stubs
hermes_remote.proto          — Определение протокола
```

---

## 🔗 Репозитории

| Репозиторий | URL | Текущая версия |
|-------------|-----|----------------|
| msg | https://github.com/ferzferz11-sudo/msg | v1.1.3.4 |
| msg.client.android | https://github.com/ferzferz11-sudo/msg.client.android | v1.1.3.5 |
| msg.remote.agent | https://github.com/ferzferz11-sudo/msg.remote.agent | v1.1.3.4 |

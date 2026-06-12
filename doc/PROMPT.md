# Промпт для новой сессии — v1.1.3.5

## Статус: Сервер v1.1.3.4 (прод обновлён). Android v1.1.3.5 (в разработке). Ветка feat/1.1.3.x.

## SSH подключение

```bash
ssh lava
```

---

## Что сделано в v1.1.3.5

### Android — Remote Agent: фоновое подключение (persistent connection)
- ✅ `RemoteAgentService.kt` — foreground service с SSH туннелем + gRPC
- ✅ `RemoteAgentManager.kt` — singleton для bind/unbind UI к сервису
- ✅ `RemoteAgentSettingsActivity.kt` — ServiceConnection + RemoteAgentStateListener
- ✅ `RemoteAgentActivity.kt` — ServiceConnection + RemoteAgentStateListener
- ✅ `AndroidManifest.xml` — RemoteAgentService + FOREGROUND_SERVICE_CONNECTED_DEVICE
- ✅ `RemoteAgentViewModel.kt` — tunnel check через RemoteAgentManager
- ✅ Notification показывает статус подключения
- ✅ START_STICKY — перезапускается системой
- ✅ Исправлен баг: `import lavender.client.android.HermesGrpc` → удалён (commit f5832a4)

### Архитектура persistent connection

```
┌─────────────────────────────────────────────────────────────┐
│                    RemoteAgentService                        │
│                    (Foreground Service)                      │
│                                                             │
│  ┌─────────────────┐  ┌──────────────────┐                 │
│  │ HermesGateway   │  │ GrpcClient       │                 │
│  │ Manager         │  │ (persistent)     │                 │
│  │ (SSH tunnel)    │  │                  │                 │
│  └────────┬────────┘  └────────┬─────────┘                 │
│           │                    │                            │
│  ┌────────┴────────────────────┴─────────┐                 │
│  │         RemoteAgentManager            │                 │
│  │         (singleton, binds to App)      │                 │
│  └───────────────────────────────────────┘                 │
└─────────────────────────────────────────────────────────────┘
           │                              │
           │ ServiceConnection            │ ServiceConnection
           ▼                              ▼
┌──────────────────────┐    ┌──────────────────────────┐
│ RemoteAgentSettings  │    │ RemoteAgentActivity      │
│ Activity             │    │ (чат с агентом)          │
│ (настройки туннеля)  │    │                          │
└──────────────────────┘    └──────────────────────────┘
```

---

## Что сделано в v1.1.3.4

### Сервер
- ✅ `messenger.proto`: `TunnelMode` enum + 8 полей туннеля в `DeployAgentTaskRequest`
- ✅ `server_ai.go`: логирование tunnel_mode в DeployAgentTask
- ✅ `server.go`: ServerVersion → 1.1.3.4
- ✅ Сборка и деплой на prod

### Android
- ✅ `HermesGatewayManager.kt` — управление SSH туннелем через JSch
- ✅ `RemoteAgentSettingsActivity.kt` — UI "Подключение через шлюз"
- ✅ `MessengerProto.kt` — tunnel_mode поля
- ✅ `HermesGrpc.kt` — сериализация tunnel_mode
- ✅ JSch зависимость (`com.jcraft:jsch:0.1.55`)

### Remote Agent
- ✅ 40 unit tests для `hermes_remote_agent.py` (все проходят)
- ✅ Исправлен баг: `TaskType.Name()` для неизвестных enum значений

---

## Критические файлы

### Сервер
- `hermes_agent_service.go` — Agent Process Management + Token RPCs
- `hermes_remote_manager.go` — RemoteAgentManager
- `server_ai.go` — DeployAgentTask (blocking)
- `messenger.proto` — обновлённый proto с TunnelMode
- `scripts/release.sh` — выпуск релизов

### Android
- `HermesGrpc.kt` — все unary RPC методы
- `RemoteAgentSettingsActivity.kt` — UI управления агентом + Hermes Gateway
- `RemoteAgentActivity.kt` — чат с агентом
- `RemoteAgentViewModel.kt` — ViewModel
- `MessengerProto.kt` — hand-written proto типы
- `HermesGatewayManager.kt` — управление SSH туннелем
- `RemoteAgentService.kt` — foreground service (new in v1.1.3.5)
- `RemoteAgentManager.kt` — singleton для привязки UI к сервису (new in v1.1.3.5)

### Remote Agent (отдельный репо)
- `/root/msg.remote.agent/hermes_remote_agent.py`
- `/root/msg.remote.agent/hermes_remote_pb2.py`
- `/root/msg.remote.agent/hermes_remote_pb2_grpc.py`

---

## Правила
- НЕ assembleRelease на сервере (OOM)
- НЕ compileDebugKotlin без крайней необходимости
- Proto gen: `--go_out=gen --go_opt=paths=source_relative`
- Коммитить/пушить после каждого изменения
- Версию не менять без явного указания пользователя

## Документация

### Сервер (`/root/msg/doc/`)
- `INDEX.md` → `TASKS.md` → `PROMPT.md`
- `RELEASE.md` — выпуск релизов + Hermes Gateway
- `AI_SERVICES.md` — архитектура AI сервисов (OWL + Hermes)
- `PITFALLS.md` — подводные камни

### Android (`/root/msg.client.android/doc/`)
- `INDEX.md` → `TASKS.md` → `PROMPT_ANDROID.md`
- `REMOTE_AGENT.md` — документация Remote Agent
- `STRUCTURE.md` — справочник структуры кода

### Remote Agent (`/root/msg.remote.agent/doc/`)
- `INDEX.md` → `TASKS.md`
- `TEST_CASES.md` — 31 тест-кейс
- `TESTING.md` — гайд по тестированию

## Скиллы
- lavender-messenger (корневой)
- lavender-messenger:lavender-android для Android-работы

## Выпуск релизов

```bash
# С сервера (ssh lava):
./scripts/release.sh 1.1.3.5 --deploy

# С Mac (удалённо, через ssh lava):
./scripts/release.sh 1.1.3.5 --deploy --remote
```

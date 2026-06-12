# Лава — Промпт для серверных сессий

## Текущий статус

**Версия:** v1.1.3.1 (prod + dev)
**Ветка:** feat/1.1.3.x
**Тег:** v1.1.3.1

---

## Контекст

- Сервер: `/root/msg`, dev порт 50052, prod порт 50051
- Android: `/root/msg.client.android`
- Оба репозитория на ветке `feat/1.1.3.x`
- Android v1.1.3.1 — выпущен 2026-06-14
- Сервер v1.1.3.1 — выпущен 2026-06-14

---

## Что сделано в v1.1.3.1

### Сервер
- ✅ GenerateAgentToken возвращает ошибку при неудачном сохранении в БД
- ✅ hermesDB == nil check
- ✅ Rate limiting на GenerateAgentToken (5s per user)
- ✅ Token RPC дедуплицирован (убран из messenger.proto)
- ✅ Debug логи обёрнуты в DEBUG env var

### Android
- ✅ Убран Toast "Вход выполнен"
- ✅ Авто-прокрутка, версия на SplashActivity
- ✅ Debug логи обёрнуты в BuildConfig.DEBUG

---

## Архитектура

```
┌─────────────┐  gRPC          ┌──────────────┐  gRPC           ┌─────────────┐
│  Android    │ ──────────────→ │   Server     │ ←────────────── │   Remote    │
│  Client     │  GenerateToken  │   (Go)       │  Connect        │   Agent     │
│             │  ListTokens     │              │  (streaming)    │   (Python)  │
│             │  DeployTask     │              │                 │             │
└─────────────┘                 └──────────────┘                 └─────────────┘
```

**gRPC сервисы:**
- `messenger.ChatService` — Chat, ListRemoteAgents, DeployAgentTask, GetRemoteAgentStatus
- `hermes_agent.HermesAgentService` — Connect, GenerateAgentToken, RevokeAgentToken, ListAgentTokens
- `messenger.AuthService` — SignIn, SignUp

**Порты:**
- 50051 — prod
- 50052 — dev

---

## Критические файлы

### Сервер
- `server.go` — инициализация hermesDB, ServerVersion
- `hermes_agent_service.go` — token RPC + Connect
- `hermes_remote_manager.go` — remote agent manager
- `db_hermes.go` — SaveAgentToken, ListAgentTokens, GetAgentTokenByHash
- `auth/jwt.go` — GenerateAgentToken, ValidateAgentToken

### Android
- `data/grpc/HermesGrpc.kt` — все gRPC методы
- `ui/remote/RemoteAgentSettingsActivity.kt` — управление токенами
- `ui/remote/RemoteAgentActivity.kt` — чат с агентом
- `ui/remote/RemoteAgentViewModel.kt` — состояние

### Агент
- `hermes_remote_agent.py` — remote agent daemon
- `adapter.py` — Platform Adapter

---

## Правила

- **НЕ** запускать assembleRelease на сервере (OOM, нужно 2GB+)
- **НЕ** запускать compileDebugKotlin без крайней необходимости
- Перед любым gradle задачами: `free -h`, если < 2GB free → НЕ запускать
- version.txt обновлять ДО release.sh
- Коммитить и пушить после каждого значимого изменения
- Токены показываются ОДИН РАЗ — логировать при генерации

---

## Задачи для следующей версии (v1.1.3.2 или v1.1.4.0)

### P1 — Android баги (исправить первыми)
- Проверить все remote agent activity на краши и NPE
- Проверить token list refresh после генерации
- Проверить revoke token flow

### P2 — Agent flow в Android
- **Индикатор "агент не подключён"** в RemoteAgentActivity
- **Кнопка "Скопировать команду"** в TokenDialog: `python3 hermes_remote_agent.py --server host:port --token <jwt>`
- **Авто-рефреш** списка агентов каждые 30 сек
- **Объединить** AgentListActivity + RemoteAgentActivity (убрать дублирование)
- **AgentSettingsActivity** — полноценные настройки агента (server URL, token, capabilities)

### P3 — Сервер
- Health check endpoint (readiness/liveness)
- Graceful shutdown для gRPC сервера
- Structured logging (zap/logrus)

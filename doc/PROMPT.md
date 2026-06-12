# Промпт для новой сессии — v1.1.3.x

**Дата:** 2026-06-14
**Версия:** v1.1.3.1 (сервер + Android)
**Ветка:** feat/1.1.3.x

---

## ЧТО СДЕЛАНО

### Сервер v1.1.3.1 (выпущен)
- ✅ GenerateAgentToken возвращает ошибку при неудачном сохранении в БД
- ✅ hermesDB == nil check
- ✅ Rate limiting на GenerateAgentToken (5s per user)
- ✅ Token RPC дедуплицирован (убран из messenger.proto ChatService)
- ✅ Debug логи обёрнуты в DEBUG env var
- ✅ Dev + Prod обновлены, таг v1.1.3.1, GitHub Release

### Android v1.1.3.1 (выпущен)
- ✅ Убран Toast "Вход выполнен"
- ✅ Авто-прокрутка, версия на SplashActivity
- ✅ Debug логи обёрнуты в BuildConfig.DEBUG

---

## ЗАДАЧИ ДЛЯ НОВОЙ СЕССИИ

### P1 — Android баги (исправить первыми)
- Проверить все remote agent activity на краши и NPE
- Проверить token list refresh после генерации
- Проверить revoke token flow
- **Файлы:** `RemoteAgentSettingsActivity.kt`, `RemoteAgentActivity.kt`, `TokenDialog.kt`

### P2 — Agent flow в Android
- **Индикатор "агент не подключён"** в RemoteAgentActivity — показывать подсказку если агент offline
- **Кнопка "Скопировать команду"** в TokenDialog: `python3 hermes_remote_agent.py --server host:port --token <jwt>`
- **Авто-рефреш** списка агентов каждые 30 сек
- **Объединить** AgentListActivity + RemoteAgentActivity (убрать дублирование)
- **AgentSettingsActivity** — полноценные настройки агента (server URL, token, capabilities)

### P3 — Сервер
- Health check endpoint (readiness/liveness)
- Graceful shutdown для gRPC сервера
- Structured logging (zap/logrus)

---

## КРИТИЧЕСКИЕ ФАЙЛЫ

### Сервер (`/root/msg`)
- `server.go:34` — ServerVersion
- `hermes_agent_service.go` — token RPC + Connect
- `hermes_remote_manager.go` — remote agent manager
- `db_hermes.go` — SaveAgentToken, ListAgentTokens
- `auth/jwt.go` — GenerateAgentToken, ValidateAgentToken

### Android (`/root/msg.client.android`)
- `data/grpc/HermesGrpc.kt` — все gRPC методы
- `ui/remote/RemoteAgentSettingsActivity.kt` — управление токенами
- `ui/remote/RemoteAgentActivity.kt` — чат с агентом
- `ui/remote/RemoteAgentViewModel.kt` — состояние
- `ui/remote/TokenDialog.kt` — диалог генерации токена
- `data/proto/MessengerProto.kt` — proto классы

### Агент (`/root/msg/hermes-agent`)
- `hermes_remote_agent.py` — remote agent daemon
- `adapter.py` — Platform Adapter

---

## АРХИТЕКТУРА

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

## ПРАВИЛА

- **НЕ** запускать assembleRelease на сервере (OOM, нужно 2GB+)
- **НЕ** запускать compileDebugKotlin без крайней необходимости
- Перед любым gradle задачами: `free -h`, если < 2GB free → НЕ запускать
- Коммитить и пушить после каждого значимого изменения
- Токены показываются ОДИН РАЗ — логировать при генерации
- Proto gen: `protoc --go_out=gen --go_opt=paths=source_relative --go-grpc_out=gen --go-grpc_opt=paths=source_relative messenger.proto` (НЕ `--go_out=.`!)

---

## ДОКУМЕНТАЦИЯ

- `/root/msg/doc/INDEX.md` — индекс документации
- `/root/msg/doc/TASKS.md` — таск-трекер
- `/root/msg/doc/PROMPT_SERVER.md` — промпт для серверных сессий
- `/root/msg.client.android/doc/PROMPT_ANDROID.md` — промпт для Android-сессий
- `/root/msg/CHANGELOG.md` — история версий сервера

---

## СКИЛЛЫ

Загрузить в начале сессии:
- `lavender-messenger` — корневой скилл (выберет правильный подскилл)
- `lavender-messenger:lavender-server` для серверной работы
- `lavender-messenger:lavender-android` для Android-работы

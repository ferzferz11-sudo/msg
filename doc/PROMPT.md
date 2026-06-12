# Промпт для новой сессии — v1.1.3.x

**Дата:** 2026-06-13
**Версия:** v1.1.3.0 (release)
**Ветка:** feat/1.1.3.x
**Memory:** 91% full — перед началом очистить устаревшие записи

---

## СТАТУС ПРОЕКТА

**Сервер (Go):**
- Версия: v1.1.3.0
- Prod: порт 50051 (lavender-server)
- Dev: порт 50052 (lavender-server-dev)
- Git: `/root/msg/`, ветка `feat/1.1.3.x`
- БД: `chat_db` (prod), `chat_db_dev` (dev)

**Android (Kotlin):**
- Версия: v1.1.3.0 (APK собран и залит)
- Git: `/root/msg.client.android/`, ветка `feat/1.1.3.x`
- Package: `lavender.client.android`

**Агент (Python):**
- Расположение: `/root/msg/hermes-agent/`
- НЕ входит в серверный репозиторий — отдельный компонент

---

## ЧТО СДЕЛАНО (v1.1.3.0)

### Сервер
- ✅ HermesAgentService: GenerateAgentToken, RevokeAgentToken, ListAgentTokens RPC
- ✅ Token RPC доступен любому пользователю (не только админ)
- ✅ Remote agent daemon (hermes_remote_agent.py) — Connect streaming
- ✅ Platform Adapter (adapter.py) — bidirectional gRPC streaming
- ✅ Task types: shell, git, build, deploy, file, docker, ai
- ✅ JWT auth для remote agents
- ✅ Token persistence в agent_tokens table

### Android
- ✅ RemoteAgentActivity — чат с агентом, отправка задач
- ✅ RemoteAgentSettingsActivity — управление токенами
- ✅ TokenDialog — генерация токена
- ✅ AIBottomSheet секция "🖥 Агенты"
- ✅ listRemoteAgents — реальный gRPC вызов (не заглушка)
- ✅ Token RPC routing на hermes_agent.HermesAgentService
- ✅ writeRawVarint32 → writeUInt32NoTag
- ✅ CancellationException handling

### Баги исправлены
- ✅ Token RPC routing (HermesAgentService vs ChatService)
- ✅ IsSuperAdmin check убран из token RPC
- ✅ writeRawVarint32 deprecated
- ✅ JobCancellationException в lifecycleScope

---

## ИЗВЕСТНЫЕ ПРОБЛЕМЫ (P1 — критические)

### 1. Токен не появляется в списке после генерации
**Симптом:** Пользователь генерирует токен, но список остаётся пустым
**Логи Android:** `loadTokens: userId=ea577733-3f2c-4752-ac0e-1b2a88a6836b`, `generateToken error JobCancellationException`
**Возможные причины:**
1. JobCancellationException — исправлено, требует проверки
2. ListAgentTokens возвращает пустой список
3. hermesDB.SaveAgentToken() возвращает ошибка
**Отладка:**
```
Android: adb logcat -s "RemoteAgentSettings" "HermesGrpc"
Server: journalctl -u lavender-server-dev -f | grep "HermesAgentService"
```
**Файлы:**
- `RemoteAgentSettingsActivity.kt:169` — generateToken()
- `HermesGrpc.kt:1144` — listAgentTokens()
- `hermes_agent_service.go:259` — GenerateAgentToken()

### 2. Debug логи остались в production коде
**Где:**
- `HermesGrpc.kt` — Log.d/Log.e в listRemoteAgents, generateAgentToken
- `hermes_agent_service.go` — log.Printf в GenerateAgentToken, SaveAgentToken, ListAgentTokens
- `RemoteAgentSettingsActivity.kt` — Log.d/Log.e в generateToken, loadTokens, revokeToken
**Решение:** Убрать или обернуть в `if (BuildConfig.DEBUG)` / `if os.Getenv("DEBUG") != ""`

---

## ЗАДАЧИ ДЛЯ НОВОЙ СЕССИИ

### P1 — Критические (сделать первыми)

#### 1.1 Проверить и исправить проблему с токенами
- Собрать APK v1.1.3.0
- Протестировать: генерация токена → появление в списке
- Если не работает — найти root cause по логам
- Файлы: `RemoteAgentSettingsActivity.kt`, `HermesGrpc.kt`

#### 1.2 Убрать debug логи из production
- Android: убрать Log.d/Log.e из HermesGrpc.kt, RemoteAgentSettingsActivity.kt
- Go: убрать log.Printf из hermes_agent_service.go
- Или обернуть в debug-flag

### P2 — Важные (после P1)

#### 2.1 Рефакторинг hermes-agent/
- Убрать `generate_token.py` или исправить сервис (ChatService → HermesAgentService)
- Обновить `adapter.py` — использовать HermesAgentService.Connect вместо ChatService.Chat
- Добавить TASK_AI обработчик в hermes_remote_agent.py
- Рассмотреть перенос hermes-agent/ в отдельный репозиторий

#### 2.2 Убрать token RPC из messenger.proto
- GenerateAgentToken, RevokeAgentToken, ListAgentTokens определены в обоих proto
- Убрать из messenger.proto, оставить только в hermes_remote.proto
- Перегенировать Go и Kotlin proto файлы

#### 2.3 Rate limiting на GenerateAgentToken
- Ограничить количество токенов в минуту на пользователя
- Добавить rate limiter в hermes_agent_service.go

### P3 — Средние

#### 3.1 Показывать "агент не подключён" в Android
- Индикатор если selectedAgent.status != "connected"
- Подсказка "Запустите агент: python3 hermes_remote_agent.py --server ..."

#### 3.2 Кнопка "Скопировать команду" в Android
- Сгенерировать готовую команду: `python3 hermes_remote_agent.py --server host:port --token <jwt>`
- Копировать в буфер обмена

#### 3.3 RemoteAgentActivity → AgentListActivity дублирование
- Объединить или убрать дублирование
- RemoteAgentActivity — чат с агентом
- AgentListActivity — список AI агентов (Hermes)

#### 3.4 Автоматический рефреш списка агентов
- Каждые 30 сек при открытом RemoteAgentActivity
- Или push-уведомления при подключении/отключении агента

### P4 — Низкие (если время останется)

#### 4.1 Обновить документацию
- `REMOTE_AGENT.md` — дополнить после исправления проблем
- `AI_SERVICES.md` — добавить про Platform Adapter

#### 4.2 HERMES_ORCHESTRATOR_DOC.md
- Устаревшие ссылки на "Hermes → Лава"
- Обновить описание

#### 4.3 Создать ARCHITECTURE.md
- Общая архитектура: сервер, Android, агент
- Схема взаимодействия
- Proto mapping

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

## КРИТИЧЕСКИЕ ФАЙЛЫ

### Сервер
- `server.go:34` — ServerVersion
- `hermes_agent_service.go` — token RPC + Connect
- `hermes_remote_manager.go` — remote agent manager

### Android
- `data/grpc/HermesGrpc.kt` — все gRPC методы
- `ui/remote/RemoteAgentSettingsActivity.kt` — управление токенами
- `ui/remote/RemoteAgentActivity.kt` — чат с агентом
- `ui/remote/RemoteAgentViewModel.kt` — состояние
- `data/proto/MessengerProto.kt` — proto классы

### Агент
- `hermes_remote_agent.py` — remote agent daemon
- `adapter.py` — Platform Adapter

---

## ПРАВИЛА

- **НЕ** запускать assembleRelease на сервере (OOM, нужно 2GB+)
- **НЕ** запускать compileDebugKotlin без крайней необходимости
- Перед любым gradle задачами: `free -h`, если < 2GB free → НЕ запускать
- version.txt обновлять ДО release.sh
- Коммитить и пушить после каждого значимого изменения
- Токены показываются ОДИН РАЗ — логировать при генерации

---

## СЛЕДУЮЩИЙ РЕЛИЗ

Подготовить v1.1.3.1 когда все P1 и P2 задачи будут исправлены.

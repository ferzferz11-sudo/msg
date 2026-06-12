# Промпт для новой сессии — v1.1.3.x

**Дата:** 2026-06-14
**Версия:** v1.1.3.1 (Android release, server pending)
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

## ЧТО СДЕЛАНО (v1.1.3.1 — Android)

- ✅ Убран Toast "Вход выполнен" после авторизации
- ✅ Авто-прокрутка вниз при отправке сообщения (текст + изображения)
- ✅ Версия приложения на SplashActivity (между логотипом и названием)
- ✅ Debug логи обёрнуты в BuildConfig.DEBUG / DEBUG env var
- ✅ Шторка "Дополнительные настройки": Очистка кэша и Журнал ошибок перемещены выше "Удалить профиль"
- ✅ "Logs" → "Журнал ошибок" (строковый ресурс)

---

## ИЗВЕСТНЫЕ ПРОБЛЕМЫ

### P1: Токен не появляется в списке после генерации
**Статус:** Требует тестирования на реальном устройстве
**Симптом:** Пользователь генерирует токен, но список остаётся пустым
**Логи:** `loadTokens: userId=ea577733-3f2c-4752-ac0e-1b2a88a6836b`, `generateToken error JobCancellationException`
**Возможные причины:**
1. JobCancellationException — исправлено, требует проверки
2. ListAgentTokens возвращает пустой список
3. hermesDB.SaveAgentToken() возвращает ошибка (или hermesDB == nil)
**Отладка:**
```
Android: adb logcat -s "RemoteAgentSettings" "HermesGrpc"
Server: journalctl -u lavender-server-dev -f | grep "HermesAgentService"
```
**Файлы:**
- `RemoteAgentSettingsActivity.kt:173` — generateToken()
- `HermesGrpc.kt:1266` — listAgentTokens()
- `hermes_agent_service.go:259` — GenerateAgentToken()

### P1.2 (решено): Debug логи в production коде
✅ Исправлено в v1.1.3.1 — все логи обёрнуты в BuildConfig.DEBUG / DEBUG env var

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

#### 2.1 ✅ Рефакторинг hermes-agent/
- ✅ generate_token.py — HermesAgentServiceStub вместо ChatServiceStub

#### 2.2 ✅ Убрать token RPC из messenger.proto
- ✅ Убраны RPC из ChatService, оставлены только в HermesAgentService
- ✅ Убраны message types из messenger.proto
- ✅ Удалена дублирующая реализация из server_ai.go
- ✅ Go proto перегенерирован

#### 2.3 ✅ Rate limiting на GenerateAgentToken
- ✅ 5 секунд между запросами на пользователя

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

v1.1.3.1 Android — выпущен 2026-06-14.

Серверная часть v1.1.3.1 — после исправления token flow и P2 задач.

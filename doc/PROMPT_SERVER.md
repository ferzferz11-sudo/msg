# Лава — Промпт для серверных сессий

## Текущий статус

**Версия:** v1.1.3.0 (prod) / v1.1.3.1 (Android выпущен, сервер в процессе)
**Ветка:** feat/1.1.3.x
**Тег:** v1.1.3.0

---

## Контекст

- Сервер: `/root/msg`, dev порт 50052, prod порт 50051
- Android: `/root/msg.client.android`
- Оба репозитория на ветке `feat/1.1.3.x`
- Android v1.1.3.1 — выпущен 2026-06-14
- Сервер v1.1.3.1 — после исправления token flow и P2 задач

---

## Что сделано в v1.1.3.0

### Сервер
- JWT аутентификация для hermes-agent daemon (HS256)
- auth/jwt.go — GenerateAgentToken, ValidateAgentToken
- Таблица agent_tokens в БД (SHA-256 хеш, не сам токен)
- 3 RPC: GenerateAgentToken, RevokeAgentToken, ListAgentTokens
- Token RPC доступен любому пользователю (не только админ)
- validateToken() — полная проверка подписи + expiration + revoked
- Секрет из JWT_SECRET env (32+ байта)
- HermesAgentService.Connect — bidirectional streaming для remote agents
- Remote agent manager (hermes_remote_manager.go)
- Dev и prod обновлены

### Android
- RemoteAgentActivity — чат с агентом, отправка задач
- RemoteAgentSettingsActivity — управление токенами
- TokenDialog — генерация токена
- AIBottomSheet секция "🖥 Агенты"
- listRemoteAgents — реальный gRPC вызов
- Token RPC routing на hermes_agent.HermesAgentService
- CancellationException handling

---

## Что сделано в v1.1.3.1 (Android, 2026-06-14)

### Android
- Убран Toast "Вход выполнен" после авторизации
- Авто-прокрутка вниз при отправке сообщения
- Версия приложения на SplashActivity
- Debug логи обёрнуты в BuildConfig.DEBUG
- Шторка настроек: очистка кэша и журнал ошибок перемещены выше "Удалить профиль"
- "Logs" → "Журнал ошибок"

### Сервер
- Debug логи в hermes_agent_service.go обёрнуты в `os.Getenv("DEBUG")`

---

## Известные проблемы

### P1: Токен не появляется в списке после генерации
**Статус:** Требует отладки
**Симптом:** Пользователь генерирует токен, но список остаётся пустым
**Возможные причины:**
1. JobCancellationException — исправлено в Android, требует проверки
2. ListAgentTokens возвращает пустой список
3. hermesDB == nil на сервере (SaveAgentToken молча пропускается)
4. Ошибка в SaveAgentToken (ON CONFLICT по token_hash)

**Критические файлы для отладки:**
- `hermes_agent_service.go:259` — GenerateAgentToken()
- `hermes_agent_service.go:329` — ListAgentTokens()
- `db_hermes.go:416` — SaveAgentToken()
- `server.go` — проверить инициализацию hermesDB

---

## Задачи для новой сессии

### P1 — Критические
1. **Исправить token flow** — найти root cause почему токен не появляется в списке
   - Проверить инициализацию hermesDB в server.go
   - Добавить возврат ошибки если hermesDB == nil в GenerateAgentToken
   - Протестировать после исправления

### P2 — Важные
2. **✅ Рефакторинг hermes-agent/**
   - ✅ generate_token.py — HermesAgentServiceStub вместо ChatServiceStub

3. **✅ Убрать token RPC из messenger.proto**
   - ✅ Убраны RPC из ChatService, оставлены только в HermesAgentService
   - ✅ Убраны message types из messenger.proto
   - ✅ Удалена дублирующая реализация из server_ai.go

4. **✅ Rate limiting на GenerateAgentToken**
   - ✅ 5 секунд между запросами на пользователя

### P3 — Средние
5. Индикатор "агент не подключён" в Android
6. Кнопка "Скопировать команду" в Android
7. Авто-рефреш списка агентов

### P4 — Низкие
8. Обновить документацию (REMOTE_AGENT.md, AI_SERVICES.md)
9. Создать ARCHITECTURE.md

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

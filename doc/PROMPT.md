# Промпт для новой сессии — v1.1.3.x

## Статус: Сервер v1.1.3.2 (прод обновлён). Ветка feat/1.1.3.x.

## Что сделано в предыдущих сессиях

### Исправлено
- SplashLoadingActivity — добавлена версия приложения
- NotificationActivity — добавлена в AndroidManifest
- RemoteAgentSettingsActivity — сохранение токена в SharedPreferences
- RemoteAgentActivity — @Suppress("UNCHECKED_CAST") для spinner adapter

### Добавлено
- Agent Process Management: StartAgent/StopAgent/GetAgentProcessStatus RPC
- Сервер запускает hermes_remote_agent.py как subprocess (hermes_agent_service.go)
- systemd сервис: scripts/hermes-agent@.service + scripts/deploy_agent.sh
- Health check endpoint (/health) на HTTP сервере
- Graceful shutdown (SIGINT/SIGTERM → GracefulStop)
- Авто-рефреш remote agent статуса (30 сек, repeatOnLifecycle)
- Кнопка "Скопировать команду" в диалоге токена
- Вкладка "Remote" в AgentListActivity
- Кнопки "Запустить/Остановить агента" в RemoteAgentSettingsActivity

## 🔴 Текущая задача (P1 — следующая сессия)

### Генерация токенов не работает
**Симптом**: В настройках удалённого агента при нажатии "Сгенерировать токен" — ничего не происходит. Диалог закрывается, но токен не появляется. Кнопка "Запустить агента" требует генерации.

**Возможные причины**:
1. Сервер возвращает `success = false` (ошибка сохранения в БД)
2. Корутина отменяется до сохранения `selectedToken`
3. `response.token` пустой
4. `userId` пустой (пользователь не залогинен)

**Что проверить**:
- Логирование добавлено в `generateToken()` — проверить logcat на наличие "generateToken CALLED", "generateToken response", "Token saved"
- Проверить что `userId` не пустой при вызове `generateToken()`
- Проверить что `response.success = true` и `response.token` не пустой
- Проверить что `saveSelectedAgent()` вызывается ДО `showTokenResultDialog()`

**Файлы для отладки**:
- `RemoteAgentSettingsActivity.kt:216` — метод `generateToken()`
- `RemoteAgentSettingsActivity.kt:378` — метод `startAgentOnServer()`
- `HermesGrpc.kt:1160` — метод `generateAgentToken()` (gRPC вызов)
- `hermes_agent_service.go:296` — серверный `GenerateAgentToken()`

## Критические файлы

### Сервер
- hermes_agent_service.go — Agent Process Management + Token RPCs
- hermes_remote.proto — обновлённый proto (StartAgent, StopAgent, GetAgentProcessStatus)
- main.go — graceful shutdown
- http_server.go — /health endpoint
- auth/jwt.go — GenerateAgentToken, ValidateAgentToken
- scripts/deploy_agent.sh — управление агентом через systemd
- scripts/hermes-agent@.service — systemd unit

### Android
- HermesGrpc.kt — все unary RPC методы (generate, revoke, list, start, stop, status)
- GrpcClient.kt — facade
- RemoteAgentSettingsActivity.kt — UI управления агентом (токены + запуск)
- RemoteAgentActivity.kt — чат с агентом
- MessengerProto.kt — hand-written proto типы

## Правила
- НЕ assembleRelease на сервере (OOM)
- НЕ compileDebugKotlin без крайней необходимости
- Proto gen: --go_out=gen (НЕ .) — иначе двойная вложенность LavenderMessenger/gen/hermes_agent/
- Коммитить/пушить после каждого изменения
- Версию не менять без явного указания пользователя

## Документация
- doc/INDEX.md → doc/TASKS.md → doc/PROMPT.md
- doc/RELEASE.md — выпуск релизов сервера

## Скиллы
- lavender-messenger (корневой)
- lavender-messenger:lavender-android для Android-работы

## Выпуск релиса

doc/RELEASE.md — полная документация по процессу.

Быстрый старт:
```bash
# С серверa (где OWL):
./scripts/release.sh 1.1.3.3 --deploy

# С Mac (удалённо):
./scripts/release.sh 1.1.3.3 --deploy --remote
```

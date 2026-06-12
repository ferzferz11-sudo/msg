# Промпт для новой сессии — v1.1.3.x

## Статус: Сервер v1.1.3.1 и Android v1.1.3.1 — в разработке. Ветка feat/1.1.3.x.

## Что сделано

### Исправлено
- SplashLoadingActivity — добавлена версия приложения
- NotificationActivity — добавлена в AndroidManifest
- RemoteAgentSettingsActivity — сохранение токена в SharedPreferences

### Добавлено
- Agent Process Management: StartAgent/StopAgent/GetAgentProcessStatus RPC
- Сервер запускает hermes_remote_agent.py как subprocess
- systemd сервис для агента (scripts/hermes-agent@.service, scripts/deploy_agent.sh)
- Health check endpoint (/health)
- Graceful shutdown для gRPC сервера
- Авто-рефреш remote agent статуса (30 сек)
- Кнопка "Скопировать команду" в диалоге токена
- Вкладка "Remote" в AgentListActivity

## Текущие задачи (приоритет)

### 1. Remote Agent — деплой и подключение
- Проверить что hermes_remote_agent.py корректно запускается с сервера
- Проверить что StartAgent RPC работает (запуск агента из приложения)
- Проверить что агент подключается к серверу через Connect stream
- Проверить что задачи (shell, git, build) выполняются агентом

### 2. P2 — Agent flow в Android
- Индикатор "агент не подключён" в RemoteAgentActivity — проверить работу
- Кнопка "Скопировать команду" — проверить формат команды
- Авто-рефреш списка агентов (30 сек) — проверить
- Объединение AgentListActivity + RemoteAgentActivity — проверить навигацию

### 3. P3 — Сервер
- Health check endpoint — проверить ответ
- Graceful shutdown — проверить что сервер корректно останавливается
- Structured loading — добавить прогресс-бар загрузки

## Критические файлы

### Сервер
- hermes_agent_service.go — Agent Process Management RPC
- hermes_remote.proto — обновлённый proto
- main.go — graceful shutdown
- http_server.go — /health endpoint
- scripts/deploy_agent.sh — управление агентом
- scripts/hermes-agent@.service — systemd unit

### Android
- HermesGrpc.kt — новые RPC методы
- GrpcClient.kt — facade для новых методов
- RemoteAgentSettingsActivity.kt — UI управления агентом
- RemoteAgentActivity.kt — чат с агентом
- MessengerProto.kt — новые proto типы

## Правила
- НЕ assembleRelease на сервере (OOM)
- НЕ compileDebugKotlin без крайней необходимости
- Proto gen: --go_out=gen (НЕ .)
- Коммитить/пушить после каждого изменения
- Версию не менять без явного указания пользователя

## Документация
- doc/INDEX.md → doc/TASKS.md → doc/PROMPT.md

## Скиллы
- lavender-messenger (корневой)
- lavender-messenger:lavender-android для Android-работы

# Промт для новой сессии — синхронизация и релиз оркестратора

## ТЕКУЩЕЕ СОСТОЯНИЕ

### Ветки:
- **main** (продакшн) — сервер v1.1.0.0, без Hermes
- **feat/1.1.0.x** (dev) — сервер v1.1.0.9 + Hermes + Android v1.1.0.10

### Что сделано на dev:
1. **Hermes Multi-Agent Orchestrator** — полностью реализован:
   - hermes_orchestrator.go: оркестратор с режимами single/parallel/pipeline
   - hermes_agent_service.go: gRPC сервис для удалённых агентов
   - hermes_remote_manager.go: реестр удалённых агентов, health check, отправка задач
   - db_hermes.go: миграции + CRUD для hermes_messages, hermes_sessions, hermes_agent_runs
   - agent/: hermes-agent daemon (подключение, регистрация, heartbeat, выполнение задач)
   - cli/: hermes CLI клиент
   - 8 пресетов агентов (developer, devops, architect, support, qa, analyst, security, custom)
   - Rate limiting: 10 req/мин на пользователя

2. **Исправления на сервере**:
   - hermes-owl агент зарегистрирован в реестре (fallback при ошибках OpenRouter)
   - UNIQUE constraint на reactions(message_id, username)
   - is_protected для серверов (dev сервер защищён от удаления)
   - Dev server infrastructure (.env.dev, deploy-dev.sh, dev-connect.sh)

3. **Android v1.1.0.10**:
   - Hermes AI в chat action sheet
   - CredentialStore для хранения серверов
   - Исправления compile errors (HermesGrpc.kt, AgentSettingsActivity)
   - ChangelogActivity fix (белый экран, таймаут)
   - Server switch с подтверждением (AlertDialog)
   - **Agent Settings Bottom Sheet** — шторка настроек агента по long press

### Что работает на dev ✅
- Оркестратор отвечает через dev сервер (порт 50052)
- Stream SSE работает
- Агенты маршрутизируются автоматически
- Favorites отображаются корректно (исправлен SQL баг)

## ЧТО НУЖНО ДЕЛАТЬ

### 1. Синхронизировать сервер → продакшн
Все изменения из `feat/1.1.0.x` перенести на `main`:
```bash
git checkout main
git merge feat/1.1.0.x
```
Проверить что ничего не конфликтует. Собрать, задеплоить на prod.

### 2. Синхронизировать Android → продакшн
Аналогично — merge ветки с Android изменениями.

### 3. Доделать Hermes (не завершено):

**3a. Welcome message для оркестратор**
- При первом сообщении в новой сессии отправлять инструкцию с списком агентов
- Код был написан но сломал сборку (streamOpenRouter не найден)
- Нужно реализовать правильно

**3b. Agent Settings Bottom Sheet — тестирование**
- Long press на кастомном агенте → шторка с полями (имя, промпт, модель, maxTokens)
- Сохранение через updateAgent RPC
- Удаление через deleteAgent RPC
- Кнопка «Удалить» только для кастомных агентов (!isPreset)
- ⚠️ AgentInfo не содержит systemPrompt/model/maxTokens — поля пустые при открытии

**3c. HermesChatActivity — полноценный чат**
- Отправить/получить сообщения оркестратора
- Streaming отображение токенов
- История сообщений

**3d. HermesSessionsActivity — список сессий**
- Создание новой сессии
- Удаление сессии
- Переход в чат

### 4. Тестирование на dev
- Собрать Android APK из последнего feat/1.1.0.x
- Протестировать шторку настроек агента
- Протестировать чат с оркестратором (welcome message)
- Проверить все flow: создание агента → редактирование → удаление

### 5. Релиз на продакшн
После тестирования на dev:
- Merge feat/1.1.0.x → main (сервер)
- Merge Android ветки
- Собрать prod бинарник + APK
- Обновить changelog

## ВАЖНО
- Сервер: 2GB RAM, Gradle OOM — собирать только локально
- Dev сервер: /root/LavenderMessenger/run/lavender-server-dev (.env.dev с ключами)
- Prod сервер: /root/LavenderMessenger/run/lavender-server (.env)
- Не коммитить секреты (.env, firebase keys, release.keystore)
- CHAT_SECRET_KEY = "passphrasewhichneedstobe32bytes!" (32 символа)
- watch-services.sh — cron каждые 15 минут проверяет 4 сервиса

## ФАЙЛЫ
- Сервер: /root/msg/ (git repo)
- Android: /root/msg.client.android/ (git repo)
- Dev .env: /root/LavenderMessenger/run/.env.dev
- Prod .env: /root/LavenderMessenger/run/.env

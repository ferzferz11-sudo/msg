# Промпт для новой сессии — v1.1.3.9 (stable)

**Дата:** 2026-06-13 (updated)
**Версия:** v1.1.3.10
**Ветка:** feat/1.1.3.x

---

## СТАТУС: v1.1.3.10 — СТАБИЛЬНАЯ ВЕРСИЯ

Прод и dev серверы обновлены. Android релиз v1.1.3.10 выпущен.

---

## АРХИТЕКТУРА

### Сервер (/root/msg)
```
main.go                    — Entry point, gRPC server, graceful shutdown
server.go                  — Структура server, общие методы (ServerVersion = "1.1.3.9")
server_*.go                — Методы по доменам
server_remote.go           — Remote Agent RPC (DeployAgentTaskStream fix: single done=True)
hermes_remote_manager.go   — HandleTaskStream + StreamDone flag
server_remote_test.go      — 6 unit-тестов для streaming
ai_chat_manager.go         — Единый менеджер AI чатов
owl.go                     — OWL AI: streaming через OpenRouter API
hermes_orchestrator.go     — Hermes: оркестрация агентов
hermes_agent_service.go    — HermesAgentService: Connect, tokens
http_server.go             — HTTP сервер (/files, /avatars, /health)
db.go / db_hermes.go       — Database layer
messenger.proto            — ChatService, Remote Agent RPC
```
auth_service.go            — AuthService (SignIn, SignUp)
jwt.go                     — JWT генерация/валидация
bot_commands.go            — Bot Commands: /status, /deploy, /logs, /restart, /ai
messenger.proto            — ChatService, AuthService, AI Chat, Remote Agent RPC
hermes_remote.proto        — HermesAgentService
```

---

## КЛЮЧЕВЫЕ РЕШЕНИЯ

### Сервер
- **server_remote.go** — все Remote Agent RPC вынесены из server_ai.go в отдельный файл
- **ensureRemoteManager()** — единая проверка зависимостей для всех Remote Agent RPC
- **Graceful degradation** — ListRemoteAgents возвращает пустой список если менеджер недоступен
- **Stale detection** — heartbeat > 120с → status="stale"
- **DeployAgentTask/Stream** — проверка существования агента перед отправкой

### Android (ключевые изменения)
- **Нет выбора сервера в логине** — сервер всегда из CredentialStore (по умолчанию prod)
- **Переключение сервера** — только через ServersActivity
- **ErrorHandler.kt** — единый обработчик ошибок с AppLog
- **Room DB version 9** — defensive migration

---

## ПРАВИЛА

1. НЕ компилировать на сервере (OOM kill)
2. Коммитить и пушить после каждого значимого изменения
3. Версия сервера в `server.go:34`, версия Android в `version.txt`
4. Разделение архитектуры — каждый домен в своём server_*.go файле
5. userId (UUID) — всегда как ключ, НЕ username
6. changelog.txt БОЛЬШЕ НЕ ИСПОЛЬЗУЕТСЯ
7. Agent tokens: в БД хранится SHA-256 хеш, не сам токен
8. JWT секрет: минимум 32 байта, НЕ коммитить

---

## КОМАНДЫ

```bash
# Сборка и деплой на dev
cd /root/msg && export PATH=$PATH:/usr/local/go/bin:~/go/bin
go build -o /tmp/lavender-server-dev .
systemctl stop lavender-server-dev
cp /tmp/lavender-server-dev /root/LavenderMessenger/run/lavender-server-dev
systemctl start lavender-server-dev

# Сборка и деплой на prod
go build -o /tmp/lavender-server .
systemctl stop lavender-server
cp /tmp/lavender-server /root/LavenderMessenger/run/lavender-server
systemctl start lavender-server

# Proto gen
protoc --go_out=./gen --go_opt=paths=source_relative \
  --go-grpc_out=./gen --go-grpc_opt=paths=source_relative messenger.proto

# Тесты
go test ./...

# Android (НЕ компилировать на сервере!)
cd /root/msg.client.android
# assembleRelease ТОЛЬКО локально!
```

---

## DEV vs PROD

| Характеристика | Dev | Prod |
|----------------|-----|------|
| Порт | 50052 | 50051 |
| Сервис | lavender-server-dev | lavender-server |
| Конфиг | .env.dev | .env |
| DB | chat_db_dev | chat_db |

---

## ДОКУМЕНТАЦИЯ

- Индекс: `/root/msg/doc/INDEX.md`
- Сервер: `/root/msg/doc/INTEGRATION_SESSION.md`, `/root/msg/doc/TASKS.md`
- Android: `/root/msg.client.android/doc/TASKS.md`, `/root/msg.client.android/doc/PROMPT_ANDROID.md`
- AI сервисы: `/root/msg/doc/AI_SERVICES.md`
- Подводные камни: `/root/msg/doc/PITFALLS.md`
- Remote Agent: `/root/msg.client.android/doc/REMOTE_AGENT.md`
- CHANGELOG: `/root/msg/CHANGELOG.md` (сервер), `/root/msg.client.android/CHANGELOG.md` (Android)

---

## ИЗВЕСТНЫЕ ПРОБЛЕМЫ

- Streaming end-to-end работает (проверено в v1.1.3.10)
- Server migration warnings: `role "lavender" does not exist` (не критично)

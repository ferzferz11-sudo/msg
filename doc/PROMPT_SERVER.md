# Lavender Messenger — Промпт для серверных сессий

## Текущий статус

**Версия:** v1.1.2.6 (prod)
**Ветка:** feat/1.1.2.x
**Тег:** v1.1.2.6 (выпущен)

---

## Контекст

- Сервер: /root/msg, dev порт 50052, prod порт 50051
- Android: /root/msg/client.android
- Оба репозитория на ветке feat/1.1.2.x
- v1.1.2.6 — prod версия (JWT auth для удалённых агентов)

---

## Что сделано в v1.1.2.6

- JWT аутентификация для hermes-agent daemon (HS256)
- auth/jwt.go — GenerateAgentToken, ValidateAgentToken
- Таблица agent_tokens в БД (SHA-256 хеш, не сам токен)
- 3 admin RPC: GenerateAgentToken, RevokeAgentToken, ListAgentTokens
- validateToken() — полная проверка подписи + expiration + revoked
- Секрет из JWT_SECRET env (32+ байта)
- Dev и prod обновлены

---

## Архитектура

```
СЕРВЕР:
├── server.go           — gRPC handlers, маршрутизация, rate limiting
├── owl.go              — OWL AI: ChatWithOWL streaming, сессии, история
├── bot_commands.go     — Bot Commands: /status, /deploy, /logs, /restart, /ai, /help, /version
├── hermes_orchestrator.go — Hermes: оркестратор, маршрутизация агентов
├── hermes_agent_service.go — Hermes: управление агентами (gRPC для hermes-agent daemon)
├── hermes_remote_manager.go — менеджер удалённых агентов
├── ai_chat_manager.go  — единый менеджер AI чатов (OWL + Hermes)
├── auth/jwt.go         — JWT генерация и валидация для удалённых агентов
└── db_hermes.go        — миграции и CRUD для Hermes + agent_tokens
```

---

## Бэклог (приоритет)

1. **Favorites при пустом списке** (Android) — высокий
2. **Модульные тесты для OWL streaming** — средний
3. **Qdrant + CLIP (production RAG)** — низкий, ночная задача

---

## Правила

- Коммитить после каждого значимого изменения, пушить в feat/1.1.2.x
- При каждом релизе: git tag, CHANGELOG.md, version.txt
- assembleRelease НЕ запускать на сервере (OOM kill)
- userId (UUID) — всегда как ключ, НЕ username
- creator_id (UUID) — для проверки владельца
- participants ВСЕГДА через json.Marshal, никогда вручную
- Proto поля: всегда сверять номера полей с messenger.proto!
- JWT секрет: JWT_SECRET в .env, минимум 32 байта, НЕ коммитить
- Agent tokens: в БД хранится SHA-256 хеш, не сам токен

---

## Команды

```bash
# Сборка и деплой на dev
cd /root/msg
export PATH=$PATH:/usr/local/go/bin:~/go/bin
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
cd /root/msg && protoc --go_out=./gen --go_opt=paths=source_relative \
  --go-grpc_out=./gen --go-grpc_opt=paths=source_relative messenger.proto
cd /root/msg && mkdir -p gen/hermes_agent && protoc --go_out=gen/hermes_agent \
  --go_opt=paths=source_relative --go-grpc_out=gen/hermes_agent \
  --go-grpc_opt=paths=source_relative hermes_remote.proto

# Android
cd /root/msg/client.android
./gradlew compileDebugKotlin
```

---

## Документация (читать в начале каждой сессии)

- Индекс: /root/msg/doc/INDEX.md
- Сервер: /root/msg/doc/INTEGRATION_SESSION.md, /root/msg/doc/TASKS.md
- Android: /root/msg/client.android/doc/TASKS.md
- AI сервисы: /root/msg/doc/AI_SERVICES.md
- Подводные камни: /root/msg/doc/PITFALLS.md
- Changelog: /root/msg/doc/CHANGELOG.md
- Memory pad: /root/.hermes/memory/pad.md

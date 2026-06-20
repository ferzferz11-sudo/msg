# Lava Messenger — Интеграционная сессия

**Текущая версия:** v1.3.0.11 (сервер prod/dev)
**Обновлено:** 2026-06-20
**Ветка сервера:** feat/1.3.0.x

---

## Текущий статус

| | Версия | Статус |
|---|--------|--------|
| **Сервер prod** | v1.3.0.11 | ✅ Работает на порту 50051 |
| **Сервер dev** | v1.3.0.11 | ✅ Работает на порту 50052 |

**Серверная часть завершена (50/50 задач).**

---

## Ключевые фичи

- **Auth:** JWT (access+refresh), device management, token rotation
- **Chat:** Bidirectional streams, typing, WebRTC calls, conference
- **ChatList v2:** Pin/Unpin, Archive, Search, unread counts, pagination
- **AI v2:** 7 providers, 6 tools, 8 presets, marketplace, usage stats, RAG (Qdrant+OpenAI)
- **E2EE:** Secret chats with AES-256-GCM
- **Push:** FCM batch, auto-cleanup invalid tokens
- **Rate limiting:** Redis-backed, per-agent configurable
- **Graceful shutdown:** SERVER_SHUTTINGDOWN broadcast, health 503

---

## Правила работы

1. Версия сервера в `server.go`
2. userId (UUID) — всегда как ключ, НЕ username
3. Auth context → `GetUserID(ctx)`, NEVER `req.UserId`
4. DB миграции: `IF NOT EXISTS`, NEVER `DROP`
5. Коммитить после каждого изменения
6. **Стабильность > фичи** — деплоим на prod, ошибки критичны
7. Android собирается ТОЛЬКО локально (нет памяти на сервере)

---

## Команды

```bash
# Деплой dev (с локальной машины)
./scripts/deploy-dev-local.sh

# Деплой prod (с локальной машины)
./scripts/deploy-prod-local.sh

# Proto gen
PATH=$PATH:~/go/bin protoc --go_out=gen --go_opt=paths=source_relative \
  --go-grpc_out=gen --go-grpc_opt=paths=source_relative messenger.proto

# Тесты
go test ./...
```

---

## DEV vs PROD

| | Dev | Prod |
|---|-----|------|
| Порт gRPC | 50052 | 50051 |
| Порт HTTP | 8083 | 8082 |
| DB | chat_db_dev | chat_db |
| Config | .env.dev | .env |

# Лава — Промпт для серверных сессий

## Текущий статус

**Версия:** v1.2.0.1
**Ветка:** feat/1.2.0.x
**Тег:** v1.2.0.1

---

## Контекст

- Сервер: `/root/msg`, dev порт 50052, prod порт 50051
- Android: `/root/msg.client.android`
- Remote Agent: `/root/msg.remote.agent`

---

## Что сделано (v1.2.0.1)

### Pin Message
- messenger.proto: PinMessage/UnPinMessage/GetPinnedMessages RPC
- db_chatlist_v2.go: pinned_messages table, PinnedMessageRow, CRUD методы
- server_chatlist_v2.go: RPC handlers (PinMessage, UnPinMessage, GetPinnedMessages)
- Все RPC используют только userId (без username)
- Валидация: пользователь должен быть участником чата
- protoc генерация выполнена

### Предыдущие версии
- ChatStream v2 (JWT auth)
- ChatList v2 (PinChat, SearchChats, ArchiveChat)
- ProfileService v2
- AuthService v2 (JWT)

---

## Правила

1. НЕ компилировать на сервере (OOM kill)
2. НЕ деплоить новую версию на prod без прямого указания ферзя
3. Использовать только userId (UUID), НЕ username в RPC
4. Все данные о пользователе через ProfileService v2
5. При смене сервера клиент очищает локальный кэш (CacheUtils.clearAllSync)
6. Форматирование строк: позиционные форматтеры (%1$s, %2$d)
7. Все серверы (включая dev) доступны всем пользователям

---

## Структура файлов

```
main.go                    — Entry point, gRPC server, graceful shutdown
server.go                  — ServerVersion = "1.2.0.1", service version constants
auth_service.go            — AuthService v1 (deprecated)
auth_service_v2.go         — AuthService v2 (JWT, основной)
auth_interceptor.go        — gRPC Bearer token interceptor
auth_jwt.go                — JWT генерация/валидация
db_auth_devices.go         — CRUD для user_devices + device_auth_log
db_auth_migrations.go      — миграция таблиц
db_chatlist_v2.go          — ChatList v2 + Pin Message DB methods
server_profile_v2.go       — ProfileService v2 (JWT, dev only)
server_chatlist_v2.go      — ChatList v2 + Pin Message RPC handlers
server_chat.go             — Chat stream v2 (JWT + password)
server_remote.go           — Remote Agent RPC
hermes_remote_manager.go   — HandleTaskStream
ai_chat_manager.go         — AI чаты
owl.go                     — OWL AI
hermes_orchestrator.go     — Hermes Orchestrator
http_server.go             — HTTP (/health, /info)
messenger.proto            — ChatService v2, AuthService v2, ProfileService v2, Pin Message
```

---

## Команды

```bash
cd /root/msg && export PATH=$PATH:/usr/local/go/bin:~/go/bin

# Proto gen (обязательно после изменений!)
protoc --go_out=gen --go_opt=paths=source_relative --go-grpc_out=gen --go-grpc_opt=paths=source_relative messenger.proto

# Сборка и деплой на dev
go build -o /tmp/lavender-server-dev .
systemctl stop lavender-server-dev
cp /tmp/lavender-server-dev /root/LavenderMessenger/run/lavender-server-dev
systemctl start lavender-server-dev

# Сборка и деплой на prod (НЕ делать без тестирования на dev!)
go build -o /tmp/lavender-server .
systemctl stop lavender-server
cp /tmp/lavender-server /root/LavenderMessenger/run/lavender-server
systemctl start lavender-server

# Тесты
go test ./...

# Логи
journalctl -u lavender-server-dev -f
journalctl -u lavender-server -f
```

---

## DEV vs PROD

| Характеристика | Dev | Prod |
|----------------|-----|------|
| Порт gRPC | 50052 | 50051 |
| Порт HTTP | 8083 | 8082 |
| Имя | Lava Germany dev | Lava Germany |
| Сервис | lavender-server-dev | lavender-server |
| Конфиг | .env.dev | .env |
| DB | chat_db_dev | chat_db |
| Версия | v1.2.0.1 | v1.1.3.10 |

---

## Документация

- Индекс: `/root/msg/doc/INDEX.md`
- CHANGELOG: `/root/msg/CHANGELOG.md`
- Android: `/root/msg.client.android/doc/INDEX.md`
- Android PROMPT: `/root/msg.client.android/doc/PROMPT_ANDROID.md`

---

## ПРИОРИТЕТЫ СЛЕДУЮЩЕЙ СЕССИИ

### Высокий приоритет
1. **Тестирование Pin Message** — RPC на dev сервере
2. **Тестирование Android v1.1.3.17** — FAB AI, AIBottomSheet

### Средний приоритет
3. **Read receipts (MarkAsRead)** — если нужно на сервере
4. **Prometheus метрики** — мониторинг

### Отложено
- Qdrant + CLIP (production RAG)
- Редеплой prod сервера — только после выхода Android клиента

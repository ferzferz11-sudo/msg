# Лава — Pitfalls (Сервер)

Подводные камни и известные проблемы сервера. Читать перед началом работы!

**Обновлено:** 2026-06-18

---

## Server

### Структура файлов
- server.go — структура server, общие методы (resolveDisplayName, logErrorOnce, logFCM)
- server_*.go — методы по доменам (chat, users, chats, messages, profile, push, contacts, themes, drafts, muted, favorites, ai)
- При добавлении новых методов — класть в соответствующий server_*.go файл
- Не добавлять методы напрямую в server.go (только структура и общие утилиты)

### hermes_sessions owner
- Таблица должна принадлежать `lavender`, не `postgres`
- Исправление: `cd /tmp && sudo -u postgres psql -d chat_db -c "ALTER TABLE hermes_sessions OWNER TO lavender;"`

### getOrCreateSession создаёт дубли
- Старое поведение: создавал сессию с `id = "hermes-" + userID` каждый раз
- Исправлено: ищет существующую сессию по `user_id`

### JSON в SQL
- Никогда не собирайте JSON через конкатенацию: `"["+username+"]"` → невалидный JSON
- Всегда `json.Marshal`

### DeleteChat и AI чаты
- DeleteChat удаляет из chats, но НЕ из hermes_sessions (orphaned sessions копятся)
- Исправлено в v1.1.2.2: каскадное удаление из hermes_sessions + hermes_messages
- Для OWL: FK CASCADE на owl_messages срабатывает, но owl_chat_settings — нет, нужно явное удаление

### Prod vs Dev
- Dev: порт 50052, DB `chat_db_dev`, config `.env.dev`
- Prod: порт 50051, DB `chat_db`, config `.env`
- Версия сервера в `server.go:33`

### Hermes история — всегда из БД
- `GetOrchestratorHistory` должен загружать из `hermes_messages` через `HermesDB.GetOrchestratorHistory()`
- НЕ использовать `session.Messages` (in-memory) — пропадает после рестарта сервера
- `getOrCreateSession` создаёт пустую сессию без загрузки истории из БД

### Rate limiter — refund on failure
- `allow()` добавляет timestamp ДО выполнения запроса
- При ошибке (OpenRouter, orchestrator) timestamp остаётся — слот потерян
- **Правило:** всегда вызывать `cancel(userID)` в failure path после успешного `allow()`
- `remaining()` возвращает `limit - len(valid)` — корректно отражает оставшиеся запросы

### /dev/null сломан после OOM
- Если `/dev/null` стал файлом вместо device node: `rm /dev/null && mknod /dev/null c 1 3 && chmod 666 /dev/null`
- Без этого `go build` падает с "open /dev/null: no such file or directory"

---

## JWT Agent Auth

### Секретный ключ
- `JWT_SECRET` — минимум 32 байта, хранится в `.env` / `.env.dev`
- Никогда не коммитить в git!
- При компрометации — немедленно перегенерировать все токены

### Валидация токена
- `validateToken()` проверяет: HMAC подпись, expiration, agent_id match, revoked в БД
- Пустой токен = отклонение (нет backward compat с неавторизованными агентами)
- Для тестирования без токена — нужно явно создать токен через `GenerateAgentToken`

### Хранение
- В БД хранится только SHA-256 хеш токена, не сам токен
- Токен показывается клиенту только один раз при генерации
- `RevokeAgentToken` — помечает `revoked = TRUE`, существующие подключения продолжают работать до реконнекта

### Admin-only
- `GenerateAgentToken`, `RevokeAgentToken`, `ListAgentTokens` — требуют `IsSuperAdmin()`
- `admin_user_id` в запросе должен совпадать с супер-админом в БД

---

## Remote Agent

### DeployAgentTaskStream — стриминг результатов

**Поток данных:**
```
Агент → AGENT_TASK_STREAM_UPDATE(done=False) → onStream → streamCh → клиент
Агент → AGENT_TASK_STREAM_UPDATE(done=True)  → streamDone flag, continue (НЕ отправляем клиенту)
Агент → AGENT_TASK_RESULT                    → onResult → close(streamCh)
Сервер → клиент: один done=True с полными Stdout/Stderr/ExitCode/DurationMs
```

**Правила:**
- При `done=True` от агента — НЕ отправлять `done=True` клиенту сразу
- Ставить флаг `streamDone = true`, `continue`
- После закрытия `streamCh` (от `onResult`) — отправить **один** `done=True` с полными данными
- НЕ использовать таймаут ожидания TaskResult — ждать через закрытие канала

### Token RPC маршрутизация

- `GenerateAgentToken`, `RevokeAgentToken`, `ListAgentTokens` — методы `HermesAgentService` (hermes_remote.proto), НЕ `ChatService`
- Полное имя: `hermes_agent.HermesAgentService/GenerateAgentToken`

### IsSuperAdmin убран (v1.1.3.0)

- Начиная с v1.1.3.0, token RPC доступны любому авторизованному пользователю
- Remote agents запускаются на сервере пользователя, не на нашем

---

## Dev Server Management

### Systemd service
- **Файл:** `/etc/systemd/system/lavender-server-dev.service`
- **НЕ редактировать напрямую** — использовать `sudo tee`:
  ```bash
  sudo tee /etc/systemd/system/lavender-server-dev.service > /dev/null << 'EOF'
  [Unit]
  Description=Lavender Messenger Server — DEV
  After=network.target postgresql.service
  Wants=postgresql.service

  [Service]
  Type=simple
  WorkingDirectory=/root/LavenderMessenger/run
  ExecStart=/root/LavenderMessenger/run/lavender-server-dev
  Restart=always
  RestartSec=5
  Environment=APP_ENV=dev

  [Install]
  WantedBy=multi-user.target
  EOF
  ```
- После изменения: `sudo systemctl daemon-reload && sudo systemctl restart lavender-server-dev`

### Environment files
- **Dev config:** `/root/LavenderMessenger/run/.env.dev` — загружается автоматически при `APP_ENV=dev`
- **Prod config:** `/root/LavenderMessenger/run/.env`
- **НЕ коммитить** .env файлы — содержат секреты (JWT_SECRET, DB credentials, API keys)
- Формат: `KEY=value` (без кавычек, без пробелов вокруг `=`)

### Common issues
- **`missing port in address`** — значит `SERVER_ADDRESS` не загрузился из .env. Проверить:
  1. `APP_ENV=dev` установлен в systemd service
  2. `.env.dev` существует и содержит `SERVER_ADDRESS=0.0.0.0:50052`
  3. Нет старого `Environment=SERVER_ADDRESS=***` в systemd service
- **Panic после `failed to listen`** — баг в main.go: нет `return` после ошибки `net.Listen`. Исправлено в v1.2.0.1+
- **Text file busy** при копировании бинарника — сначала `systemctl stop`, потом `kill -9 <PID>`, потом копировать

### Server info endpoint
- `GET http://host:8082/info` — возвращает версии сервисов
- Используется Android клиентом для capability negotiation
- `services.auth >= "2.0"` → JWT workflow, иначе legacy

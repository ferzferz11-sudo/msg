# Лава — Pitfalls (Сервер)

Подводные камни и известные проблемы сервера. Читать перед началом работы!

**Обновлено:** 2026-08-01

---

## Server

### Структура файлов
- server.go — структура server, общие методы (resolveDisplayName, logErrorOnce)
- server_*.go — методы по доменам (chat, users, chats, messages_v2, profile_v2, push, contacts, themes, drafts, muted, saved_messages, ai, company)
- При добавлении новых методов — класть в соответствующий server_*.go файл
- Не добавлять методы напрямую в server.go (только структура и общие утилиты)
- v1 совместимость удалена: только ChatV2 stream, ProfileService v2, Messages v2

### hermes_sessions owner
- Таблица должна принадлежать `lavender`, не `postgres`
- Исправление: `sudo -u postgres psql -d chat_db -c "ALTER TABLE hermes_sessions OWNER TO lavender;"`

### JSON в SQL
- Никогда не собирайте JSON через конкатенацию: `"["+username+"]"` → невалидный JSON
- Всегда `json.Marshal`

### Prod vs Dev
- Dev: порт 50052, DB `chat_db_dev`, config `.env.dev`
- Prod: порт 50051, DB `chat_db`, config `.env`
- Версия сервера в `server.go`

### Rate limiter — refund on failure
- `allow()` добавляет timestamp ДО выполнения запроса
- При ошибке timestamp остаётся — слот потерян
- **Правило:** всегда вызывать `cancel(userID)` в failure path после успешного `allow()`

### /dev/null сломан после OOM
- Если `/dev/null` стал файлом вместо device node: `rm /dev/null && mknod /dev/null c 1 3 && chmod 666 /dev/null`
- Без этого `go build` падает с "open /dev/null: no such file or directory"

---

## Company System

### Множественные компании
- Пользователь может быть в нескольких компаниях одновременно
- `UNIQUE(company_id, user_id)` — одна позиция на компанию, но unlimited компаний на пользователя
- `primary_company_id` на users определяет какая компания показывается в профиле
- **Fallback**: если primary_company_id не задан, GetProfile показывает компанию с самым высоким уровнем позиции

### Chat filtering
- `company_chats.access_level` + `min_position_level` определяют видимость чата
- Фильтрация идёт по позиции пользователя **в ЭТОЙ компании**, не по primary
- Тип чата: `type="company"` в таблице `chats`

### Builtin позиции
- Owner (3), Top Manager (2), Manager (1), Employee (0) — нельзя удалить
- Проверка: `level >= 0 && level <= 3 && title in (Owner, Top Manager, Manager, Employee)`

### Owner constraints
- Владелец не может покинуть компанию (нужно transfer ownership)
- Владелец не может быть удалён из компании
- Только владелец может удалить компанию

---

## JWT Auth

### Секретный ключ
- `JWT_SECRET` — минимум 32 байта, хранится в `.env` / `.env.dev`
- Никогда не коммитить в git!
- При компрометации — немедленно перегенерировать все токены

### Хранение
- В БД хранится только SHA-256 хеш токена, не сам токен
- Токен показывается клиенту только один раз при генерации

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
- После закрытия `streamCh` — отправить **один** `done=True` с полными данными

---

## Self-Destruct Timer

### Timer values
- Только `0, 30, 60, 300, 3600, 86400` — другие значения отклоняются handler'ом
- `0` = выключен (default)

### Background cleanup
- Goroutine проверяет каждые 30 секунд
- Удаляет сообщения старше `self_destruct_timer` секунд
- Перед удалением записывает в `deleted_messages` (для persistence)
- Широковещает `DELETE_MESSAGE_V2` для каждого удалённого сообщения

### deleted_messages table
- Хранит ID удалённых сообщений (physical delete + tracking)
- `GetHistoryV2` фильтрует по этой таблице — удалённые не появляются при перезагрузке
- Cleanup: записи старше 30 дней удаляются автоматически (каждый час)

### DeleteMessageV2 persistence
- Теперь при удалении сообщения сначала записывается в `deleted_messages`, потом физически удаляется
- Это исправляет баг: ранее удалённые сообщения появлялись при GetHistoryV2

---

## Dev Server Management

### Systemd service
- **Файл:** `/etc/systemd/system/lavender-server-dev.service`
- **НЕ редактировать напрямую** — использовать `sudo tee`
- После изменения: `sudo systemctl daemon-reload && sudo systemctl restart lavender-server-dev`

### Environment files
- **Dev:** `/root/LavenderMessenger/run/.env.dev`
- **Prod:** `/root/LavenderMessenger/run/.env`
- **НЕ коммитить** .env файлы — содержат секреты

### Server info endpoint
- `GET http://host:8082/info` — возвращает версии сервисов
- Используется клиентами для capability negotiation

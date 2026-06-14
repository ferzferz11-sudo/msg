# Заметки сессии 8 — 2026-06-14 (сервер)

## Что сделано

### Миграция UNIQUE constraint на user_devices
- Добавлена миграция в `db_auth_devices.go` — `DO $$` блок который добавляет UNIQUE constraint если его нет
- Проблема: `CREATE TABLE IF NOT EXISTS` не пересоздаёт таблицу без UNIQUE constraint на существующих инсталляциях
- Решение: `ALTER TABLE ... ADD CONSTRAINT ... UNIQUE (user_id, device_id)` через `DO $$` блок

### Деплой на prod
- Собран и деплоен бинарник v1.2.0.1 на prod
- Prod сервер: `Listening clients at [::]:50051`, `HTTP server started on port 8082`

## Известные проблемы

### 42P10 ошибка на prod (НЕ исправлена)
- `Failed to register device ... pq: there is no unique or exclusion constraint matching the ON CONFLICT specification (42P10)`
- Причина: таблица `user_devices` на prod создана **до** добавления UNIQUE constraint
- Миграция добавлена в код, но на prod сервере таблица уже существует без UNIQUE
- **Нужно**: вручную выполнить `ALTER TABLE user_devices ADD CONSTRAINT ... UNIQUE (user_id, device_id)` на prod БД
- Это **не критично** — пользователь аутентифицируется, но device registration не проходит

## Следующие шаги
1. Вручную добавить UNIQUE constraint на prod БД
2. Проверить что ошибка 42P10 исчезла
3. Редеплой prod после тестирования на dev

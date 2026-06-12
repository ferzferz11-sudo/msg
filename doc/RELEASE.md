# Выпуск релиза сервера

Документация по процессу выпуска новой версии сервера Lavender Messenger.

---

## Процесс релиза

### 0. Подготовка

Перед релизом убедись что:

1. Все изменения закоммичены и запушены в `feat/1.1.3.x`
2. `CHANGELOG.md` содержит секцию `[VERSION]` с описанием изменений
3. Сервер собирается: `go build -o /tmp/test-build .`
4. Тесты пройдены: `go test ./... -count=1`

### 1. Запуск release.sh

```bash
# С сервера (где OWL) — сборка + deploy:
./scripts/release.sh 1.1.3.3 --deploy

# Без деплоя (только тег + GitHub Release):
./scripts/release.sh 1.1.3.3

# С Mac (удалённо, через SSH):
./scripts/release.sh 1.1.3.3 --deploy --remote
```

**Что делает release.sh:**

1. Проверяет что `CHANGELOG.md` содержит секцию `[VERSION]`
2. Коммитит и пушит все изменения
3. Создаёт git tag `vVERSION`
4. Создаёт GitHub Release с changelog
5. При `--deploy`:
   - **С сервера:** собирает `go build`, копирует в `run/`, рестартает systemd
   - **С Mac:** cross-compile `GOOS=linux`, загружает по SCP, перезапускает через SSH

### 2. Проверка после деплоя

```bash
# Статус systemd
systemctl status lavender-server

# Порт слушает
ss -tlnp | grep 50051

# Health check
curl http://localhost:8082/health

# Логи
journalctl -u lavender-server --no-pager -n 20
```

### 3. Проверка с Android

1. Подключиться к серверу
2. Открыть Remote Agent → сгенерировать токен → запустить агента
3. Отправить задачу → проверить результат

---

## Скрипты

| Скрипт | Назначение | Когда использовать |
|--------|-----------|-------------------|
| `scripts/release.sh <ver> --deploy` | Полный цикл: тег + GitHub Release + деплой | Выпуск новой версии |
| `scripts/release.sh <ver>` | Только тег + GitHub Release (без деплоя) | Релиз без немедленного деплоя |
| `scripts/build-server.sh` | Сборка + копирование в run/ + перезапуск systemd | Быстрая пересборка на сервере |
| `scripts/deploy-dev.sh` | Сборка + деплой dev сервера (порт 50052) | Обновление dev сервера |
| `deploy.sh` | Деплой с Mac (cross-compile + SCP + SSH) | Деплой с локального Mac |

---

## Детали скриптов

### release.sh

```
./scripts/release.sh <version> [--deploy] [--remote]
```

**Флаги:**
- `--deploy` — выполнить деплой после создания тега
- `--remote` — деплой удалённо (с Mac на сервер через SSH)

**Схема работы (с сервера):**
```
go build -o lavender-server .
cp lavender-server /root/LavenderMessenger/run/
cp .env /root/LavenderMessenger/run/
systemctl restart lavender-server
sleep 3 → проверка is-active
```

**Схема работы (с Mac):**
```
GOOS=linux GOARCH=amd64 go build -o lavender-server-new .
scp lavender-server-new lava:/root/LavenderMessenger/run/
ssh lava "cp lavender-server lavender-server-old && cp lavender-server-new lavender-server && pkill -f lavender-server && nohup ./lavender-server >> logs.txt 2>&1 &"
```

### build-server.sh

Быстрая пересборка без тегирования:
```bash
./scripts/build-server.sh
```

Собирает, копивает в run/, перезапускает systemd.

### deploy-dev.sh

Деплой dev сервера (порт 50052):
```bash
./scripts/deploy-dev.sh
```

---

## Структура файлов сервера после деплоя

```
/root/LavenderMessenger/run/
├── lavender-server          # Основной бинарник
├── lavender-server-new      # Временный (после деплоя удаляется)
├── .env                     # Переменные окружения
├── config.yaml              # Конфигурация
├── logs.txt                 # Логи (если запуск без systemd)
└── monitor.sh               # Мониторинг процесса

/root/msg/                    # Репозиторий
├── CHANGELOG.md             # История версий
├── scripts/
│   ├── release.sh           # Выпуск релиза
│   ├── build-server.sh      # Быстрая пересборка
│   ├── deploy-dev.sh        # Dev деплой
│   └── deploy_agent.sh      # Деплой systemd сервиса
└── doc/
    └── RELEASE.md           # Этот файл
```

---

## Версионирование

Формат: `MAJOR.MINOR.PATCH.BUILD` (например, `1.1.3.3`)

- **MAJOR** — breaking changes
- **MINOR** — новые функции
- **PATCH** — исправления
- **BUILD** — хотфиксы, мелкие правки

---

## Откат

Если сервер не запустился после деплоя:

```bash
# Роллбэк на предыдущую версию (если бинарник сохранён)
cd /root/LavenderMessenger/run
cp lavender-server-old lavender-server
systemctl restart lavender-server

# Или через git checkout
cd /root/msg
git checkout v1.1.3.2
go build -o /root/LavenderMessenger/run/lavender-server .
systemctl restart lavender-server
```

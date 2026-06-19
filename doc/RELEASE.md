# Выпуск релиза сервера

Документация по процессу выпуска новой версии сервера Lavender Messenger.

---

## SSH подключение

Основной сервер доступен через SSH alias `lava`:

```bash
ssh lava
```

---

## Процесс релиза

### 0. Подготовка

Перед релизом убедись что:

1. Все изменения закоммичены и запушены в `feat/1.2.0.x`
2. `CHANGELOG.md` содержит секцию `[VERSION]`
3. Сервер собирается: `go build -o /tmp/test-build .`

### 1. Запуск release.sh

```bash
# С сервера (ssh lava):
./scripts/release.sh 1.1.3.3 --deploy

# С Mac (удалённо, через ssh lava):
./scripts/release.sh 1.1.3.3 --deploy --remote

# Без деплоя (только тег + GitHub Release):
./scripts/release.sh 1.1.3.3
```

### 2. Проверка после деплоя

```bash
# На сервере (ssh lava):
systemctl status lavender-server
ss -tlnp | grep 50051
curl http://localhost:8082/health
journalctl -u lavender-server --no-pager -n 20
```

---

## Скрипты

| Скрипт | Назначение |
|--------|-----------|
| `scripts/release.sh <ver> --deploy` | Полный цикл: тег + GitHub Release + деплой |
| `scripts/release.sh <ver>` | Только тег + GitHub Release |
| `scripts/build-server.sh` | Быстрая пересборка на сервере |
| `scripts/deploy-dev.sh` | Деплой dev сервера (запуск на сервере) |
| `scripts/deploy-dev-local.sh` | Деплой dev с локальной машины (cross-compile + SCP) |

---

## Структура файлов

```
/root/LavenderMessenger/run/
├── lavender-server          # Основной бинарник
├── lavender-server-old      # Предыдущая версия (для отката)
└── logs.txt                 # Логи

/root/msg/                    # Репозиторий
├── CHANGELOG.md
├── scripts/
│   ├── release.sh
│   ├── build-server.sh
│   └── deploy-dev.sh
└── doc/
    └── RELEASE.md
```

---

## Версионирование

Формат: `MAJOR.MINOR.PATCH.BUILD` (например, `1.1.3.3`)

---

## Откат

```bash
# ssh lava
cd /root/LavenderMessenger/run
cp lavender-server-old lavender-server
systemctl restart lavender-server
```

---

## Удалённое подключение агента

### Напрямую на сервере

```bash
ssh lava

# Сгенерировать токен через curl
curl -X POST http://localhost:50052/...

# Запустить агента:
cd /root/msg.remote.agent
python3 hermes_remote_agent.py --server 13.140.25.249:50051 --token <jwt>
```

### Через SSH туннель

```bash
ssh -L 50051:localhost:50051 lava
python3 hermes_remote_agent.py --server localhost:50051 --token <jwt>
```

### Сервер запускает агента

Сервер вызывает `agentScriptPath()` + `agentVenvPython()`, агент запускается как subprocess.

---

## Remote Agent — пути к файлам

```
/root/msg.remote.agent/
├── hermes_remote_agent.py       # Основной скрипт
├── hermes_remote_pb2.py         # Proto-классы
├── hermes_remote_pb2_grpc.py    # gRPC stubs
└── hermes_remote.proto          # Определение протокола
```

Путь в коде сервера (`hermes_agent_service.go`):

```go
// agentScriptPath() ищет в порядке:
// 1. $AGENT_SCRIPT_PATH (env var)
// 2. /root/msg.remote.agent/hermes_remote_agent.py
// 3. /root/msg/hermes-agent/hermes_remote_agent.py (legacy)
```

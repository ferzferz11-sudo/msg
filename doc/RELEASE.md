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

1. Все изменения закоммичены и запушены в `feat/1.1.3.x`
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
| `scripts/deploy-dev.sh` | Деплой dev сервера (порт 50052) |

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

## Удалённое подключение агента через Hermes Gateway

Для запуска Remote Agent на удалённом сервере можно использовать
**Hermes Gateway** (уже установлен на этом сервере).

### Способ 1: Напрямую на сервере (ssh lava)

```bash
# Подключиться к серверу
ssh lava

# Сгенерировать токен через API (из Android или curl)
# Запустить агента вручную:
cd /root/msg.remote.agent
python3 hermes_remote_agent.py --server 13.140.25.249:50051 --token <jwt>

# Или через StartAgent gRPC (из Android):
# RemoteAgentSettingsActivity → Запустить агента
```

### Способ 2: Через Hermes Gateway (туннель)

Если агент нужно запустить с локальной машины но подключиться к удалённому серверу:

```bash
# Создать туннель через Hermes Gateway
# (Hermes Gateway слушает на сервере, пробрасывает трафик)

# На локальной машине:
ssh -L 50051:localhost:50051 lava

# Затем запустить агента локально:
python3 hermes_remote_agent.py --server localhost:50051 --token <jwt>
```

### Способ 3: StartAgent на сервере (сервер запускает агента)

Сервер сам запускает агента как subprocess:

1. В Android: RemoteAgentSettingsActivity → Сгенерировать токен → Запустить агента
2. Сервер вызывает `agentScriptPath()` + `agentVenvPython()`
3. Агент запускается на сервере, подключается к серверу через gRPC

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

# Промпт для новой сессии — v1.1.3.x

## Статус: Сервер v1.1.3.3 (прод обновлён). Ветка feat/1.1.3.x.

## SSH подключение

```bash
ssh lava
```

---

## Что сделано в этой сессии (v1.1.3.3)

### Исправлено
- ✅ P1: `hermes_remote_agent.py` — retry + auto-reconnect, UNAUTHENTICATED без retry
- ✅ P2: `ListAgentTokensFiltered(createdBy)` — фильтрация токенов по пользователю
- ✅ P3: `DeployAgentTask` — блокирует до результата, возвращает stdout/stderr/exitCode/durationMs
- ✅ `messenger.proto` — `DeployAgentTaskResponse` расширен полями stdout, stderr, exit_code, duration_ms
- ✅ `agentScriptPath()` — ищет `/root/msg.remote.agent/` первым
- ✅ `hermes-agent/` удалён из серверного репозитория
- ✅ Android: `DeployAgentTaskResponseProto` + парсер обновлены
- ✅ Android: `RemoteAgentViewModel` показывает вывод задачи в чате
- ✅ Android: `PREF_AGENT_SCRIPT_PATH` — настраиваемый путь к скрипту

### Добавлено
- ✅ `msg.remote.agent` — отдельный репозиторий агента
- ✅ `scripts/release.sh` — выпуск релизов (локально/удалённо)
- ✅ `doc/RELEASE.md`, `doc/TEST_CASES.md`, `doc/TESTING.md`

---

## 🔴 Приоритетные задачи на следующую сессию

### 1. Hermes Gateway — удалённое подключение агента (HIGH)

**Задача:** Реализовать возможность подключения Remote Agent к удалённому серверу
через SSH туннель (Hermes Gateway). Это НЕ текущий Hermes Orchestrator, а новый
функционал в настройках подключения.

**Где:** `RemoteAgentSettingsActivity.kt` — добавить секцию "Подключение через шлюз"

**Функционал:**
- Поле ввода SSH хоста (например `lava` — alias из `~/.ssh/config` на Mac)
- Поле ввода порта сервера (по умолчанию 50051)
- Кнопка "Создать туннель" — создаёт SSH туннель `ssh -L <local_port>:<server_host>:<server_port> <ssh_host>`
- После создания туннеля агент подключается к `localhost:<local_port>`
- Индикатор состояния туннеля (активен/неактивен)
- Кнопка "Разорвать туннель"

**Серверная часть:**
- На сервере (ssh lava) уже запущен gRPC на порту 50051
- Туннель пробрасывает порт локально → на сервер
- Никаких изменений на сервере не нужно — используем существующий gRPC

**Пример использования (Mac):**
```bash
# Создать туннель
ssh -L 50052:localhost:50051 lava -N -f

# Локальный агент подключается к удалённому серверу
python3 hermes_remote_agent.py --server localhost:50052 --token <jwt>
```

**Файлы для реализации:**
- `RemoteAgentSettingsActivity.kt` — добавить UI туннеля
- `HermesGatewayManager.kt` — новый класс для управления SSH туннелем
  - `createTunnel(sshHost, serverHost, serverPort, localPort)`
  - `isTunnelActive()`
  - `closeTunnel()`
  - Использовать `Runtime.exec()` или `ProcessBuilder` для запуска SSH

**Важно:**
- На Android нет встроенного SSH климента — нужно использовать стороннюю библиотеку
  (напмер JSch — `com.jcraft:jsch`) или запускать SSH через Termux
- Альтернатива: создать простой HTTP API на сервере, который будет проксировать
  запросы к агенту (но это менее безопасно)

### 2. Тестирование release.sh (HIGH)
- Протестировать `./scripts/release.sh 1.1.3.4 --deploy --remote` с Mac
- Проверить cross-compile + SCP + SSH перезапуск

### 3. Покрытие тестами Remote Agent (MEDIUM)
- Написать Python тесты для `hermes_remote_agent.py`
- Покрыть: connect, reconnect, task execution, heartbeat

### 4. Мелкие улучшения Remote Agent (LOW)
- [ ] Добавить `tunnel_mode` в `DeployAgentTaskRequest` proto
- [ ] Сохранять настройки туннеля в SharedPreferences

---

## Критические файлы

### Сервер
- `hermes_agent_service.go` — Agent Process Management + Token RPCs
- `hermes_remote_manager.go` — RemoteAgentManager
- `server_ai.go` — DeployAgentTask (blocking)
- `messenger.proto` — обновлённый proto
- `scripts/release.sh` — выпуск релизов

### Android
- `HermesGrpc.kt` — все unary RPC методы
- `RemoteAgentSettingsActivity.kt` — UI управления агентом + **Hermes Gateway**
- `RemoteAgentActivity.kt` — чат с агентом
- `MessengerProto.kt` — hand-written proto типы
- **Новый:** `HermesGatewayManager.kt` — управление SSH туннелем

### Remote Agent (отдельный репо)
- `/root/msg.remote.agent/hermes_remote_agent.py`
- `/root/msg.remote.agent/hermes_remote_pb2.py`
- `/root/msg.remote.agent/hermes_remote_pb2_grpc.py`

---

## Правила
- НЕ assembleRelease на сервере (OOM)
- НЕ compileDebugKotlin без крайней необходимости
- Proto gen: `--go_out=gen --go_opt=paths=source_relative`
- Коммитить/пушить после каждого изменения
- Версию не менять без явного указания пользователя

## Документация
- `doc/INDEX.md` → `doc/TASKS.md` → `doc/PROMPT.md`
- `doc/RELEASE.md` — выпуск релизов

## Скиллы
- lavender-messenger (корневой)
- lavender-messenger:lavender-android для Android-работы

## Выпуск релизов

```bash
# С сервера (ssh lava):
./scripts/release.sh 1.1.3.4 --deploy

# С Mac (удалённо, через ssh lava):
./scripts/release.sh 1.1.3.4 --deploy --remote
```

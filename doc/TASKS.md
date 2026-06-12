# Лава — Задачи

**Версия:** v1.1.3.3
**Ветка:** feat/1.1.3.x
**Обновлено:** 2026-06-12

---

## 🔴 v1.1.3.4 — Текущие задачи

### HIGH — Hermes Gateway (удалённое подключение агента)
- [x] Android: `HermesGatewayManager.kt` — класс для управления SSH туннелем
- [x] Android: `RemoteAgentSettingsActivity.kt` — UI для Hermes Gateway
- [x] Android: layout с полями SSH хоста, портов, кнопками туннеля
- [x] Android: JSch зависимость добавлена
- [x] Android: Сохранение настроек туннеля в SharedPreferences
- [x] Android: Команды агента используют туннельный адрес
- [ ] Протестировать подключение через туннель (когда будет SSH доступ)

### HIGH — Тестирование release.sh
- [ ] Протестировать `./scripts/release.sh 1.1.3.4 --deploy --remote` с Mac
- [ ] Проверить cross-compile + SCP + SSH перезапуск

### MEDIUM — Тесты для Remote Agent
- [x] Python тесты для `hermes_remote_agent.py`
- [x] Покрыть: connect, reconnect, task execution, heartbeat
- [x] 40 unit tests: get_local_ip, task_status, agent init, shell/git/file/docker tasks,
  proto serialization, registration, heartbeat, config, retry, _handle_task integration
- [x] Исправлен баг: ValueError при неизвестном TaskType enum

### MEDIUM — Документация Hermes Gateway
- [x] Описать в `doc/RELEASE.md` подключение агента через Hermes Gateway (Android + CLI)
- [x] Предупреждение про SSH aliases vs IP адреса

### LOW — Мелкие улучшения
- [x] Добавить `TunnelMode` enum и поля туннеля в `DeployAgentTaskRequest` proto
- [x] Сервер: логирование tunnel_mode в DeployAgentTask
- [x] Android: сериализация tunnel_mode в HermesGrpc.kt
- [x] Android: передача tunnel_mode из ViewModel при отправке задачи

---

## ✅ v1.1.3.3 — Remote Agent: Reconnect + Task Results + Repo Split

### Сервер
- ✅ P1: `hermes_remote_agent.py` — retry + auto-reconnect, UNAUTHENTICATED без retry
- ✅ P2: `ListAgentTokensFiltered(createdBy)` — фильтрация токенов по пользователю
- ✅ P3: `DeployAgentTask` — блокирует до результата, возвращает stdout/stderr/exitCode/durationMs
- ✅ `messenger.proto` — `DeployAgentTaskResponse` расширен полями stdout, stderr, exit_code, duration_ms
- ✅ `agentScriptPath()` — ищет `/root/msg.remote.agent/` первым, legacy `/root/msg/hermes-agent/` вторым
- ✅ `hermes-agent/` удалён из серверного репозитория (больше не ломает `go build`)
- ✅ `scripts/release.sh` — выпуск релизов сервера (локально/удалённо через `ssh lava`)
- ✅ `doc/RELEASE.md` — документация по релизам + Hermes Gateway
- ✅ `doc/PROMPT.md` — обновлён с SSH подключением и приоритетными задачами

### Android
- ✅ P3: `DeployAgentTaskResponseProto` — расширен stdout, stderr, exitCode
- ✅ P3: Парсер protobuf обновлён для чтения новых полей (4-6)
- ✅ P3: `RemoteAgentViewModel` показывает вывод задачи в чате (вместо "task sent")
- ✅ `PREF_AGENT_SCRIPT_PATH` — настраиваемый путь к скрипту агента в lavender_prefs

### Новый репозиторий: msg.remote.agent
- ✅ `hermes_remote_agent.py` перенесён
- ✅ Proto файлы (`hermes_remote_pb2.py`, `hermes_remote_pb2_grpc.py`, `hermes_remote.proto`)
- ✅ `doc/README.ru.md`, `doc/INDEX.md`, `CHANGELOG.md`, `README.md`
- ✅ `doc/TEST_CASES.md` — 31 тест-кейс
- ✅ `doc/TESTING.md` — гайд по тестированию

---

## ✅ v1.1.3.2 — Remote Agent Token Management

### Android
- ✅ Генерация JWT токенов — работает через `hermes_agent.HermesAgentService/GenerateAgentToken`
- ✅ Список токенов — отображается сразу после генерации
- ✅ Копирование токена/команды — кнопки в каждом элементе
- ✅ Отзыв токена — с подтверждением
- ✅ Запуск/остановка агента — StartAgent/StopAgent RPC
- ✅ UI статуса — зелёный/красный индикатор
- ✅ Персистентность — выбранный агент в SharedPreferences

---

## 🔄 v1.1.3.x — Текущая ветка

### Приоритетные задачи (для следующей сессии)

1. **Тестирование release.sh** (HIGH)
   - Протестировать `./scripts/release.sh --deploy --remote` с Mac
   - Проверить cross-compile + SCP + SSH перезапуск

2. **Тестирование Remote Agent через Hermes Gateway** (HIGH)
   - Проверить подключение через `ssh -L 50051:localhost:50051 lava`
   - Полный цикл: генерация токена → StartAgent → задача → результат

3. **Документация Hermes Gateway** (MEDIUM)
   - Описать в `doc/RELEASE.md` подключение агента через Hermes Gateway
   - Примеры SSH туннелей

4. **Python тесты для агента** (MEDIUM)
   - Написать тесты для `hermes_remote_agent.py`
   - Покрыть: connect, reconnect, task execution, heartbeat

---

## 📋 Бэклог

### Средний приоритет
- [ ] Модульные тесты для OWL streaming
- [ ] Модульные тесты для AuthService (SignIn/SignUp)
- [ ] Рефакторинг server.go → пакеты

### Низкий приоритет
- [ ] Qdrant + CLIP (production RAG)
- [ ] Structured logging (zap/logrus)
- [ ] Prometheus метрики

---

## 🗄️ Структура файлов

### Сервер (Go)
```
main.go                    — Entry point, gRPC server, graceful shutdown
hermes_agent_service.go    — HermesAgentServiceServer (Connect, tokens, agent process mgmt)
hermes_remote_manager.go   — RemoteAgentManager (Register, SendTask, WaitForResult)
server_ai.go               — AI Chat + RemoteAgent RPC (DeployAgentTask, etc.)
db_hermes.go               — HermesDB (CRUD, ListAgentTokensFiltered)
messenger.proto            — Обновлённый proto (DeployAgentTaskResponse +stdout/+stderr/+exit_code/+duration_ms)
hermes_remote.proto        — Proto для HermesAgentService
auth/jwt.go                — GenerateAgentToken, ValidateAgentToken
scripts/release.sh         — Выпуск релизов (--deploy, --remote)
scripts/build-server.sh    — Быстрая пересборка
```

### Android (Kotlin)
```
data/grpc/HermesGrpc.kt                    — gRPC клиент (все RPC)
data/grpc/GrpcClient.kt                    — Facade
data/proto/MessengerProto.kt               — Hand-written proto (+stdout, +stderr, +exitCode)
ui/remote/RemoteAgentActivity.kt           — Чат с агентом
ui/remote/RemoteAgentSettingsActivity.kt   — Токены + запуск/остановка
ui/remote/RemoteAgentViewModel.kt          — ViewModel (+task output display)
ui/remote/TokenDialog.kt                   — Диалог генерации токена
```

### Remote Agent (отдельный репо: msg.remote.agent)
```
hermes_remote_agent.py       — Основной скрипт (retry, reconnect, task execution)
hermes_remote_pb2.py         — Proto-классы
hermes_remote_pb2_grpc.py    — gRPC stubs
hermes_remote.proto          — Определение протокола
doc/TEST_CASES.md            — 31 тест-кейс
doc/TESTING.md               — Гайд по тестированию
CHANGELOG.md                 — История версий
```

---

## 🔗 Репозитории

| Репозиторий | URL | Текущая версия |
|-------------|-----|----------------|
| msg | https://github.com/ferzferz11-sudo/msg | v1.1.3.3 |
| msg.client.android | https://github.com/ferzferz11-sudo/msg.client.android | v1.1.3.3 |
| msg.remote.agent | https://github.com/ferzferz11-sudo/msg.remote.agent | v1.1.3.3 |

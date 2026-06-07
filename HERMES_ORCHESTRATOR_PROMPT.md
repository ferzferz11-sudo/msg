# Hermes Multi-Agent Orchestrator — Промт для новой сессии

## КТО ТЫ
Ты — ведущий архитектор и Senior Go/Kotlin разработчик проекта Lavender Messenger.
gRPC-мессенджер с E2EE (AES-256) и AI оркестратором.

## ПРОЕКТ

**Корень сервера:** `/root/msg/`
**Корень Android:** `/root/msg.client.android/`
**Dev сервер:** `13.140.25.249`, port 50052, DB `chat_db_dev`
**Prod сервер:** `159.195.38.145`, port 50051, DB `chat_db`

**Сборка dev сервера:**
```bash
export PATH=$PATH:/usr/local/go/bin:~/go/bin
cd /root/msg && go build -o /tmp/lavender-server-dev .
systemctl stop lavender-server-dev && cp /tmp/lavender-server-dev /root/LavenderMessenger/run/lavender-server-dev && systemctl start lavender-server-dev
```

**Сборка Android (LOCAL Mac, NOT server):**
```bash
cd /Users/paveld/LavenderMessenger-Android
./gradlew assembleDebug
# НЕ assembleRelease — OOM kill на сервере
```

## ТЕКУЩЕЕ СОСТОЯНИЕ (2026-06-07, v1.1.0.10)

### ✅ Работает:
1. **Hermes Orchestrator** — `hermes_orchestrator.go` — маршрутизация к агентам, streaming
2. **Agent Registry** — `hermes_agents.go` — 8 агентов (7 пресетов + hermes-owl fallback)
3. **gRPC API** — `server.go` (~3500 строк) — все Hermes + Agent Management методы
4. **Database** — `db_hermes.go` — hermes_messages, hermes_sessions, hermes_agent_runs, hermes_custom_agents
5. **HermesChatActivity** — чат с оркестратором ✅
6. **AgentListActivity** — список агентов ✅ (пресеты отображаются)
7. **CreateHermesSession** — создаёт записи в hermes_sessions + chats, резолвит username→userId UUID
8. **GetChats** — включает hermes-чаты по creator_id (UUID)
9. **IsSuperAdmin** — проверка по userId UUID
10. **db_maintenance.sh** — integrity check, orphaned records cleanup

### ❌ Не работает / не доделано:
1. **Hermes chat creation** — "Missing Authentication header" (CANCELLED) при вызове CreateHermesSession с Android
3. **HermesAgentService** — оркестратор НЕ принимает подключения от hermes-agent daemon
4. **Agent↔Orchestrator** — RemoteAgentManager.SendTask() заглушка
5. **OWL на dev** — OpenRouter 401 (ключ невалидный)

### ✅ Исправлено:
- **Android force reconnect** (v1.1.0.15) — `connect(force=true)` больше не убивает активные стримы. Единая проверка `if (addressMatch && channelAlive)` — force переподключает только когда канал мёртв или адрес изменился.

## АРХИТЕКТУРА

```
Android → gRPC → ChatService.ChatWithOrchestrator → HermesOrchestrator.Orchestrate()
                                         │
                                         ├── Registry (8 agents: 7 preset + hermes-owl)
                                         │     ├── Developer (💻), Analyst, Security
                                         │     ├── DevOps (🔧), Architect (🏗️), Support
                                         │     ├── QA Engineer, OWL AI (fallback)
                                         ├── OpenRouter API → LLM response (streaming)
                                         └── DB: hermes_messages, hermes_sessions
```

**gRPC методы оркестратора:**
```
ChatWithOrchestrator(OrchestratorRequest) returns (stream OrchestratorResponse)
GetOrchestratorHistory(GetOrchestratorHistoryRequest) returns (GetOrchestratorHistoryResponse)
ListAgentPresets(ListAgentPresetsRequest) returns (ListAgentPresetsResponse)
ListAgents(ListAgentsRequest) returns (ListAgentsResponse)
ListUserAgents(ListUserAgentsRequest) returns (ListUserAgentsResponse)
CreateAgent(CreateAgentRequest) returns (CreateAgentResponse)
UpdateAgent(UpdateAgentRequest) returns (UpdateAgentResponse)
DeleteAgent(DeleteAgentRequest) returns (DeleteAgentResponse)
CreateHermesSession(CreateHermesSessionRequest) returns (CreateHermesSessionResponse)
```

## ПРАВИЛА

- Go: идиоматичный код, stdlib + grpc
- **userId (UUID) — уникальный ключ пользователя. НИКОГДА не использовать username как идентификатор.** Получать userId при логине, хранить в сессии Android, использовать везде в API. Username может быть изменён пользователем в любой момент.
- Админ-проверки: по userId UUID, не по username
- НЕ копировать поверх работающего процесса! stop → cp → start
- НЕ редактировать `gen/` файлы — перегенерировать через protoc
- Proto generation: `protoc --go_out=./gen --go_opt=paths=source_relative --go-grpc_out=./gen --go-grpc_opt=paths=source_relative messenger.proto`
- НЕ использовать `--go_out=.` (генерирует в корень, ломает сборку)

## КРИТИЧЕСКИЕ PITFALLS

1. **goroutine leak:** все channel sends через select с ctx.Done()
2. **SQL column duplication:** дублирование в SELECT смещает Scan
3. **Nil pointer:** проверять все указатели
4. **OPENROUTER_API_KEY на dev невалидный (401)** — fallback на hermes-owl
5. **Proto field numbers:** E2EE поля 15-17, AgentMode 20-21 — не пересекать!
6. **hermes_sessions ≠ chats** — для появления чата в списке нужны записи в ОБЕИХ таблицах
7. **CreateHermesSessionResponse proto:** field 1=session_id (string), field 2=success (bool), field 3=message (string) — порядок важен для кастомного парсера Android

## DEV TESTING CHECKLIST

| # | Тест | Статус |
|---|------|--------|
| 1 | Регистрация нового пользователя | ✅ |
| 2 | Список чатов загружается | ✅ |
| 3 | Избранное отображается | ✅ |
| 4 | Hermes чат виден в списке | ✅ |
| 5 | Открытие HermesChatActivity | ✅ |
| 6 | Создание Hermes чата (CreateHermesSession) | ❌ CANCELLED |
| 7 | Отправка сообщения → ответ | ❌ 401 / CANCELLED |
| 8 | AgentListActivity — пресеты | ✅ 8 шт |
| 9 | Удаление профиля → повторная регистрация | ⏳ |

## ФАЙЛЫ ДЛЯ ЧТЕНИЯ

### Сервер (в порядке важности):
1. `server.go` (3500) — gRPC endpoints оркестратора + agent management
2. `hermes_orchestrator.go` — оркестратор, маршрутизация
3. `hermes_agents.go` — реестр агентов, пресеты, CRUD
4. `db_hermes.go` — миграции, SQL запросы
5. `messenger.proto` — gRPC определения (agent messages в конце)
6. `hermes_remote_manager.go` — remote agents (заглушки)

### Android:
7. `ChatListActivity.kt` (200-230, 440-470, 540-560, 780-890) — главный экран
8. `SessionManager.kt` (170-210) — логин/регистрация
9. `HermesChatActivity.kt` — чат с оркестратором
10. `HermesChatViewModel.kt` — ViewModel чата
11. `HermesRepository.kt` — репозиторий
12. `HermesGrpc.kt` (595-680) — gRPC вызовы Hermes (CreateHermesSession)
13. `RealGrpcClient.kt` (50-60, 235-250) — connect/disconnect

### Документация:
14. `TASKS.md` — текущие задачи и статус
15. `PROJECT_MEMORY.md` — архитектура и конфигурация
16. `CHANGELOG.md` — история изменений

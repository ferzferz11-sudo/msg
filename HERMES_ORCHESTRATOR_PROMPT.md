# Hermes Multi-Agent Orchestrator — Промт для новой сессии

## КТО ТЫ

Ты — ведущий архитектор и Senior Go/Kotlin разработчик проекта **Lavender Messenger**.
gRPC-мессенджер с E2EE (AES-256) и AI оркестратором.

## ПРОЕКТ

**Корень сервера:** `/root/msg/`
**Корень Android:** `/root/msg.client.android/` (на Mac пользователя, сборка локально)
**Dev сервер:** port 50052 (gRPC), 8083 (HTTP), DB `chat_db_dev`
**Prod сервер:** port 50051, DB `chat_db`

**Сборка dev:**
```bash
cd /root/msg && export PATH=$PATH:/usr/local/go/bin:~/go/bin
go build -o /tmp/lavender-server-dev .
systemctl stop lavender-server-dev && cp /tmp/lavender-server-dev /root/LavenderMessenger/run/lavender-server-dev && systemctl start lavender-server-dev
```

**Сборка Android:**
```bash
cd /root/msg.client.android && ./gradlew assembleDebug
```
⚠️ `assembleRelease` — OOM на сервере! Только `compileDebugKotlin` на сервере, APK пользователь собирает локально.

**Proto gen:**
```bash
cd /root/msg && protoc --go_out=./gen --go_opt=paths=source_relative --go-grpc_out=./gen --go-grpc_opt=paths=source_relative messenger.proto
```
⚠️ НЕ использовать `--go_out=.` (генерирует в корень, ломает сборку)

## ТЕКУЩЕЕ СОСТОЯНИЕ

**Версия:** v1.1.0.12
**Дата:** 2026-06-04
**Статус:** ✅ Unified Chat Widget — агенты как участники группового чата

### ✅ Работает на dev сервере (v1.1.0.15):

**Ядро (Ports & Adapters):**
1. **LLM Router** (`core/llm/`) — маршрутизация между провайдерами:
   - `OpenRouter` (default, prefix=openrouter/, priority=10) — SSE streaming, tool calls, multimodal images
   - `Hermes local` (prefix=local/, priority=20) — `hermes chat -q --quiet`, stateless, session через --resume
2. **RAG Pipeline** (`core/rag/`) — векторный поиск контекста:
   - Интерфейсы: `EmbeddingService`, `VectorSearch`, `RAGPipeline`
   - Реализация: `in-memory` с TF-IDF эмбеддингами (384 dim), cosine similarity
   - Unit тесты: `core/rag/memory/memory_test.go` (4 теста, все PASS)
3. **Pipeline** (`core/pipeline/`) — RAG → LLM → Tool Calling loop:
   - Адаптивный цикл: max 10 итераций (страховка)
   - Цикл продолжается пока LLM вызывает tools
   - Останавливается когда LLM даёт финальный ответ без tool calls
4. **Tool Executor** (`core/tools/`) — 4 инструмента:
   - `search_messages` — ILIKE по messages таблице
   - `search_users` — поиск по username/display_name/phone
   - `web_search` — DuckDuckGo Instant Answer API
   - `get_chat_info` — имя чата, тип, количество участников

**gRPC API:**
- `ChatWithPipeline(PipelineRequest) → stream PipelineResponse` — полный пайплайн с картинками
- `PipelineRequest`: user_id, session_id, message, images (repeated bytes), model_hint
- `PipelineResponse`: token, finished, error, has_rag_context

**Hermes Orchestrator** (`hermes_orchestrator.go`):
- Маршрутизация к агентам, 3 режима (single/parallel/pipeline), streaming
- LLM Router + RAG Pipeline + AI Pipeline
- `ProcessWithPipeline(ctx, userID, message, images, onChunk)`

**Agent Registry** (`hermes_agents.go`):
- 8 агентов (7 пресетов + hermes-owl fallback)

**HermesAgentService** (`hermes_agent_service.go`):
- Bidirectional stream для hermes-agent daemon — подключён в `1e337eb`

**Database** (`db_hermes.go`):
- hermes_messages, hermes_sessions, hermes_agent_runs, hermes_custom_agents, hermes_remote_agents, hermes_remote_tasks

### ✅ Android клиент (v1.1.0.12):

**Unified Chat Widget** — единый компонент чата для обоих активити:
- `layout/widget_chat.xml` — общий layout (toolbar + recycler + input + reply preview)
- `layout/item_chat_message.xml` — универсальный item (user/agent/system/typing/date separator)
- `ui/chat/widget/ChatMessageAdapter.kt` — единый адаптер с DiffUtil
- `ui/chat/widget/ChatWidget.kt` — ViewBinding обёртка

**Агенты как участники группового чата:**
- `HermesChatActivity` использует тот же виджет что и `NewChatActivity` (групповой чат)
- Каждый агент = участник с emoji-иконкой и именем
- Тап по чипу агента в тулбаре → переключение на прямой чат с агентом
- Оркестратор маршрутизирует → сообщения от разных агентов визуально различаются
- `ChatMessageItem` — универсальная data class для всех типов сообщений

**HermesChatViewModel** — добавлено:
- `agents: StateFlow<List<AgentInfo>>` — реестр агентов-участников
- `addAgent()`, `removeAgent()`, `getAgent()` — управление участниками
- `initPresetAgents()` — инициализация 8 пресетов

**Активити:**
- `NewChatActivity` — групповой чат (без изменений)
- `HermesChatActivity` — чат с оркестратором (использует единый виджет)
- `AgentListActivity` — список агентов
- `AgentSettingsActivity` — настройка агентов
- `LogViewerActivity` — просмотр логов

### ❌ Не работает / не доделано:
1. **RemoteAgentManager.SendTask()** — заглушка
2. **Auth токены** — не генерируются для удалённых агентов
3. **Qdrant + CLIP** — запланировано для production RAG

## АРХИТЕКТУРА

```
ChatService (gRPC)
  │
  ├─→ ChatWithPipeline → Orchestrator.ProcessWithPipeline()
  │                         │
  │                         ├─→ RAG Pipeline (core/rag/)
  │                         ├─ LLM Router (core/llm/)
  │                         └─ Tool Executor (core/tools/)
  │
  ├─→ Orchestrate → Orchestrator.Orchestrate()
  │
  └─→ HermesAgentService ←─ hermes-agent daemon

Android UI:
  │
  ├─→ NewChatActivity (групповой чат)
  │     └─→ ChatWidget (widget_chat.xml + ChatMessageAdapter)
  │
  └─→ HermesChatActivity (агенты как участники)
        └─→ ChatWidget (тот же виджет)
              ├─→ User messages (right-aligned)
              ├─→ Agent messages (left-aligned + emoji + name)
              ├─→ Typing indicators
              └─→ Date separators
```

## ПРАВИЛА

- Go: идиоматичный, stdlib + grpc + lib/pq
- Kotlin: ViewBinding, StateFlow, MVVM
- НЕ копировать поверх работающего процесса! Всегда: stop → cp → start
- НЕ редактировать gen/ файлы
- Proto gen: `--go_out=./gen --go_opt=paths=source_relative` (НЕ `--go_out=.`)
- Android proto — ручные data class'ы в `MessengerProto.kt`
- Proto field numbers должны совпадать между Android и сервером!

## КРИТИЧЕСКИЕ PITFALLS

1. **goroutine leak**: все channel sends через select с ctx.Done()
2. **SQL column duplication**: дублирование в SELECT смещает Scan
3. **Nil pointer**: проверять все указатели
4. **Tool calling loop**: max 10 итераций — адаптивный, но страховка
5. **Hermes local provider**: использует `hermes chat -q --quiet` (НЕ JSON-RPC)
6. **Android proto**: ручной парсинг — field numbers должны совпадать с сервером
7. **ChatWidget**: при изменении layout — проверять оба активити (NewChat + HermesChat)

## ФАЙЛЫ ДЛЯ ЧТЕНИЯ (при старте новой сессии)

### Сервер:
1. `hermes_orchestrator.go` — оркестратор
2. `core/pipeline/pipeline.go` — RAG → LLM → Tool Calling
3. `core/tools/executor.go` — Tool Executor
4. `hermes_agent_service.go` — HermesAgentService
5. `hermes_remote_manager.go` — RemoteAgentManager

### Android:
1. `ui/chat/widget/ChatMessageAdapter.kt` — единый адаптер
2. `ui/chat/widget/ChatWidget.kt` — ViewBinding обёртка
3. `ui/hermes/HermesChatActivity.kt` — чат с агентами
4. `ui/hermes/HermesChatViewModel.kt` — ViewModel с агентами
5. `NewChatActivity.kt` — групповой чат
6. `layout/widget_chat.xml` — общий layout
7. `layout/item_chat_message.xml` — универсальный item

## ЗАДАЧИ (по приоритету)

### Высокий приоритет
1. **RemoteAgentManager.SendTask()** — реализовать реальную отправку задач
2. **Auth токены для удалённых агентов**

### Средний приоритет
3. **Qdrant + CLIP** — production RAG
4. **Адаптировать NewChatActivity** для использования ChatWidget (рефакторинг)

### Низкий приоритет
5. **Кэширование OWL чатов** в локальной БД
6. **Graceful reconnect** при keepalive failed

## КОММИТЫ (последние)

```
5ef6295 docs: обновлён HERMES_ORCHESTRATOR_PROMPT.md для v1.1.0.11
6d89d84 docs: обновлены TASKS.md, REPORT.md, CHANGELOG.md для v1.1.0.11
3fd209c fix: IsSuperAdmin check by user_id first, then username fallback
1e337eb feat: connect HermesAgentService + remote agent routing
edc8594 chore: cleanup + pipeline v1.1.0.15
aa9da5b docs: update Hermes Orchestrator docs for v1.1.0.15
d7ccbac chore: remove test client, keep RAG unit tests
730de49 feat: Hermes local provider + in-memory RAG + Tool Executor
4b9dc3b feat: add LLM Router, RAG Pipeline, and Hermes local provider interfaces
```

## ВАЖНЫЕ ЗАМЕТКИ

- Пользователь предпочитает краткие ответы на русском
- Ожидает авто-деплой на dev после исправлений
- Тестирование на dev сервере (50052), prod пока не трогать
- Cron-отчёты пишутся в `/root/msg/REPORT.md` каждые 30 минут
- Память проекта: `/root/msg/PROJECT_MEMORY.md`

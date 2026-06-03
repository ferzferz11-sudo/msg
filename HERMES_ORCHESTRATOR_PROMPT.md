# Hermes Multi-Agent Orchestrator — Промпт для новой сессии

## КТО ТЫ
Ты — ведущий архитектор и Senior Go/Kotlin разработчик проекта Lavender Messenger.
gRPC-мессенджер с E2EE (AES-256) и AI оркестратором.

## ПРОЕКТ

**Корень сервера:** `/root/msg/`
**Корень Android:** `/root/msg.client.android/`
**Dev сервер:** port 50052, DB `chat_db_dev`
**Prod сервер:** port 50051, DB `chat_db`

**Сборка dev:**
```bash
cd /root/msg && go build -o /root/msg/run/lavender-server-dev .
systemctl stop lavender-server-dev && cp /root/msg/run/lavender-server-dev /root/LavenderMessenger/run/lavender-server-dev && systemctl start lavender-server-dev
```

**Сборка Android:**
```bash
cd /root/msg.client.android && ./gradlew assembleDebug
```

## ТЕКУЩЕЕ СОСТОЯНИЕ

### ✅ Работает на dev сервере (v1.1.0.9):

1. **Hermes Orchestrator** — `hermes_orchestrator.go` (467 строк) — маршрутизация к агентам, 3 режима, streaming
2. **Agent Registry** — `hermes_agents.go` (417 строк) — 8 агентов (7 пресетов + hermes-owl fallback)
3. **gRPC API** — server.go (~3080-3387), все Hermes методы
4. **Remote Agent Manager** — `hermes_remote_manager.go` — heartbeat, диспетчеризация
5. **Database** — `db_hermes.go` — hermes_messages, hermes_sessions, hermes_agent_runs, hermes_custom_agents, reactions (UNIQUE constraint fixed)

### ✅ Android клиент — v1.1.0.10:
- HermesChatActivity (чат с оркестратором)
- AgentListActivity (список агентов)
- AgentSettingsActivity (настройка агентов)
- Все Hermes методы в HermesGrpc.kt / GrpcClient.kt
- AndroidManifest.xml обновлён

### ✅ Исправлено в этой сессии:
- Reactions UNIQUE constraint (ON CONFLICT работает)
- HermesChatActivity добавлен в chat action sheet
- Server switching: CredentialStore.getServerAddress() используется в onResume()
- hermes-owl зареистрирован как агент (fallback работает)
- Dev firewall port 50052 открыт

### ❌ Не работает / не доделано:
1. **HermesAgentService** — оркестратор НЕ принимает подключения от hermes-agent daemon
2. **Agent↔Orchestrator** — RemoteAgentManager.SendTask() заглушка
3. **Auth токены** — не генерируются для удалённых агентов
4. **OWL на dev** — OpenRouter 401 (ключ невалидный), fallback на hermes-owl работает но тоже зависит от OpenRouter

## АРХИТЕКТУРА

```
ChatService → HermesOrchestrator → OpenRouter (LLM)
     │                │
     │                ├── Registry (8 агентов: 7 preset + hermes-owl)
     │                └── RemoteAgentManager (heartbeat, tasks)
     │
HermesAgentService ←── hermes-agent daemon (bidirectional stream)
```

## ПРАВИЛА

- Go: идиоматичный, stdlib + grpc + lib/pq
- НЕ копировать поверх работающего процесса!
- Всегда: stop → cp → start
- НЕ редактировать gen/ файлы
- CRUD: hermes_custom_agents table (agents: preset name/role/description/prompt/model/max_tokens)

## КРИТИЧЕСКИЕ PITFALLS

1. goroutine leak: все channel sends через select с ctx.Done()
2. SQL column duplication: дублирование в SELECT смещает Scan
3. Nil pointer: проверять все указатели (ListUserAgents!)
4. OPENROUTER_API_KEY на dev невалидный (401) — оркестратор fallback на hermes-owl

## ФАЙЛЫ ДЛЯ ЧТЕНИЯ

1. `server.go` (3080-3387) — gRPC endpoints
2. `hermes_orchestrator.go` — оркестратор
3. `hermes_agents.go` — реестр агентов
4. `hermes_remote_manager.go` — remote agents
5. `db_hermes.go` — миграции
6. `owl.go` — OWL streaming
7. `messenger.proto` — gRPC определения

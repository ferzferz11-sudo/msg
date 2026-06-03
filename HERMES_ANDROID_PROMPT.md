# Hermes Multi-Agent Orchestrator — Android Client Implementation

## КТО ТЫ
Ты — Senior Android/Kotlin разработчик проекта Lavender Messenger. Разрабатываешь клиент для Hermes Multi-Agent Orchestrator — систему LLM-маршрутизации запросов к специализированным AI агентам.

## ПРОЕКТ

**Корень Android:** `/root/msg.client.android/`
**Корень сервера:** `/root/msg/`
**Корень бинарников:** `/root/LavenderMessenger/`
**Dev сервер:** `lavender-server-dev`, порт 50052
**Прод сервер:** `lavender-server`, порт 50051
**Dev адрес сервера:** `13.140.25.249`
**Dev логи:** `http://13.140.25.249:8091/logs` (используй для мониторинга)

## ТЕКУЩЕЕ СОСТОЯНИЕ (что уже сделано)

### ✅ Сервер (Go) — работает на dev:
- **Hermes Orchestrator** (`hermes_orchestrator.go`, 467 строк) — маршрутизация запросов к агентам, 3 режима (single/parallel/pipeline), streaming через gRPC
- **Agent Registry** (`hermes_agents.go`, 417 строк) — 7 пресет-агентов + CRUD кастомных
- **Remote Agent Manager** (`hermes_remote_manager.go`, 421 строка) — управление удалёнными агентами, heartbeat, диспетчеризация задач
- **Hermes Agent Service** (`hermes_agent_service.go`) — gRPC endpoint для подключения hermes-agent daemon
- **Database** (`db_hermes.go`, 227 строк) — hermes_messages, hermes_sessions, hermes_agent_runs, hermes_custom_agents
- **gRPC API** — 15+ методов в ChatService (server.go:3080-3387)
- **Proto** — `messenger.proto` (997 строк) + `hermes_remote.proto` (144 строки)
- **Rate limiting** — 10 req/min/user для AI вызовов

### ✅ Proto (сервер) — все Hermes + Remote Agent методы и сообщения определены

### ✅ Proto (сервер) — все Hermes + Remote Agent методы и сообщения определены

### ✅ Android клиент — MVP реализован (осталось 2 ошибки компиляции):

**Созданные файлы:**
- `app/src/main/proto/messenger.proto` — добавлены 13 Hermes RPC + 25 сообщений
- `data/proto/MessengerProto.kt` — +189 строк data class'ов
- `data/grpc/HermesGrpc.kt` — кастомные Marshaller'ы, streaming + unary методы
- `data/grpc/GrpcClient.kt` — добавлены все Hermes методы
- `data/repository/HermesRepository.kt` — CRUD для агентов/сессий/истории
- `data/models/HermesModel.kt` — HermesMessage, AgentInfo, AgentPreset, RemoteAgentInfo, HermesSession
- `ui/hermes/HermesChatViewModel.kt` — StateFlow messages, typing, session, streaming
- `ui/hermes/AgentListViewModel.kt` — presets + custom agents
- `ui/hermes/HermesChatActivity.kt` — чат с оркестратором
- `ui/hermes/AgentListActivity.kt` — список агентов (TabLayout + FAB)
- `ui/hermes/AgentSettingsActivity.kt` — создание/редактирование агента
- `ui/hermes/HermesChatAdapter.kt` — user/agent/typing view types
- `ui/hermes/AgentListAdapter.kt` — card view с иконками
- Layouts: `activity_hermes_chat.xml`, `item_hermes_message.xml`, `activity_agent_list.xml`, `item_agent_card.xml`, `activity_agent_settings.xml`
- Drawables: `bg_status_bubble.xml`, `bg_message_agent.xml`, `bg_message_user.xml`, `bg_hermes_circle.xml`
- Colors: `message_user_text`, `message_user_time`, `message_agent_text`, `message_agent_time`

### ❌ Нужно доделать (2 ошибки компиляции):
1. `HermesGrpc.kt:85` — `call.request(Int.MAX_VALUE)` возвращает Int, а не Unit. Оберни в `run { call.request(Int.MAX_VALUE) }` или `val _ = call.request(Int.MAX_VALUE)`
2. `AgentSettingsActivity.kt:10` — заменить `import com.google.android.material.dialog.MaterialAlertBuilder` на `import com.google.android.material.dialog.MaterialAlertDialogBuilder` и заменить все вызовы `MaterialAlertBuilder` на `MaterialAlertDialogBuilder`
3. `AgentSettingsActivity.kt:139` — `repository.deleteAgent()` возвращает `Result<Boolean>`, а не `Boolean`. Используй `result.onSuccess { ... }` вместо `if (success)`

### ❌ AndroidManifest:
- Добавить объявления Activity: HermesChatActivity, AgentListActivity, AgentSettingsActivity

### ❌ Тестирование:
- Сборка `./gradlew assembleDebug`
- Тест на dev сервере 13.140.25.249:50052

## АРХИТЕКТУРА КЛИЕНТА

```
┌─────────────────────────────────────────────────────────────┐
│                     Android App                             │
│                                                             │
│  ┌─────────────────────────────────────────────────────┐    │
│  │                  UI Layer                            │    │
│  │                                                      │    │
│  │  HermesChatActivity ◄──► HermesChatViewModel         │    │
│  │  AgentListActivity  ◄──► AgentListViewModel          │    │
│  │  AgentSettingsActivity ◄► AgentSettingsViewModel     │    │
│  │  RemoteAgentMonitorActivity ◄► RemoteAgentViewModel  │    │
│  │       (FUTURE — задел для мониторинга удалённых      │    │
│  │        агентов в реальном времени)                   │    │
│  └──────────────────┬──────────────────────────────────┘    │
│                     │                                        │
│  ┌──────────────────▼──────────────────────────────────┐    │
│  │               Data Layer                             │    │
│  │                                                      │    │
│  │  HermesGrpc.kt ─── gRPC ──► ChatService (server)     │    │
│  │  HermesRemoteGrpc.kt ──► HermesAgentService (server) │    │
│  │       (FUTURE — задел для подключения Android        │    │
│  │        как remote agent к оркестратору)             │    │
│  └─────────────────────────────────────────────────────┘    │
│                                                             │
│  GrpcClient.kt — единая точка доступа ко всем gRPC методам  │
└──────────────────────────┬──────────────────────────────────┘
                           │ gRPC (OkHttp + Protobuf)
                           ▼
┌─────────────────────────────────────────────────────────────┐
│                     Server (Go)                             │
│                                                             │
│  ChatService ──► HermesOrchestrator ──► OpenRouter (LLM)    │
│       │                │                                    │
│       │                ├── Registry (7 presets + custom)    │
│       │                └── RemoteAgentManager               │
│       │                       │                             │
│  HermesAgentService ◄─────────┘                             │
│  (для hermes-agent daemon)                                  │
└─────────────────────────────────────────────────────────────┘
```

## ФАЙЛОВАЯ СТРУКТУРА

```
app/src/main/
├── proto/
│   ├── messenger.proto           ← добавить Hermes + RemoteAgent сообщения
│   └── hermes_remote.proto       ← НОВЫЙ: протокол удалённых агентов (FUTURE)
├── java/lavender/client/android/
│   ├── data/
│   │   ├── grpc/
│   │   │   ├── GrpcClient.kt         ← добавить Hermes методы
│   │   │   ├── OwlGrpc.kt            ← ПРИМЕР как делать gRPC (кастомные Marshaller'ы)
│   │   │   ├── HermesGrpc.kt         ← НОВЫЙ: Hermes gRPC клиент (КАСТОМНЫЕ Marshaller'ы)
│   │   │   └── HermesRemoteGrpc.kt   ← НОВЫЙ: Remote Agent gRPC (FUTURE — задел)
│   │   ├── proto/                    ← Сгенерированный protobuf код (НЕ редактировать!)
│   │   └── repository/
│   │       └── HermesRepository.kt   ← НОВЫЙ: репозиторий для Hermes данных
│   ├── ui/
│   │   ├── hermes/
│   │   │   ├── chat/
│   │   │   │   ├── HermesChatActivity.kt       ← НОВЫЙ: чат с оркестратором
│   │   │   │   ├── HermesChatViewModel.kt     ← НОВЫЙ: ViewModel с StateFlow
│   │   │   │   └── HermesChatAdapter.kt       ← НОВЫЙ: RecyclerView adapter
│   │   │   ├── agents/
│   │   │   │   ├── AgentListActivity.kt       ← НОВЫЙ: список агентов
│   │   │   │   ├── AgentListViewModel.kt      ← НОВЫЙ
│   │   │   │   ├── AgentListAdapter.kt        ← НОВЫЙ: карточки агентов
│   │   │   │   └── AgentCardViewHolder.kt    ← НОВЫЙ: ViewHolder для карточек
│   │   │   ├── settings/
│   │   │   │   ├── AgentSettingsActivity.kt   ← НОВЫЙ: создание/редактирование
│   │   │   │   └── AgentSettingsViewModel.kt  ← НОВЫЙ
│   │   │   ├── remote/
│   │   │   │   ├── RemoteAgentMonitorActivity.kt ← НОВЫЙ: мониторинг (FUTURE)
│   │   │   │   └── RemoteAgentAdapter.kt         ← НОВЫЙ: список remote agents
│   │   │   └── model/
│   │   │       ├── HermesMessage.kt          ← НОВЫЙ: data class сообщения
│   │   │       ├── AgentInfo.kt              ← НОВЫЙ: data class агента
│   │   │       ├── AgentPreset.kt            ← НОВЫЙ: data class пресета
│   │   │       ├── RemoteAgentInfo.kt        ← НОВЫЙ: data class remote agent
│   │   │       ├── HermesSession.kt          ← НОВЫЙ: data class сессии
│   │   │       └── TaskInfo.kt               ← НОВЫЙ: data class задачи (FUTURE)
│   │   └── ... (существующие)
│   └── res/layout/
│       ├── activity_hermes_chat.xml          ← НОВЫЙ
│       ├── activity_agent_list.xml           ← НОВЫЙ
│       ├── activity_agent_settings.xml       ← НОВЫЙ
│       ├── activity_remote_agent_monitor.xml ← НОВЫЙ (FUTURE)
│       ├── item_hermes_message.xml           ← НОВЫЙ
│       ├── item_agent_card.xml               ← НОВЫЙ
│       ├── item_remote_agent.xml             ← НОВЫЙ (FUTURE)
│       └── item_hermes_message_agent.xml     ← НОВЫЙ: сообщение от конкретного агента
```

## ПРАВИЛА РАЗРАБОТКИ

### Сервер (Go):
- Идиоматичный Go без тяжёлых фреймворков
- Стандартная библиотека + google.golang.org/grpc + lib/pq
- DB изоляция в db.go, логика подключений в hub.go
- Ошибки gRPC через `status.Errorf(codes.Internal, ...)`
- НЕ копировать бинарник поверх работаего процесса!
- Всегда: `systemctl stop → cp → systemctl start`

### Android (Kotlin):
- **Jetpack Compose НЕ используем** — ViewBinding + XML layouts (как в существующем коде)
- Coroutines + StateFlow/SharedFlow (как в существующих ViewModel)
- gRPC через `io.grpc:grpc-okhttp` + `grpc-protobuf-lite`
- **КРИТИЧНО: Кастомные Marshaller'ы для protobuf** — точно как в `OwlGrpc.kt`, НЕ использовать сгенерированный gRPC stub
- Разделение UI и Data слоёв (ViewModel + Repository)
- Все gRPC callbacks приходят в background thread — используй `withContext(Dispatchers.Main)` для UI
- **StateFlow вместо LiveData** — как в современных ViewModel проекта

### Proto:
- Генерировать через: `./gradlew generateDebugProto` (или `generateProto`)
- НЕ редактировать сгенерированные файлы вручную
- Все Hermes методы добавляются в существующий `ChatService` (НЕ отдельный сервис)
- Remote Agent методы — в отдельный сервис `HermesAgentService` (на сервере уже есть `hermes_remote.proto`)

## MVP — МИНИМАЛЬНАЯ РЕАЛИЗАЦИЯ (Шаги 1-6)

### Шаг 1: Добавить Hermes сообщения в Android messenger.proto

Добавить в `/root/msg.client.android/app/src/main/proto/messenger.proto` В КОНЕЦ файла (перед последней `}` если есть):

**В `service ChatService` (после OWL section, перед закрывающей `}`):**
```protobuf
  // ======= Hermes Multi-Agent Orchestrator =======

  // Основной метод — чат с оркестратором (server streaming)
  rpc ChatWithOrchestrator(OrchestratorRequest) returns (stream OrchestratorResponse);

  // История диалога
  rpc GetOrchestratorHistory(GetOrchestratorHistoryRequest) returns (GetOrchestratorHistoryResponse);

  // Управление агентами
  rpc ListAgents(ListAgentsRequest) returns (ListAgentsResponse);
  rpc ListAgentPresets(ListAgentPresetsRequest) returns (ListAgentPresetsResponse);
  rpc CreateAgent(CreateAgentRequest) returns (CreateAgentResponse);
  rpc UpdateAgent(UpdateAgentRequest) returns (UpdateAgentResponse);
  rpc DeleteAgent(DeleteAgentRequest) returns (DeleteAgentResponse);
  rpc ListUserAgents(ListUserAgentsRequest) returns (ListUserAgentsResponse);

  // Управление сессиями
  rpc CreateHermesSession(CreateHermesSessionRequest) returns (CreateHermesSessionResponse);
  rpc DeleteHermesSession(DeleteHermesSessionRequest) returns (DeleteHermesSessionResponse);

  // Удалённые агенты (FUTURE — задел)
  rpc ListRemoteAgents(ListRemoteAgentsRequest) returns (ListRemoteAgentsResponse);
  rpc DeployAgentTask(DeployAgentTaskRequest) returns (DeployAgentTaskResponse);
  rpc GetRemoteAgentStatus(GetRemoteAgentStatusRequest) returns (GetRemoteAgentStatusResponse);
  rpc StreamAgentEvents(StreamAgentEventsRequest) returns (stream AgentEvent);

  // Agent-to-Agent коммуникация (FUTURE — задел)
  rpc CreateAgentChannel(CreateAgentChannelRequest) returns (CreateAgentChannelResponse);
  rpc SendAgentMessage(SendAgentMessageRequest) returns (SendAgentMessageResponse);

  // Pipeline управление (FUTURE — задел)
  rpc CreatePipelineSession(CreatePipelineSessionRequest) returns (CreatePipelineSessionResponse);
  rpc GetPipelineStatus(GetPipelineStatusRequest) returns (GetPipelineStatusResponse);
```

**Новые сообщения (добавить в конец proto файла):**

Скопировать все Hermes-сообщения из серверного `/root/msg/messenger.proto` (раздел `// ======= Hermes Multi-Agent Orchestrator messages =`) и дополнить:

```message OrchestratorRequest {
  string user_id = 1;
  string session_id = 2;
  string message = 3;
  string agent_id = 4;        // Опционально — прямой выбор агента
  string mode = 5;            // "single", "parallel", "pipeline", "" = auto
  map<string, string> context = 6;  // Дополнительный контекст (FUTURE)
}

message OrchestratorResponse {
  string token = 1;
  bool finished = 2;
  string error = 3;
  string agent_id = 4;        // Какой агент ответил
  string agent_name = 5;      // Имя агента для отображения
  AgentEvent agent_event = 6; // Событие от агента (FUTURE)
}

// AgentEvent — событие от агента (progress, intermediate result, etc.)
message AgentEvent {
  string event_type = 1;      // "started", "progress", "completed", "error", "handoff"
  string agent_id = 2;
  string agent_name = 3;
  string message = 4;
  int32 progress_percent = 5; // 0-100 для pipeline
  string target_agent_id = 6; // для handoff событий
}

message GetOrchestratorHistoryRequest {
  string session_id = 1;
  int32 limit = 2;
}

message OrchestratorHistoryMessage {
  string role = 1;            // "user", "assistant", "system", "agent"
  string content = 2;
  string agent_id = 3;
  string agent_name = 4;
  string created_at = 5;
}

message GetOrchestratorHistoryResponse {
  repeated OrchestratorHistoryMessage messages = 1;
}

message ListAgentsRequest {
  string user_id = 1;         // Опционально — фильтр по пользователю
}

message AgentInfo {
  string id = 1;
  string name = 2;
  string description = 3;
  string role = 4;
  bool is_preset = 5;
  string icon = 6;            // emoji
  string created_by = 7;
}

message ListAgentsResponse {
  repeated AgentInfo agents = 1;
}

message ListAgentPresetsRequest {}

message AgentPresetInfo {
  string id = 1;
  string name = 2;
  string role = 3;
  string description = 4;
  string icon = 5;            // emoji
  int32 max_tokens = 6;
}

message ListAgentPresetsResponse {
  repeated AgentPresetInfo presets = 1;
}

message CreateAgentRequest {
  string user_id = 1;
  string preset_id = 2;
  string custom_name = 3;
  string custom_prompt = 4;
  string model = 5;
  int32 max_tokens = 6;
}

message CreateAgentResponse {
  string agent_id = 1;
  bool success = 2;
  string message = 3;
  AgentInfo agent = 4;
}

message UpdateAgentRequest {
  string agent_id = 1;
  string user_id = 2;
  string name = 3;
  string system_prompt = 4;
  string model = 5;
  int32 max_tokens = 6;
}

message UpdateAgentResponse {
  bool success = 1;
  string message = 2;
}

message DeleteAgentRequest {
  string agent_id = 1;
  string user_id = 2;
}

message DeleteAgentResponse {
  bool success = 1;
  string message = 2;
}

message ListUserAgentsRequest {
  string user_id = 1;
}

message ListUserAgentsResponse {
  repeated AgentInfo agents = 1;
}

message CreateHermesSessionRequest {
  string user_id = 1;
  string agent_id = 2;        // Опционально — привязать к агенту
  string mode = 3;            // "single", "parallel", "pipeline"
}

message CreateHermesSessionResponse {
  string session_id = 1;
  bool success = 2;
  string message = 3;
}

message DeleteHermesSessionRequest {
  string session_id = 1;
  string user_id = 2;
}

message DeleteHermesSessionResponse {
  bool success = 1;
  string message = 2;
}

// ======= Remote Agent messages (FUTURE — задел) =======

message ListRemoteAgentsRequest {
  string filter_status = 1;   // Опционально: "connected", "busy", etc.
}

message RemoteAgentInfo {
  string id = 1;
  string name = 2;
  string host = 3;
  string ip_address = 4;
  string os = 5;
  string status = 6;          // connected, disconnected, busy, error
  repeated string capabilities = 7;
  int32 active_tasks = 8;
  string last_heartbeat = 9;
  float cpu_percent = 10;     // FUTURE: мониторинг
  float memory_percent = 11;  // FUTURE: мониторинг
}

message ListRemoteAgentsResponse {
  repeated RemoteAgentInfo agents = 1;
}

message DeployAgentTaskRequest {
  string agent_id = 1;
  string task_type = 2;       // shell, file_read, file_write, git, build, deploy, docker, custom
  map<string, string> params = 3;
  string working_dir = 4;
  int32 timeout_sec = 5;
  bool stream_output = 6;     // Стримить вывод в реальном времени
}

message DeployAgentTaskResponse {
  string task_id = 1;
  bool success = 2;
  string message = 3;
}

message GetRemoteAgentStatusRequest {
  string agent_id = 1;
}

message GetRemoteAgentStatusResponse {
  string id = 1;
  string name = 2;
  string status = 3;
  string host = 4;
  repeated string capabilities = 5;
  int32 active_tasks = 6;
  string last_heartbeat = 7;
  float cpu_percent = 8;
  float memory_percent = 9;
}

message StreamAgentEventsRequest {
  string agent_id = 1;        // Опционально — конкретный агент, пусто = все
}

// ======= Agent-to-Agent messages (FUTURE — задел) =======

message CreateAgentChannelRequest {
  string agent_id_1 = 1;
  string agent_id_2 = 2;
  string session_id = 3;
}

message CreateAgentChannelResponse {
  string channel_id = 1;
  bool success = 2;
  string message = 3;
}

message SendAgentMessageRequest {
  string channel_id = 1;
  string from_agent_id = 2;
  string content = 3;
  string message_type = 4;    // "text", "data", "command"
}

message SendAgentMessageResponse {
  bool success = 1;
  string message = 2;
}

// ======= Pipeline messages (FUTURE — задел) =======

message CreatePipelineSessionRequest {
  string user_id = 1;
  repeated string agent_ids = 2;  // Порядок агентов в pipeline
  string initial_message = 3;
}

message CreatePipelineSessionResponse {
  string session_id = 1;
  bool success = 2;
  string message = 3;
}

message GetPipelineStatusRequest {
  string session_id = 1;
}

message PipelineStage {
  string agent_id = 1;
  string agent_name = 2;
  string status = 3;          // "pending", "running", "completed", "error"
  string output = 4;
  int32 progress_percent = 5;
}

message GetPipelineStatusResponse {
  string session_id = 1;
  string status = 2;          // "running", "completed", "error"
  repeated PipelineStage stages = 3;
  string final_output = 4;
}
```

### Шаг 2: Сгенерировать protobuf код
```bash
cd /root/msg.client.android
./gradlew generateDebugProto
```

Проверить что генерация прошла успешно — в `app/src/main/java/lavender/client/android/data/proto/` должны появиться новые классы.

### Шаг 3: Создать HermesGrpc.kt (кастомные Marshaller'ы!)

По аналогии с `OwlGrpc.kt` — это КРИТИЧНО:

```kotlin
// HermesGrpc.kt — кастомные Marshaller'ы для Hermes методов
// ПРАВИЛО: НЕ использовать сгенерированные gRPC stub'ы!
// Делать точно как OwlGrpc.kt: MethodDescriptor + кастомные Marshaller'ы

class OrchestratorRequestMarshaller : MethodDescriptor.Marshaller<OrchestratorRequestProto> {
    override fun stream(v: OrchestratorRequestProto): java.io.InputStream {
        val baos = ByteArrayOutputStream()
        val cos = com.google.protobuf.CodedOutputStream.newInstance(baos)
        if (v.userId.isNotEmpty()) cos.writeString(1, v.userId)
        if (v.sessionId.isNotEmpty()) cos.writeString(2, v.sessionId)
        if (v.message.isNotEmpty()) cos.writeString(3, v.message)
        if (v.agentId.isNotEmpty()) cos.writeString(4, v.agentId)
        if (v.mode.isNotEmpty()) cos.writeString(5, v.mode)
        // context map — сериализовать как вложенные сообщения
        cos.flush()
        return ByteArrayInputStream(baos.toByteArray())
    }
    override fun parse(s: java.io.InputStream): OrchestratorRequestProto = OrchestratorRequestProto()
}

class OrchestratorResponseMarshaller : MethodDescriptor.Marshaller<OrchestratorResponseProto> {
    override fun stream(v: OrchestratorResponseProto): java.io.InputStream {
        val baos = ByteArrayOutputStream()
        val cos = com.google.protobuf.CodedOutputStream.newInstance(baos)
        if (v.token.isNotEmpty()) cos.writeString(1, v.token)
        if (v.finished) cos.writeBool(2, v.finished)
        if (v.error.isNotEmpty()) cos.writeString(3, v.error)
        if (v.agentId.isNotEmpty()) cos.writeString(4, v.agentId)
        if (v.agentName.isNotEmpty()) cos.writeString(5, v.agentName)
        cos.flush()
        return ByteArrayInputStream(baos.toByteArray())
    }
    override fun parse(s: java.io.InputStream): OrchestratorResponseProto {
        val cis = com.google.protobuf.CodedInputStream.newInstance(s)
        var token = ""
        var finished = false
        var error = ""
        var agentId = ""
        var agentName = ""
        while (!cis.isAtEnd) {
            val tag = cis.readTag()
            if (tag == 0) break
            when (com.google.protobuf.WireFormat.getTagFieldNumber(tag)) {
                1 -> token = cis.readString()
                2 -> finished = cis.readBool()
                3 -> error = cis.readString()
                4 -> agentId = cis.readString()
                5 -> agentName = cis.readString()
                else -> cis.skipField(tag)
            }
        }
        return OrchestratorResponseProto(token, finished, error, agentId, agentName)
    }
}

// Streaming state — как в OwlGrpc.kt
private val _hermesResponses = MutableSharedFlow<OrchestratorResponseProto>(extraBufferCapacity = 64)
val hermesResponses: SharedFlow<OrchestratorResponseProto> = _hermesResponses

private val _hermesTyping = MutableSharedFlow<Boolean>(extraBufferCapacity = 8)
val hermesTyping: SharedFlow<Boolean> = _hermesTyping

// Основной метод — чат с оркестратором (server streaming)
fun chatWithOrchestrator(
    userId: String,
    sessionId: String,
    message: String,
    agentId: String = "",
    mode: String = "",
    scope: CoroutineScope,
    onResponse: (token: String, finished: Boolean, error: String?, agentId: String, agentName: String) -> Unit
) {
    // Реализация по аналогии с chatWithOWL из OwlGrpc.kt
    // MethodDescriptor.newBuilder → SERVER_STREAMING → "messenger.ChatService/ChatWithOrchestrator"
    // channel.newCall → ClientCall.Listener → onMessage/onClose
}

// Unary методы — по аналогии с createOwlChat/deleteOwlChat из OwlGrpc.kt
suspend fun listAgents(userId: String = ""): List<AgentInfoProto>
suspend fun listAgentPresets(): List<AgentPresetInfoProto>
suspend fun createAgent(userId: String, presetId: String, customName: String, customPrompt: String, model: String, maxTokens: Int): CreateAgentResponseProto
suspend fun updateAgent(agentId: String, userId: String, name: String, systemPrompt: String, model: String, maxTokens: Int): Boolean
suspend fun deleteAgent(agentId: String, userId: String): Boolean
suspend fun listUserAgents(userId: String): List<AgentInfoProto>
suspend fun createHermesSession(userId: String, agentId: String = "", mode: String = ""): CreateHermesSessionResponseProto
suspend fun deleteHermesSession(sessionId: String, userId: String): Boolean
suspend fun getOrchestratorHistory(sessionId: String, limit: Int = 50): List<OrchestratorHistoryMessageProto>

// Remote Agent методы (FUTURE — задел, можно оставить как TODO stubs)
suspend fun listRemoteAgents(filterStatus: String = ""): List<RemoteAgentInfoProto>
suspend fun deployAgentTask(agentId: String, taskType: String, params: Map<String, String>, workingDir: String, timeoutSec: Int): DeployAgentTaskResponseProto
suspend fun getRemoteAgentStatus(agentId: String): GetRemoteAgentStatusResponseProto
```

### Шаг 4: Добавить методы в GrpcClient.kt

Добавить в `object GrpcClient` (после OWL секции):

```kotlin
// ======= Hermes Multi-Agent Orchestrator =======

// Streaming — чат с оркестратором
fun chatWithOrchestrator(
    userId: String,
    sessionId: String,
    message: String,
    agentId: String = "",
    mode: String = "",
    scope: CoroutineScope,
    onResponse: (token: String, finished: Boolean, error: String?, agentId: String, agentName: String) -> Unit
) {
    lavender.client.android.data.grpc.chatWithOrchestrator(
        userId, sessionId, message, agentId, mode, scope, onResponse
    )
}

// StateFlow для Hermes ответов
val hermesResponses: SharedFlow<OrchestratorResponseProto> = lavender.client.android.data.grpc.hermesResponses
val hermesTyping: SharedFlow<Boolean> = lavender.client.android.data.grpc.hermesTyping

// Unary методы
suspend fun listAgents(userId: String = ""): List<AgentInfoProto> =
    lavender.client.android.data.grpc.listAgents(userId)

suspend fun listAgentPresets(): List<AgentPresetInfoProto> =
    lavender.client.android.data.grpc.listAgentPresets()

suspend fun createAgent(userId: String, presetId: String, customName: String, customPrompt: String, model: String, maxTokens: Int): CreateAgentResponseProto =
    lavender.client.android.data.grpc.createAgent(userId, presetId, customName, customPrompt, model, maxTokens)

suspend fun updateAgent(agentId: String, userId: String, name: String, systemPrompt: String, model: String, maxTokens: Int): Boolean =
    lavender.client.android.data.grpc.updateAgent(agentId, userId, name, systemPrompt, model, maxTokens)

suspend fun deleteAgent(agentId: String, userId: String): Boolean =
    lavender.client.android.data.grpc.deleteAgent(agentId, userId)

suspend fun listUserAgents(userId: String): List<AgentInfoProto> =
    lavender.client.android.data.grpc.listUserAgents(userId)

suspend fun createHermesSession(userId: String, agentId: String = "", mode: String = ""): CreateHermesSessionResponseProto =
    lavender.client.android.data.grpc.createHermesSession(userId, agentId, mode)

suspend fun deleteHermesSession(sessionId: String, userId: String): Boolean =
    lavender.client.android.data.grpc.deleteHermesSession(sessionId, userId)

suspend fun getOrchestratorHistory(sessionId: String, limit: Int = 50): List<OrchestratorHistoryMessageProto> =
    lavender.client.android.data.grpc.getOrchestratorHistory(sessionId, limit)

// Remote Agent методы (FUTURE)
suspend fun listRemoteAgents(filterStatus: String = ""): List<RemoteAgentInfoProto> =
    lavender.client.android.data.grpc.listRemoteAgents(filterStatus)

suspend fun getRemoteAgentStatus(agentId: String): GetRemoteAgentStatusResponseProto =
    lavender.client.android.data.grpc.getRemoteAgentStatus(agentId)
```

### Шаг 5: Создать data classes и Repository

**HermesRepository.kt** — репозиторий для работы с Hermes данными:
```kotlin
class HermesRepository {
    suspend fun getAgents(): List<AgentInfo> { ... }
    suspend fun getPresets(): List<AgentPreset> { ... }
    suspend fun createAgent(...): Result<AgentInfo> { ... }
    suspend fun deleteAgent(...): Result<Boolean> { ... }
    suspend fun getHistory(sessionId: String): List<HermesMessage> { ... }
    suspend fun getRemoteAgents(): List<RemoteAgentInfo> { ... }
}
```

**Data classes** (в `ui/hermes/model/`):
```kotlin
data class HermesMessage(
    val id: String,
    val role: String,        // "user", "assistant", "agent"
    val content: String,
    val agentId: String?,
    val agentName: String?,
    val timestamp: Long,
    val isStreaming: Boolean = false
)

data class AgentInfo(
    val id: String,
    val name: String,
    val description: String,
    val role: String,
    val isPreset: Boolean,
    val icon: String,
    val createdBy: String
)

data class AgentPreset(
    val id: String,
    val name: String,
    val role: String,
    val description: String,
    val icon: String,
    val maxTokens: Int
)

data class RemoteAgentInfo(
    val id: String,
    val name: String,
    val host: String,
    val ipAddress: String,
    val os: String,
    val status: String,
    val capabilities: List<String>,
    val activeTasks: Int,
    val lastHeartbeat: String,
    val cpuPercent: Float = 0f,      // FUTURE
    val memoryPercent: Float = 0f    // FUTURE
)

data class HermesSession(
    val id: String,
    val userId: String,
    val activeAgentId: String?,
    val mode: String,
    val createdAt: Long,
    val updatedAt: Long
)
```

### Шаг 6: Создать UI

#### HermesChatActivity + HermesChatViewModel

**HermesChatViewModel.kt:**
```kotlin
class HermesChatViewModel : ViewModel() {
    // State
    val messages: StateFlow<List<HermesMessage>> = ...
    val isTyping: StateFlow<Boolean> = ...
    val currentSession: StateFlow<HermesSession?> = ...
    val error: StateFlow<String?> = ...

    // Actions
    fun sendMessage(text: String, agentId: String = "", mode: String = "")
    fun loadHistory(sessionId: String)
    fun createSession(agentId: String = "", mode: String = "")
    fun deleteSession(sessionId: String)
    fun switchAgent(agentId: String)
}
```

**HermesChatActivity.kt:**
- Toolbar с названием текущего агента/оркестратора
- RecyclerView с сообщениями (user сообщения справа, agent — слева с иконкой агента)
- Поле ввода + кнопка отправки
- Индикатор набора (typing indicator) — когда `isTyping = true`
- Автоскролл к последнему сообщению
- Обработка ошибок — Snackbar

**HermesChatAdapter.kt:**
- ViewType: USER_MESSAGE, AGENT_MESSAGE, SYSTEM_MESSAGE
- Для AGENT_MESSAGE — показывать иконку и имя агента
- Streaming сообщения — обновлять последний элемент списка

#### AgentListActivity + AgentListViewModel

**AgentListViewModel.kt:**
```kotlin
class AgentListViewModel : ViewModel() {
    val presets: StateFlow<List<AgentPreset>> = ...
    val customAgents: StateFlow<List<AgentInfo>> = ...
    val remoteAgents: StateFlow<List<RemoteAgentInfo>> = ...  // FUTURE
    val isLoading: StateFlow<Boolean> = ...

    fun loadAgents()
    fun createCustomAgent(presetId: String, name: String)
    fun deleteCustomAgent(agentId: String)
    fun openAgentChat(agentId: String)
    fun openAgentSettings(agentId: String)
}
```

**AgentListActivity.kt:**
- TabLayout: "Пресеты" | "Мои агенты" | "Удалённые" (FUTURE)
- RecyclerView с карточками агентов
- Карточка: иконка (emoji), имя, роль, описание
- FAB для создания нового агента
- Swipe-to-refresh

#### AgentSettingsActivity

- Выбор пресета (Spinner/RecyclerView)
- Поля: имя, системный промпт, модель, max_tokens
- Кнопки: Сохранить / Удалить
- Валидация полей

### Шаг 7: Тестирование

- Мониторить логи сервера: `http://13.140.25.249:8091/logs`
- Проверить что gRPC вызовы доходят до сервера
- Проверить что ChatWithOrchestrator стримит ответы
- Проверить что ListAgents/ListAgentPresets возвращают данные
- Проверить что CreateAgent/DeleteAgent работают
- Проверить что история загружается корректно

## КРИТИЧЕСКИЕ PITFALLS

1. **gRPC Marshaller** — используй кастомные Marshaller'ы как в OwlGrpc.kt, НЕ сгенерированный код. Это КРИТИЧНО — проект использует именно этот подход.

2. **Потокобезопасность** — все gRPC callbacks приходят в background thread, используй `withContext(Dispatchers.Main)` для UI. Используй StateFlow/SharedFlow для реактивного обновления UI.

3. **Streaming** — ChatWithOrchestrator это server-side stream, не забудь обработку `finished=true` и ошибок в `onClose()`.

4. **Proto generation** — после изменения proto обязательно запустить `./gradlew generateDebugProto`. Проверь что сгенерированные классы появились.

5. **Dev сервер** — тестируй на `13.140.25.249:50052`, мониторь логи на `http://13.140.25.249:8091/logs`.

6. **State management** — используй StateFlow вместо LiveData. Подписка на StateFlow в Activity через `lifecycleScope.launch { repeatOnLifecycle(STARTED) { ... } }`.

7. **Error handling** — всегда обрабатывай ошибки gRPC в `onClose()` listener'е. Показывай пользователю через Snackbar/Toast.

8. **Memory leaks** — отменяй CoroutineScope в `onDestroy()` Activity. Используй `viewModelScope` в ViewModel.

9. **Session management** — храни текущую сессию в ViewModel, восстанавливай при повороте экрана.

10. **Backpressure** — SharedFlow с `extraBufferCapacity = 64` как в OwlGrpc — не теряй сообщения при быстром стриминге.

## ЗАДЕЛ ДЛЯ БУДУЩИХ УЛУЧШЕНИЙ (FUTURE)

Эти элементы должны быть заложены в архитектуру, но НЕ реализованы в MVP:

### 1. Remote Agent Monitor
- `RemoteAgentMonitorActivity` — мониторинг удалённых агентов в реальном времени
- Показывает: статус, CPU, память, активные задачи
- `StreamAgentEvents` — подписка на события агентов
- Заложить proto сообщения: `StreamAgentEventsRequest`, `AgentEvent`

### 2. Agent-to-Agent коммуникация
- `CreateAgentChannel` / `SendAgentMessage` — агенты обмениваются данными
- UI: визуализация каналов между агентами
- Заложить proto сообщения в messenger.proto

### 3. Pipeline визуализация
- `CreatePipelineSession` / `GetPipelineStatus` — управление pipeline
- UI: визуализация этапов pipeline с progress bars
- Заложить proto сообщения в messenger.proto

### 4. Автоскейлинг агентов
- Proto сообщения для автоматического создания/удаления агентов
- UI: настройки автоскейлинга (min/max agents, triggers)

### 5. Android как Remote Agent
- `HermesRemoteGrpc.kt` — Android устройство как remote agent
- Подключение к `HermesAgentService` на сервере
- Выполнение задач: shell (через Termux API), file operations

### 6. Мониторинг и алерты
- Health check endpoint для оркестратора
- Уведомления о проблемах с агентами
- Дашборд состояния в Android

## ФАЙЛЫ ДЛЯ ЧТЕНИЯ ПРИ СТАРТЕ

Обязательно прочитай:
1. `/root/msg.client.android/app/src/main/proto/messenger.proto` — текущий Android proto (748 строк, БЕЗ Hermes)
2. `/root/msg/messenger.proto` — серверный proto с Hermes методами (997 строк)
3. `/root/msg.client.android/app/src/main/java/lavender/client/android/data/grpc/OwlGrpc.kt` — пример gRPC клиента с кастомными Marshaller'ами (405 строк, ЧИТАТЬ ПОЛНОСТЬЮ)
4. `/root/msg.client.android/app/src/main/java/lavender/client/android/data/grpc/GrpcClient.kt` — куда добавлять методы (420 строк)
5. `/root/msg.client.android/app/src/main/java/lavender/client/android/data/grpc/RealGrpcClient.kt` — реализация gRPC подключения
6. `/root/msg/server.go` — серверная реализация (3387 строк, читай строки 3080-3387 — Hermes методы)
7. `/root/msg/hermes_orchestrator.go` — оркестратор (467 строк)
8. `/root/msg/hermes_agents.go` — реестр агентов (417 строк)
9. `/root/msg/hermes_remote_manager.go` — менеджер удалённых агентов (421 строка)
10. `/root/msg/hermes_remote.proto` — протокол удалённых агентов (144 строки)
11. `/root/msg/db_hermes.go` — миграции БД (227 строк)

## ВАЖНО

- НЕ торопись — проверяй каждый файл перед изменением
- НЕ задавай уточняющих вопросов если код доступен — читай и делай
- НЕ используй gradle на сервере (только на твоей локальной машине для Android)
- НЕ редактируй сгенерированные proto файлы в `data/proto/`
- Всегда проверяй что proto генерация прошла успешно
- Тестируй gRPC вызовы через dev сервер перед коммитом
- Следуй паттернам из `OwlGrpc.kt` — это эталон для проекта

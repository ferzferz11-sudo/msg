# Lavender Messenger — AI Services

Документация по AI-сервисам: OWL AI и Hermes Orchestrator.

**Обновлено:** 2026-06-09
**Ветка:** feat/1.1.2.x

---

## Архитектура

```
┌─────────────────────────────────────────────────────────────┐
│                        ANDROID CLIENT                       │
│                                                             │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐  │
│  │  OwlGrpc.kt  │  │ HermesGrpc.kt│  │  GrpcClient.kt   │  │
│  │  (OWL API)   │  │ (Hermes API) │  │  (единая точка)  │  │
│  └──────┬───────┘  └──────┬───────┘  └────────┬─────────┘  │
│         │                 │                    │            │
│  ┌──────┴───────┐  ┌──────┴───────┐           │            │
│  │OwlChatActivity│  │HermesChatActivity│        │            │
│  │OwlChatViewModel│ │HermesChatViewModel│       │            │
│  └──────────────┘  └──────────────┘           │            │
└─────────────────────────┬───────────────────────┘            │
                          │ gRPC                              │
┌─────────────────────────┴───────────────────────────────────┐
│                        SERVER                                │
│                                                             │
│  ┌─────────────────────────────────────────────────────┐    │
│  │                    server.go                         │    │
│  │  gRPC handlers, маршрутизация, rate limiting         │    │
│  └─────────────┬──────────────────────┬────────────────┘    │
│                │                      │                      │
│  ┌─────────────┴──────────┐  ┌───────┴────────────────┐    │
│  │       owl.go           │  │  hermes_orchestrator.go │    │
│  │  OWL AI: streaming,    │  │  Hermes: маршрутизация  │    │
│  │  sessions, history     │  │  агентов, RAG, tools    │    │
│  └─────────────┬──────────┘  └───────┬────────────────┘    │
│                │                      │                      │
│  ┌─────────────┴──────────┐  ┌───────┴────────────────┐    │
│  │   owlSessionManager    │  │  HermesAgentRegistry   │    │
│  │  (DB-backed sessions)  │  │  (агенты, пресеты)     │    │
│  └─────────────┬──────────┘  └───────┬────────────────┘    │
│                │                      │                      │
│  ┌─────────────┴──────────────────────┴────────────────┐    │
│  │                    PostgreSQL                        │    │
│  │  chats │ owl_messages │ hermes_sessions │            │    │
│  │  owl_chat_settings │ hermes_messages │              │    │
│  └─────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────┘
```

**Принцип:** полная изоляция OWL и Hermes — разные файлы, разные таблицы БД, разные gRPC методы.

---

## Таблицы БД

### chats
Единая таблица для всех чатов (обычные, OWL, Hermes).
- `id` — уникальный ID (для OWL: `owl-<uuid>`, для Hermes: `hermes-<uuid>`)
- `type` — `'regular'`, `'owl'`, `'hermes'`
- `creator_id` — UUID создателя (НЕ username!)
- `participants` — JSON массив UUID
- `last_message_text` — текст последнего сообщения (для UI)
- `last_message_time` — время последнего сообщения

### owl_messages
История сообщений OWL чатов.
- `chat_id` → `chats.id` (FK, CASCADE)
- `role` — `'user'`, `'assistant'`
- `content` — текст сообщения

### owl_chat_settings
Настройки каждого OWL чата.
- `chat_id` → `chats.id` (FK, CASCADE)
- `user_api_key` — персональный API ключ
- `model` — модель OpenRouter

### hermes_sessions
Сессии оркестратора (внутренние, не для UI).
- `id` — `hermes-<uuid>` (уникальный)
- `user_id` — UUID пользователя
- `name` — название сессии
- `active_agent_id` — текущий активный агент
- `agent_mode` — `'single'`, `'parallel'`, `'pipeline'`

### hermes_messages
История сообщений Hermes.
- `session_id` → `hermes_sessions.id` (FK, CASCADE)
- `role` — `'user'`, `'assistant'`, `'system'`, `'agent'`
- `agent_id` — ID агента (пусто = оркестратор)
- `content` — текст сообщения

### hermes_chat_settings
Настройки Hermes чатов.
- `chat_id` → `chats.id` (FK, CASCADE)
- `user_api_key` — персональный API ключ
- `model` — модель

---

## OWL AI

### Поток данных

```
Client                          Server
  │                               │
  │── ChatWithOWL(stream) ───────►│
  │                               │── addMessage(chatID, "user", msg)
  │                               │── getHistory(chatID)
  │                               │── callOpenRouter(history)
  │                               │── addMessage(chatID, "assistant", response)
  │                               │── UPDATE chats SET last_message_text
  │◄── stream chunks ────────────│
  │                               │
```

### API

| Метод | Тип | Описание |
|-------|-----|----------|
| `CreateOwlChat` | Unary | Создание OWL чата |
| `ChatWithOWL` | Server streaming | Стриминг ответа AI |
| `GetOwlHistory` | Unary | История сообщений |
| `DeleteOwlChat` | Unary | Удаление чата |
| `GetOwlSettings` | Unary | Настройки (ключ, модель) |
| `SaveOwlSettings` | Unary | Сохранение настроек |

### Rate Limiting
- С ключом: 10 запросов/минуту
- Без ключа: 20 запросов/час

---

## Hermes Orchestrator

### Поток данных

```
Client                    Server
  │                         │
  │── CreateHermesSession ─►│── INSERT hermes_sessions
  │                         │── INSERT chats (type='hermes')
  │                         │
  │── ChatWithOrchestrator ►│── getOrCreateSession(userID)
  │                         │── analyzeRequest() → OpenRouter
  │                         │── routing decision
  │                         │── runSingleAgent(agentID)
  │◄── stream chunks ──────│
  │                         │── SaveOrchestratorMessage(user)
  │                         │── SaveOrchestratorMessage(assistant)
  │                         │── UPDATE chats SET last_message_text
  │                         │
  │── GetOrchestratorHistory►│── SELECT hermes_messages
  │                         │
  │── DeleteHermesSession ─►│── DELETE hermes_sessions
  │                         │── DELETE chats
```

### API

| Метод | Тип | Описание |
|-------|-----|----------|
| `CreateHermesSession` | Unary | Создание сессии |
| `ChatWithOrchestrator` | Server streaming | Стриминг ответа |
| `GetOrchestratorHistory` | Unary | История сообщений |
| `DeleteHermesSession` | Unary | Удаление сессии |
| `ListAgents` | Unary | Список агентов |
| `ListAgentPresets` | Unary | Пресеты агентов |
| `CreateAgent` | Unary | Создание агента |
| `UpdateAgent` | Unary | Обновление агента |
| `DeleteAgent` | Unary | Удаление агента |
| `GetHermesSettings` | Unary | Настройки |
| `SaveHermesSettings` | Unary | Сохранение настроек |

### Агенты (пресеты)

| ID | Имя | Роль |
|----|-----|------|
| `hermes-owl` | OWL AI | Универсальный AI (fallback) |
| `hermes-developer` | Developer | Разработка кода |
| `hermes-devops` | DevOps | Сервер, деплой, мониторинг |
| `hermes-architect` | Architect | Архитектура систем |
| `hermes-support` | Support | Поддержка пользователей |
| `hermes-qa` | QA Engineer | Тестирование |
| `hermes-analyst` | Analyst | Анализ данных |
| `hermes-security` | Security | Безопасность |

### Маршрутизация

Оркестратор анализирует запрос через OpenRouter и выбирает агента:
1. Формирует промпт со списком агентов
2. Отправляет в OpenRouter
3. Парсит JSON ответ: `{"mode": "single", "agents": ["hermes-xxx"], "reason": "..."}`
4. Выполняет через выбранного агента

Fallback: если ошибка анализа или агент не найден → `hermes-owl`.

---

## Proto field mapping

**ВАЖНО:** при изменении `.proto` файлов всегда сверять номера полей с кодом клиента!

### CreateHermesSessionResponse
```
message CreateHermesSessionResponse {
  bool success = 1;        // field 1 = success (bool)
  string session_id = 2;   // field 2 = session_id (string)
  string name = 4;         // field 4 = name (string)
  string error = 3;        // field 3 = error (string)
}
```

### CreateAgentResponse
```
message CreateAgentResponse {
  bool success = 1;        // field 1 = success (bool)
  string agent_id = 2;     // field 2 = agent_id (string)
  string error = 3;        // field 3 = error (string)
}
```

### AgentInfo
```
message AgentInfo {
  string id = 1;
  string name = 2;
  string description = 3;
  bool is_preset = 4;
  string system_prompt = 5;
  string model = 6;
}
```

### OrchestratorResponse
```
message OrchestratorResponse {
  string token = 1;
  bool finished = 2;
  string error = 3;
}
```

---

## Известные проблемы и решения

### Дублирование Hermes сессий
**Проблема:** `getOrCreateSession` создаёт сессию с `id = "hermes-" + userID`, а `CreateHermesSession` создаёт с `id = "hermes-" + UUID`. В результате две записи в `hermes_sessions` для одного пользователя.

**Решение:** `getOrCreateSession` должен искать существующую сессию по `user_id`, а не создавать новую с фиксированным ID.

### Пустой last_message_text для Hermes
**Проблема:** `ChatWithOrchestrator` сохранял сообщения в `hermes_messages`, но не обновлял `chats.last_message_text`.

**Решение:** добавлен `UPDATE chats SET last_message_text` после ответа.

### Дубли чатов в UI
**Проблема:** `GetAIChats` брал Hermes из `hermes_sessions`, а OWL из `chats`. Hermes сессии дублировались.

**Решение:** `GetAIChats` берёт оба типа из `chats`.

---

## Команды

```bash
# Сборка и деплой на dev
cd /root/msg
export PATH=$PATH:/usr/local/go/bin:~/go/bin
go build -o /tmp/lavender-server-dev .
systemctl stop lavender-server-dev
cp /tmp/lavender-server-dev /root/LavenderMessenger/run/lavender-server-dev
systemctl start lavender-server-dev

# Сборка и деплой на prod
go build -o /tmp/lavender-server .
systemctl stop lavender-server
cp /tmp/lavender-server /root/LavenderMessenger/run/lavender-server
systemctl start lavender-server

# Proto gen
cd /root/msg && protoc --go_out=./gen --go_opt=paths=source_relative \
  --go-grpc_out=./gen --go-grpc_opt=paths=source_relative messenger.proto

# Android
cd /root/msg.client.android
./gradlew compileDebugKotlin
```

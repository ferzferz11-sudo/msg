# Lavender Messenger — Project Memory
# Created: 2026-05-28
# Updated: 2026-06-04 — Hermes Orchestrator v1.1.0.12 (Unified Chat Widget)

## Репозитории

### Сервер
- **Git:** `ferzferz11-sudo/msg`
- **Dev сервер:** `13.140.25.249`, путь `/root/msg`
- **Production:** `159.195.38.145`, путь `/root/LavenderMessenger/run`
- **Ветка:** `main`
- **PostgreSQL:** user `lavender`, database `chat_db` (prod), `chat_db_dev` (dev)
- **systemd:** `lavender-server.service` (prod), `lavender-server-dev.service` (dev)

### Клиент (Android)
- **Git:** `ferzferz11-sudo/msg.client.android`
- **Dev сервер:** `/root/msg.client.android` на `13.140.25.249`
- **Ветка:** `master`

## Сервер — ключевые файлы

### Ядро
- `main.go` — gRPC + HTTP серверы, точка входа
- `server.go` — основные gRPC хендлеры (~3550 строк)
- `secret_chat.go` — E2EE хендлеры
- `server_management.go` — мультисерверность, админ RPC
- `db.go` — PostgreSQL, миграции
- `hub.go` — менеджер подключений
- `http_server.go` — HTTP (8081 APK, 8082 uploads)
- `crypto.go` — AES-256-GCM + bcrypt
- `email.go` — email уведомления

### Hermes Orchestrator (v1.1.0.15)
- `hermes_orchestrator.go` — оркестратор, LLM Router + RAG + Pipeline init
- `hermes_agents.go` — реестр агентов (8 агентов: 7 пресетов + hermes-owl)
- `hermes_remote_manager.go` — remote agents (heartbeat, tasks)
- `hermes_agent_service.go` — bidirectional stream для hermes-agent daemon (НЕ РЕАЛИЗОВАНО)
- `db_hermes.go` — миграции Hermes таблиц
- `owl.go` — OWL AI assistant

### Core (Ports & Adapters)
- `core/llm/provider.go` — LLMProvider, LLMRouter, SimpleRouter, StreamChunk, Message interfaces
- `core/llm/openrouter/provider.go` — OpenRouter SSE provider (tool calls, multimodal)
- `core/llm/hermes/provider.go` — Hermes local provider (`hermes chat -q --quiet`)
- `core/rag/interfaces.go` — EmbeddingService, VectorSearch, RAGPipeline interfaces
- `core/rag/memory/memory.go` — in-memory RAG (TF-IDF, 384 dim, cosine similarity)
- `core/rag/memory/memory_test.go` — unit тесты RAG (4 теста)
- `core/pipeline/pipeline.go` — RAG → LLM → Tool Calling loop (adaptive, max 10)
- `core/tools/executor.go` — DefaultToolExecutor (search_messages, search_users, web_search, get_chat_info)

## Клиент — ключевые файлы

- `RealGrpcClient.kt` — gRPC (~3000 строк, protobuf-lite ручной парсинг)
- `GrpcClient.kt` — фасад
- `CredentialStore.kt` — EncryptedSharedPreferences (server_address, key)
- `SessionManager.kt` — StateFlow<UserSession>
- `E2EEManager.kt` — ECDH + AES-256-GCM
- `data/db/` — Room (AppDatabase, Daos, Entities)
- `ChatViewModel.kt` / `ChatListViewModel.kt` — MVVM
- `theme/` — система тем

## Версионирование

- Android: `version.txt` в корне
- Сервер: `const ServerVersion` в server.go
- versionCode = major*1000000 + minor*10000 + patch*100 + build

## Технический стек

### Сервер
- Go 1.26, gRPC, PostgreSQL, Firebase Cloud Messaging
- AES-256-GCM, bcrypt, keepalive 20s/20s
- systemd сервис, .env конфигурация
- Hermes Agent v0.14.0 (Python 3.11.15) — локальный LLM провайдер

### Клиент
- Kotlin, gRPC (protobuf-lite manual), Room, Firebase, WebRTC
- minSdk 29, compileSdk 37, targetSdk 35
- MVVM + StateFlow + ViewBinding
- Material Design 3

## Архитектурные решения

### E2EE Secret Chats
- ECDH (secp256r1) обмен ключами
- AES-256-GCM шифрование
- Сервер НЕ может расшифровать

### Credential Storage
- EncryptedSharedPreferences (AndroidX Security)
- Авто-миграция из plaintext

### Мультисерверность
- `ListServers` RPC (публичный)
- Admin методы используют `AdminAuth`
- Android: CredentialStore хранит server_address

### Обратная совместимость
- Новые proto поля — optional (proto3)
- Новые RPC — старые клиенты не вызывают

### Hermes Orchestrator — LLM Router
- OpenRouter (default) — SSE streaming, tool calls, multimodal images
- Hermes local (prefix=local/) — CLI wrapper, stateless, session через --resume
- Регистрация: `llmRouter.Register(llm.RouteRule{ModelPrefix, Provider, Priority})`

### Hermes Orchestrator — RAG Pipeline
- Интерфейсы: EmbeddingService, VectorSearch, RAGPipeline
- Текущая реализация: in-memory TF-IDF (384 dim)
- Production план: Qdrant + CLIP для мультимодальных эмбеддингов

### Hermes Orchestrator — Tool Executor
- search_messages: ILIKE по messages таблице
- search_users: поиск по username/display_name/phone
- web_search: DuckDuckGo Instant Answer API
- get_chat_info: имя чата, тип, количество участников

### Hermes Orchestrator — Pipeline
- Адаптивный tool calling loop: max 10 итераций (страховка)
- Цикл продолжается пока LLM вызывает tools
- Останавливается когда LLM даёт финальный ответ без tool calls

### Android — Unified Chat Widget (v1.1.0.12)
- **layout/widget_chat.xml** — единый layout для NewChatActivity и HermesChatActivity
- **layout/item_chat_message.xml** — универсальный item (user/agent/system/typing/date)
- **ui/chat/widget/ChatMessageAdapter.kt** — единый адаптер с DiffUtil
- **ui/chat/widget/ChatWidget.kt** — ViewBinding обёртка с общим API
- **HermesChatActivity** — агенты как участники группового чата (emoji + name)
- **ChatMessageItem** — универсальная data class для всех типов сообщений

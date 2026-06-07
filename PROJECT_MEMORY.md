# Lavender Messenger — Project Memory
# Created: 2026-05-28
# Updated: 2026-07-15

## Репозитории

### Сервер
- **Git:** `ferzferz11-sudo/msg`
- **Dev сервер:** `13.140.25.249`, путь `/root/msg`
- **Production:** `159.195.38.145`, путь `/root/LavenderMessenger/run`
- **Ветка:** `feat/1.1.0.x` (dev), `main` (prod)
- **PostgreSQL:** user `lavender`, database `chat_db` (prod), `chat_db_dev` (dev)
- **systemd:** `lavender-server.service` (prod), `lavender-server-dev.service` (dev)

### Клиент (Android)
- **Git:** `ferzferz11-sudo/msg.client.android`
- **Dev сервер:** `/root/msg.client.android` на `13.140.25.249`
- **Ветка:** `feat/1.1.0.x`
- **Сборка:** `./gradlew compileDebugKotlin` (НЕ assembleRelease — OOM kill)

## Ключевые серверные файлы

| Файл | Назначение | Строк |
|------|-----------|-------|
| `main.go` | gRPC + HTTP серверы, точка входа | ~200 |
| `server.go` | Основные gRPC хендлеры + agent management + orchestrator | ~3440 |
| `hermes_orchestrator.go` | Оркестратор — маршрутизация к агентам | ~500 |
| `hermes_agents.go` | Реестр агентов, пресеты, CRUD кастомных агентов | ~450 |
| `hermes_remote_manager.py` | Remote agent daemon, heartbeat, task dispatch | ~300 |
| `db_hermes.go` | Миграции hermes_* таблиц | ~230 |
| `db.go` | PostgreSQL, основные миграции | ~300 |
| `hub.go` — менеджер подключений | ~350 |
| `messenger.proto` | gRPC определения | ~960 |
| `gen/messenger.pb.go` | Сгенерированный код (НЕ редактировать) | ~9500 |
| `hermes_agent_service.go` | HermesAgentService — подключение agent daemon | ~200 |
| `owl.go` | OWL AI streaming | ~1500 |
| `secret_chat.go` | E2EE хендлеры | ~200 |
| `http_server.go` | HTTP (8083 dev, 8081 prod) | ~250 |
| `crypto.go` | AES-256-GCM + bcrypt | ~150 |

## Ключевые клиентские файлы

| Файл | Назначение | Строк |
|------|-----------|-------|
| `RealGrpcClient.kt` | gRPC клиент (protobuf-lite ручный парсинг) | ~3000 |
| `GrpcClient.kt` — фасад | ~430 |
| `ChatListActivity.kt` | Главный экран — список чатов | ~2750 |
| `SessionManager.kt` | Логин, регистрация, сессия | ~280 |
| `HermesChatActivity.kt` | Чат с оркестратором | ~800 |
| `AgentListActivity.kt` | Список агентов | ~240 |
| `AgentListViewModel.kt` | VM для агентов | ~100 |
| `HermesRepository.kt` | Репозиторий агентов | ~80 |
| `HermesGrpc.kt` | Hermes gRPC вызовы | ~830 |
| `CredentialStore` | EncryptedSharedPreferences | ~60 |

## Версионирование

- Android: `versionCode` 1100150, `versionName` 1.1.0.15
- Сервер: `const ServerVersion = "1.1.0.10"`
- versionCode = major*1000000 + minor*10000 + patch*100 + build

## Технический стек

### Сервер
- Go 1.26+, gRPC, PostgreSQL, Firebase Cloud Messaging
- AES-256-GCM, bcrypt, keepalive 15s/10s
- systemd сервис, .env конфигурация
- OpenRouter API (AI агенты) — prod key working, dev key invalid (401)

### Клиент
- Kotlin, gRPC (protobuf-lite manual), Room, Firebase, WebRTC
- minSdk 29, compileSdk 37, targetSdk 35
- MVVM + StateFlow + ViewBinding
- Material Design 3

## Архитектура Hermes Orchestrator

```
Android → gRPC → ChatService_ChatWithOrchestrator → HermesOrchestrator.Orchestrate()
                                         │
                                         ├── Registry (8 agents: 7 preset + OWL fallback)
                                         │     ├── Developer (💻), Analyst, Security, DevOps (🔧)
                                         │     ├── Architect (🏗️), Support, QA Engineer
                                         │     └── hermes-owl (AI Assistant)
                                         ├── OpenRouter API → LLM response
                                         └── RemoteAgentManager (heartbeat, tasks)
```

## Конфигурация dev/prod

### Dev
- gRPC port: 50052, HTTP: 8083
- DB: `chat_db_dev`
- Binary: `/root/LavenderMessenger/run/lavender-server-dev`
- `.env.dev`: `OPENROUTER_MODEL=openrouter/owl-alpha`
- Log monitor: `server-logs-dev` (port гарантирован)

### Prod
- gRPC port: 50051, HTTP: 8081
- DB: `chat_db`
- Binary: `/root/LavenderMessenger/run/lavender-server`
- `.env`: `OPENROUTER_MODEL=openrouter/owl-alpha`

## Полезные команды

```bash
# Proto generation
cd /root/msg
protoc --go_out=./gen --go_opt=paths=source_relative --go-grpc_out=./gen --go-grpc_opt=paths=source_relative messenger.proto
# НЕ использовать --go_out=. — генерирует в корень, ломает сборку

# Dev server rebuild + deploy
export PATH=$PATH:/usr/local/go/bin:~/go/bin
go build -o /tmp/lavender-server-dev .
systemctl stop lavender-server-dev && cp /tmp/lavender-server-dev /root/LavenderMessenger/run/lavender-server-dev && systemctl start lavender-server-dev

# Prod server rebuild + deploy
export PATH=$PATH:/usr/local/go/bin:~/go/bin
go build -o /root/msg/run/lavender-server .
systemctl stop lavender-server && cp /root/msg/run/lavender-server /root/LavenderMessenger/run/lavender-server && systemctl start lavender-server

# Log monitor rebuild + deploy
cd /root/msg/log-monitor
go build -o /root/LavenderMessenger/run/log-monitor-dev log-monitor-dev.go
systemctl restart log-monitor-dev

# Android build (LOCAL Mac, NOT server — OOM kill)
cd /Users/paveld/LavenderMessenger-Android
./gradlew assembleDebug
# или ./gradlew compileDebugKotlin для проверки компиляции
```

## Deployment rules

- **Сервер:** OWL деплоит на dev напрямую через systemctl
- **Android:** OWL коммитит и пушит, пользователь собирает локально на Mac
- **НЕ запускать** `./gradlew assembleRelease` на сервере — OOM kill (нужно 2GB+)
- **НЕ редактировать** `gen/` файлы — перегенерировать через protoc

## Credentials & Environment

- SSH: `ssh root@13.140.25.249` (dev), `ssh root@159.195.38.145` (prod)
- Git access: HTTPS token for `ferzferz11-sudo/msg` and `ferzferz11-sudo/msg.client.android`
- PostgreSQL user: `lavender`
- Firebase: project `lavender-messenger`

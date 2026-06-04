# Hermes Orchestrator — Промт для новой сессии (v1.1.0.11)

## Статус проекта

**Версия:** v1.1.0.11
**Дата:** 2026-06-04
**Статус:** ✅ Hermes Orchestrator работает — Android подключается, оркестратор отвечает

## Архитектура

```
Android App ←→ gRPC ←→ Dev Server (50052)
                            ↓
                    Hermes Orchestrator
                            ↓
                    LLM Router (OpenRouter / Hermes local)
                            ↓
                    RAG Pipeline (in-memory TF-IDF, 384 dim)
                            ↓
                    Tool Executor (search_messages, search_users, web_search, get_chat_info)
                            ↓
                    Pipeline (RAG → LLM → Tool Calling loop, max 3 iter)
```

## Ключевые файлы

### Сервер (`/root/msg/`)
- `main.go` — gRPC server registration, dev/prod binary paths
- `server.go` — все gRPC методы (auth, chat, Hermes sessions)
- `hermes_orchestrator.go` — Orchestrator, LLM Router, RAG, Pipeline, Tool Executor
- `core/llm/` — LLM providers (OpenRouter, Hermes local)
- `core/rag/memory/` — in-memory RAG (TF-IDF)
- `core/tools/` — Tool Executor
- `core/pipeline/` — Pipeline
- `messenger.proto` — proto definitions
- `gen/` — сгенерированный Go код

### Android (`msg.client.android` на Mac пользователя)
- `HermesGrpc.kt` — gRPC методы для Hermes (createSession, chatWithOrchestrator)
- `HermesChatActivity.kt` — UI чата с оркестратором
- `HermesChatViewModel.kt` — ViewModel для чата
- `AppLog.kt` — система логирования ошибок
- `LogViewerActivity.kt` — просмотр логов из админки
- `SuperAdminActivity.kt` — админ панель
- `RealGrpcClient.kt` — gRPC клиент (канал, стримы)
- `AndroidManifest.xml` — регистрация Activity

## Что работает

- ✅ Авторизация на dev сервере (port 50052)
- ✅ SuperAdmin по user_id (UUID)
- ✅ CreateHermesSession — создание сессии
- ✅ ChatWithOrchestrator — стриминг ответов оркестратора
- ✅ Оркестратор отвечает приветственным сообщением
- ✅ LogViewerActivity — просмотр логов из админки
- ✅ AppLog — логирование ошибок

## Известные проблемы

- ⚠️ Tool Calling Loop — жёсткий лимит 3 итерации, нужна авто-финализация
- ⚠️ Hermes local provider — может не работать если `hermes` CLI не установлен
- ⚠️ RemoteAgentManager.SendTask() — только заглушка

## Что делать дальше

### Приоритет 1: Tool Calling Loop
Файл: `core/pipeline/pipeline.go`
- Убрать жёсткий лимит `maxIterations = 3`
- Добавить детекцию завершения: если LLM не вызывает tools в ответе → финализировать

### Приоритет 2: Тестирование Hermes на Android
- Проверить все агенты реестра
- Проверить маршрутизацию между агентами
- Проверить tool calling (search_messages и т.д.)

### Приоритет 3: Agent Settings Bottom Sheet
- Привязать long click по аватарке агента в чате
- Показать настройки агента (model, system prompt и т.д.)

## Деплой

### Dev сервер
```bash
export PATH=$PATH:/usr/local/go/bin:~/go/bin
cd /root/msg
go build -o /tmp/lavender-server-dev .
systemctl stop lavender-server-dev
cp /tmp/lavender-server-dev /root/LavenderMessenger/run/lavender-server-dev
systemctl start lavender-server-dev
```

### Android
- Пользователь собирает APK локально на Mac
- `compileDebugKotlin` OK, `assembleRelease` — OOM на сервере!

## Важные заметки

- **НЕ использовать `--go_out=.`** — генерирует в корень, ломает сборку
- **Proto gen:** `cd /root/msg && protoc --go_out=./gen --go_opt=paths=source_relative --go-grpc_out=./gen --go-grpc_opt=paths=source_relative messenger.proto`
- **Android proto** — это ручные data class'ы в `MessengerProto.kt`, НЕ сгенерированные из .proto
- **Proto field numbers должны совпадать** между Android и сервером — проверять при изменении!

## Коммиты

- `7b87739` — fix: proto mismatch в CreateHermesSession
- `ba53e00` — fix: LogViewerActivity в AndroidManifest
- `6d89d84` — docs: v1.1.0.11
- `d7ccbac` — feat: Hermes Orchestrator full architecture

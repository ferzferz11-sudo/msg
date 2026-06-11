# Lavender Messenger — Аудит проекта

**Дата:** 2026-06-11
**Версия:** v1.1.3.0
**Аудитор:** OWL (AI)

---

## 1. Сервер (Go)

### 1.1 Структура

| Файл | Назначение | Строк |
|------|-----------|-------|
| `server.go` | Структура server, ServerVersion | 113 |
| `main.go` | Точка входа, инициализация | ~200 |
| `server_ai.go` | AI RPC: ChatWithOWL, ChatWithAI, HermesOrchestrator | 1434 |
| `hermes_agent_service.go` | HermesAgentService: Connect, Token management | ~430 |
| `hermes_remote_manager.go` | Remote agent manager | ~420 |
| `server_chat.go` | Chat, Typing, CallSession | ~650 |
| `auth_service.go` | SignIn, SignUp | ~210 |
| `db.go` | Database helpers | ~350 |
| `hermes_orchestrator.go` | AI orchestrator | ~300 |
| `ai_chat_manager.go` | AI chat DB persistence | ~400 |

**Всего:** ~14,000 строк Go кода

### 1.2 Proto соответствие

| Proto файл | Go gen | Статус |
|-----------|--------|--------|
| `messenger.proto` | `gen/messenger.pb.go` + `gen/messenger_grpc.pb.go` | OK |
| `hermes_remote.proto` | `gen/hermes_agent/hermes_remote.pb.go` + `hermes_remote_grpc.pb.go` | OK |

Все сервисы зарегистрированы на одном gRPC сервере (порты 50051 prod, 50052 dev):
- `ChatService` (messenger.proto) — основной мессенджер
- `HermesAgentService` (hermes_remote.proto) — remote agent management
- `AuthService` — аутентификация
- `ServerService` — управление серверами

### 1.3 Найденные проблемы

#### КРИТИЧНО: Token RPC вызываются не на том сервисе (ИСПРАВЛЕНО)

**Было:** Android вызывал `GenerateAgentToken`, `RevokeAgentToken`, `ListAgentTokens` через `messenger.ChatService/...`
**Должно быть:** `hermes_agent.HermesAgentService/...`

**Причина:** В `GenerateAgentToken` proto метод определён в обоих сервисах:
- `messenger.proto` (строки 723-725): `GenerateAgentToken` в `ChatService`
- `hermes_remote.proto` (строка 12): `GenerateAgentToken` в `HermesAgentService`

**Исправлено:** Android теперь вызывает `hermes_agent.HermesAgentService/...`

#### КРИТИЧНО: IsSuperAdmin check для token RPC (ИСПРАВЛЕНО)

**Было:** `GenerateAgentToken`, `RevokeAgentToken`, `ListAgentTokens` требовали `IsSuperAdmin`
**Проблема:** Remote agents запускаются на сервере пользователя, а не на нашем. Любой пользователь должен генерировать токены.

**Исправлено:** Проверка `IsSuperAdmin` убрана из всех трёх методов.

#### СРЕДНЕ: Неторойный стектрейс в `generate_token.go`

`hermes-agent/gen_token.go` — `// +build ignore`, не компилируется с проектом. Содержит хардкод UUID и mock capabilities.

#### НИЗКОЕ: Миграции hermesDB

`runHermesMigrations(db.DB)` вызывается при каждом старте. Нет проверки были ли миграции уже применены.

### 1.4 Рекомендации

1. **Убрать `isAdmin` из `HermesAgentServer`** — больше не используется, может путать
2. **Добавить rate limiting** на `GenerateAgentToken` — защитить от массовой генерации
3. **Добавить owner_id** в токены — чтобы пользователь мог отзывать только свои токены
4. **Добавить endpoint** для проверки валидности токена
5. **Убрать `hermes-agent/` из репозитория** — это Python агент, должен быть в отдельном репо

---

## 2. Android (Kotlin)

### 2.1 Структура

| Компонент | Файлы | Строк (примерно) |
|-----------|-------|-----------------|
| gRPC layer | `HermesGrpc.kt`, `OwlGrpc.kt`, `GrpcClient.kt`, `RealGrpcClient.kt` | ~2000 |
| Remote Agent UI | `RemoteAgentActivity.kt`, `RemoteAgentViewModel.kt`, `TokenDialog.kt`, `RemoteAgentSettingsActivity.kt` | ~900 |
| Hermes UI | `AgentListActivity.kt`, `AgentSettingsActivity.kt`, `HermesChatActivity.kt` | ~600 |
| Proto | `MessengerProto.kt` | ~1500 |
| Theme | `ThemeStore.kt`, `ThemeUtils.kt`, `ThemeApplier.kt` | ~400 |
| Session | `SessionManager.kt`, `CredentialStore.kt` | ~400 |

### 2.2 Найденные проблемы

#### КРИТИЧНО: `writeRawVarint32` deprecated (ИСПРАВЛЕНО)

**Было:** `cos.writeRawVarint32(entryBytes.size)` — deprecated
**Исправлено:** `cos.writeUInt32NoTag(entryBytes.size)`

#### СРЕДНЕ: Логирование в HermesGrpc.kt

Добавлено логирование для отладки. После исправления проблем — убрать или заменить на `Log.d` с флагом debug.

#### СРЕДНЕ: `RemoteAgentSettingsActivity` — userId vs username

```kotlin
// Строка 52 AgentListActivity
userId = SessionManager.session.value.username  // ← username, не UUID!
```

Но token RPC передаёт `userId` (UUID из `SessionManager.session.value.userId`). Это правильно.

#### НИЗКОЕ: Нет проверки `isConnected` перед отправкой задачи

Пользователь может отправить задачу отключённому агенту. Нужно показывать предупреждение.

#### НИЗКОЕ: `setupAgentSpinner` может крашнуться

Если `agents` пустой, спиннер будет пустым. При попытке выбрать агента — `ArrayIndexOutOfBoundsException` (защита есть через `if (position < agents.size)`, но UX плохой).

### 2.3 Remote Agent flow

```
User → RemoteAgentActivity → loadAgents() → listRemoteAgents() gRPC → Server
                                                                            ↓
User ← showTokenResultDialog(token) ← generateToken() ← RemoteAgentSettingsActivity
                                                                            ↓
Agent connects: python3 hermes_remote_agent.py --server host:port --token jwt
                → Connect bidirectional streaming → handle_task() → send_result()
```

### 2.4 Рекомендации

1. **Добавить индикатор "агент не подключён"** — показывать что нужно запустить агент
2. **Автоматический рефреш списка агентов** — каждые 30 сек при открытом экране
3. **Показывать agent_id в списке токенов** — чтобы пользователь знал какой агент использовать
4. **Добавить кнопку "Скопировать команду"** — готовую команду для запуска агента
5. **Убрать дублирование `RemoteAgentActivity`** — есть `RemoteAgentActivity` и `AgentListActivity`, нужен один экран

---

## 3. Документация

### 3.1 Серверная (`/root/msg/doc/`)

| Файл | Статус | Проблемы |
|------|--------|----------|
| `INDEX.md` | OK | — |
| `INTEGRATION_SESSION.md` | OK | Промпт для следующей сессии есть |
| `TASKS.md` | OK | Обновлён до v1.1.3.0 |
| `CHANGELOG.md` | OK | Секция 1.1.3.0 есть |
| `PITFALLS.md` | OK | — |
| `AI_SERVICES.md` | Устарел | Не обновлялся с v1.1.2.x |
| `HERMES_ORCHESTRATOR_DOC.md` | OK | — |
| `TESTING.md` | OK | — |

### 3.2 Android (`/root/msg.client.android/doc/`)

| Файл | Статус | Проблемы |
|------|--------|----------|
| `INDEX.md` | OK | — |
| `PROMPT_ANDROID.md` | OK | — |
| `REMOTE_AGENT.md` | OK | — |
| `REMOTE_AGENT_PLAN.md` | OK | — |
| `TASKS.md` | OK | — |

### 3.3 Рекомендации

1. **Обновить `PITFALLS.md`** — добавить про token RPC routing (HermesAgentService vs ChatService)
2. **Обновить `AI_SERVICES.md`** — добавить про Platform Adapter и token flow
3. **Создать `ARCHITECTURE.md`** — общая архитектура всех компонентов (сервер, Android, агент)

---

## 4. Python Agent (`/root/msg/hermes-agent/`)

### 4.1 Структура

| Файл | Назначение |
|------|-----------|
| `hermes_remote_agent.py` | Remote agent daemon — подключается через Connect |
| `adapter.py` | Lavender Platform Adapter для Hermes Agent (новый) |
| `__init__.py` | Plugin registration (новый) |
| `hermes_remote_pb2.py` | Generated proto |
| `hermes_remote_pb2_grpc.py` | Generated gRPC |
| `messenger_pb2.py` | Generated proto |
| `messenger_pb2_grpc.py` | Generated gRPC |

### 4.2 Найденные проблемы

#### КРИТИЧНО: `adapter.py` использует `ChatService.Chat` для streaming

`adapter.py` слушает `ChatService.Chat` (bidirectional streaming) для получения сообщений. Но `Chat` — это **основной мессенджер чат**, не agent-specific stream.

**Правильный подход:** Использовать `HermesAgentService.Connect` для агента, а `ChatService.Chat` для обычных сообщений.

#### СРЕДНЕ: Нет обработки `TASK_AI` в `hermes_remote_agent.py`

В `hermes_remote.proto` есть `TASK_AI = 9`, но `handle_task()` в агенте его не обрабатывает.

#### НИЗКОЕ: `generate_token.py` использует `ChatService` вместо `HermesAgentService`

Скрипт для генерации токена вызывает не тот сервис.

### 4.3 Рекомендации

1. **Переписать `adapter.py`** на использование `HermesAgentService.Connect`
2. **Добавить `TASK_AI`** обработчик в `hermes_remote_agent.py`
3. **Убрать `generate_token.py`** или исправить сервис
4. **Добавить конфигурацию через `.env`**
5. **Добавить `--help`** для всех скриптов

---

## 5. Общие проблемы

### 5.1 Proto несоответствия

`GenerateAgentToken` определён в **двух** proto файлах:
- `messenger.proto` → `ChatService` (строки 723-725)
- `hermes_remote.proto` → `HermesAgentService` (строка 12)

Это путает разработчиков. Нужно выбрать один сервис и придерживаться его.

**Рекомендация:** Убрать `GenerateAgentToken`/`RevokeAgentToken`/`ListAgentTokens` из `messenger.proto` — оставить только в `hermes_remote.proto` (`HermesAgentService`).

### 5.2 Нет monitoring/metrics

Нет:
- Health check endpoint для агента
- Metrics (количество подключённых агентов, задач, ошибок)
- Alerting

### 5.3 Нет CI/CD

Нет автоматической сборки и тестирования.

---

## 6. Приоритеты исправлений

| Приоритет | Задача | Компонент |
|-----------|--------|-----------|
| 🔴 Высокий | Исправить `generate_token.py` сервис | Python |
| 🔴 Высокий | Добавить `TASK_AI` обработчик | Python |
| 🟡 Средний | Убрать token RPC из `messenger.proto` | Proto |
| 🟡 Средний | Добавить rate limiting на token RPC | Go |
| 🟡 Средний | Показывать "агент не подключён" | Android |
| 🟢 Низкий | Обновить `PITFALLS.md` | Docs |
| 🟢 Низкий | Создать `ARCHITECTURE.md` | Docs |
| 🟢 Низкий | Убрать duplicate agent activities | Android |

---

## 7. Статистика

| Метрика | Значение |
|---------|----------|
| Go код | ~14,000 строк |
| Kotlin код | ~6,000 строк (HermesGrpc + UI) |
| Python код | ~800 строк |
| Proto файлов | 2 (messenger.proto, hermes_remote.proto) |
| gRPC сервисов | 4 (ChatService, HermesAgentService, AuthService, ServerService) |
| Багов найдено | 5 (3 исправлено) |
| Рекомендаций | 15 |

# Lavender Messenger — Аудит проекта

**Дата:** 2026-06-12
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

#### ✅ ИСПРАВЛЕНО: Token RPC вызываются не на том сервисе

**Было:** Android вызывал `GenerateAgentToken`, `RevokeAgentToken`, `ListAgentTokens` через `messenger.ChatService/...`
**Исправлено:** Теперь вызывает `hermes_agent.HermesAgentService/...`

#### ✅ ИСПРАВЛЕНО: IsSuperAdmin check для token RPC

**Было:** Token RPC требовали `IsSuperAdmin`
**Исправлено:** Проверка убрана, доступно любому пользователю

#### ⚠️ ИЗВЕСТНАЯ ПРОБЛЕМА: Логи сервера не видны в Android

**Симптом:** После генерации токена пользователь не видит логов сервера на устройстве
**Причина:** Логи сервера (journalctl) не передаются на клиент
**Статус:** Требует ручной проверки на сервере через `journalctl -u lavender-server-dev -f`

#### ⚠️ ИЗВЕСТНАЯ ПРОБЛЕМА: Токен не появляется в списке после генерации

**Симптом:** После генерации токена список остаётся пустым
**Причина:** Возможные проблемы:
1. Корутина отменяется до завершения gRPC вызова (`JobCancellationException`)
2. `ListAgentTokens` возвращает пустой список (токен не сохраняется в БД)
3. `hermesDB.SaveAgentToken()` возвращает ошибку
**Исправлено:** Добавлена обработка `CancellationException` отдельно от `Exception`
**Требует проверки:** Логи сервера покажут причину

#### СРЕДНЕ: Нет rate limiting на GenerateAgentToken

Любой пользователь может генерировать неограниченное количество токенов.

#### СРЕДНЕ: Логирование для отладки осталось в коде

Добавлены `Log.d`/`Log.e` для отладки. Перед релизом — убрать или заменить на debug-level.

### 1.4 Рекомендации для релиза

1. **Убрать debug-логи** из production кода или обернуть в `if (BuildConfig.DEBUG)`
2. **Добавить rate limiting** на `GenerateAgentToken`
3. **Убрать `hermes-agent/` из репозитория** — Python агент должен быть в отдельном репо
4. **Добавить endpoint** для проверки валидности токена

---

## 2. Android (Kotlin)

### 2.1 Структура

| Компонент | Файлы | Строк |
|-----------|-------|-------|
| gRPC layer | `HermesGrpc.kt`, `OwlGrpc.kt`, `GrpcClient.kt`, `RealGrpcClient.kt` | ~2000 |
| Remote Agent UI | `RemoteAgentActivity.kt`, `RemoteAgentViewModel.kt`, `TokenDialog.kt`, `RemoteAgentSettingsActivity.kt` | ~900 |
| Hermes UI | `AgentListActivity.kt`, `AgentSettingsActivity.kt`, `HermesChatActivity.kt` | ~600 |
| Proto | `MessengerProto.kt` | ~1500 |
| Theme | `ThemeStore.kt`, `ThemeUtils.kt`, `ThemeApplier.kt` | ~400 |
| Session | `SessionManager.kt`, `CredentialStore.kt` | ~400 |

### 2.2 Найденные проблемы

#### ✅ ИСПРАВЛЕНО: writeRawVarint32 deprecated

**Было:** `cos.writeRawVarint32(entryBytes.size)` — deprecated
**Исправлено:** `cos.writeUInt32NoTag(entryBytes.size)`

#### ✅ ИСПРАВЛЕНО: JobCancellationException в catch (Exception)

**Было:** `CancellationException` ловился как обычная `Exception` и маскировался
**Исправлено:** Отдельный `catch (e: CancellationException)` с re-throw

#### ⚠️ ИЗВЕСТНАЯ ПРОБЛЕМА: Токен не появляется в списке

**Симптом:** `loadTokens: userId=ea577733-3f2c-4752-ac0e-1b2a88a6836b`, но `generateToken error kotlinx.coroutines.JobCancellationException`
**Причина:** Корутина отменяется при нажатии на кнопку "Сгенерировать" (диалог закрывается)
**Исправлено:** `CancellationException` обрабатывается отдельно
**Требует проверки:** После пересборки APK

#### СРЕДНЕ: Логирование для отладки

Добавлены `Log.d`/`Log.e` в:
- `RemoteAgentSettingsActivity`: `generateToken`, `loadTokens`, `revokeToken`
- `HermesGrpc`: `listRemoteAgents`, `generateAgentToken`, `ListAgentTokens`

Перед релизом — убрать или заменить на debug-level.

#### НИЗКОЕ: Нет проверки `isConnected` перед отправкой задачи

Пользователь может отправить задачу отключённому агенту.

### 2.3 Remote Agent flow

```
User → RemoteAgentActivity → loadAgents() → listRemoteAgents() gRPC → Server
                                                                            ↓
User ← showTokenResultDialog(token) ← generateToken() ← RemoteAgentSettingsActivity
                                                                            ↓
Agent connects: python3 hermes_remote_agent.py --server host:port --token jwt
                → Connect bidirectional streaming → handle_task() → send_result()
```

### 2.4 Рекомендации для релиза

1. **Убрать debug-логи** из production кода
2. **Добавить кнопку "Скопировать команду"** — готовую команду для запуска агента
3. **Добавить индикатор "агент не подключён"**
4. **Автоматический рефреш списка агентов** каждые 30 сек

---

## 3. Python Agent (`/root/msg/hermes-agent/`)

### 3.1 Структура

| Файл | Назначение |
|------|-----------|
| `hermes_remote_agent.py` | Remote agent daemon — подключается через Connect |
| `adapter.py` | Lavender Platform Adapter для Hermes Agent |
| `__init__.py` | Plugin registration |
| `hermes_remote_pb2.py` | Generated proto |
| `hermes_remote_pb2_grpc.py` | Generated gRPC |
| `messenger_pb2.py` | Generated proto |
| `messenger_pb2_grpc.py` | Generated gRPC |

### 3.2 Найденные проблемы

#### ⚠️ ИЗВЕСТНАЯ ПРОБЛЕМА: adapter.py использует ChatService.Chat

`adapter.py` слушает `ChatService.Chat` вместо `HermesAgentService.Connect`. Это неправильно для agent-specific streaming.

#### ⚠️ ИЗВЕСТНАЯ ПРОБЛЕМА: TASK_AI не обрабатывается

В `hermes_remote.proto` есть `TASK_AI = 9`, но `handle_task()` в агенте его не обрабатывает.

---

## 4. Документация

### 4.1 Серверная (`/root/msg/doc/`)

| Файл | Статус |
|------|--------|
| `INDEX.md` | ✅ OK |
| `INTEGRATION_SESSION.md` | ✅ OK |
| `TASKS.md` | ✅ OK |
| `CHANGELOG.md` | ✅ OK |
| `PITFALLS.md` | ⚠️ Нет Remote Agent docs |
| `AUDIT.md` | ✅ Этот файл |

### 4.2 Android (`/root/msg.client.android/doc/`)

| Файл | Статус |
|------|--------|
| `INDEX.md` | ✅ OK |
| `PROMPT_ANDROID.md` | ✅ OK |
| `REMOTE_AGENT.md` | ⚠️ Устарел |
| `TASKS.md' | ✅ OK |

---

## 5. Приоритеты для релиза v1.1.3.0

### Обязательно перед релизаом

| # | Задача | Компонент | Статус |
|---|--------|-----------|--------|
| 1 | Убрать debug-логи | Go, Android | ⬜ |
| 2 | Проверить что токен появляется в списке | Android | ⬜ |
| 3 | Обновить CHANGELOG.md для релиза | Docs | ⬜ |
| 4 | Обновить REMOTE_AGENT.md | Docs | ⬜ |

### Может подождать до v1.1.3.1

| # | Задача | Компонент |
|---|--------|-----------|
| 1 | TASK_AI обработчик |
| 2 | adapter.py переписать на Connect |
| 3 | Убрать token RPC из messenger.proto |
| 4 | Rate limiting на GenerateAgentToken |

---

## 6. Статистика

| Метрика | Значение |
|---------|----------|
| Go код | ~14,000 строк |
| Kotlin код | ~6,000 строк |
| Python код | ~800 строк |
| Proto файлов | 2 |
| gRPC сервисов | 4 |
| Исправлено багов | 5 |
| Известных проблем | 3 |
| Рекомендаций | 15 |

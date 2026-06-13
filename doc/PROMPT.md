# Промпт для новой сессии — v1.1.3.7

## Статус: Сервер v1.1.3.7, Android v1.1.3.7 (feat/1.1.3.x).

## Последние изменения (v1.1.3.7)

### Сервер — DeployAgentTaskStream (commit ecab8e4)
- ✅ `messenger.proto`: `DeployAgentTaskStream` RPC (server-side streaming)
- ✅ `server_ai.go`: streaming handler с onStream callback
- ✅ `hermes_remote_manager.go`: `HandleTaskStream` + `RemoteTaskStreamUpdate` + `onStream` callback
- ✅ Dev сервер обновлён и работает

### Android — ErrorHandler + Streaming + AppLog (commit ebd7d7b)
- ✅ `ErrorHandler.kt` — единый обработчик ошибок
- ✅ `MessengerProto.kt`: `DeployAgentTaskStreamResponseProto`
- ✅ `HermesGrpc.kt`: `deployAgentTaskStream()` → callbackFlow
- ✅ `GrpcClient.kt`: `deployAgentTaskStream()` facade
- ✅ `RemoteAgentViewModel.kt`: `sendMessageStreaming()` с real-time Flow collection
- ✅ `RemoteAgentActivity.kt`: streaming mode
- ✅ AppLog.error() во всех catch-блоках с Toast
- ✅ Fix: "Job was cancelled" тост подавлен
- ✅ version.txt → 1.1.3.7

## Промпт для следующей сессии (feat/1.1.3.x — v1.1.3.8)

```
ЗАДАЧА: Продолжить работу над Lava Messenger. v1.1.3.7 выпущена.

Текущая версия: Сервер v1.1.3.7, Android v1.1.3.7
Ветка: feat/1.1.3.x

Что сделано в v1.1.3.7:
- DeployAgentTaskStream — server-side streaming (сервер + клиент)
- ErrorHandler — единый обработчик ошибок с AppLog
- AppLog.error() во всех catch-блоках с Toast ошибками
- Fix: CancellationException больше не показывает тост
- AuthService unit tests (10 tests + benchmarks)

ЗАДАЧИ НА СЛЕДУЮЩУЮ ССЕССИЮ:

## 1. Обновить hermes_remote_agent.py — streaming output

Агент должен отправлять TaskStreamUpdate через gRPC Connect stream:
- Промежуточный stdout/stderr по мере выполнения команды
- Progress updates (шаги, проценты)
- Финальный результат с done=true

Файл: /root/msg.remote.agent/hermes_remote_agent.py

## 2. Модульные тесты для OWL streaming

Файл: owl_test.go

Тесты:
- TestChatWithOWL_Success — отправка сообщения → получение ответа через stream
- TestChatWithOWL_RateLimit — превышение лимита → ошибка rate limit
- TestChatWithOWL_EmptyMessage — пустое сообщение → ошибка валидации
- TestGetOwlHistory_ReturnsMessages — история чата возвращает сообщения из БД

Требования:
- Мокать OpenRouter API через httptest.Server
- Тесты быстрые (< 5 сек)
- t.Parallel() где возможно

## 3. Модульные тесты для DeployAgentTaskStream

Файл: server_ai_test.go

Тесты:
- TestDeployAgentTaskStream_Success — стриминг stdout/stderr → финальный done
- TestDeployAgentTaskStream_AgentNotFound — агент не подключён
- TestDeployAgentTaskStream_Timeout — таймаут задачи

ОБЩИЕ ТРЕБОВАНИЯ:
- go test ./... должен проходить без ошибок
- Тесты не должны зависеть от внешних сервисов
- Использовать t.Parallel() где возможно

Известные проблемы (не исправлено):
- Сообщения пользователя видны только после ответа агента (нужна отладка на устройстве)

Бэклог (после тестов):
- Structured logging на сервере (низкий)
- Graceful shutdown для gRPC сервера (низкий)
- Health check endpoint (низкий)
- Prometheus метрики (низкий)
```

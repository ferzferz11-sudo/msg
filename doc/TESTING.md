# Тестирование сервера Lavender

Документация по модульным тестам: как запускать, что покрыто, как писать новые тесты.

**Актуально:** v1.2.0.8 (2026-06-19)

---

## Быстрый старт

```bash
# Запустить все тесты
cd /root/msg && go test ./... -count=1

# Запустить с подробным выводом
go test -v -count=1 .

# Запустить конкретный тест
go test -v -run TestSignIn_Success -count=1 .

# Запустить бенчмарки
go test -bench=. -benchmem -count=1 .

# Проверить покрытие
go test -coverprofile=/tmp/cover.out -count=1 . && go tool cover -func=/tmp/cover.out
```

---

## Структура тестов

| Файл | Что тестирует | Тестов |
|------|--------------|--------|
| `auth_jwt_test.go` | JWT generation, validation, expiry, tamper | 2 |
| `auth_service_test.go` | AuthService v1 (SignIn, SignUp) + v2 (SignInV2, SignUpV2, TokenPair) | 20 |
| `owl_test.go` | OWL rate limiter, mock OpenRouter API, streaming | 15 |
| `bot_commands_test.go` | Bot commands, rate limiter, notifications | 20 |
| `server_push_test.go` | Hub.IsUserOnline (v1+v2, grace period) | 6 |
| `server_remote_test.go` | Remote agent deployment, stream updates, done filtering | 8 |
| `server_stability_test.go` | Pinned messages fix, type assertion, graceful shutdown, panic recovery | 13 |
| `core/rag/memory/memory_test.go` | In-memory RAG: embeddings, vector DB, pipeline | 4 |

**Всего:** ~88 тестов (все проходят)

---

## auth_jwt_test.go (2 теста)

| Тест | Что проверяет |
|------|--------------|
| `TestGenerateAndValidateTokenPair` | Генерация access+refresh tokens, валидация claims, JTI extraction, tamper/wrong secret |
| `TestValidateTokenExpired` | Свежий токен валиден |

---

## auth_service_test.go (20 тестов)

### V1 Auth (deprecated)

| Тест | Что проверяет |
|------|--------------|
| `TestSignIn_Success` | Успешный вход, токен, user data |
| `TestSignIn_WrongPassword` | Неверный пароль → "invalid password" |
| `TestSignIn_UserNotFound` | Несуществующий пользователь |
| `TestSignIn_EmptyUsername` | Пустой username |
| `TestSignIn_EmptyPassword` | Пустой password |
| `TestSignUp_Success` | Успешная регистрация |
| `TestSignUp_WithEmail` | Регистрация с email |
| `TestSignUp_DuplicateUsername` | Дубль username |
| `TestSignUp_DuplicateEmail` | Дубль email |
| `TestSignUp_EmptyUsername` | Пустой username |
| `TestSignUp_EmptyPassword` | Пустой password |

### V2 Auth (JWT)

| Тест | Что проверяет |
|------|--------------|
| `TestSignInV2_Success` | V2 вход с JWT + device info |
| `TestSignInV2_WrongPassword` | Неверный пароль |
| `TestSignInV2_EmptyCredentials` | Пустые credentials |
| `TestSignInV2_UserNotFound` | Неуществующий пользователь |
| `TestSignUpV2_Success` | V2 регистрация с JWT |
| `TestSignUpV2_DuplicateUsername` | Дубль username |
| `TestTokenPair_Validation` | Полная валидация access+refresh tokens, tamper check |

### Бенчмарки

| Бенчмарк | Описание |
|----------|----------|
| `BenchmarkSignIn` | Параллельный SignIn с bcrypt |
| `BenchmarkSignUp` | Последовательный SignUp |

---

## owl_test.go (15 тестов)

### Rate Limiter (7)

| Тест | Что проверяет |
|------|--------------|
| `TestOwlRateLimiter_AllowExtended` | 10 запросов разрешены, 11-й заблокирован |
| `TestOwlRateLimiter_Cancel` | Отмена (refund) слота |
| `TestOwlRateLimiter_Remaining` | Счётчик оставшихся |
| `TestOwlRateLimiter_WindowReset` | Сброс окна после TTL |
| `TestOwlRateLimiter_Concurrent` | 100 горутин, race-free |

### Mock OpenRouter API (3)

| Тест | Что проверяет |
|------|--------------|
| `TestMockOpenRouterAPI` | HTTP запрос/ответ, заголовки |
| `TestMockOpenRouterAPI_Streaming` | SSE streaming |
| `TestMockOpenRouterAPI_Error` | 429 Too Many Requests |

### Контекст (2)

| Тест | Что проверяет |
|------|--------------|
| `TestCallOpenRouterContext_Success` | Мок сервер отвечает |
| `TestCallOpenRouterContext_Cancelled` | Context cancellation |

### Интеграционные (3)

| Тест | Что проверяет |
|------|--------------|
| `TestOwlFullFlow_Success` | allow → cancel → retry |
| `TestOwlFullFlow_RateLimitExceeded` | Исчерпание лимита |
| `TestStreamOpenRouter_Success` | Rate limiter при стриминге |

---

## bot_commands_test.go (20 тестов)

### Rate Limiter (4)

| Тест | Что проверяет |
|------|--------------|
| `TestBotRateLimiter_Allow` | 3 разрешены, 4-й заблокирован |
| `TestBotRateLimiter_DifferentUsers` | Разные пользователи — отдельные счётчики |
| `TestBotRateLimiter_WindowReset` | Сброс окна |
| `TestOwlRateLimiter_Allow` | OWL rate limiter |

### Bot Commands (7)

| Тест | Что проверяет |
|------|--------------|
| `TestHandleBotStatus` | /status → "Статус сервера" |
| `TestHandleBotVersion` | /version → ServerVersion |
| `TestHandleBotHelp` | /help → "Доступные команды" (7 команд) |
| `TestHandleBotDeploy_NonAdmin` | SKIP (требует DB) |
| `TestHandleBotDeploy_InvalidTarget` | SKIP (требует DB) |
| `TestHandleBotRestart_NonAdmin` | SKIP (требует DB) |
| `TestHandleBotAI_NoMessage` | /ai без аргументов → ошибка |

### Dispatcher (3)

| Тест | Что проверяет |
|------|--------------|
| `TestDispatchBotCommand_UnknownCommand` | Неизвестная команда → ошибка |
| `TestDispatchBotCommand_KnownCommands` | /status, /version, /help, /logs |
| `TestDispatchBotCommand_RateLimit` | Rate limit → ошибка |

### Notification Service (3)

| Тест | Что проверяет |
|------|--------------|
| `TestNotificationService_Broadcast` | Broadcast → subscriber получает |
| `TestNotificationService_History` | maxHistory=5, 10 сообщений → 5 в истории |
| `TestNotificationService_SubscribeUnsubscribe` | subscribe/unsubscribe |

### Utils (3)

| Тест | Что проверяет |
|------|--------------|
| `TestFormatDuration` | 30м, 2ч, 25ч, 90м |
| `TestTruncateString` | Обрезка строк |
| `TestBotCommandList_AllPresent` | Все 7 команд в реестре |

---

## server_push_test.go (6 тестов)

| Тест | Что проверяет |
|------|--------------|
| `TestIsUserOnline_UserPresent` | v2 клиент (userId) онлайн |
| `TestIsUserOnline_UserNotPresent` | Несуществующий пользователь |
| `TestIsUserOnline_FallbackToUsername` | v1 клиент (только username) |
| `TestIsUserOnline_BothUserIdAndUsername` | v2 клиент: userId + username fallback |
| `TestIsUserOnline_MultipleStreams` | 2 стрима, 2 пользователя |
| `TestIsUserOnline_AfterUnregister` | Grace period после отключения |

---

## server_remote_test.go (8 тестов)

| Тест | Что проверяет |
|------|--------------|
| `TestDeployAgentTaskStream_Unit` | Nil manager → ошибка |
| `TestDeployAgentTaskStream_EmptyAgentId` | Пустой agent_id → done=true |
| `TestStreamResponse_Fields` | Поля промежуточных/финальных сообщений |
| `TestStreamUpdate_DoneTrueFiltering` | Фильтрация done=true (8 под-тестов) |
| `TestFinalResult_SingleDoneTrue` | done=True ровно один раз |
| `TestDeployAgentTaskStream_Integration_NilManager` | Graceful degradation |
| `TestDeployAgentTaskStream_Integration_InvalidAgent` | Несуществующий агент |
| `TestDeployAgentTaskStream_Integration_WithRegisteredAgent` | Зарегистрированный агент |

---

## server_stability_test.go (13 тестов)

### Pinned Messages Fix (4)

| Тест | Что проверяет |
|------|--------------|
| `TestPinMessage_ChatIDIsString` | Chat ID — строка, не UUID |
| `TestPinMessage_RoomIDTypeVarchar` | SQL не кастят room_id в uuid |
| `TestGetPinnedMessages_JoinUsesMessageID` | JOIN: `m.message_id` (varchar) ≠ `m.id` (integer) |
| `TestPinMessage_ValidateMessageExists` | Проверка по `message_id`, не по `id` |

### Type Assertion Safe Check (4)

| Тест | Что проверяет |
|------|--------------|
| `TestUpdateConference_PanicOnBadJSON` | Кривой JSON → нет паники |
| `TestUpdateConference_ValidJSON` | Валидный JSON парсится |
| `TestUpdateConference_MissingStartTime` | Отсутствие start_time → fallback |
| `TestUpdateConference_NilPayload` | Пустой payload безопасен |

### Infrastructure (5)

| Тест | Что проверяет |
|------|--------------|
| `TestHTTPServer_GracefulShutdown` | HTTP сервер шатдаунится |
| `TestHTTPServer_HealthEndpoint` | /health → 200 |
| `TestConnectDB_HaltOnFailure` | Сервер останавливается при падении БД |
| `TestStreamHandler_PanicRecovery` | Panic recovery в stream handlers |
| `TestUpdateUsername_TransactionErrorHandling` | Транзакция проверяет ошибки |

---

## core/rag/memory/memory_test.go (4 теста)

| Тест | Что проверяет |
|------|--------------|
| `TestInMemoryEmbeddingService` | TF-IDF embeddings, нормализация, детерминированность |
| `TestInMemoryVectorDB` | Upsert, search (cosine similarity), delete |
| `TestInMemoryRAGPipeline` | BuildContext: RAG → augmented prompt |
| `TestCosineSimilarity` | Косинусное сходство: 1.0 (идентичные), 0.0 (ортогональные) |

---

## Архитектура тестирования

### Mock DB (auth_service_test.go)

`mockAuthDB` — in-memory реализация интерфейса `authDB`:
- Хранит users и emails в map
- Генерирует UUID-подобные ID
- Поддерживает все методы v1 + v2 auth (UpsertDevice, ValidateRefreshToken, LogAuthEvent)

### Mock OpenRouter API (owl_test.go)

`httptest.NewServer` для мокирования OpenRouter API:
- Проверяет HTTP заголовки (Authorization, Content-Type)
- Тестирует SSE streaming (text/event-stream)
- Тестирует ошибки (429 Too Many Requests)

### Mock Streams (server_push_test.go, server_remote_test.go)

- `mockChatStream` — mock для `gen.ChatService_ChatServer`
- `mockDeployStream` — mock для `gen.ChatService_DeployAgentTaskStreamServer`

---

## Написание новых тестов

### Добавить тест для AuthService v2

```go
func TestMyNewV2Test(t *testing.T) {
    os.Setenv("JWT_SECRET", "test-secret-key-that-is-32-bytes-long!")
    defer os.Unsetenv("JWT_SECRET")

    db := newMockAuthDB()
    srv := newAuthServerV2(nil)
    srv.db = db

    resp, err := srv.SignInV2(context.Background(), &gen.SignInRequestV2{
        Username: "testuser",
        Password: "pass123",
        Device: &gen.DeviceInfo{DeviceId: "device-123"},
    })

    if err != nil {
        t.Fatalf("error: %v", err)
    }
    if !resp.Success {
        t.Errorf("expected success: %s", resp.Message)
    }
}
```

### Добавить тест стабильности

```go
func TestMyFix(t *testing.T) {
    t.Parallel()
    // Проверяйте конкретный фикс
    // Например: safe type assertion
    var data map[string]interface{}
    json.Unmarshal([]byte(`{"key": "not-a-number"}`), &data)

    val, ok := data["key"].(float64)
    if ok {
        t.Error("should not be a number")
    }
    // val = 0, no panic
    _ = val
}
```

---

## Полезные команды

```bash
# Запуск тестов с race detector
go test -race -count=1 .

# Запуск тестов с покрытием в HTML
go test -coverprofile=/tmp/cover.out -count=1 .
go tool cover -html=/tmp/cover.out -o /tmp/coverage.html

# Запуск только быстрых тестов (без бенчмарков)
go test -short -count=1 .

# Запуск конкретного пакета
go test -v -count=1 LavenderMessenger

# Проверка что ничего не сломалось после изменений
go build ./...
go vet ./...
```

---

## Известные ограничения

1. **sqlmock не подключён** — моки через интерфейсы и map
2. **Некоторые тесты пропущены** — требуют реального DB (`t.Skip("Requires DB connection")`)
3. **OWL HTTP функции используют хардкод URL** — тесты мокают HTTP логику отдельно
4. **Нет интеграционных тестов стриминга** — требуют реального gRPC подключения

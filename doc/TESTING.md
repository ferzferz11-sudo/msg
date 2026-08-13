# Тестирование сервера Lavender

Документация по модульным тестам: как запускать, что покрыто, как писать новые тесты.

**Актуально:** v1.4.0.2 (2026-08-13)

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
| `crypto_test.go` | AES encrypt/decrypt, HashPassword/CheckPassword, GenerateResetToken, getSecretKey | 22 |
| `ai_v2_test.go` | ProviderRegistry, ToolRegistry, HybridRouter, AgentExecutor, resolveAPIKey, toolCache, isURLSafe SSRF, OpenRouter SSE, query_database security, tool interfaces | 76 |
| `auth_jwt_test.go` | JWT generation, validation, expiry, tamper | 2 |
| `auth_service_test.go` | AuthService v2 (SignInV2, SignUpV2, TokenPair) + v1 compat | 20 |
| `owl_test.go` | OWL rate limiter, mock OpenRouter API, streaming | 15 |
| `bot_commands_test.go` | Bot commands, rate limiter, notifications | 20 |
| `server_push_test.go` | Hub.IsUserOnline (v2 only, grace period) | 6 |
| `server_remote_test.go` | Remote agent deployment, stream updates, done filtering | 8 |
| `server_stability_test.go` | Pinned messages fix, type assertion, graceful shutdown, panic recovery | 13 |
| `chatv2_test.go` | ChatV2 stream: auth, message routing, typing | 12 |
| `messages_v2_test.go` | Messages v2: CRUD, cursor pagination, reactions | 8 |
| `company_test.go` | removeParticipant, access level thresholds, position hierarchy, chat type validation, default positions, builtin position protection, owner constraints, participants JSON | 9 |
| `self_destruct_test.go` | allowedTimerValues validation, ChatV2Row self_destruct_timer proto, rowToProtoV2 forwarded_from/mentions/system, SetSelfDestructTimerResponse proto, timerChangeMessage bilingual labels | 8 |
| `core/rag/memory/memory_test.go` | In-memory RAG: embeddings, vector DB, pipeline | 4 |

**Всего:** ~200+ тестов (все проходят)

---

## auth_jwt_test.go (2 теста)

| Тест | Что проверяет |
|------|--------------|
| `TestGenerateAndValidateTokenPair` | Генерация access+refresh tokens, валидация claims, JTI extraction, tamper/wrong secret |
| `TestValidateTokenExpired` | Свежий токен валиден |

---

## auth_service_test.go (20 тестов)

### Auth (SignIn, SignUp)

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
| `TestIsUserOnline_UserPresent` | Пользователь онлайн |
| `TestIsUserOnline_UserNotPresent` | Несуществующий пользователь |
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

## company_test.go (9 тестов)

### Position Levels & Access (5)

| Тест | Что проверяет |
|------|--------------|
| `TestRemoveParticipant` | Удаление участника из JSON массива (7 под-тестов: list of 3, only user, first, last, not in list, empty, double quotes) |
| `TestCompanyPositions_AccessLevelThresholds` | Пороги доступа: member→0, management→1, owner_only→3, minLevel override (5 под-тестов) |
| `TestCompanyPositions_LevelHierarchy` | Иерархия позиций: Owner > Top Manager > Manager > Employee; проверка management и owner-only видимости |
| `TestCompanyChat_CreationLogic` | Тип "company" является валидным типом чата |
| `TestCompanyChat_AccessLevelValidation` | Валидные уровни: none, member, management, owner_only, all |

### Defaults & Constraints (4)

| Тест | Что проверяет |
|------|--------------|
| `TestCompany_DefaultPositions` | Автосоздание 4 позиций (Owner/Top Manager/Manager/Employee), без дубликатов, уровни 0-3 |
| `TestCompany_CannotDeleteBuiltinPositions` | Owner/Top Manager/Manager/Employee нельзя удалить |
| `TestCompany_OwnerCannotLeave` | Владелец не может покинуть свою компанию |
| `TestCompany_OwnerCannotBeRemoved` | Владелец не может быть удалён |
| `TestCompanyChat_ParticipantsJSON` | Формирование JSON массива участников |

---

## self_destruct_test.go (8 тестов)

| Тест | Что проверяет |
|------|--------------|
| `TestAllowedTimerValues` | Валидные (0,30,60,300,3600,86400) и невалидные значения таймера |
| `TestChatV2RowToProto_SelfDestructTimer` | ChatV2Row → ChatInfo proto с self_destruct_timer=3600 |
| `TestChatV2RowToProto_SelfDestructTimerZero` | ChatV2Row → ChatInfo proto с self_destruct_timer=0 (default) |
| `TestRowToProtoV2_ForwardedFrom` | rowToProtoV2 correctly maps forwarded_from field |
| `TestRowToProtoV2_Mentions` | rowToProtoV2 correctly parses mentions JSON array |
| `TestSetSelfDestructTimerResponse_Proto` | Proto response construction (success + error cases) |
| `TestTimerChangeMessage` | Human-readable labels for all timer values (7 sub-tests) |
| `TestRowToProtoV2_SystemMessage` | System message with sender_id=00000000-... |

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

## crypto_test.go (22 теста)

### AES Encrypt/Decrypt (9)

| Тест | Что проверяет |
|------|--------------|
| `TestEncryptDecrypt_RoundTrip` | Полный цикл encrypt→decrypt |
| `TestEncryptDecrypt_EmptyString` | Пустая строка |
| `TestEncryptDecrypt_Unicode` | Unicode (кириллица, эмодзи,日本語) |
| `TestEncrypt_WrongKeyLength` | Ключ < 32 байт → ошибка |
| `TestDecrypt_WrongKeyLength` | Ключ < 32 байт → ошибка |
| `TestDecrypt_TooShortCiphertext` | Шифротекст < nonce size → ошибка |
| `TestDecrypt_TamperedCiphertext` | Подмена байта → ошибка |
| `TestDecrypt_ServiceMarkers` | SERVICE_VOICE_MSG, SERVICE_MEDIA_MSG, FIXED_BY_MAINTENANCE и др. |
| `TestEncrypt_DifferentCiphertextEachTime` | Random nonce → разный шифротекст |

### HashPassword/CheckPassword (5)

| Тест | Что проверяет |
|------|--------------|
| `TestHashPassword_CheckPassword_Success` | Хеширование + проверка пароля |
| `TestHashPassword_CheckPassword_WrongPassword` | Неверный пароль → false |
| `TestHashPassword_CheckPassword_EmptyPassword` | Пустой пароль |
| `TestHashPassword_DifferentHashesSamePassword` | bcrypt random salt |
| `TestHashPassword_HashFormat` | Префикс $2a$ или $2b$ |

### GenerateResetToken (3)

| Тест | Что проверяет |
|------|--------------|
| `TestGenerateResetToken_Length` | 64 hex chars (32 bytes) |
| `TestGenerateResetToken_HexFormat` | Только 0-9, a-f |
| `TestGenerateResetToken_Unique` | Уникальность токенов |

### getSecretKey (5)

| Тест | Что проверяет |
|------|--------------|
| `TestGetSecretKey_Valid` | 32-byte key → ok |
| `TestGetSecretKey_TooShort` | < 32 → ошибка |
| `TestGetSecretKey_Empty` | Пустой env → ошибка |
| `TestGetSecretKey_TooLong` | > 32 → ошибка |

---

## ai_v2_test.go (76 тестов)

### Mock Infrastructure

- `mockProvider` — in-memory AgentProvider с настраиваемым каналом StreamChunk
- `mockTool` — in-memory Tool с настраиваемой executeFunc

### ProviderRegistry (4)

| Тест | Что проверяет |
|------|--------------|
| `TestProviderRegistry_AllBuiltInRegistered` | 8 built-in провайдеров зарегистрированы |
| `TestProviderRegistry_CreateUnknown` | Неизвестный тип → ошибка |
| `TestProviderRegistry_RegisterCustom` | Кастомный провайдер |
| `TestProviderRegistry_ConcurrentRegister` | 10 goroutines регистрация |

### ToolRegistry (8)

| Тест | Что проверяет |
|------|--------------|
| `TestToolRegistry_RegisterAndGet` | Регистрация + получение |
| `TestToolRegistry_GetNotFound` | Несуществующий инструмент |
| `TestToolRegistry_GetAll` | Все инструменты |
| `TestToolRegistry_Execute` | Выполнение инструмента |
| `TestToolRegistry_Execute_NotFound` | toolNotFoundError |
| `TestToolRegistry_GetDefs_Whitelist` | Фильтрация по whitelist |
| `TestToolRegistry_GetDefs_NoWhitelist` | Все инструменты без whitelist |
| `TestToolRegistry_ListInfo` | Метаданные инструментов |

### HybridRouter (8)

| Тест | Что проверяет |
|------|--------------|
| `TestHybridRouter_BoundAgent` | Привязанный агент |
| `TestHybridRouter_ExplicitAgent` | Явно указанный агент |
| `TestHybridRouter_BoundOverExplicit` | Bound > Explicit |
| `TestHybridRouter_KeywordCode` | "bug", "debug" → developer |
| `TestHybridRouter_KeywordDeploy` | "deploy", "server" → devops |
| `TestHybridRouter_KeywordTranslate` | "переведи" → translator |
| `TestHybridRouter_KeywordWrite` | "story", "creative" → writer |
| `TestHybridRouter_DefaultAssistant` | Без совпадений → assistant |

### AgentExecutor (5)

| Тест | Что проверяет |
|------|--------------|
| `TestAgentExecutor_SimpleResponse` | Mock провайдер, простой ответ |
| `TestAgentExecutor_WithToolCalls` | Tool calling loop |
| `TestAgentExecutor_ModelOverride` | Настройки пользователя перезаписывают модель |
| `TestAgentExecutor_CloseProvider` | Провайдер закрывается после Execute |
| `TestAgentExecutor_UnknownProvider` | Неизвестный тип → ошибка |

### resolveAPIKey (5)

| Тест | Что проверяет |
|------|--------------|
| `TestResolveAPIKey_FromSettings` | Ключ из настроек пользователя |
| `TestResolveAPIKey_FromAgentConfig` | Ключ из provider_config |
| `TestResolveAPIKey_FromEnv` | Ключ из env var |
| `TestResolveAPIKey_PrioritySettingsOverAgent` | Settings > AgentConfig |
| `TestResolveAPIKey_NoKey` | Нет ключа → пустая строка |

### ToolCache (6)

| Тест | Что проверяет |
|------|--------------|
| `TestToolCache_SetGet` | Базовый set/get |
| `TestToolCache_Expiry` | TTL expiration |
| `TestToolCache_MaxSize` | LRU eviction |
| `TestToolCache_ConcurrentAccess` | 100 goroutines |
| `TestCachedTool_CachesResult` | Кеширование (1 call, 2 get) |
| `TestCachedTool_DifferentKeys` | Разные ключи → разные вызовы |

### isURLSafe / SSRF (10)

| Тест | Что проверяет |
|------|--------------|
| `TestIsURLSafe_ValidHTTPS` | https://example.com → ok |
| `TestIsURLSafe_ValidHTTP` | http://example.com → ok |
| `TestIsURLSafe_FTPBlocked` | ftp:// → blocked |
| `TestIsURLSafe_LocalhostBlocked` | localhost → blocked |
| `TestIsURLSafe_IP127Blocked` | 127.0.0.1 → blocked |
| `TestIsURLSafe_IPv6LoopbackBlocked` | [::1] → blocked |
| `TestIsURLSafe_MetadataEndpointBlocked` | 169.254.169.254 → blocked |
| `TestIsURLSafe_GoogleMetadataBlocked` | metadata.google.internal → blocked |
| `TestIsURLSafe_EmptyURL` | Пустой URL → error |
| `TestIsURLSafe_NoScheme` | Без scheme → error |

### OpenRouter Provider (7)

| Тест | Что проверяет |
|------|--------------|
| `TestOpenRouterProvider_SSEStreamParsing` | SSE stream → content chunks |
| `TestOpenRouterProvider_NoAPIKey` | Нет ключа → ошибка |
| `TestOpenRouterProvider_Capabilities` | Images, Tools, Streaming |
| `TestOpenRouterProvider_HealthCheck` | Ключ есть → ok |
| `TestOpenRouterProvider_HealthCheck_NoKey` | Нет ключа → error |
| `TestMockOpenRouterAPI_SSE` | Mock SSE endpoint |
| `TestMockOpenRouterAPI_ToolCalls` | Tool calls в SSE stream |
| `TestMockOpenRouterAPI_HTTPError` | 429 → ошибка |

### query_database Security (4)

| Тест | Что проверяет |
|------|--------------|
| `TestQueryDatabaseTool_Security_OnlySelect` | DROP/DELETE/INSERT → blocked |
| `TestQueryDatabaseTool_Security_NonSelectRejected` | Все не-SELECT → blocked |
| `TestQueryDatabaseTool_Security_BlockedKeywords` | WITH, RECURSIVE, pg_* → blocked |
| `TestQueryDatabaseTool_Security_BlockedTables` | users, hermes_sessions → blocked |
| `TestQueryDatabaseTool_EmptyQuery` | Пустой query → error |

### Tool Interfaces (6)

| Тест | Что проверяет |
|------|--------------|
| `TestWebFetchTool_EmptyURL` | Пустой URL → error |
| `TestWebFetchTool_BlockedLocalhost` | localhost → blocked |
| `TestWebSearchTool_EmptyQuery` | Пустой query → error |
| `TestWebSearchTool_MockServer` | Tool interface check |
| `TestSearchMessagesTool_EmptyQuery` | Пустой query → error |
| `TestSearchMessagesTool_ParameterSchema` | JSON Schema parameters |
| `TestSearchUsersTool_EmptyQuery` | Пустой query → error |
| `TestGetChatInfoTool_EmptyChatID` | Пустой chat_id → error |

### AIGateway Helpers (3)

| Тест | Что проверяет |
|------|--------------|
| `TestAIGateway_GenerateChatName` | simple/agent/pipeline/unknown |
| `TestAIGateway_GetUserLock` | Per-user mutex (same=match, diff=miss) |
| `TestAIGateway_RecordUsage_ZeroTokens` | 0 tokens → no-op |

### Utility Functions (3)

| Тест | Что проверяет |
|------|--------------|
| `TestConvertMessages` | AIMessageInput → map |
| `TestConvertToolDefs` | ToolDefInput → map |
| `TestJoinStrings` | joinStrings helper |

---

## Известные ограничения

1. **sqlmock не подключён** — моки через интерфейсы и map
2. **Некоторые тесты пропущены** — требуют реального DB (`t.Skip("Requires DB connection")`)
3. **OWL HTTP функции используют хардкод URL** — тесты мокают HTTP логику отдельно
4. **Нет интеграционных тестов стриминга** — требуют реального gRPC подключения

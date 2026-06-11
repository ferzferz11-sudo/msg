# Тестирование сервера Lavender

Документация по модульным тестам: как запускать, что покрыто, как писать новые тесты.

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

| Файл | Что тестирует | Тестов | Покрытие |
|------|--------------|--------|----------|
| `auth_service_test.go` | AuthService (SignIn, SignUp) | 11 | SignIn 85.7%, SignUp 74.2% |
| `owl_test.go` | OWL rate limiter, mock OpenRouter API | 15+ | rateLimiter 92-100% |
| `bot_commands_test.go` | Bot commands, rate limiter, notifications | 20+ | ~4% общий |

---

## auth_service_test.go

### Тесты SignIn (5)

| Тест | Что проверяет |
|------|--------------|
| `TestSignIn_Success` | Успешный вход, токен, user data, email |
| `TestSignIn_WrongPassword` | Неверный пароль → "invalid password" |
| `TestSignIn_UserNotFound` | Несуществующий пользователь → "user not found" |
| `TestSignIn_EmptyUsername` | Пустой username → "required" |
| `TestSignIn_EmptyPassword` | Пустой password → "required" |

### Тесты SignUp (6)

| Тест | Что проверяет |
|------|--------------|
| `TestSignUp_Success` | Успешная регистрация, токен, user data |
| `TestSignUp_WithEmail` | Регистрация с email |
| `TestSignUp_DuplicateUsername` | Дубль username → "already taken" |
| `TestSignUp_DuplicateEmail` | Дубль email → "email already in use" |
| `TestSignUp_EmptyUsername` | Пустой username → "required" |
| `TestSignUp_EmptyPassword` | Пустой password → "required" |
| `TestSignUp_EmptyEmail` | Пустой email — допустимо (опциональное поле) |

### Бенчмарки

| Бенчмарк | Описание |
|----------|----------|
| `BenchmarkSignIn` | Параллельный SignIn с bcrypt |
| `BenchmarkSignUp` | Последовательный SignUp с уникальными username |
| `BenchmarkHashPassword` | Скорость bcrypt hashing |

### Mock DB

`mockAuthDB` — in-memory реализация интерфейса `authDB`:
- Thread-safe (sync.Mutex)
- Хранит users и emails в map
- Генерирует UUID-подобные ID
- Поддерживает все методы интерфейса authDB

---

## owl_test.go

### Тесты rate limiter (7)

| Тест | Что проверяет |
|------|--------------|
| `TestOwlRateLimiter_AllowExtended` | 10 запросов разрешены, 11-й заблокирован |
| `TestOwlRateLimiter_Cancel` | Отмена (refund) слота после ошибки |
| `TestOwlRateLimiter_Remaining` | Правильный счётчик оставшихся запросов |
| `TestOwlRateLimiter_WindowReset` | Сброс окна после истечения TTL |
| `TestOwlRateLimiter_Concurrent` | 100 горутин, race-free |

### Тесты mock OpenRouter API (3)

| Тест | Что проверяет |
|------|--------------|
| `TestMockOpenRouterAPI` | HTTP запрос/ответ, заголовки, тело |
| `TestMockOpenRouterAPI_Streaming` | SSE streaming (text/event-stream) |
| `TestMockOpenRouterAPI_Error` | 429 Too Many Requests |

### Интеграционные тесты OWL (2)

| Тест | Что проверяет |
|------|--------------|
| `TestOwlFullFlow_Success` | Полный флоу: allow → cancel → retry |
| `TestOwlFullFlow_RateLimitExceeded` | Исчерпание лимита → блокировка |

### Тесты контекста (2)

| Тест | Что проверяет |
|------|--------------|
| `TestCallOpenRouterContext_Success` | Мок сервер отвечает корректно |
| `TestCallOpenRouterContext_Cancelled` | Context cancellation |

### Бенчмарки

| Бенчмарк | Описание |
|----------|----------|
| `BenchmarkOwlRateLimiter` | Параллельный allow() |
| `BenchmarkOwlRateLimiter_Remaining` | remaining() после allow() |

---

## Архитектура тестирования

### Интерфейс authDB

Для тестируемости AuthService введён интерфейс `authDB`:

```go
type authDB interface {
    UserExists(user string) (bool, error)
    EmailExists(email string) (bool, error)
    GetUserPasswordHash(user string) (string, error)
    SaveUserWithEmail(user, hash, email string) error
    GetUserIdByUsername(user string) (string, error)
    GetUserAvatar(user string) (string, error)
    UpdateLastSeen(user string) error
    queryUserProfile(username string) (email, bio, status string, createdAt, lastSeenAt time.Time, err error)
}
```

`*DB` реализует этот интерфейс автоматически. В тестах используется `mockAuthDB`.

### Мок OpenRouter API

Тесты OWL используют `httptest.Server` для мокирования OpenRouter API:
- Проверяют HTTP заголовки (Authorization, Content-Type)
- Проверяют тело запроса (model, messages)
- Тестируют SSE streaming (text/event-stream)
- Тестируют ошибки (429 Too Many Requests)

---

## Написание новых тестов

### Добавить тест для AuthService

1. Создайте тестовую функцию в `auth_service_test.go`
2. Используйте `newMockAuthDB()` для создания мока
3. При необходимости добавьте пользователей через `db.addUser(username, password, email)`
4. Вызовите метод `authServer` и проверьте результат

Пример:
```go
func TestSignIn_NewCase(t *testing.T) {
    t.Parallel()
    db := newMockAuthDB()
    db.addUser("testuser", "password123", "")

    s := &authServer{db: db}
    resp, err := s.SignIn(context.Background(), &gen.SignInRequest{
        Username: "testuser",
        Password: "password123",
    })

    if err != nil {
        t.Fatalf("SignIn returned error: %v", err)
    }
    if !resp.Success {
        t.Fatal("SignIn should succeed")
    }
}
```

### Добавить тест для OWL

1. Создайте тестовую функцию в `owl_test.go`
2. Для тестирования rate limiter используйте `newRateLimiter(limit, window)`
3. Для тестирования HTTP используйте `httptest.NewServer()`
4. Для тестирования потоков используйте SSE формат

### Добавить бенчмарк

```go
func BenchmarkNewFunction(b *testing.B) {
    // Подготовка
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        // Тестируемый код
    }
}
```

Для параллельных бенчмарков:
```go
b.RunParallel(func(pb *testing.PB) {
    for pb.Next() {
        // Тестируемый код
    }
})
```

---

## Покрытие кода

Текущее покрытие (v1.1.2.11):

| Функция | Покрытие |
|---------|----------|
| `SignIn` | 85.7% |
| `SignUp` | 74.2% |
| `rateLimiter.allow` | 100% |
| `rateLimiter.cancel` | 100% |
| `rateLimiter.remaining` | 92.3% |
| Общее | 4.3% |

Общее покрытие низкое потому что проект большой (30+ файлов), а тесты покрывают только auth и owl части. Для самих тестируемых функций покрытие >80%.

---

## Известные ограничения

1. **sqlmock не подключён** — нет зависимости `DATA-DOG/go-sqlmock` в go.mod. Моки реализованы вручную через интерфейсы и map.
2. **QueryRow нельзя мокать напрямую** — `*sql.Row` имеет приватные поля. Решение: вызов обёрнут в метод `queryUserProfile`.
3. **OWL HTTP функции используют хардкод URL** — `callOpenRouterContext` и `streamOpenRouter` вызывают `openrouter.ai` напрямую. В тестах мок-сервер проверяет HTTP логику, но не интегрируется с этими функциями напрямую.
4. **Некоторые тесты bot_commands_test.go пропущены** — требуют реального DB соединения (`t.Skip("Requires DB connection")`).

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

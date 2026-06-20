# Marketplace Agents — Руководство по тестированию на Android

**Сервер:** v1.3.0.7 | **Дата:** 2026-06-20

---

## Быстрый старт

### 1. Загрузить список агентов

```kotlin
val response = chatService.listAIAgents(
    ListAIAgentsRequest.newBuilder()
        .setIncludePublic(true) // свои + пресеты + публичные
        .build()
)

for (agent in response.agentsList) {
    Log.d("Agent", "${agent.id} | ${agent.name} | ${agent.providerType} | tools=${agent.toolsEnabled}")
}
```

### 2. Начать диалог с пресетом

```kotlin
val stream = chatService.chatWithAIV2(
    ChatWithAIV2Request.newBuilder()
        .setMessage("Привет!")
        .setAgentId("assistant") // или "developer", "mimo", и т.д.
        .build()
)

for (resp in stream) {
    if (resp.error.isNotEmpty()) {
        // Ошибка (rate limit и т.д.)
        showError(resp.error)
    }
    if (resp.token.isNotEmpty()) {
        appendToUI(resp.token)
    }
    if (resp.finished) {
        // Готово. resp.tokenCount — сколько токенов потрачено
    }
}
```

---

## Пресеты (встроенные агенты)

| ID | Имя | Провайдер | Модель | Tools | RAG | Для чего |
|----|-----|-----------|--------|-------|-----|----------|
| `mimo` | MiMo | mimo | mimo-auto | ✅ | ✅ | Универсальный (бесплатный) |
| `assistant` | Assistant | openrouter | claude-sonnet-4 | ✅ | ✅ | Универсальный (платный) |
| `developer` | Developer | openrouter | claude-sonnet-4 | ✅ | ❌ | Код на любом языке |
| `devops` | DevOps | openrouter | claude-sonnet-4 | ✅ | ❌ | Инфра, Docker, CI/CD |
| `architect` | Architect | openrouter | claude-sonnet-4 | ❌ | ❌ | Архитектура, дизайн-паттерны |
| `writer` | Writer | openrouter | gpt-4o | ❌ | ❌ | Тексты, статьи |
| `analyst` | Analyst | openrouter | claude-sonnet-4 | ✅ | ✅ | Анализ данных, документы |
| `translator` | Translator | openrouter | gpt-4o-mini | ❌ | ❌ | Переводы |

**Совет для тестирования:** начни с `mimo` — он бесплатный и с инструментами.

---

## Создание своего агента

### Минимальный (простой LLM)

```kotlin
val response = chatService.createAIAgent(
    CreateAIAgentRequest.newBuilder()
        .setName("Мой бот")
        .setProviderType("openrouter")
        .setProviderConfig("""{"api_key_source": "user"}""")
        .setSystemPrompt("Ты полезный помощник. Отвечай на русском.")
        .setModel("anthropic/claude-sonnet-4")
        .build()
)

val agentId = response.agentId
// Используем: ChatWithAIV2Request.setAgentId(agentId)
```

### С инструментами (web search + поиск сообщений)

```kotlin
val response = chatService.createAIAgent(
    CreateAIAgentRequest.newBuilder()
        .setName("Research Bot")
        .setDescription("Ищет информацию в интернете и в чатах")
        .setProviderType("openrouter")
        .setProviderConfig("""{"api_key_source": "user"}""")
        .setSystemPrompt("Ты research-ассистент. Используй инструменты для поиска информации.")
        .setModel("anthropic/claude-sonnet-4")
        .setToolsEnabled(true)
        .setToolWhitelist("web_search") // только web_search
        .setRateLimit(10) // 10 запросов в минуту
        .build()
)
```

### Свои инструменты (tool whitelist)

Доступные инструменты:
| ID | Описание |
|----|----------|
| `search_messages` | Поиск по сообщениям в чатах |
| `search_users` | Поиск пользователей |
| `web_search` | Веб-поиск (DuckDuckGo) |
| `web_fetch` | Загрузка содержимого URL |
| `get_chat_info` | Метаданные чата |

```kotlin
.setToolWhitelist("web_search", "search_messages") // только эти два
```

Пустой `tool_whitelist` = все доступные инструменты.

---

## Marketplace (каталог агентов)

### Поиск публичных агентов

```kotlin
val response = chatService.listMarketplaceAgents(
    ListMarketplaceAgentsRequest.newBuilder()
        .setQuery("code") // поиск по имени/описанию
        .setLimit(20)
        .setOffset(0)
        .build()
)

for (agent in response.agentsList) {
    // agent.id, agent.name, agent.avgRating, agent.installCount
}
```

### Оценить агента

```kotlin
chatService.rateAIAgent(
    RateAIAgentRequest.newBuilder()
        .setAgentId("agent-abc123")
        .setRating(5) // 1-5
        .setReview("Отличный агент!")
        .build()
)
```

### Поделиться агентом (получить share code)

```kotlin
val response = chatService.shareAIAgent(
    ShareAIAgentRequest.newBuilder()
        .setAgentId("agent-abc123")
        .build()
)

val shareCode = response.shareCode
// Показать пользователю: "Поделитесь кодом: abc123"
```

### Установить агента по коду

```kotlin
val response = chatService.installAIAgent(
    InstallAIAgentRequest.newBuilder()
        .setShareCode("abc123")
        .setNewName("Копия агента") // опционально
        .build()
)

val newAgentId = response.agentId
```

### Статистика агента

```kotlin
val stats = chatService.getAIAgentStats(
    GetAIAgentStatsRequest.newBuilder()
        .setAgentId("agent-abc123")
        .build()
)

Log.d("Stats", "Installs: ${stats.installCount}, Rating: ${stats.avgRating}")
```

### Отзывы на агента

```kotlin
val reviews = chatService.getAIAgentReviews(
    GetAIAgentReviewsRequest.newBuilder()
        .setAgentId("agent-abc123")
        .setLimit(20)
        .build()
)

for (review in reviews.reviewsList) {
    // review.userId, review.rating, review.review, review.createdAt
}
```

### Моя статистика использования

```kotlin
val stats = chatService.getAIUsageStats(
    GetAIUsageStatsRequest.newBuilder().build()
)

Log.d("Usage", "Tokens: ${stats.totalTokens}, Requests: ${stats.totalRequests}")
for (stat in stats.statsList) {
    // stat.agentId, stat.agentName, stat.totalTokens, stat.requestCount
}
```

---

## Tool Calling Flow (клиентский цикл)

Когда агент вызывает инструмент, клиент должен:

```
1. Получить ChatWithAIV2Response с tool_calls
2. Выполнить инструмент
3. Отправить результат через ChatWithAIV2Request с tool_calls
4. Получить следующий chunk стриминга
```

### Пример implementation

```kotlin
suspend fun chatWithAgent(
    sessionId: String?,
    message: String,
    agentId: String
): Flow<String> = flow {
    var currentSessionId = sessionId

    val request = ChatWithAIV2Request.newBuilder().apply {
        if (currentSessionId != null) setSessionId(currentSessionId)
        setMessage(message)
        setAgentId(agentId)
    }.build()

    val stream = chatService.chatWithAIV2(request)

    for (resp in stream) {
        if (resp.error.isNotEmpty()) {
            throw AIException(resp.error)
        }

        // Tool calls — нужно выполнить и отправить результат
        if (resp.toolCallsList.isNotEmpty()) {
            for (toolCall in resp.toolCallsList) {
                val result = executeToolLocally(toolCall.name, toolCall.arguments)

                val followUp = ChatWithAIV2Request.newBuilder().apply {
                    setSessionId(currentSessionId)
                    addToolCalls(ToolCallV2.newBuilder()
                        .setId(toolCall.id)
                        .setName(toolCall.name)
                        .setArguments(toolCall.arguments)
                        .setResult(result)
                        .build())
                }.build()

                // Отправить результат и получить продолжение
                val toolStream = chatService.chatWithAIV2(followUp)
                for (toolResp in toolStream) {
                    if (toolResp.token.isNotEmpty()) emit(toolResp.token)
                    if (toolResp.finished) { /* done */ }
                }
            }
        }

        if (resp.token.isNotEmpty()) emit(resp.token)
        if (resp.finished) return@flow
    }
}

// Локальное выполнение встроенных инструментов
suspend fun executeToolLocally(name: String, args: String): String {
    val params = JSONObject(args)
    return when (name) {
        "web_search" -> webSearch(params.getString("query"))
        "search_messages" -> searchMessages(params.getString("query"))
        "search_users" -> searchUsers(params.getString("query"))
        "web_fetch" -> webFetch(params.getString("url"))
        "get_chat_info" -> getChatInfo(params.getString("chat_id"))
        else -> "Unknown tool: $name"
    }
}
```

---

## Rate Limiting

| Лимит | Значение | Окно |
|-------|----------|------|
| По умолчанию | 10 запросов | 1 минута |
| Кастомный (rate_limit в агенте) | N запросов | 1 минута |

**При превышении:** сервер возвращает `ChatWithAIV2Response{error: "rate limit exceeded"}`.

**Кэширование на клиенте:**
```kotlin
// Проверить перед отправкой
if (rateLimitCache.getRemaining(agentId) <= 0) {
    val waitMs = rateLimitCache.getTimeUntilReset(agentId)
    showRateLimitDialog(waitMs)
    return
}
rateLimitCache.recordRequest(agentId)
```

---

## Отладка

### Где смотреть логи
- Dev: `http://13.140.25.249/server-logs-dev`
- Prod: `http://13.140.25.249/server-logs`

### Типичные ошибки

| Ошибка | Причина | Решение |
|--------|---------|---------|
| `unauthorized` | Нет JWT в metadata | Добавить `authorization: Bearer <token>` |
| `rate limit exceeded` | Много запросов | Подождать 1 минуту |
| `agent not found` | Неверный agent_id | Проверить `ListAIAgents` |
| `OpenRouter API key not configured` | user-ключ не передан | Передать `api_key_source: "user"` в provider_config |
| `permission denied` | Чужой приватный агент | Использовать `CloneAIAgent` или `InstallAIAgent` |

### curl для быстрой проверки

```bash
# Список агентов
curl -H "authorization: Bearer $TOKEN" \
  localhost:50052/messenger.ChatService/ListAIAgents

# Marketplace
curl -H "authorization: Bearer $TOKEN" \
  localhost:50052/messenger.ChatService/ListMarketplaceAgents

# Chat (стриминг)
grpcurl -plaintext -H "authorization: Bearer $TOKEN" \
  -d '{"message":"Привет","agent_id":"mimo"}' \
  localhost:50052 messenger.ChatService/ChatWithAIV2
```

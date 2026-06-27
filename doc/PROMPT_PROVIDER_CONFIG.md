# Промпт: AgentInfoV2 — добавить provider_config (field 22)

**Тип:** Доработка сервера
**Клиент:** Android (LavenderMessenger-Android)
**Дата:** 2026-06-27
**Причина:** Клиент не может показать API ключ при редактировании агента — `AgentInfoV2` не возвращает `provider_config`

---

## Контекст

Android клиент обновил форму создания/редактирования AI агентов (`AiAgentSetupActivity`):
- Поле "API Key" (textPassword) вместо JSON "Provider Config"
- Слайдер "Temperature" (0–2)
- Поле "Max Tokens"

Проблема: при редактировании существующего агента клиент не может предзаполнить поле API Key, потому что серверный `AgentInfoV2` proto не содержит `provider_config`. Сервер хранит `provider_config` в БД (`agents_v2.provider_config JSONB`), но не возвращает его в ответах.

---

## Что нужно сделать

### 1. Добавить поле в proto

В `messenger.proto`, сообщение `AgentInfoV2` (строка 1856), добавить **field 22**:

```protobuf
message AgentInfoV2 {
  // ... существующие поля 1-21 ...
  string share_code = 21;
  string provider_config = 22;  // JSON string — API key, model override и т.д.
}
```

### 2. Перегенерировать proto

```bash
PATH=$PATH:~/go/bin protoc --go_out=gen --go_opt=paths=source_relative \
  --go-grpc_out=gen --go-grpc_opt=paths=source_relative messenger.proto
```

### 3. Обновить `agentToProto()` в `server_ai_v2.go`

Текущий код (строка 575):
```go
func agentToProto(a *AgentV2) *gen.AgentInfoV2 {
    caps := &gen.AgentCapabilitiesV2{...}
    return &gen.AgentInfoV2{
        // ... все поля кроме provider_config ...
    }
}
```

Изменить на:
```go
func agentToProto(a *AgentV2) *gen.AgentInfoV2 {
    caps := &gen.AgentCapabilitiesV2{
        SupportsImages:    a.ProviderType == "openrouter" || a.ProviderType == "mimo",
        SupportsTools:     a.ToolsEnabled,
        SupportsStreaming: true,
        MaxTokens:         int32(a.MaxTokens),
    }

    // Marshal provider_config to JSON string
    pcJSON := ""
    if a.ProviderConfig != nil {
        if b, err := json.Marshal(a.ProviderConfig); err == nil {
            pcJSON = string(b)
        }
    }

    return &gen.AgentInfoV2{
        Id:              a.ID,
        Name:            a.Name,
        Description:     a.Description,
        ProviderType:    a.ProviderType,
        Model:           a.Model,
        SystemPrompt:    a.SystemPrompt,
        ToolsEnabled:    a.ToolsEnabled,
        RagEnabled:      a.RAGEnabled,
        IsPreset:        a.IsPreset,
        IsPublic:        a.IsPublic,
        MaxTokens:       int32(a.MaxTokens),
        Temperature:     float32(a.Temperature),
        CreatedBy:       a.CreatedBy,
        Capabilities:    caps,
        InstallCount:    int32(a.InstallCount),
        AvgRating:       float32(a.AvgRating),
        ReviewCount:     int32(a.ReviewCount),
        Tags:            a.Tags,
        OriginalAgentId: a.OriginalAgentID,
        Version:         int32(a.Version),
        ShareCode:       a.ShareCode,
        ProviderConfig:  pcJSON,  // ← НОВОЕ ПОЛЕ
    }
}
```

### 4. Проверить все RPC, которые возвращают `AgentInfoV2`

Убедиться, что `agentToProto()` используется везде (Create, Get, List, Clone, Marketplace). Если где-то `AgentInfoV2` собирается вручную — добавить `ProviderConfig` и туда.

**RPC для проверки:**
- `CreateAIAgent` → возвращает `GetAIAgentResponse` с `agent`
- `GetAIAgent` → возвращает `GetAIAgentResponse` с `agent`
- `ListAIAgents` → возвращает `ListAIAgentsResponse` с `agents[]`
- `CloneAIAgent` → возвращает `GetAIAgentResponse` с `agent`
- `ListMarketplaceAgents` → возвращает `ListMarketplaceAgentsResponse` с `agents[]`

---

## Безопасность

- **User agents**: `provider_config` содержит `{"apiKey": "user-key"}` — пользователь владеет ключом, возвращаем полный конфиг
- **Preset agents**: `provider_config` = `{"api_key_source": "server", "default_model": "..."}` — нет реального ключа, безопасно
- Маскирование ключа (`sk-...xxxx`) — на стороне клиента в UI

---

## Тестирование

1. Создать агента с API ключом через `CreateAIAgent`
2. Получить агента через `GetAIAgent` — проверить что `provider_config` возвращается
3. Список агентов через `ListAIAgents` — проверить что `provider_config` есть у каждого агента
4. Preset агенты — проверить что возвращается `{"api_key_source":"server","default_model":"..."}`

---

## Изменённые файлы (ожидаемые)

| Файл | Изменение |
|------|-----------|
| `messenger.proto` | Добавить `string provider_config = 22` в `AgentInfoV2` |
| `gen/messenger.pb.go` | Перегенерация |
| `server_ai_v2.go` | `agentToProto()` — добавить `ProviderConfig: pcJSON` |

---

## Связанные документы

- Текущий промпт: `doc/PROMPT.md`
- AI сервисы: `doc/AI_SERVICES.md`
- Android клиент: `/Users/paveld/LavenderMessenger-Android/doc/PROMPT_NEXT_SESSION.md`

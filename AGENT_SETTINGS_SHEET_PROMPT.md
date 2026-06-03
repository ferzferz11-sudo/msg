# Задача: Шторка настроек агента (Agent Settings Bottom Sheet)

## КОНТЕКСТ
Пользователь нажимает на агента в списке (AgentListActivity) → должна открываться шторка (BottomSheet) с настройками агента.

## ЧТО СДЕЛАТЬ

### 1. Добавить click handler в AgentListActivity

В `AgentListActivity.kt` в `setupViews()` добавить long click на карточку агента:

```kotlin
// В setupViews(), после adapter.setItems():
adapter.setOnAgentLongClick = { agent ->
    showAgentSettingsSheet(agent)
}
```

### 2. Создать AgentSettingsBottomSheet

Новый файл: `ui/hermes/AgentSettingsBottomSheet.kt`

```kotlin
class AgentSettingsBottomSheet(
    private val agent: AgentInfo,
    private val onSave: (name: String, prompt: String, model: String, maxTokens: Int) -> Unit,
    private val onDelete: () -> Unit
) : StandardBottomSheet(...) {
    
    // Поля: имя, системный промпт, модель, max_tokens
    // Кнопки: Сохранить / Удалить / Отмена
}
```

### 3. Макет: bottom_sheet_agent_settings.xml

- TextInputLayout + TextInputEditText для имени (agentNameInput)
- TextInputLayout + TextInputEditText для промпта (systemPromptInput) — multiline
- TextInputLayout + TextInputEditText для модели (modelInput)
- TextInputLayout + TextInputEditText для max_tokens (maxTokensInput) — number
- MaterialButton "Сохранить" (saveBtn)
- MaterialButton "Удалить" (deleteBtn) — только для кастомных агентов
- TextButton "Отмена" (cancelBtn)

### 4. В AgentListActivity добавить метод

```kotlin
private fun showAgentSettingsSheet(agent: Any) {
    // Cast to AgentInfo или AgentPreset в зависимости от типа
    // Открыть AgentSettingsBottomSheet
}
```

### 5. Цвета и стили

Использовать существующие цвета из colors.xml, Material3 стили.

## ФАЙЛЫ ДЛЯ ИЗМЕНЕНИЯ

1. `ui/hermes/AgentListActivity.kt` — добавить long click handler
2. `ui/hermes/AgentSettingsBottomSheet.kt` — НОВЫЙ
3. `res/layout/bottom_sheet_agent_settings.xml` — НОВЫЙ

## ПРИМЕРЫ КОДА

Если нужны примеры — посмотри:
- `ui/hermes/AgentSettingsActivity.kt` — существующая логика создания/редактирования
- `res/layout/activity_agent_settings.xml` — существующий layout
- `ui/hermes/AgentListActivity.kt:60-64` — текущий click handler

## ВАЖНО
- Использовать ViewBinding + XML layouts (НЕ Jetpack Compose)
- Сохранить существующий функционал AgentSettingsActivity (создание нового агента через FAB)
- BottomSheet только для редактирования существующих агентов

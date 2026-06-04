# Hermes Orchestrator — Задачи

**Версия:** v1.1.0.12
**Обновлено:** 2026-06-04
**Статус:** ✅ Unified Chat Widget — агенты как участники группового чата

---

## ✅ Сделано (v1.1.0.12)

### Android — Unified Chat Widget
- **widget_chat.xml** — единый layout для группового чата и Hermes
- **item_chat_message.xml** — универсальный item (user/agent/system/typing/date)
- **ChatMessageAdapter** — единый адаптер с DiffUtil, поддерживает:
  - User messages (right-aligned, primary bubble)
  - Agent messages (left-aligned, avatar/emoji + name)
  - Typing indicators
  - Date separators
  - Reply quotes
- **ChatWidget.kt** — ViewBinding обёртка с общим API
- **HermesChatActivity** — переписан на единый виджет:
  - Агенты отображаются как участники группового чата
  - Тап по чипу агента → переключение на прямой чат
  - Сообщения от разных агентов визуально различаются
- **HermesChatViewModel** — добавлено:
  - `agents: StateFlow<List<AgentInfo>>` — реестр агентов
  - `addAgent()`, `removeAgent()`, `getAgent()` — управление
  - `initPresetAgents()` — 8 пресетов

---

## ✅ Сделано (v1.1.0.11)

- **Proto mismatch в CreateHermesSession** — исправлен
- **LogViewerActivity** — добавлена в AndroidManifest, убран ThemeUi.bind()
- **SuperAdmin** — проверка по user_id (UUID) с fallback на username
- **AppLog + LogViewerActivity** — система логирования ошибок

---

## 🔧 В процессе

- Адаптация NewChatActivity для использования ChatWidget (рефакторинг)

---

## ⏳ Не начато

- RemoteAgentManager.SendTask() — заглушка
- Auth токены для удалённых агентов
- Qdrant + CLIP для production RAG
- Agent settings bottom sheet

---

## 🧪 Готово к проверке

- ✅ Unified Chat Widget — скомпилировано, готово к тестированию
- ✅ HermesChatActivity — агенты как участники группового чата
- ✅ ChatMessageAdapter — универсальный адаптер

---

## Следующий шаг

Адаптировать NewChatActivity для использования ChatWidget (рефакторинг). Затем — RemoteAgentManager.SendTask().

# Hermes Orchestrator — Задачи

**Версия:** v1.1.0.14
**Обновлено:** 2026-06-05
**Статус:** ✅ Hermes сессии в списке чатов — сервер + Android готовы

---

## ✅ Сделано (v1.1.0.14)

### Hermes сессии в списке чатов
- **Сервер:** `GetChats` теперь включает hermes_sessions как `type="hermes"`
- **Сервер:** `db_hermes.go` — `GetUserHermesSessions()` с LATERAL JOIN для последнего сообщения
- **Proto:** `ChatInfo` расширен — `active_agent_id = 20`, `agent_mode = 21`
- **Android:** `ChatInfoProto`, `ChatInfo`, `ChatEntity` обновлены с новыми полями
- **Android:** `RealGrpcClient` — оба парсера парсят fields 21/22
- **Android:** `ChatListActivity.onChatClick` — при `type == "hermes"` → `HermesChatActivity`
- **Android:** `HermesChatActivity` — принимает существующую сессию из intent
- **Android:** `HermesChatViewModel` — `setExistingSession(sessionId, userId, agentId, mode)`

---

## ✅ Сделано (v1.1.0.13)

### Android — ChatWidget refactor
- HermesChatActivity переписан на ChatWidget (убрано дублирование findViewById)
- activity_hermes_chat.xml → FrameLayout + ChatWidget + ProgressBar (было 178 строк → 20)
- Agent chips: активный агент выделен (фон + обводка primary color)
- ProgressBar для loading state
- Typing indicator с именем агента

### Android — Mention system
- @ в поле ввода → popup со списком агентов
- MentionAdapter + MentionItem в ui.chat.widget
- item_mention_agent.xml — emoji + name + description + tag
- Исправлен SpannableBuilder IndexOutOfBoundsException
- Исправлена рекursion в MentionAdapter (submitList → setItems)

### Два отдельных MentionAdapter
- ui.chat.widget.MentionAdapter — агенты (emoji, item_mention_agent.xml)
- ui.adapter.MentionAdapter — пользователи (аватары, item_mention.xml)

---

## ✅ Сделано (ранее)

### v1.1.0.12 — Unified Chat Widget
- widget_chat.xml, item_chat_message.xml, ChatMessageAdapter, ChatWidget.kt
- HermesChatActivity: агенты как участники
- HermesChatViewModel: agents registry, initPresetAgents()

### v1.1.0.12 — OWL удалён
- OwlActivity, OwlGrpc, OwlMessageAdapter, все OWL layouts/drawables/menu/proto (-2425 строк)

### v1.1.0.15 — Сервер
- LLM Router, RAG Pipeline, Tool Executor, Pipeline
- HermesAgentService + remote agent routing
- GRANT ALL PRIVILEGES для lavender user

---

## ⏳ Не начато (по приоритету)

### Высокий приоритет
1. **Тестирование** — проверить что hermes сессии появляются в списке чатов, открываются, история загружается

### Средний приоритет
2. **Auth токены для удалённых агентов** — JWT при регистрации, валидация при каждом запросе
3. **Qdrant + CLIP** — production RAG

### Низкий приоритет
4. **Graceful reconnect** при keepalive failed
5. **NewChatActivity** — миграция на ChatWidget (рефакторинг)

---

## Следующий шаг

**Тестирование** — после сборки APK пользователем, проверить:
1. Hermes сессия появляется в списке чатов после первого диалога с оркестратором
2. Тап на hermes чат открывает HermesChatActivity с загрузкой истории
3. При выходе из HermesChatActivity чат остаётся в списке

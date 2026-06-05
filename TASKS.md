# Hermes Orchestrator — Задачи

**Версия:** v1.1.0.13
**Обновлено:** 2026-07-15
**Статус:** ✅ ChatWidget + Mention system работают

---

## ✅ Сделано (v1.1.0.13)

### Android — ChatWidget refactor
- HermesChatActivity переписан на ChatWidget (убрано дублирование findViewById)
- activity_hermes_chat.xml → FrameLayout + ChatWidget + ProgressBar (было 178 строк → 20)
- Agent chips: активный агент выделен (фон + обводка primary color)
- ProgressBar для loading state
- Typing indicator с именем агента
- Новые цвета: chip_background_active, chip_text_active, chip_stroke_active
- bg_hermes_circle.xml — круглая подложка для emoji

### Android — Mention system
- @ в поле ввода → popup со списком агентов
- MentionAdapter + MentionItem в ui.chat.widget
- item_mention_agent.xml — emoji + name + description + tag
- Фильтрация по имени/тегу при вводе
- Вставка @tag при выборе агента
- Исправлен SpannableBuilder IndexOutOfBoundsException
- Исправлена рекursion в MentionAdapter (submitList → setItems)

### Два отдельных MentionAdapter
- ui.chat.widget.MentionAdapter — агенты (emoji, item_mention_agent.xml)
- ui.adapter.MentionAdapter — пользователи (аватары, item_mention.xml)
- НЕ МЕРЖИТЬ!

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

### Высокий приоритет (текущий спринт — публикация на prod)
1. **Hermes сессии в списке чатов** — чат с оркестратором должен сохраняться в списке чатов как групповой
   - Сервер: добавить hermes_sessions в GetChats как type="hermes"
   - Android: показывать hermes чаты в ChatListActivity, при тапе → HermesChatActivity
   - Сохранение истории переписки при выходе из чата

### Средний приоритет
2. **Auth токены для удалённых агентов** — JWT при регистрации, валидация при каждом запросе
3. **Qdrant + CLIP** — production RAG

### Низкий приоритет
4. **Graceful reconnect** при keepalive failed
5. **NewChatActivity** — миграция на ChatWidget (рефакторинг)

---

## Следующий шаг

**Hermes сессии в списке чатов** — чтобы при выходе из HermesChatActivity чат не исчезал, а оставался в списке как групповой чат. Сервер должен возвращать hermes_sessions в GetChats, Android — отображать их и открывать по тапу.

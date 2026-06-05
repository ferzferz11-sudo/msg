# Lavender Messenger - Changelog

**Author:** Pavel Davydov (ferz)

## [1.1.0.13] - 2026-07-15

### Android — ChatWidget рефакторинг
- **HermesChatActivity** переписан на ChatWidget (убрано дублирование findViewById)
- **activity_hermes_chat.xml** → FrameLayout + ChatWidget + ProgressBar (было 178 строк → 20)

### Android — UI полировка
- **Агенты-чипы** — активный агент выделен (фон + обводка primary color)
- **ProgressBar** — индикатор загрузки истории
- **Typing indicator** — имя агента в субтитле тулбара
- Новые цвета: chip_background_active, chip_text_active, chip_stroke_active
- bg_hermes_circle.xml — круглая подложка для emoji

### Android — Mention system
- **@ в поле ввода** → popup со списком агентов (MentionAdapter + MentionItem)
- **item_mention_agent.xml** — emoji + name + description + tag
- Фильтрация по имени/тегу при вводе, вставка @tag при выборе
- Исправлен SpannableBuilder IndexOutOfBoundsException (toString() перед substring)
- Исправлена рекурсия в MentionAdapter (submitList → setItems)
- Два отдельных MentionAdapter: ui.chat.widget (агенты) и ui.adapter (пользователи) — НЕ МЕРЖИТЬ

## [1.1.0.12] - 2026-06-04

### Android — Unified Chat Widget
- **widget_chat.xml** — единый layout для группового чата и Hermes (toolbar + recycler + input + reply preview)
- **item_chat_message.xml** — универсальный item (user/agent/system/typing/date separator)
- **ChatMessageAdapter** — единый адаптер с DiffUtil, поддерживает все типы сообщений
- **ChatWidget.kt** — ViewBinding обёртка с общим API
- **bg_chip.xml** — drawable для чипов агентов

### Android — HermesChatActivity переписан
- Агенты отображаются как участники группового чата (MaterialChip emoji + name в тулбаре)
- Тап по чипу агента → переключение на прямой чат с этим агентом
- Сообщения от разных агентов визуально различаются (emoji + имя)
- Typing indicator показывает имя агента

### Android — HermesChatViewModel расширен
- `agents: StateFlow<List<AgentInfo>>` — реестр агентов-участников
- `addAgent()`, `removeAgent()`, `getAgent()` — управление участниками
- `initPresetAgents()` — инициализация 8 пресетов (Developer, Designer, Writer, Analyst, Translator, Researcher, Tester, OWL)

### Android — OWL полностью удалён (-2425 строк)
- Удалено: OwlActivity, OwlGrpc, OwlMessageAdapter
- Удалено: activity_owl.xml, item_owl_message.xml, dialog_owl_settings.xml
- Удалено: owl_bubble_user.xml, owl_bowl_owl.xml, owl_bubble_owl.xml, owl_menu.xml
- Удалено: OWL proto классы из MessengerProto
- Удалено: OWL ссылки из GrpcClient, ChatAdapter, ChatListActivity
- Удалён метод createNewOwlChat() из ChatListActivity

### Android — Bottom Sheet
- Вместо "Чат с AI" → "Hermes AI" + "Агенты" (AgentListActivity)
- Добавлен ic_agents.xml vector drawable
- Добавлена строка action_hermes_agents в strings.xml

### Сервер — GRANT права
- GRANT ALL PRIVILEGES для lavender user на все hermes-таблицы и sequences

---

## [1.1.0.11] - 2026-06-04

### Android + Server — Hermes Orchestrator работает
- ✅ Оркестратор отвечает приветственным сообщением на Android
- ✅ CreateHermesSession — создание сессии работает
- ✅ ChatWithOrchestrator — стриминг ответов работает

### Android — исправления
- **Proto mismatch в CreateHermesSession** — response marshaller: перепутаны номера полей (field 1=success/bool, field 2=session_id/string). Вызывало `CANCELLED: Failed to read message`
- **LogViewerActivity** — добавлена в AndroidManifest, убран ThemeUi.bind()
- **SuperAdmin** — проверка по user_id (UUID) с fallback на username

---

## [1.1.0.10] - 2026-06-03
- Предыдущие версии...

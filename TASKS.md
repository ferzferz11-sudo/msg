# Hermes Orchestrator — Задачи

**Версия:** v1.1.0.12
**Обновлено:** 2026-06-04
**Статус:** ✅ Unified Chat Widget + OWL удалён

---

## ✅ Сделано (v1.1.0.12)

### Android — Unified Chat Widget
- **widget_chat.xml** — единый layout для группового чата и Hermes
- **item_chat_message.xml** — универсальный item (user/agent/system/typing/date)
- **ChatMessageAdapter** — единый адаптер с DiffUtil
- **ChatWidget.kt** — ViewBinding обёртка
- **HermesChatActivity** — переписан на единый виджет, агенты как участники
- **HermesChatViewModel** — agents registry (addAgent/removeAgent/getAgent/initPresetAgents)
- **MaterialChip** для чипов агентов (вместо TextView + bg_status_bubble)

### Android — OWL полностью удалён
- Удалено: OwlActivity, OwlGrpc, OwlMessageAdapter
- Удалено: activity_owl.xml, item_owl_message.xml, dialog_owl_settings.xml
- Удалено: owl_bubble_user.xml, owl_bowl_owl.xml, owl_bubble_owl.xml
- Удалено: owl_menu.xml, OWL proto классы из MessengerProto
- Удалено: OWL ссылки из GrpcClient, ChatAdapter, ChatListActivity
- Итого: -2425 строк

### Bottom Sheet
- Вместо "Чат с AI" → "Hermes AI" + "Агенты" (AgentListActivity)
- Добавлен ic_agents.xml vector drawable

### Сервер
- GRANT ALL PRIVILEGES для lavender user на hermes-таблицы

---

## ✅ Сделано (v1.1.0.11)
- Proto mismatch в CreateHermesSession — исправлен
- LogViewerActivity — добавлена в AndroidManifest
- SuperAdmin — проверка по user_id (UUID)

---

## 🔧 В процессе
- Адаптация NewChatActivity для использования ChatWidget (рефакторинг)

---

## ⏳ Не начато
- RemoteAgentManager.SendTask() — заглушка
- Auth токены для удалённых агентов
- Qdrant + CLIP для production RAG

---

## 🧪 Готово к проверке
- ✅ Unified Chat Widget — скомпилировано
- ✅ HermesChatActivity — агенты как участники
- ✅ OWL удалён — нет ссылок

---

## Следующий шаг
Адаптировать NewChatActivity для использования ChatWidget (рефакторинг). Затем — RemoteAgentManager.SendTask().

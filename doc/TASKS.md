# Lavender Messenger — Задачи

**Версия:** v1.1.2.0
**Ветка:** feat/1.1.2.x
**Обновлено:** 2026-06-09
**Статус:** 🔄 Багфикс AI чатов — передеплоен на prod для тестирования

---

## 🔄 Текущие баги (feat/1.1.2.x)

### 1. Hermes оркестратор — ошибка создания сессии ✅
- **Причина:** `permission denied` на `hermes_sessions` в prod DB
- **Исправлено:** пермишены `ALTER TABLE hermes_sessions OWNER TO lavender`
- **Статус:** исправлено, тестирование успешно

### 2. HermesGrpc — неправильный маппинг proto полей ✅
- **Причина:** в `CreateHermesSessionResponse` поля 1 и 2 перепутаны (success/session_id), аналогично в `CreateAgentResponse`, в `AgentInfo` поля 4-6 не соответствуют proto
- **Исправлено:** HermesGrpc.kt — поля 1=success(bool), 2=session_id(string); MessengerProto.kt — AgentInfo: 4=is_preset(bool), 5=system_prompt(string), 6=model(string)
- **Статус:** закоммичено в `feat/1.1.2.x` (77bff6f), тестирование успешно

### 3. last_message_text пустой для Hermes чатов ✅
- **Причина:** `ChatWithOrchestrator` сохранял в `hermes_messages`, но не обновлял `chats.last_message_text`
- **Исправлено:** добавлен `UPDATE chats SET last_message_text` после ответа
- **Статус:** закоммичено (3fd62c3), ожидает тестирования

### 4. Дубли чатов в UI ✅
- **Причина:** `GetAIChats` брал Hermes из `hermes_sessions`, а OWL из `chats` — Hermes сессии дублировались
- **Исправлено:** `GetAIChats` берёт оба типа из `chats`
- **Статус:** закоммичено (a59055e), ожидает тестирования

### 5. getOrCreateSession создаёт дубли сессий ✅
- **Причина:** `getOrCreateSession` создавал сессию с `id = "hermes-" + userID`, а `CreateHermesSession` создавал с `id = "hermes-" + UUID` — два разных ID
- **Исправлено:** `getOrCreateSession` теперь ищет существующую сессию по `user_id` в `hermes_sessions`
- **Статус:** закоммичено (9d847bf), ожидает тестирования

### 6. OWL — сообщения не сохраняются в БД 🔬
- **Причина:** `addMessage` определена, но INSERT в `owl_messages` не происходит (0 записей)
- **Исправлено:** добавлены дебог-логи в `owlSessionManager.addMessage`
- **Статус:** деплоен на dev и prod, нужно протестить OWL и проверить логи

---

## ✅ v1.1.2.0 — Prod Релиз
- Сервер prod обновлён с v1.1.0.15 до v1.1.1.15
- Пермишены на hermes_sessions исправлены

---

## 📋 Бэклог (после багфикса)
- Модульные тесты для OWL streaming
- Auth токены для удалённых агентов (JWT)
- Qdrant + CLIP (production RAG)

# Lavender Messenger — Задачи

**Версия:** v1.1.1.12
**Обновлено:** 2026-06-09
**Статус:** ✅ v1.1.1.12 — Bug fixes + Command bottom sheet + Notification badge

---

## ✅ Сделано (v1.1.1.12 — Night session)

### Android
- **Bug fix: messages disappearing** — user messages now via ViewModel (addUserMessage/addBotMessage), not directly in adapter
- **Bug fix: unread counter** — refreshUnreadCount() added to onResume()
- **CommandBottomSheet** — unified command picker in AIBottomSheet style (OWL + Hermes)
- **Notifications + badge in AIBottomSheet** — unread count shown at top of AI sheet
- **version.txt** — bumped to 1.1.1.12
- **changelog.txt** — updated with user-facing RU changelog
- compileDebugKotlin ✅

### Server
- **ServerVersion 1.1.1.12** — version bump (no server-side changes, all v1.1.1.11 features work)
- Dev deployed ✅

---

## ✅ Сделано (v1.1.1.11)

### Server
- **ServerVersion 1.1.1.11** — version bump

### Android
- **ic_ai.xml** — replaced with robot vector drawable (960x960 viewport, Material style)
- **widget_chat.xml** — added `toolbarInfo` TextView for key/model banner
- **ChatWidget.kt** — added `toolbarInfo` field + `setToolbarInfo()` method
- **OwlChatActivity.kt** — loads OWL settings on chat open, shows key/model info in header
- **HermesChatActivity.kt** — loads Hermes settings on chat open, shows key/model info in header
- Info format: "Ваш ключ · {model}" or "Общий ключ · 20 запросов/час"
- compileDebugKotlin ✅

---

## ✅ Сделано (v1.1.1.10)

### Server
- **hermes_chat_settings table**: per-session API key + model storage
- **GetHermesSettings RPC**: returns api_key, model, is_using_custom_key
- **UpdateHermesSettings RPC**: updates per-session settings with ownership check
- **Rate limiting**: custom key = 10/min, free tier = 20/hour (freeTierRateLimiter)
- **GetOwlSettings**: now returns is_using_custom_key
- **GetAIChats**: returns is_using_custom_key + model for all chats
- **ChatWithOWL**: rate limit check (custom vs free tier)
- **ChatWithOrchestrator**: rate limit check (custom vs free tier)
- Proto updated with new messages and fields
- Dev deployed and running

### Android
- **AIBottomSheet redesign**: unified chat list (all types), long-press popup menu (delete/settings), divider, Hermes/OWL create sections
- **widget_ai_chat_item.xml**: new layout with icon, name, type label, settings gear
- **OwlSettingsActivity**: unified OWL+Hermes settings screen, key source indicator, rate limit info, dynamic model list
- **HermesGrpc.kt**: getHermesSettings/updateHermesSettings methods
- **OwlGrpc.kt**: updated parser for GetOwlSettingsResponse (isUsingCustomKey)
- **RealGrpcClient.kt**: updated AIChatInfo parser (isUsingCustomKey, model)
- **ChatListActivity.kt**: rewritten showAIActionSheet, unified currentAiChats, openHermesSettings/openOwlSettings
- **AIChatInfo / AIChatInfoProto**: added isUsingCustomKey + model fields
- compileDebugKotlin ✅ | go build ✅ | Dev deployed

---

## ✅ Сделано (v1.1.1.9)
- Grace period (30s) в hub
- Exponential backoff reconnect
- Notification retry
- compileDebugKotlin ✅

---

## ✅ Сделано (v1.1.1.8)
- participants хранит UUID
- AI chats excluded from GetUserChats/GetAllChats
- GetAIChats, RenameAIChat RPCs
- AIBottomSheet selection mode
- compileDebugKotlin ✅

---

## ✅ Сделано (v1.1.1.7)
- Notification badge (server + Android)
- GetUnreadCount RPC
- compileDebugKotlin ✅

---

## ✅ Сделано (v1.1.1.6)
- Множественные OWL/Hermes чаты с нумерацией
- createOwlChat() RPC
- compileDebugKotlin ✅

---

## ✅ Сделано (v1.1.1.5)
- OwlSettingsActivity
- getOwlSettings/updateOwlSettings
- creator_id миграция
- compileDebugKotlin ✅

---

## ✅ Сделано (v1.1.1.4)
- [AI] кнопка рядом с [+]
- AIBottomSheet (группы + divider)
- OWL FK fix
- compileDebugKotlin ✅

---

## ✅ Сделано (ранее)
- v1.1.1.3 — NotificationActivity, bot tests
- v1.1.1.2 — SendServerNotification, OWL/Hermes разделение
- v1.1.1.1 — Bot Commands, Rate Limiting, NotificationService
- v1.1.0.16 — Favorites fix
- v1.1.0.15 — Force reconnect + Registration fix
- v1.1.0.14 — Hermes sessions in chat list
- v1.1.0.13 — ChatWidget + Mention system
- v1.1.0.12 — Unified Chat Widget
- v1.1.0.11 — Hermes Orchestrator
- v1.1.0.10 — Agent Management gRPC

---

## ⏳ Не начато (по приоритету)

### Высокий приоритет (v1.1.1.11 — тестирование)
1. ~~**Показать ключ/модель в шапке AI чатов**~~ ✅ v1.1.1.11
2. **Полное тестирование** — graceful reconnect, notification retry, AI chats, настройки, rate limits
3. **Исправление найденных багов**
4. **Деплой на prod → v1.1.2.0**

### Средний приоритет
- Модульные тесты для OWL streaming

### Низкий приоритет
- Auth токены для удалённых агентов (JWT)
- Qdrant + CLIP (production RAG)
- NewChatActivity → ChatWidget миграция

---

## Известные проблемы

- Server migration warnings: `role "lavender" does not exist` (не критично, сервер работает)

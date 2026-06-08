# Lavender Messenger — Задачи

**Версия:** v1.1.1.x
**Обновлено:** 2026-06-08
**Статус:** AI Bottom Sheet редизайн + Hermes settings

---

## ✅ Сделано (v1.1.1.x — AI Bottom Sheet + Settings)

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

### Server
- **Grace period в hub:** при разрыве соединения пользователь переходит в grace period (30s) вместо немедленного удаления
- **Grace Period API:** `StartGracePeriod()`, `IsInGracePeriod()`, `ClearGracePeriod()`, `GetGracePeriodRemaining()`
- **GetOnlineUsers:** пользователи в grace period отображаются как online
- **ClearGracePeriod:** вызывается при успешной ре-аутентификации
- ServerVersion: 1.1.1.8 → 1.1.1.9

### Android
- **ConnectionStatus.RECONNECTING:** новый enum — промежуточный статус при переподключении
- **RealGrpcClient.connect():** RECONNECTING вместо CONNECTING при повторном подключении
- **Exponential backoff:** 5s → 60s для channel reconnect, 3s → 30s для stream retry
- **subscribeNotifications:** автоматический retry с backoff — уведомления не теряются
- **chatWithOwl:** автоматический retry (10 попыток, 3s → 30s backoff)
- **chatWithOrchestrator:** автоматический retry (10 попыток, 3s → 30s backoff)
- **onError:** FAILED заменён на RECONNECTING, ускоренный retry
- **Keep-alive:** keepAliveTime 10s, keepAliveTimeout 5s
- compileDebugKotlin ✅ | go build ✅

### Сборка и деплой
- compileDebugKotlin ✅ | go build ✅
- Dev сервер обновлён и работает (v1.1.1.9)

---

## ✅ Сделано (v1.1.1.8)

### Server
- **Исправлен невалидный JSON в participants:** заменена конкатенация на `json.Marshal([]string{userId})`. Теперь хранится UUID вместо username.
- **GetUserChats исключает AI-чаты:** `WHERE c.type NOT IN ('owl', 'hermes')`
- **GetAllChats не включает AI-чаты:** отдельный RPC GetAIChats
- **Новый RPC GetAIChats:** возвращает все AI-чаты пользователя (OWL + Hermes)
- **Новый RPC RenameAIChat:** переименование AI-чата с проверкой владельца
- **DeleteChat:** убрано уведомление участников для AI-чатов
- **Proto:** добавлены GetAIChatsRequest/Response, RenameAIChatRequest/Response, AIChatInfo
- ServerVersion: 1.1.1.7 → 1.1.1.8

### Android
- **refreshAiChats():** загружает через GetAIChats RPC
- **AIBottomSheet:** selection mode с тулбаром удаления/переименования
- **showAIActionSheet():** подключены кнопки удаления и переименования
- **AIChatInfo:** новый data class
- **GrpcClient:** добавлены getAIChats(), renameAIChat(), deleteChat() overload
- **RealGrpcClient:** новые RPC методы + marshallers
- **MessengerProto.kt:** новые proto классы
- **widget_ai_bottom_sheet.xml:** selection toolbar layout
- compileDebugKotlin ✅

### Сборка и деплой
- compileDebugKotlin ✅ | go build ✅
- Dev сервер обновлён и работает
- Теги v1.1.1.8 на обоих репозиториях

---

## ✅ Сделано (v1.1.1.7)
- **Notification badge (серверная часть):** per-user read tracking для серверных уведомлений
- **notificationService:** добавлено readStates — map[userID]map[notificationID]bool
- **MarkNotificationsRead:** теперь реально отмечает уведомления как прочитанные
- **GetNotificationHistory:** возвращает уведомления с флагом is_read для текущего пользователя
- **GetUnreadCount RPC:** новый RPC — возвращает количество непрочитанных уведомлений
- **Proto:** добавлено поле is_read (field 7) в ServerNotification
- **Proto:** добавлены GetUnreadCountRequest/Response
- ServerVersion: 1.1.1.6 → 1.1.1.7

### Android
- **ServerNotificationProto:** добавлено поле isRead: Boolean
- **OwlGrpc.kt:** getUnreadCount() + парсеры isRead для history и streaming
- **GrpcClient.kt:** getUnreadCount() метод
- **NotificationAdapter:** bold title + accent background для непрочитанных, click → mark read
- **NotificationActivity:** отмечает все загруженные уведомления как прочитанные при открытии
- **SheetAction:** добавлено поле badge: Int
- **AIBottomSheet + ActionBottomSheet:** показывают badge (красный кружок с числом)
- **Layouts:** widget_action_item.xml добавлен actionBadge, badge_background.xml
- **Colors:** notification_unread_bg для accent-фона непрочитанных
- **ChatListActivity:** unreadNotifCount + refreshUnreadCount(), badge на "Уведомления"
- compileDebugKotlin ✅

### Сборка и деплой
- compileDebugKotlin ✅ | go build ✅
- Dev сервер обновлён и работает (v1.1.1.7)

---

## ✅ Сделано (v1.1.1.6)

### Server
- **Множественные OWL/Hermes чаты с нумерацией**: каждый новый чат уникален с порядковым номером
- **getNextChatNumber()**: SQL MAX(existing)+1, номера не переиспользуются при удалении
- **generateChatName()**: локализованные имена (русский: "Лава ИИ #N", "Оркестратор #N")
- **CreateOwlChat**: UUID-based chatID + имя с номером, возвращает name в ответе
- **CreateHermesSession**: UUID-based sessionID + имя с номером, возвращает name в ответе
- **Proto**: добавлено поле name в CreateOwlChatResponse и CreateHermesSessionResponse
- ServerVersion: 1.1.1.5 → 1.1.1.6

### Android
- **createOwlChat()**: новый unary RPC для создания пронумерованных OWL чатов на сервере
- **OwlChatActivity**: убрана локальная генерация chatId, вызывает createOwlChat если пустой
- **OwlSettingsActivity**: убрана локальная генерация chatId, читает из intent
- **ChatListActivity**: AIBottomSheet показывает существующие чаты с номерами + кнопку "Создать новый"
- **refreshAiChats()**: фильтрует owl/hermes чаты из основного списка для AIBottomSheet
- **MessengerProto**: добавлены CreateOwlChatRequestProto, CreateOwlChatResponseProto
- **HermesGrpc**: добавлен парсинг name в CreateHermesSessionResponseProto
- compileDebugKotlin ✅

### Сборка и деплой
- compileDebugKotlin ✅ | go build ✅
- Dev сервер обновлён и работает
- Оба репозитория запушены
- Теги v1.1.1.6 на обоих репозиториях

---

## ✅ Сделано (v1.1.1.5)

### Server
- **HermesSession → chats**: INSERT INTO chats (type="hermes") при создании сессии
- **DeleteChat для Hermes**: fallback на hermes_sessions если не найден в chats
- **GetOwlSettings RPC**: новый RPC для получения per-chat настроек (api_key, model)
- **UpdateOwlSettings fix**: проверка владельца по creator_id (UUID) вместо creator_username
- **creator_id миграция**: добавлена колонка creator_id в chats, все проверки владельца по UUID
- **HermesDB**: добавлены GetSessionID() и DeleteSession()
- **GetAllChats OWL SELECT**: поиск через creator_id с subquery
- ServerVersion: 1.1.1.4 → 1.1.1.5

### Android
- **OwlSettingsActivity**: экран настроек OWL (API key input, model selector)
- **activity_owl_settings.xml**: layout с темой
- **OwlGrpc.kt**: getOwlSettings() и updateOwlSettings() unary RPC
- **MessengerProto.kt**: proto классы для OWL settings
- **AndroidManifest.xml**: регистрация OwlSettingsActivity
- **ChatListActivity.kt**: AIBottomSheet → OwlSettingsActivity вместо Toast

### Сборка и деплой
- compileDebugKotlin ✅ | go build ✅
- Dev сервер обновлён и работает
- Оба репозитория запушены
- Теги v1.1.1.5 на обоих репозиториях

### Документация
- Создана папка doc/ с индексом INDEX.md
- Все MD файлы перенесены в doc/
- В корне остались только README.md и CHANGELOG.md

---

## ✅ Сделано (v1.1.1.4)

### Android
- **[AI] кнопка** рядом с [+] в ChatListActivity
- **AIBottomSheet** — шторка с группировкой (Оркестратор / OWL)
- **AI-пункты перенесены** из [+] в [AI] шторку
- **HermesChatActivity:** userId (UUID) вместо username

### Server
- **OWL FK fix**: авто-создание OWL чата в chats при первом сообщении
- **HermesSession username→UUID**: резолвинг в CreateHermesSession

---

## ✅ Сделано (ранее)

- v1.1.1.3 — NotificationActivity, bot tests
- v1.1.1.2 — SendServerNotification, OWL/Hermes разделение
- v1.1.1.1 — Bot Commands, Rate Limiting, NotificationService
- v1.1.0.16 — Favorites fix
- v1.1.0.15 — Force reconnect + Registration fix + Cache clearing
- v1.1.0.14 — Hermes sessions in chat list
- v1.1.0.13 — ChatWidget + Mention system
- v1.1.0.12 — Unified Chat Widget
- v1.1.0.11 — Hermes Orchestrator
- v1.1.0.10 — Agent Management gRPC

---

## ⏳ Не начато (по приоритету)

### Высокий приоритет (v1.1.1.6)
1. **Множественные OWL/Hermes чаты с нумерацией**
   - Старый подход: один OWL чат на пользователя (chatId = "owl-$userId"), один Hermes ("hermes-$userId")
   - **Новый подход**: каждый новый чат уникален с порядковым номером
   - OWL: `Лава ИИ #1`, `Лава ИИ #2`, ... (русский) / `Lava AI #1`, `Lava AI #2`, ... (english)
   - Hermes: `Оркестратор #1`, `Оркестратор #2`, ... (русский) / `Orchestrator #1`, `Orchestrator #2`, ... (english)
   - Номер = MAX(existing) + 1 для данного пользователя и типа чата
   - При удалении номера НЕ переиспользуются (всегда инкремент)
   - Server: изменить CreateOwlChat и CreateHermesSession — генерировать имя с номером
   - Android: убрать генерацию chatId на клиенте, вызывать серверный CreateOwlChat/CreateHermesSession
   - AIBottomSheet: "Lava AI" / "Оркестратор" → создать новый чат, показывать существующие с номерами
   - Локализация: Locale.getDefault().language == "ru" → русский, иначе английский

### Средний приоритет
2. **NotificationActivity badge** — счётчик непрочитанных на иконке колокольчика
3. **Graceful reconnect** — переподключение без потери стримов

### Низкий приоритет
4. **Auth токены для удалённых агентов** (JWT)
5. **Qdrant + CLIP** (production RAG)
6. **NewChatActivity** — миграция на ChatWidget
7. **Деплой на prod** — только после завершения всех задач интеграции

---

## Известные проблемы

- Server migration warnings: `role "lavender" does not exist` (не критично, сервер работает)

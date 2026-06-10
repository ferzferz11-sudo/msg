# Lavender Messenger — Задачи

**Версия:** v1.1.2.3
**Ветка:** feat/1.1.2.x
**Обновлено:** 2026-06-09
**Статус:** ⚠️ Есть нерешённые проблемы

---

## ⚠️ v1.1.2.3 — Найденные проблемы (требуют исправления)

### 1. Hermes история не загружается ⭐ ВЫСОКИЙ ПРИОРИТЕТ
- **Симптом**: пользователь не видит историю чата с оркестратором
- **Возможная причина**: AiChatGrpc использует новый ChatWithAI RPC, но HermesChatActivity всё ещё вызывает старый chatWithOrchestrator. Новая активити не интегрирована, старая не работает с новыми таблицами
- **Что делать**: проверить какой RPC вызывает HermesChatActivity, либо мигрировать на ChatWithAI, либо убедиться что старый ChatWithOrchestrator сохраняет в ai_chat_messages (сейчас сохраняет в hermes_messages которой больше нет)
- **Статус**: не исправлено

### 2. Счётчик запросов показывает максимум 19 ⭐ ВЫСОКИЙ ПРИОРИТЕТ
- **Симптом**: в чате с агентом счётчик идёт только до 19, а лимит 20
- **Возможная причина**: ошибка на 1 в rate limiter — условие `>= limit` вместо `> limit`, или remaining вычисляется как `limit - used` но used считается включая текущий запрос до того как он добавлен
- **Что делать**: проверить owlSessionManager (10/мин) vs freeTierRateLimiter (20/час) — какой используется для агента. Проверить логику remaining() — должно быть `limit - len(valid)`, возможно off-by-one
- **Статус**: не исправлено

---

## ✅ v1.1.2.3 — AI Chat Refactor

### 1. Единый менеджер AI чатов ✅
- ai_chat_manager.go: CreateSession, GetSession, DeleteSession, AddMessage, GetHistory, GetSettings, SaveSettings
- ai_chat_sessions, ai_chat_messages, ai_chat_settings таблицы
- FK CASCADE на все AI-таблицы
- Dev и prod обновлены

### 2. Новые proto сообщения и RPC ✅
- AIChatRequest, AIChatResponse, AIChatMessage, AIChatSettings
- ChatWithAI, GetAIChatHistory, GetAIChatSettings, UpdateAIChatSettings
- Старые RPC пометены deprecated

### 3. AI Chat handlers на сервере ✅
- ChatWithAI: маршрутизация owl→OpenRouter, hermes→Orchestrator
- GetAIChatHistory, GetAIChatSettings, UpdateAIChatSettings
- server.go + main.go обновлены

### 4. Android AiChatGrpc.kt ✅
- AIChatRequestProto, AIChatResponseProto, AIChatMessageProto, AIChatSettingsProto
- chatWithAI streaming + unary RPCs
- GrpcClient.kt facade обновлён
- compileDebugKotlin passes

---

## ✅ v1.1.2.2 — DeleteChat cascade + очистка AI чатов

### 1. DeleteChat не удалял hermes_sessions ✅
- **Причина:** DeleteChat удалял только из chats, но не из hermes_sessions — оставались orphan-записи
- **Исправлено:** добавлено каскадное удаление из hermes_sessions + hermes_messages для hermes, owl_messages + owl_chat_settings для owl
- **Статус:** исправлено, деплоено на dev и prod

### 2. Очистка всех AI чатов на dev и prod ✅
- **Причина:** накопились orphan-записи в hermes_sessions/hermes_messages после удаления чатов
- **Исправлено:** полная очистка chats, owl_messages, owl_chat_settings, hermes_sessions, hermes_messages, hermes_chat_settings на обоих серверах
- **Статус:** выполнено — 0 AI-записей на dev и prod

---

## ✅ v1.1.2.1 — История из БД + счётчик запросов

### 1. Hermes история из БД ✅
- **Причина:** GetOrchestratorHistory брал из in-memory session.Messages — после рестарта сервера история пропадала
- **Исправлено:** GetOrchestratorHistory загружает из hermes_messages через HermesDB.GetOrchestratorHistory()
- **Статус:** исправлено, деплоено на dev и prod

### 2. Проверка владельца в GetOwlHistory ✅
- **Причина:** любой мог запросить историю любого OWL чата
- **Исправлено:** добавлена проверка creator_id == userId
- **Статус:** исправлено, деплоено

### 3. Счётчик оставшихся запросов ✅
- **Причина:** в тулбаре показывалась только статичная строка "20 запросов/час"
- **Исправлено:** rate limiter.remaining(), GetOwlSettings/GetHermesSettings возвращают remaining/limit/window_seconds
- **Статус:** исправлено, деплоено

### 4. HermesChatActivity — loadHistory при новом чате ✅
- **Причина:** loadHistory вызывался только если chatId не пустой, но при новом чате chatId пуст до создания сессии
- **Исправлено:** loadHistory вызывается всегда когда session.id не пуст
- **Статус:** исправлено

---

## ✅ v1.1.2.0 — Все баги закрыты

### 1. Hermes оркестратор — ошибка создания сессии ✅
- **Причина:** `permission denied` на `hermes_sessions` в prod DB
- **Исправлено:** пермишены `ALTER TABLE hermes_sessions OWNER TO lavender`
- **Статус:** исправлено

### 2. HermesGrpc — неправильный маппинг proto полей ✅
- **Причина:** в `CreateHermesSessionResponse` поля 1 и 2 перепутаны
- **Исправлено:** HermesGrpc.kt — поля 1=success(bool), 2=session_id(string)
- **Статус:** закоммичено, протестировано

### 3. last_message_text пустой для Hermes чатов ✅
- **Причина:** ChatWithOrchestrator не обновлял chats.last_message_text
- **Исправлено:** добавлен UPDATE chats после ответа
- **Статус:** закоммичено, протестировано

### 4. Дубли чатов в UI ✅
- **Причина:** GetAIChats брал Hermes из hermes_sessions, а OWL из chats
- **Исправлено:** GetAIChats берёт оба типа из chats
- **Статус:** закоммичено, протестировано

### 5. getOrCreateSession создаёт дубли сессий ✅
- **Причина:** создавал сессию с id = "hermes-" + userID каждый раз
- **Исправлено:** ищет существующую сессию по user_id
- **Статус:** закоммичено, протестировано

### 6. OWL — сообщения не сохраняются в БД ✅
- **Причина:** debug-логи показали что INSERT работает
- **Статус:** подтверждено что работает — 5 записей в prod DB, 10 в dev DB

### 7. Log-monitor — все логи красным ✅
- **Причина:** неправильное экранирование `\n` в Go raw string literal
- **Исправлено:** `\\n` вместо `\\\\n` в prod log-monitor
- **Статус:** исправлено, деплоено

### 8. Log-monitor — показывал старые логи ✅
- **Причина:** `--since "24 hours ago"` + `-n 100` брал старые 100 записей
- **Исправлено:** убран `--since`, `-n 100` берёт последние 100
- **Статус:** исправлено, деплоено

---

## ✅ Prod Релиз v1.1.2.0
- Сервер prod обновлён (v1.1.1.15 → v1.1.2.0)
- Пермишены на hermes_sessions исправлены
- Log-monitor обновлён и работает
- compileDebugKotlin проходит
- Dev сервер обновлён и работает

---

## 📋 Бэклог (v1.1.2.1 — исправления и улучшения AI чатов)

### В работе
- Тестирование OWL/Hermes чатов на dev и prod
- Исправление найденных багов
- Улучшения UX AI чатов

### Средний приоритет
- Модульные тесты для OWL streaming

### Низкий приоритет
- Auth токены для удалённых агентов (JWT)
- Qdrant + CLIP (production RAG)

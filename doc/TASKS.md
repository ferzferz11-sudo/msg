# Lavender Messenger — Задачи

**Версия:** v1.1.2.0
**Ветка:** feat/1.1.2.x
**Обновлено:** 2026-06-09
**Статус:** 🔄 Багфикс AI чатов

---

## 🔄 Текущие баги (feat/1.1.2.x)

### 1. Hermes оркестратор — ошибка создания сессии
- **Причина:** `permission denied` на `hermes_sessions` в prod DB
- **Исправлено:** пермишены исправлены ✅
- **Статус:** ожидает тестирования

### 2. HermesGrpc — неправильный маппинг proto полей
- **Причина:** в `CreateHermesSessionResponse` поля 1 и 2 перепутаны (success/session_id), в `CreateAgentResponse` аналогично, в `AgentInfo` поля 4-6 не соответствуют proto
- **Исправлено:** HermesGrpc.kt + MessengerProto.kt ✅
- **Статус:** закоммичено, ожидает сборки APK и тестирования

### 3. OWL — сообщения не сохраняются в БД
- **Причина:** `addMessage` определена но возможно не вызывается при стриминге, или проблема с FK constraint
- **Исправлено:** добавлены дебаг-логи в `owlSessionManager.addMessage` ✅
- **Статус:** деплоено на dev, ожидает тестирования

### 4. Удаление ИИ чата — удаляются оба
- **Причина:** требует уточнения
- **Статус:** ожидает тестирования и уточнения

---

## ✅ v1.1.2.0 — Prod Релиз

### Сервер
- Prod обновлён с v1.1.0.15 до v1.1.1.15
- Бэкап: lavender-server-backup-20260609
- Порт 50051, systemd сервис lavender-server

### Клиент
- APK v1.1.1.16 доступен для скачивания
- compileDebugKotlin ✅

---

## ✅ v1.1.1.16 — Багфикс + полировка (клиент)

### Android
- SplashActivity: логотип 🦞 → ic_notification_logo, надпись "Лава"/"Lava"
- AI навигация: return из AI активити → AI шторка открывается снова
- AIBottomSheet: после удаления чата шторка перестраивается
- ThemeApplier: aiFab добавлен в список FAB для кастомных тем
- Save button: style="@style/PrimaryButton"
- compileDebugKotlin ✅

### Сервер
- Без изменений (v1.1.1.15)

---

## ✅ v1.1.1.15 — Бесплатные модели + своя модель

### Server
- free_openrouter_models table
- GetFreeModels RPC, SetFreeModel/RemoveFreeModel RPC (admin)
- GetOwlSettings возвращает free_models
- Dev deployed

### Android
- Бесплатные модели с сервера
- Своя модель — текстовый ввод ID
- Favorites flickering fix

---

## 📋 Бэклог (после багфикса)

### Средний приоритет
- Модульные тесты для OWL streaming

### Низкий приоритет
- Auth токены для удалённых агентов (JWT)
- Qdrant + CLIP (production RAG)
- NewChatActivity → ChatWidget миграция

---

## Известные проблемы (не критично)
- Server migration warnings: `role "lavender" does not exist` — сервер работает

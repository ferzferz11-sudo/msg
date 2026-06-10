# Lavender Messenger — Задачи

**Версия:** v1.1.2.7
**Ветка:** feat/1.1.2.x
**Обновлено:** 2026-06-11

---

## ✅ v1.1.2.7 — Splash улучшения, онбординг удалён, чекбокс чата

### Android v1.1.2.7
- **SplashActivity**: увеличено расстояние логотип→текст (60px → 90dp)
- **SplashLoadingActivity**: новый оверлей загрузки для логина/регистрации
- **Login/Register**: показывается SplashLoadingActivity во время авторизации
- **Онбординг удалён**: welcomeContainer, onboardingProfileBubble, onboardingFabBubble
- **Чекбокс "Создать чат"**: в шторке добавления контакта, включён по умолчанию
- **Исправления**: crash при выборе чатов, getSelectedChats offset, loadingContainer удалён, statusBarColor deprecation
- compileDebugKotlin ✅
- APK: /var/www/lavender/lavender.apk
- GitHub релиз: https://github.com/ferzferz11-sudo/msg.client.android/releases/tag/v1.1.2.7

### Сервер v1.1.2.7
- Без изменений (v1.1.2.4)

---

## ✅ v1.1.2.6 — ChangelogActivity: bundled changelog + ссылки на GitHub

### Android v1.1.2.6
- **Bundled changelog**: `app/src/main/assets/changelog_bundled.txt` — встроен в APK, показывается мгновенно
- **Новая логика загрузки**: bundled → GitHub API → server fallback
- **Ссылки на CHANGELOG.md**: кнопки «Ченджлог сервера» и «Ченджлог клиента» на GitHub
- **changelog.txt удалён** из проекта и из деплоя на сервер
- **scripts/deploy_android.sh обновлён**: убрана загрузка changelog.txt
- **Старый deploy_android.sh удалён** (сервер 159.195.38.145 больше не поддерживается)
- **Документация обновлена**: INDEX.md, PITFALLS.md, TASKS.md
- compileDebugKotlin ✅

### Сервер v1.1.2.6
- Без изменений (v1.1.2.4)

---

## ✅ v1.1.2.5 — ChangelogActivity тема

### 1. ChangelogActivity — белый экран при кастомных темах ✅ ИСПРАВЛЕНО
- **Симптом**: при тёмной/кастомной теме ChangelogActivity показывала белый экран
- **Причина**: ChangelogActivity не вызывал ThemeUi.bind для применения кастомной темы
- **Исправлено**: добавлен ThemeUi.bind(this, "") в onCreate
- **Статус**: исправлено, compileDebugKotlin OK

---

## ✅ v1.1.2.4 — Все проблемы закрыты

### 1. Hermes история не загружалась ✅ ИСПРАВЛЕНО
- **Симптом**: пользователь не видит историю чата с оркестратором
- **Причина**: HermesChatActivity вызывает старый chatWithOrchestrator RPC, который сохраняет в hermes_messages — но эта таблица дропнута в v1.1.2.3
- **Исправлено**: все вызовы переведены на AIChatManager (ai_chat_messages таблица)
- **Статус**: исправлено в v1.1.2.4

### 2. Rate limiter — failed requests потребляли слоты ✅ ИСПРАВЛЕНО
- **Симптом**: при ошибке OpenRouter/orchestrator слот rate limiter не возвращался, счётчик показывал меньше чем должно
- **Причина**: allow() добавлял timestamp до выполнения запроса, при ошибке timestamp оставался
- **Исправлено**: добавлен rateLimiter.cancel(userID) во всех failure paths (ChatWithOWL, ChatWithOrchestrator, ChatWithAI, ChatWithPipeline, /ai bot command)
- **Статус**: исправлено в v1.1.2.4

### 3. Avatar delete — новый файл удалялся при совпадении хеша ✅ ИСПРАВЛЕНО
- **Симптом**: при смене аватара полная версия не отображалась (бесконечный индикатор загрузки)
- **Причина**: UpdateAvatar удалял старый файл по имени, но если новый файл имел тот же MD5 хеш, он перезаписывал старый, и потом удалялся
- **Исправлено**: UpdateAvatar теперь сравнивает имена старых и новых файлов, и не удаляет новый если хеш совпадает
- **Статус**: исправлено в v1.1.2.4

---

## ✅ v1.1.2.3 — Подтверждённые работающие фичи

### OWL чат ✅
- История чата загружается корректно
- Счётчик запросов в тулбаре показывает правильно (20/20)
- Стриминг ответов работает

### Архитектура AI Chat Refactor ✅
- ai_chat_manager.go, ai_chat_sessions/messages/settings таблицы
- ChatWithAI RPC, GetAIChatHistory, GetAIChatSettings, UpdateAIChatSettings
- AiChatGrpc.kt + GrpcClient facade
- compileDebugKotlin passes
- Dev и prod обновлены

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

## 📋 Бэклог

### Высокий приоритет
- [ ] Favorites при пустом списке — не отображается при входе после очистки памяти (Android)

### Средний приоритет
- [ ] Модульные тесты для OWL streaming

### Низкий приоритет
- [ ] Auth токены для удалённых агентов (JWT)
- [ ] Qdrant + CLIP (production RAG)

---

## 🟡 Известные проблемы

### Favorites — отображение при пустом списке чатов (Android)
- **Статус:** не исправлено, v1.1.2.7
- **Симптом:** при входе после очистки памяти Favorites не отображается если нет созданных чатов
- **Приоритет:** высокий

### ChangelogAdapter — цвета на кастомных темах (Android)
- **Статус:** не исправлено, приоритет низкий
- Fallback (bundled) работает корректно

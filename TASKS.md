# Lavender Messenger — Известные проблемы и задачи в работе

**Последнее обновление:** 2026-07-15
**Ветка:** feat/1.1.0.x
**Версия:** 1.1.0.15 (dev)

---

## 🔧 В процессе

### 1. Повторная регистрация после удаления профиля
- **Статус:** частично исправлено, требует тестирования
- **Описание:** После удаления профиля повторная регистрация не работала («соединение не удалось»)
- **Причина:** `GrpcClient.connect()` видел что канал уже READY и не пересоздавал соединение; `CredentialStore.save()` использовал `apply()` (async) — новый ChatListActivity читал старый адрес
- **Что сделано:**
  - `SessionManager.login()` теперь вызывает `disconnect()` + `connect(forceReconnect = true)` перед логином
- **Файлы:** `SessionManager.kt`, `ChatListActivity.kt`

### 2. loadingContainer крутится бесконечно при ошибке
- **Статус:** ✅ исправлено (0327d25)
- **Причина:** В `catch (e: Exception)` блоке `loadChats()` не скрывался `loadingContainer`
- **Исправление:** Добавлено скрытие `loadingContainer` + показ `chatsRecyclerView` при ошибке
- **Файлы:** `ChatListActivity.kt:880-888`

### 3. Dev server connection при re-registration
- **Статус:** ✅ исправлено
- **Описание:** Сервер не отвечал на повторные запросы после удаления профиля
- **Исправление:** Перезапуск dev сервера с обновлённым кодом

---

## ✅ Исправлено в этой сессии

### 4. Пресеты агентов не отображались
- **Статус:** ✅ исправлено (60efcef)
- **Причина:** На сервере не было реализованы gRPC методы `ListAgentPresets`, `ListAgents`, `ListUserAgents`, `CreateAgent`, `UpdateAgent`, `DeleteAgent`, `GetOrchestratorHistory`
- **Что сделано:**
  - Добавлены proto-сообщения: `AgentPresetInfo`, `AgentInfo`, `ListAgentPresetsRequest/Response`, `ListAgentsRequest/Response`, `ListUserAgentsRequest/Response`, `CreateAgentRequest/Response`, `UpdateAgentRequest/Response`, `DeleteAgentRequest/Response`
  - Добавлены RPC в `ChatService`
  - Добавлен `Icon` field в `AgentDefinition`
  - Добавлены методы `GetPresets()`, `LoadCustomAgents()`, `getSession()` в `HermesAgentRegistry`
  - Все 7 методов реализованы в `server.go`
  - Проверено: `ListAgentPresets` возвращает 8 пресетов ✅
- **Файлы:** `messenger.proto`, `server.go`, `hermes_agents.go`, `hermes_orchestrator.go`

### 5. Избранное не появлялось без создания чата
- **Статус:** наблюдалось ранее; после создания первого чата — работает
- **Причина:** `loadingContainer` блокировал интерфейс при проблемах с загрузкой чатов

### 6. Favorites: сообщения не отображались (empty encrypted data)
- **Статус:** ✅ исправлено ранее (e0d5dd5)
- **Причина:** Дубликат `COALESCE(m.is_e2ee, false)` в SQL запросе `GetMessages`

### 7. Hermes: сессия не создавалась (column "mode")
- **Статус:** ✅ исправлено ранее
- **Причина:** Дубликат `mode` в миграции `db_hermes.go`

### 8. Hermes: proto field number mismatch
- **Статус:** ✅ исправлено ранее
- **Причина:** Сервер поля 15-17 (E2EE), парсил как Timestamp; клиент читал 18-22 как activeAgentId/agentMode
- **Исправление:** Серверные поля 20/21 для `active_agent_id`/`agent_mode`

### 9. Hermes → Lava AI rebrand
- **Статус:** ✅ сделано ранее (Android 681e9c0, Server 5b2372d)

### 10. Force reconnect убивал стримы
- **Статус:** ✅ исправлено (v1.1.0.15, коммит 1976d5c)
- **Причина:** `connect(force=true)` при живом канале (READY) вызывал `shutdownNow()`, стримы падали с `UNAVAILABLE: Channel shutdownNow invoked`
- **Исправление:** Единая проверка `if (addressMatch && channelAlive)` — если канал живой и адрес совпадает, force не трогает канал. Переподключение только при мёртвом канале или смене адреса.
- **Файлы:** `RealGrpcClient.kt`
- **Проверено:** пользователь подтвердил — ошибка ушла

### 11. OpenRouter 401 — "Missing Authentication header"
- **Статус:** ⏳ ключ обновлён, требуется тестирование
- **Описание:** При отправке сообщения в Hermes чат сервер возвращает 401 от OpenRouter
- **Что сделано:**
  - Пользователь вручную обновил OPENROUTER_API_KEY в `/root/LavenderMessenger/run/.env.dev`
  - Сервер перезапущен (daemon-reload + stop + start)
  - Python-тест к OpenRouter API вернул 200 с новым ключом
- **Требуется:** тестирование из приложения — отправить сообщение в Hermes чат
- **Если снова 401:** возможно сервер не перечитал EnvironmentFile, нужно проверить как owl.go получает ключ (os.Getenv vs godotenv)

---

## 📋 Бэклог

### Высокий приоритет
- [ ] Re-registration flow — полное тестирование (текущая задача пользователя)
- [ ] Favorites перестал работать с нуля (если чатов нет) — воспроизвести и исправить

### Средний приоритет
- [ ] Секретный чат — заглушка "not implemented in this build"
- [ ] OWL чат: поле ввода перекрывает кнопки навигации (adjustResize + edge-to-edge)
- [ ] OWL keepalive failed при длительном простое

### Низкий приоритет
- [ ] Mac session logout issue
- [ ] Кэширование OWL чатов в локальной БД
- [ ] Оптимизация списка моделей OWL
- [ ] Graceful reconnect при keepalive failed

---

## 🔑 Ключевые решения

| Решение | Обоснование |
|---------|-------------|
| OWL чаты хранятся в `chats` с `type='owl'` | Единая таблица, не нужна отдельная |
| Participants формат: `["username"]` JSON array | Совместимость с существующим парсером |
| `ThemeUi.bind()` для тем | Единообразие с остальным приложением |
| `CoroutineScope` вместо `lifecycleScope` для `loadHistory()` | Предотвращает отмену корутины при смене activity |
| Favorites: сообщения хранятся в messages с `room_id='favorites_*'` | Единая таблица сообщений |
| Hermes поля 20/21 в proto | Избегает конфликта с E2EE полями 15-17 |
| `forceReconnect = true` при логине | Гарантирует пересоздание канала при re-registration |
| `disconnect()` + `connect()` в SessionManager.login() | Сброс закэшированного READY канала |

---

## 🧪 Dev Testing Checklist (July 15, 2026)

**Тест:** ferz11, dev server 50052

| # | Тест | Статус |
|---|------|--------|
| 1 | Регистрация нового пользователя | ✅ Auth success |
| 2 | Список чатов загружается | ✅ |
| 3 | Избранное отображается | ✅ |
| 4 | Hermes чат виден в списке | ✅ |
| 5 | Открытие HermesChatActivity | ✅ |
| 6 | Force reconnect не убивает стримы | ✅ v1.1.0.15 |
| 7 | Отправка сообщения → ответ оркестратора | ❌ OpenRouter 401 |
| 8 | AgentListActivity — пресеты видны | ✅ 8 пресетов |
| 9 | Удаление профиля → повторная регистрация | ⏳ |
| 10 | login flow без сбоя | ⏳ |

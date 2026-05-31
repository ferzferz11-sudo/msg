# Lavender Messenger — Известные проблемы и задачи в работе

**Последнее обновление:** 2026-06-01
**Ветка:** feat/1.1.0.x
**Версия:** 1.1.0.4

---

## 🔧 В процессе

### 1. OWL чат: поле ввода перекрывает кнопки навигации
- **Статус:** частично исправлено, требует проверки на устройстве
- **Описание:** При открытии OWL чата поле ввода (bottomPanel) расположено поверх навигационных кнопок телефона. Клавиатура при открытии не сдвигает layout.
- **Что сделано:**
  - Добавлен `android:fitsSystemWindows="true"` → не помогло
  - Добавлен `adjustResize` в манифест → не помогло
  - Добавлен `adjustNothing` + ручная обработка insets → в процессе
- **Текущий подход:** `adjustResize` + `translationY` на bottomPanel при открытии клавиатки
- **Проблема:** `enableEdgeToEdge()` в `ThemeApplier.kt` конфликтует с `adjustResize`
- **Файлы:** `activity_owl.xml`, `OwlActivity.kt`, `AndroidManifest.xml`

### 2. OWL чат: keepalive failed при длительном простое
- **Статус:** наблюдается, не критично
- **Описание:** `UNAVAILABLE: Keepalive failed. The connection is gone` — gRPC канал теряется при длительном простое на мобильной сети
- **Сервер:** keepalive настроен (15s ping, 10s timeout)
- **Клиент:** автоматический reconnect в `onClose` RealGrpcClient
- **Файлы:** `RealGrpcClient.kt`

### 3. OWL чат: /key команда не работает при использовании серверного ключа
- **Статус:** исправлено
- **Описание:** Команда `/key` не показывалась в приветственном сообщении
- **Исправлено:** добавлена в `showWelcomeMessage()` и в `/help`
- **Файлы:** `OwlActivity.kt`

---

## ✅ Исправлено

### 1. Ветки переименованы
- `feat/1.2.0.owl` → `feat/1.1.0.x`
- Версия: 1.1.0.4

### 2. OWL чаты в списке после возврата
- Исправлен `CreateOwlChat` — резолвит username из DB
- Убран `creator == userId` check в `GetOwlHistory`
- Восстановлена колонка `last_message_text` в `chats`

### 3. Поле ввода в OWL чате
- Исправлен `textInputType` — убран невалидный `textUri`
- Добавлены потолстевшие импорты

### 4. Дублирующиеся сообщения при стриминге  OWL
- Заменён `addMessage` на `updateLastAssistantMessage` для стриминга
- Убран дубль `finished=true` из `onClose`

### 5. Темы в OWL чате
- Добавлен `ThemeUi.bind(this, userId)` в `onCreate`
- Layout приведён к виду `activity_new_chat.xml`

### 6. Удаление OWL чатов
- **Было:** `Failed to parse participants: invalid character 'e' in literal false`
- **Причина:** старый формат participants `[ferz]` вместо `["ferz"]`
- **Исправлено:** обновлены данные в БД

---

## 📋 Бэклог

### Высокий приоритет
- [ ] Секретный чат — заглушка "not implemented in this build"
- [ ] Медленная загрузка "Избранное" при переключении стрима

### Средний приоритет
- [ ] Mac session logout issue — не исследовано
- [ ] Кэширование OWL чатов в локальной БД

### Низкий приоритет
- [ ] Оптимизация списка моделей OWL (23 модели — можно кэшировать)
- [ ] Graceful reconnect при keepalive failed без потери сообщений

---

## 🔑 Ключевые решения

| Решение | Обоснование |
|---------|-------------|
| OWL чаты хранятся в `chats` с `type='owl'` | Единая таблица, не нужна отдельная |
| Participants формат: `["username"]` JSON array | Совместимость с существующим парсером |
| `ThemeUi.bind()` для тем | Единообразие с остальным приложением |
| `adjustResize` + `translationY` | Edge-to-edge конфликт с adjustResize |
| `CoroutineScope` вместо `lifecycleScope` для `loadHistory()` | Предотвращает отмену корутины при смене activity |

---

## 📁 Ключевые файлы

| Файл | Назначение |
|------|------------|
| `/root/msg/server.go` | Сервер, версия 1.1.0.4 |
| `/root/msg/server/owl.go` | OWL сессии и БД |
| `/root/msg/messenger.proto` | gRPC определения |
| `OwlActivity.kt` | OWL чат UI |
| `OwlGrpc.kt` | gRPC вызовы OWL |
| `RealGrpcClient.kt` | gRPC канал и reconnect |
| `GrpcClient.kt` | Фасад для gRPC |
| `ChatListActivity.kt` | Список чатов, `createNewOwlChat()` |
| `ThemeApplier.kt` | Применение тем, `enableEdgeToEdge()` |
| `activity_owl.xml` | Layout OWL чата |
| `AndroidManifest.xml` | `windowSoftInputMode` для OWL |

# Lavender Chat — Полноценная замена Telegram внутри Lavender Messenger

## Цель
Сделать так, чтобы весь рабочий общение (включая общение с OWL AI) происходило через Lavender, без необходимости переключаться на Telegram. Lavender становится единным мессенджером для всего.

## Текущее состояние

### Что уже есть в Lavender
- gRPC чат с real-time сообщениями (streaming)
- Групповые чаты, личные сообщения
- E2EE секретные чаты
- Голосовые звонки (WebRTC)
- Файлы, изображения, голосовые сообщения
- Реакции, редактирование, удаление
- Темы, настройки профиля
- OWL AI асинхронный чат (уже есть!)
- Hermes Orchestrator (уже есть!)
- Push уведомления (FCM)

### Что НЕ хватает для замены Telegram
1. **Быстрый старт чата с OWL** — нет отдельного UI для AI-чата прямо в списке чатов
2. **Нет команд (/help, /status)** — нет интеграции с AI через команды в любом чате
3. **Нет бота-ассистента** — нет возможности писать AI в личку как боту
4. **Нет быстрого доступа к серверу** — нельзя выполнить команду сервера через чат
5. **Нет интеграции с задачами** — нельзя создать/посмотреть задачи через чат
6. **Нет уведомлений о деплое** — сервер не может сам написать "деплой завершён"

---

## Архитектура решения

### Концепция: "OWL Bot" внутри Lavender

Пользователь создаёт чат с "OWL" как с обычным контактом. Сервер определяет что это AI-чат и подключает OWL к диалогу. Всё работает через существующий протокол `ChatWithOWL`.

### Компоненты

```
┌─────────────────────────────────────────────────────────┐
│                    Android Client                        │
│  ┌─────────────┐  ┌──────────────┐  ┌───────────────┐  │
│  │  Chat List   │  │  OWL Chat UI │  │  Bot Commands │  │
│  │  (existing)  │  │  (new)       │  │  (new)        │  │
│  └─────────────┘  └──────────────┘  └───────────────┘  │
└─────────────────────┬───────────────────────────────────┘
                      │ gRPC (existing ChatService)
┌─────────────────────┴───────────────────────────────────┐
│                   Lavender Server                        │
│  ┌─────────────┐  ┌──────────────┐  ┌───────────────┐  │
│  │ ChatService │  │ OWL Handler  │  │ Bot Command   │  │
│  │ (existing)  │  │ (existing)   │  │ Processor(new)│  │
│  └─────────────┘  └──────────────┘  └───────────────┘  │
│  ┌─────────────┐  ┌──────────────┐  ┌───────────────┐  │
│  │ Hermes      │  │ Notification │  │ Server Status │  │
│  │ Orchestrator│  │ Service (new)│  │ Reporter (new)│  │
│  └─────────────┘  └──────────────┘  └───────────────┘  │
└─────────────────────────────────────────────────────────┘
```

---

## Фаза 1: OWL Bot — базовый AI-чат (1-2 дня)

### 1.1 Сервер: улучшение OWL интеграции

**Файл:** `msg/messenger.proto` — уже есть `ChatWithOWL` RPC

**Что сделать:**
- Добавить поле `session_type` в `OWLRequest` — чтобы различать обычный чат и "командный" режим
- Добавить поле `context_messages` — передавать последние N сообщений для контекста
- Добавить RPC `GetOWLStatus` — проверка доступности AI

```protobuf
// Новые сообщения для OWL
message OWLStatusRequest {
  string user_id = 1;
}

message OWLStatusResponse {
  bool available = 1;
  string model = 2;
  int32 queue_length = 3;
  string status = 4; // "ready", "busy", "offline"
}

message OWLMessageContext {
  string role = 1; // "user", "assistant"
  string content = 2;
  string created_at = 3;
}

message OWLRequest {
  // ... существующие поля ...
  repeated OWLMessageContext context = 6; // контекст диалога
  string session_type = 7; // "chat", "command", "debug"
}
```

### 1.2 Сервер: Bot Command Processor

**Новый файл:** `msg/bot_commands.proto`

```protobuf
service BotService {
  rpc ProcessCommand(BotCommandRequest) returns (BotCommandResponse);
  rpc GetAvailableCommands(GetCommandsRequest) returns (GetCommandsResponse);
}

message BotCommandRequest {
  string user_id = 1;
  string chat_id = 2;
  string command = 3; // "/status", "/deploy", "/help"
  repeated string args = 4;
}

message BotCommandResponse {
  bool success = 1;
  string response_text = 2;
  bool is_error = 3;
  string error_message = 4;
}

message BotCommandInfo {
  string command = 1; // "/status"
  string description = 2;
  string usage = 3;
  string category = 4; // "server", "ai", "system"
}

message GetCommandsRequest {
  string user_id = 1;
}

message GetCommandsResponse {
  repeated BotCommandInfo commands = 1;
}
```

**Список команд:**
| Команда | Описание | Категория |
|---------|----------|-----------|
| `/status` | Статус сервера (CPU, RAM, uptime) | server |
| `/deploy dev` | Деплой на dev сервер | server |
| `/deploy prod` | Деплой на prod сервер | server |
| `/logs [N]` | Последние N логов сервера | server |
| `/restart dev` | Перезапуск dev сервера | server |
| `/ai <сообщение>` | Прямой запрос к OWL AI | ai |
| `/help` | Список всех команд | system |
| `/version` | Версия сервера и клиента | system |

### 1.3 Android: UI для OWL чата

**Что сделать:**
- Добавить кнопку "OWL AI" на главном экране (FAB или в баре)
- Создать `OwlChatActivity` — экран чата с AI
- Интегрировать с существующим `ChatService.ChatWithOWL`
- Показывать typing indicator пока AI думает
- Поддержка markdown в ответах AI

**Структура UI:**
```
MainActivity
├── ChatListFragment (existing)
├── ContactsFragment (existing)
├── OwlChatFragment (NEW)
│   ├── MessageList (RecyclerView)
│   ├── InputField + SendButton
│   ├── TypingIndicator
│   └── CommandSuggestions (auto-complete для /команд)
├── SettingsFragment (existing)
└── ServerStatusFragment (NEW — опционально)
```

### 1.4 Android: Bot Commands UI

**Что сделать:**
- При вводе `/` показывать выпадающий список команд
- Автодополнение команд с описанием
- Отправка команды на сервер через `BotService.ProcessCommand`
- Отображение ответа как обычное сообщение в чате

---

## Фаза 2: Серверные уведомления (1 день)

### 2.1 Сервер: Notification Service

**Новый компонент:** сервис который может отправлять сообщения в чат от имени сервеса.

**Файл:** `msg/server_notifications.proto`

```protobuf
message ServerNotification {
  string id = 1;
  string type = 2; // "deploy", "error", "info", "warning"
  string title = 3;
  string message = 4;
  string timestamp = 5;
  map<string, string> metadata = 6;
}

message SubscribeNotificationsRequest {
  string user_id = 1;
  repeated string types = 2; // фильтр по типам
}

service NotificationService {
  rpc Subscribe(SubscribeNotificationsRequest) returns (stream ServerNotification);
  rpc GetHistory(GetNotificationHistoryRequest) returns (GetNotificationHistoryResponse);
  rpc MarkRead(MarkNotificationReadRequest) returns (MarkNotificationReadResponse);
}
```

### 2.2 Сервер: интеграция с деплоем

**Что сделать:**
- После каждого `systemctl restart` отправлять уведомление в чат
- При ошибке — отправлять alert
- При успешном деплое — отправлять подтверждение

**Пример сообщения:**
```
🦞 Lavender Server
✅ Dev сервер перезапущен
Время: 19:33:24
Версия: v1.1.0.16
Uptime: 0m 5s
```

### 2.3 Android: отображение уведомлений

**Что сделать:**
- Показывать серверные уведомления в отдельном чате "Lavender Server"
- Или в том же OWL чате (как системные сообщения)
- Badge с количеством непрочитанных уведомлений

---

## Фаза 3: Интеграция с задачами и сервером (1-2 дня)

### 3.1 Сервер: Task Integration

**Что сделать:**
- Команда `/tasks` — показать текущие задачи (TODO из OWL)
- Команда `/task add <text>` — добавить задачу
- Команда `/task done <id>` — отметить выполненной
- Команда `/task cancel <id>` — отменить задачу

### 3.2 Сервер: Server Management

**Что сделать:**
- Команда `/status` — полный статус сервера
- Команда `/logs [N]` — последние N строк логов
- Команда `/restart dev|prod` — перезапуск сервера
- Команка `/update` — обновить и перезапустить

### 3.3 Android: быстрые действия

**Что сделать:**
- В OWL чате добавить кнопки быстрых действий:
  - 🔄 Перезапустить dev
  - 📊 Статус сервера
  - 📋 Логи
  - ✅ Задачи
- Кнопки отправляют соответствующие команды

---

## Фаза 4: Продвинутые функции (2-3 дня)

### 4.1 Сервер: AI в любом чате

**Концепция:** в любом групповом чате можно написать `@OWL <вопрос>` и AI ответит.

**Что сделать:**
- Парсер сообщений в `ChatService.Chat` — искать `@OWL` или `/ai`
- Перенаправлять запрос в OWL handler
- Ответ отправлять от имени "OWL Bot"

### 4.2 Сервер: контекстные ответы

**Что сделать:**
- AI видит последние N сообщений чата как контекст
- AI может отвечать на вопросы о проекте (читать файлы через Hermes)
- AI может выполнять команды на сервере (через Hermes Orchestrator)

### 4.3 Android: улучшенный UI

**Что сделать:**
- Отдельный раздел "AI Chats" в списке чатов
- История всех AI-сессий
- Возможность создать новую AI-сессию
- Настройки AI (модель, температура, system prompt)

---

## Фаза 5: Полировка (1-2 дня)

### 5.1 Сервер: rate limiting и безопасность

**Что сделать:**
- Rate limit на AI-запросы (max 10/мин на пользователя)
- Rate limit на команды (max 30/мин)
- Валидация всех входных данных
- Логирование всех команд

### 5.2 Android: UX улучшения

**Что сделать:**
- Pull-to-refresh в чате
- Infinite scroll для истории
- Поиск по сообщениям
- Копирование сообщения
- Ответ на сообщение (reply)
- Форматирование кода в ответах AI

### 5.3 Сервер: мониторинг

**Что сделать:**
- Метрики: количество AI-запросов, ошибок, latency
- Health check endpoint
- Graceful shutdown

---

## Технические детали

### Структура файлов сервера

```
lavender-server/
├── cmd/
│   └── server/
│       └── main.go
├── internal/
│   ├── chat/
│   │   ├── handler.go
│   │   ├── service.go
│   │   └── types.go
│   ├── owl/
│   │   ├── handler.go
│   │   ├── service.go
│   │   └── client.go
│   ├── bot/
│   │   ├── handler.go
│   │   ├── commands.go
│   │   └── processor.go
│   ├── notification/
│   │   ├── service.go
│   │   └── sender.go
│   └── servermgmt/
│       ├── status.go
│       ├── deploy.go
│       └── logs.go
├── proto/
│   ├── messenger.proto
│   ├── bot_commands.proto (NEW)
│   └── server_notifications.proto (NEW)
└── gen/
    └── ...
```

### Структура файлов Android

```
app/src/main/java/lavender/client/
├── ui/
│   ├── chat/
│   │   ├── ChatActivity.kt
│   │   ├── ChatViewModel.kt
│   │   └── ChatAdapter.kt
│   ├── owl/
│   │   ├── OwlChatFragment.kt (NEW)
│   │   ├── OwlChatViewModel.kt (NEW)
│   │   └── OwlChatAdapter.kt (NEW)
│   ├── bot/
│   │   ├── CommandProcessor.kt (NEW)
│   │   └── CommandAdapter.kt (NEW)
│   └── notification/
│       ├── NotificationFragment.kt (NEW)
│       └── NotificationAdapter.kt (NEW)
├── data/
│   ├── grpc/
│   │   ├── ChatServiceClient.kt
│   │   ├── OwlServiceClient.kt (NEW)
│   │   ├── BotServiceClient.kt (NEW)
│   │   └── NotificationServiceClient.kt (NEW)
│   └── repository/
│       └── ...
└── service/
    └── ...
```

### Приоритеты реализации

| Фаза | Приоритет | Время | Зависимости |
|------|-----------|-------|-------------|
| 1.1 — OWL proto update | 🔴 High | 0.5 дня | — |
| 1.2 — Bot Commands proto | 🔴 High | 0.5 дня | — |
| 1.3 — OWL Chat UI | 🔴 High | 1 день | 1.1 |
| 1.4 — Bot Commands UI | 🟡 Medium | 0.5 дня | 1.2 |
| 2.1 — Notification Service | 🟡 Medium | 0.5 дня | — |
| 2.2 — Deploy integration | 🟡 Medium | 0.5 дня | 2.1 |
| 3.1 — Task Integration | 🟢 Low | 1 день | 1.2 |
| 3.2 — Server Management | 🟢 Low | 1 день | 1.2 |
| 4.1 — AI in any chat | 🟢 Low | 1 день | 1.1, 1.2 |
| 4.2 — Context awareness | 🟢 Low | 1 день | 4.1 |
| 5.1 — Rate limiting | 🟡 Medium | 0.5 дня | 1.2 |
| 5.2 — UX polish | 🟢 Low | 1 день | все |

---

## Критерии готовности

- [ ] Пользователь может открыть чат с OWL из приложения
- [ ] Пользователь может писать команды (/status, /deploy, /help)
- [ ] Сервер отправляет уведомления о деплое
- [ ] Сервер отправляет уведомления об ошибках
- [ ] Работает rate limiting
- [ ] История AI-чатов сохраняется
- [ ] Можно создать новую AI-сессию
- [ ] Команды работают в групповых чатах через @OWL

---

## Риски и митигация

| Риск | Вероятность | Импакт | Митигация |
|------|-------------|--------|-----------|
| OWL AI не отвечает | Medium | High | Retry + fallback message |
| gRPC timeout | Medium | Medium | Увеличить timeout до 30s |
| Rate limit блокирует | Low | Medium | Настраиваемые лимиты |
| Контекст переполняет память | Low | High | Лимит 20 сообщений контекста |
| Сервер падает при деплое | Medium | High | Graceful restart + health check |

---

## Следующие шаги

1. **Обсудить и утвердить план** — внести правки
2. **Начать с Фазы 1** — самое важное и быстрое
3. **Протестировать на dev** — деплой, тест, фикс
4. **Итерации** — по 0.5-1 день на фазу

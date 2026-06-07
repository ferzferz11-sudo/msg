# Интеграция Lavender Chat — Новая сессия

## Контекст

Мы работаем над интеграцией полноценного чата в Lavender Messenger, чтобы заменить Telegram для общения с OWL AI и управления сервером.

**Текущая ветка:** `feat/1.1.1.x` (и на сервере, и на Android)
**Предыдущая ветка:** `feat/1.1.0.x` (удалена после merge в main)

## Что уже сделано

### Android (v1.1.1.1)
- ✅ Тег v1.1.0.16 создан и запушен
- ✅ Favorites вынесен из серверного GetChats (клиент сам создаёт placeholder)
- ✅ Favorites добавлен в начало списка чатов как статический элемент
- ✅ Bot Commands UI: OwlChatActivity + OwlChatViewModel созданы
- ✅ Бот-команды gRPC: processBotCommand, getBotCommands, getOWLStatus добавлены
- ✅ OWL AI кнопка добавлена в bottom sheet меню ChatListActivity
- ✅ Slash command detection в поле ввода (при вводе /)
- ✅ compileDebugKotlin проходит успешно
- ✅ Все изменения закоммичены и запушены в feat/1.1.1.x

### Сервер
- ✅ Ветка feat/1.1.1.x создана от main
- ✅ Dev сервер собран и работает (порт 50052)
- ✅ Удалена серверная инъекция Favorites из GetChats
- ✅ Bot Commands proto добавлены (BotCommandRequest/Response/Info, GetBotCommands)
- ✅ OWLStatus proto добавлены
- ✅ ServerNotification proto добавлены (Subscribe, GetHistory, MarkRead)
- ✅ bot_commands.go создан с обработчиками: /status, /deploy, /logs, /restart, /ai, /help, /version
- ✅ Rate limiting: 30 cmd/min для бота, 10 req/min для AI
- ✅ Notification service с broadcast и history
- ✅ Bot command detection в Chat stream (messages starting with /)
- ✅ Все изменения закоммичены и запушены в feat/1.1.1.x
- ✅ Версия сервера обновлена до 1.1.1.1

## Что нужно сделать (Фаза 1 из LAVENDER_CHAT_PROJECT.md)

### 1.1 Bot Commands (сервер) ✅ СДЕЛАНО
- Добавить обработку команд в любом чате: `/status`, `/deploy`, `/help`, `/logs`
- Команды работают через существующий gRPC протокол
- Rate limiting на команды (max 30/мин)

### 1.2 OWL Bot UI (Android) ✅ СДЕЛАНО
- Кнопка "OWL AI" на главном экране
- Отдельный экран чата с AI (OwlChatActivity)
- Typing indicator пока AI думает

### 1.3 Bot Commands UI (Android) ✅ СДЕЛАНО
- При вводе `/` показывается выпадающий список команд
- Отправка команды на сервер через ProcessBotCommand

### 1.4 Серверные уведомления ✅ СДЕЛАНО
- NotificationService с broadcast и history
- SubscribeNotifications gRPC для real-time уведомлений
- SendServerNotification хелпер для отправки из кода

## Правила работы

1. **Коммитить после каждого значимого изменения**
2. **Пушить в ветку feat/1.1.1.x** (не в main!)
3. **Деплоить на dev сервер для тестирования**
4. **Обновлять CHANGELOG.md** с каждым фиксом
5. **Не ломать существующий функционал**

## Чек-лист для проверки утром

### Сервер
- [x] Dev сервер запускается без ошибок
- [x] gRPC соединение работает
- [ ] OWL AI отвечает на сообщения (нужно тестировать)
- [x] Hermes сессии работают
- [x] Favorites не дублируется в списке чатов
- [ ] Команды `/status`, `/help` работают в чате (нужно тестировать)

### Android
- [x] Приложение собирается без ошибок (compileDebugKotlin passes)
- [ ] Авторизация работает (нужно тестировать)
- [ ] Список чатов загружается (нужно тестировать)
- [ ] OWL AI чат открывается и работает (нужно тестировать)
- [ ] Hermes чаты работают (нужно тестировать)

### Общее
- [x] Нет ошибок компиляции
- [x] Все изменения закоммичены и запушены

## Файлы для работы

### Сервер
- `/root/msg/server.go` — основной gRPC сервер
- `/root/msg/owl.go` — OWL AI интеграция
- `/root/msg/hermes_orchestrator.go` — Hermes оркестратор
- `/root/msg/messenger.proto` — protobuf определения

### Android
- `/root/msg.client.android/app/src/main/java/lavender/client/android/ChatListActivity.kt`
- `/root/msg.client.android/app/src/main/java/lavender/client/android/ui/adapter/ChatAdapter.kt`
- `/root/msg.client.android/app/src/main/java/lavender/client/android/ui/hermes/HermesChatActivity.kt`
- `/root/msg.client.android/app/src/main/res/layout/activity_chat_list.xml`

## Команды

```bash
# Сервер
cd /root/msg
export PATH=$PATH:/usr/local/go/bin:~/go/bin
go build -o /tmp/lavender-server-dev .
systemctl stop lavender-server-dev
cp /tmp/lavender-server-dev /root/LavenderMessenger/run/lavender-server-dev
systemctl start lavender-server-dev

# Android
cd /root/msg.client.android
./gradlew compileDebugKotlin
./gradlew assembleDebug
```

## Важно

- **НЕ использовать --go_out=. при proto gen** (генерирует в корень, ломает сборку)
- **go PATH:** `export PATH=$PATH:/usr/local/go/bin:~/go/bin`
- **Proto gen:** `cd /root/msg && protoc --go_out=./gen --go_opt=paths=source_relative --go-grpc_out=./gen --go-grpc_opt=paths=source_relative messenger.proto`

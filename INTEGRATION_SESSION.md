# Интеграция Lavender Chat — Новая сессия

## Контекст

Мы работаем над интеграцией полноценного чата в Lavender Messenger, чтобы заменить Telegram для общения с OWL AI и управления сервером.

**Текущая ветка:** `feat/1.1.1.x` (и на сервере, и на Android)
**Предыдущая ветка:** `feat/1.1.0.x` (удалена после merge в main)

## Что уже сделано

### Android (v1.1.0.16)
- ✅ Тег v1.1.0.16 создан и запушен
- ✅ Favorites вынесен из серверного GetChats (клиент сам создаёт placeholder)
- ✅ Favorites добавлен в начало списка чатов как статический элемент
- ✅ Коммит bc36f7e: extract Favorites from RecyclerView into static view
- ⚠️ Favorites всё ещё мигает при обновлениях (известный баг в TASKS.md)

### Сервер
- ✅ Ветка feat/1.1.1.x создана от main
- ✅ Merge main → feat/1.1.0.x → main выполнен
- ✅ Dev сервер собран и работает
- ✅ Удалена серверная инъекция Favorites из GetChats

## Что нужно сделать (Фаза 1 из LAVENDER_CHAT_PROJECT.md)

### 1.1 Bot Commands (сервер)
- Добавить обработку команд в любом чате: `/status`, `/deploy`, `/help`, `/logs`
- Команды должны работать через существующий gRPC протокол
- Rate limiting на команды (max 30/мин)

### 1.2 OWL Bot UI (Android)
- Кнопка "OWL AI" на главном экране
- Отдельный экран чата с AI (OwlChatFragment)
- Интеграция с существующим ChatService.ChatWithOWL
- Typing indicator пока AI думает
- Поддержка markdown в ответах

### 1.3 Bot Commands UI (Android)
- При вводе `/` показывать выпадающий список команд
- Автодополнение команд с описанием
- Отправка команды на сервер

### 1.4 Серверные уведомления
- Сервер отправляет уведомления о деплое/ошибках в чат
- Push уведомления о событиях сервера

## Правила работы

1. **Коммитить после каждого значимого изменения**
2. **Пушить в ветку feat/1.1.1.x** (не в main!)
3. **Деплоить на dev сервер для тестирования**
4. **Обновлять CHANGELOG.md** с каждым фиксом
5. **Не ломать существующий функционал**

## Чек-лист для проверки утром

### Сервер
- [ ] Dev сервер запускается без ошибок
- [ ] gRPC соединение работает
- [ ] OWL AI отвечает на сообщения
- [ ] Hermes сессии работают
- [ ] Favorites не дублируется в списке чатов
- [ ] Команды `/status`, `/help` работают в чате

### Android
- [ ] Приложение собирается без ошибок
- [ ] Авторизация работает
- [ ] Список чатов загружается
- [ ] Favorites виден в списке чатов
- [ ] Клик по Favorites открывает избранное
- [ ] OWL AI чат открывается и работает
- [ ] Hermes чаты работают
- [ ] Звонки работают
- [ ] Секретные чаты работают

### Общее
- [ ] Нет крашей в логах сервера
- [ ] Нет ошибок в Android Studio
- [ ] Все изменения закоммичены и запушены

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

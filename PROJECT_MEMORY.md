# Lavender Messenger — Project Memory
# Created: 2026-05-28

## Структура проекта

### Репозитории
- **Сервер**: `/Users/paveld/GolandProjects/LavenderMessenger` (git: `ferzferz11-sudo/msg`)
- **Android клиент**: `/Users/paveld/GolandProjects/LavenderMessenger/client/android` (git: `ferzferz11-sudo/msg.client.android`)
- Это ОТДЕЛЬНЫЕ git-репозитории, не субмодули

### Ветки
- Сервер: `main`
- Клиент: `master`

### Файлы сервера
- `server.go` — gRPC хендлеры (2704 строк)
- `secret_chat.go` — E2EE handlers (вынесены в 1.0.7.1)
- `db.go` — SQL запросы и миграции
- `hub.go` — менеджер WebSocket-подобных gRPC подключений
- `main.go` — точка входа, gRPC + HTTP серверы
- `http_server.go` — HTTP для файлов (порты 8081, 8082)
- `crypto.go` — AES-256-GCM + bcrypt
- `messenger.proto` — protobuf определения
- `gen/` — автогенерированный Go-код из proto

### Файлы клиента (Android)
- `data/grpc/RealGrpcClient.kt` — gRPC клиент (~3000 строк)
- `data/grpc/GrpcClient.kt` — фасад для RealGrpcClient
- `data/session/CredentialStore.kt` — EncryptedSharedPreferences
- `data/session/SessionManager.kt` — управление сессией
- `data/crypto/E2EEManager.kt` — E2EE (ECDH + AES-256-GCM)
- `data/db/` — Room БД (AppDatabase, Daos, Entities)
- `ui/chat/ChatViewModel.kt` — ViewModel чата
- `ui/viewmodel/ChatListViewModel.kt` — ViewModel списка чатов
- `theme/` — система тем (ThemeStore, ThemeRepository)

### Версионирование
- Android: `version.txt` в корне (1.0.7.1)
- Сервер: `const ServerVersion = "1.0.7.1"` в server.go:30
- versionCode = major*1000000 + minor*10000 + patch*100 + build

## Технический стек

### Сервер
- Go 1.26, gRPC, PostgreSQL, Firebase Cloud Messaging
- AES-256-GCM шифрование, bcrypt для паролей
- keepalive: 15s ping, 10s timeout

### Клиент
- Kotlin, gRPC, Room, Firebase, Glide, ExoPlayer, WebRTC
- minSdk 29, targetSdk 35, compileSdk 37
- Material Design 3

## Архитектурные решения

### E2EE Secret Chats
- ECDH (secp256r1) для обмена ключами
- AES-256-GCM для шифрования сообщений
- Сервер НЕ может расшифровать E2EE сообщения
- `secret_chat_keys` таблица в БД
- Push для секретных чатов: "New encrypted message"

### Credential Storage
- Пароли хранятся в EncryptedSharedPreferences (AndroidX Security)
- Автоматическая миграция из plaintext prefs при первом запуске

### Обратная совместимость
- Новые proto поля — optional (proto3)
- Новые RPC — старые клиенты просто не вызывают
- `.gitignore`: client/macos/, client/console/, client/android/ исключены

## Деплой

### Сервер
- `./deploy.sh` — запуск на удалённой машине
- Порт gRPC: 50051
- HTTP серверы: 8081 (APK), 8082 (uploads)
- GitHub Actions: `.github/workflows/go.yml`

### Клиент
- `./deploy_android.sh` — сборка APK + rsync на сервер
- APK: `http://159.195.38.145:8081/lavender.apk`
- Signing: release.keystore (password: lavender123)

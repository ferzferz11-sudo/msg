# Lavender Messenger — Project Memory
# Created: 2026-05-28
# Updated: 2026-05-29 — split into two repos

## Репозитории

### Сервер
- **Git:** `ferzferz11-sudo/msg`
- **Dev сервер:** `13.140.25.249`, путь `/root/msg`
- **Production:** `159.195.38.145`, путь `/root/LavenderMessenger/run`
- **Ветка:** `main` (prod), `feat/remove-username-compat` (dev)
- **PostgreSQL:** user `lavender`, database `chat_db`
- **systemd:** `lavender-server.service`

### Клиент (Android)
- **Git:** `ferzferz11-sudo/msg.client.android`
- **Dev сервер:** `/root/msg.client.android` на `13.140.25.249`
- **Ветка:** `master`

## Сервер — ключевые файлы

- `main.go` — gRPC + HTTP серверы, точка входа
- `server.go` — основные gRPC хендлеры (~2704 строк)
- `secret_chat.go` — E2EE хендлеры
- `server_management.go` — мультисерверность, админ RPC
- `db.go` — PostgreSQL, миграции
- `hub.go` — менеджер подключений
- `http_server.go` — HTTP (8081 APK, 8082 uploads)
- `crypto.go` — AES-256-GCM + bcrypt
- `email.go` — email уведомления

## Клиент — ключевые файлы

- `RealGrpcClient.kt` — gRPC (~3000 строк, protobuf-lite ручной парсинг)
- `GrpcClient.kt` — фасад
- `CredentialStore.kt` — EncryptedSharedPreferences (server_address, key)
- `SessionManager.kt` — StateFlow<UserSession>
- `E2EEManager.kt` — ECDH + AES-256-GCM
- `data/db/` — Room (AppDatabase, Daos, Entities)
- `ChatViewModel.kt` / `ChatListViewModel.kt` — MVVM
- `theme/` — система тем

## Версионирование

- Android: `version.txt` в корне (1.0.7.1)
- Сервер: `const ServerVersion = "1.0.7.1"` в server.go
- versionCode = major*1000000 + minor*10000 + patch*100 + build

## Технический стек

### Сервер
- Go 1.26, gRPC, PostgreSQL, Firebase Cloud Messaging
- AES-256-GCM, bcrypt, keepalive 15s/10s
- systemd сервис, .env конфигурация

### Клиент
- Kotlin, gRPC (protobuf-lite manual), Room, Firebase, WebRTC
- minSdk 29, compileSdk 37, targetSdk 35
- MVVM + StateFlow + ViewBinding
- Material Design 3

## Архитектурные решения

### E2EE Secret Chats
- ECDH (secp256r1) обмен ключами
- AES-256-GCM шифрование
- Сервер НЕ может расшифровать

### Credential Storage
- EncryptedSharedPreferences (AndroidX Security)
- Авто-миграция из plaintext

### Мультисерверность
- `ListServers` RPC (публичный)
- Admin методы используют `AdminAuth`
- Android: CredentialStore хранит server_address

### Обратная совместимость
- Новые proto поля — optional (proto3)
- Новые RPC — старые клиенты не вызывают

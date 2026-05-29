# Lavender Messenger — Skills & Commands
# Created: 2026-05-28
# Updated: 2026-05-29 — split into two repos

## Репозитории (ОТДЕЛЬНЫЕ)

### Сервер
- **Удалённый (dev):** `/root/msg` на `13.140.25.249`
- **Локальный:** `/Users/paveld/GolandProjects/LavenderMessenger` (Mac)
- **Git:** `ferzferz11-sudo/msg`
- **Ветки:** `main` (production), `feat/remove-username-compat` (новый сервер)

### Android клиент
- **Удалённый (build):** `/root/msg.client.android` на `13.140.25.249`
- **Локальный:** `/Users/paveld/GolandProjects/LavenderMessenger/client/android` (Mac)
- **Git:** `ferzferz11-sudo/msg.client.android`
- **Ветка:** `master`

## Git команды

### Сервер
```bash
cd /root/msg                # или /Users/paveld/GolandProjects/LavenderMessenger
git status
git add -A
git commit -m "message"
git push
```

### Клиент
```bash
cd /root/msg.client.android  # или /Users/paveld/GolandProjects/LavenderMessenger/client/android
git status
git add -A
git commit -m "message"
git push
```

## Сборка и тестирование

### Сервер
```bash
cd /root/msg
go build -o /dev/null . && echo "build OK"
go vet ./... && echo "vet OK"
go test . && echo "test OK"
```

### Клиент
```bash
cd /root/msg.client.android
./gradlew assembleRelease
./gradlew clean assembleRelease  # чистая сборка
```

## Генерация proto

```bash
cd /root/msg
export PATH=$PATH:/root/go/bin
protoc --go_out=gen --go_opt=paths=source_relative \
       --go-grpc_out=gen --go-grpc_opt=paths=source_relative \
       messenger.proto server.proto
```

**ОБЯЗАТЕЛЬНО:** после каждого изменения .proto — перегенерировать ОБА файла!

## Деплой

- Сервер: Павел через `./deploy.sh`
- Клиент: Павел через `./deploy_android.sh`
- APK: `http://159.195.38.145:8081/lavender.apk`

## Credentials

### Сервер
- `.env` — DATABASE_URL, CHAT_SECRET_KEY, FIREBASE_CREDENTIALS_PATH
- CHAT_SECRET_KEY: ровно 32 символа

### Клиент
- `release.keystore` — password "lavender123", keyAlias "lavender"
- НЕ коммитить: .env, .keystore, google-services.json

## Android build конфигурация

- `version.txt` в корне репо (e.g., "1.0.7.1")
- versionCode = major*1000000 + minor*10000 + patch*100 + build
- compileSdk 37, minSdk 29, targetSdk 35
- ProGuard: ОТКЛЮЧЁН

## Архитектура

### Сервер (Go)
- main.go + хендлеры по файлам (server.go, secret_chat.go, server_management.go)
- hub.go управляет подключениями
- db.go — миграции + запросы
- Мультисерверность: ListServers, AdminAuth

### Клиент (Kotlin)
- MVVM: Activities → ViewModels → GrpcClient → RealGrpcClient
- RealGrpcClient — object (singleton)
- CredentialStore — EncryptedSharedPreferences
- Мультисерверность: список серверов в CredentialStore

## gRPC
- Bidirectional streaming: Chat, Typing, CallSession
- Unary: все остальные RPC
- Keepalive: 15s interval, 10s timeout

## Серверы

- Dev: `13.140.25.249:50051`
- Production (old): `159.195.38.145:50051`
- Local: `192.168.1.135:50051`

## Важно

1. Сервер и клиент — ОТДЕЛЬНЫЕ git репозитории!
2. Павел ВСЕГДА делает деплой сам
3. Proto регенерировать после каждого изменения .proto

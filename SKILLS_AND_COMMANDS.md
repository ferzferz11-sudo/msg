# Lavender Messenger — Skills & Commands
# Created: 2026-05-28

## Пути

### Сервер (удалённый)
- Директория: `/root/msg`
- IP: `13.140.25.249`
- Ветка: `main` (production), `feat/remove-username-compat` (новый сервер)

### Сервер (локальный у Павла)
- Директория: `/Users/paveld/GolandProjects/LavenderMessenger`

### Клиент Android (локальный у Павла)
- Директория: `/Users/paveld/GolandProjects/LavenderMessenger/client/android`
- Отдельный git-репозиторий: `ferzferz11-sudo/msg.client.android`

## Git команды

### Сервер (удалённый)
```bash
cd /root/msg
git status
git add -A
git commit -m "message"
git push
```

### Сервер (локальный)
```bash
cd /Users/paveld/GolandProjects/LavenderMessenger
git status
git add -A
git commit -m "message"
git push
```

### Клиент
```bash
cd /Users/paveld/GolandProjects/LavenderMessenger/client/android
git status
git add -A
git commit -m "message"
git push
```

### Важно: директория
- Всегда проверяй `pwd` перед git командами!
- Сервер и клиент — ОТДЕЛЬНЫЕ репозитории
- `client/android/` находится ВНУТРИ серверной директории локально, но это отдельный git

## Сборка и тестирование

### Сервер
```bash
cd /root/msg                    # или локально
go build -o /dev/null . && echo "build OK"
go vet ./... && echo "vet OK"
go test . && echo "test OK"
```

### Клиент
```bash
cd /Users/paveld/GolandProjects/LavenderMessenger/client/android
./gradlew assembleRelease
./gradlew clean assembleRelease  # чистая сборка
```

### Генерация proto
```bash
cd /root/msg                    # или локально
protoc --go_out=gen --go_opt=paths=source_relative --go-grpc_out=gen --go-grpc_opt=paths=source_relative messenger.proto
```

## Деплой

### Сервер
- Павел делает сам через `./deploy.sh`

### Клиент
- Павел делает сам через `./deploy_android.sh`
- APK: `http://159.195.38.145:8081/lavender.apk`

## Приватные ключи и credentials

### Сервер
- `.env` файл: DATABASE_URL, CHAT_SECRET_KEY, FIREBASE_CREDENTIALS_PATH
- DATABASE_URL: Supabase pooler (порт 6543) или localhost:5432
- CHAT_SECRET_KEY: ровно 32 символа

### Клиент
- release.keystore: password "lavender123", keyAlias "lavender"
- НЕ коммитить .env, .keystore, google-services.json

## Android build конфигурация

### Версия
- Файл: `version.txt` (напр. "1.0.7.1")
- versionCode = major*1000000 + minor*10000 + patch*100 + build
- Build type: compileSdk 37, minSdk 29, targetSdk 35
- ProGuard: ОТКЛЮЧЁН (isMinifyEnabled = false)

### Зависимости
- gRPC: grpc-okhttp, grpc-protobuf-lite, grpc-stub
- Room: room-runtime, room-ktx, room-compiler (KSP)
- Security: security-crypto (EncryptedSharedPreferences)
- Firebase: firebase-messaging (BOM 34.13.0)
- WebRTC: io.github.webrtc-sdk
- WorkManager: work-runtime-ktx (2.11.2)

## Архитектурные паттерны

### Сервер (Go)
- Один main.go, хендлеры по файлам (server.go, secret_chat.go)
- DB methods в db.go
- Hub в hub.go (управление подключениями)
- Миграции на лету в db.go ConnectDB()

### Клиент (Kotlin)
- MVVM: Activities → ViewModels → GrpcClient → RealGrpcClient
- RealGrpcClient — object (singleton), содержит gRPC логику
- GrpcClient — фасад, пробрасывает вызовы
- CredentialStore — EncryptedSharedPreferences
- SessionManager — StateFlow<UserSession>

### gRPC
- Bidirectional streaming: Chat, Typing, CallSession
- Unary: все остальные RPC
- Keepalive: 15s interval, 10s timeout

## Совместимость

### .gitignore (сервер)
- client/android/ — отдельный репозиторий
- client/macos/ — Fyne client, не нужен серверу
- client/console/ — console client, не нужен серверу
- server собирается из корня: `go build .`

### Обратная совместимость клиентов
- Новые proto fields: optional (proto3 default)
- Новые RPC: старые клиенты не вызывают
- Никаких breaking changes в существующих сообщениях

## GitHub Actions

### Pipeline (go.yml)
- Trigger: push/PR to main
- Steps: setup-go 1.26 → build . → test . → vet .
- НЕТ X11/OpenGL зависимостей (убрано в 1.0.7.1)

## Полезные команды

### Проверка что всё работает
```bash
# Сервер
cd /root/msg
go build . && go vet . && go test .

# Клиент
cd /Users/paveld/GolandProjects/LavenderMessenger/client/android
./gradlew assembleRelease
```

### Проверка git status
```bash
cd /root/msg && git status
cd /Users/paveld/GolandProjects/LavenderMessenger/client/android && git status
```

### Proto regeneration (после изменения messenger.proto)
```bash
protoc --go_out=gen --go_opt=paths=source_relative --go-grpc_out=gen --go-grpc_opt=paths=source_relative messenger.proto
```

## Контакты и URL

### Серверы
- Production (старый): `159.195.38.145:50051`
- New (текущий): `13.140.25.249:50051`
- Local: `192.168.1.135:50051`

### APK download
- `http://159.195.38.145:8081/lavender.apk`

### Сервер HTTP порты
- 8081: APK updates (StartAPKServer)
- 8082: File uploads (StartHTTPServer)

## Важные заметки

1. Сервер и клиент — ОТДЕЛЬНЫЕ git репозитории!
2. Павел ВСЕГДА делает деплой сам
3. Версия сервера в server.go: `const ServerVersion = "X.X.X.X"`
4. Версия клиента в version.txt
5. Proto регенерировать после каждого изменения messenger.proto
6. Ветка `feat/remove-username-compat` — для нового сервера 13.140.25.249, без обратной совместимости username→user_id

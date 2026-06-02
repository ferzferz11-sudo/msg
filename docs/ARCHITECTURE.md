# Lavender Messenger — Архитектурный анализ

**Дата:** 2026-06-01
**Автор:** OWL (автоматический анализ)
**Версия сервера:** 1.1.0.7 (server.go)
**Версия клиента:** 1.1.0.8 (TASKS.md)

---

## 1. Общая архитектура

```
┌─────────────┐     gRPC (bidirectional)    ┌──────────────┐
│   Android   │◄────────────────────────────►│              │
│   Client    │     gRPC (unary)             │   Go Server  │
│  (Kotlin)   │◄────────────────────────────►│   (main.go)  │
└─────────────┘                              └──────┬───────┘
                                                    │
┌─────────────┐     gRPC (bidirectional)           │
│    iOS      │◄──────────────────────────────────►│
│   Client    │                                    │
│   (Swift)   │                              ┌─────┴─────┐
└─────────────┘                              │ PostgreSQL │
                                             │   (DB)     │
┌─────────────┐                              └───────────┘
│   macOS     │     gRPC
│   Client    │◄────────────────────────────►│
│   (Swift)   │                              │
└─────────────┘                    ┌─────────┴─────────┐
                                   │       FCM         │
                                   │ (Firebase Cloud   │
                                   │   Messaging)      │
                                   └───────────────────┘
```

### Стек сервера:
- **Язык:** Go
- **Фреймворк:** gRPC (grpc-go)
- **База данных:** PostgreSQL (github.com/lib/pq)
- **Файл-сервер:** net/http (порт 8082)
- **Push:** Firebase Admin SDK (firebase.google.com/go/v4)
- **AI:** OpenRouter API (owl.go)

### Стек клиентов:
- **Android:** Kotlin, gRPC, WebRTC, Room DB, FCM
- **iOS:** Swift, gRPC-Web, SwiftUI
- **macOS:** Swift, AppKit

---

## 2. Ключевые компоненты сервера

### 2.1 Hub (hub.go) — менеджер соединений
- Хранит `map[stream]username` для chat, typing и call стримов
- Безопасность: `sync.RWMutex` для конкурентного доступа
- Broadcast: room-based для сообщений, user-based для call сигналов
- Конференции: in-memory структура `Conference` с участниками и приглашёнными

### 2.2 Server (server.go) — основная логика
- **Chat()**: bidirectional streaming для сообщений (основной метод)
- **CallSession()**: bidirectional streaming для WebRTC сигналов (OFFER/ANSWER/ICE)
- **Typing()**: streaming для индикатора набора текста
- **GetHistory()**: unary RPC для загрузки истории
- **CRUD**: чаты, пользователи, сообщения, устройства, контакты

### 2.3 FCM (server.go, sendPushNotification, sendCallPushNotification)
- Использует Firebase Admin SDK
- Push для сообщений: Notification + Data payload
- Push для звонков: только Data payload (type=VOIP_CALL), Android high priority
- Muted chats: проверка перед отправкой

### 2.4 OWL (owl.go) — AI ассистент
- OpenRouter API для LLM
- Rate limiter: 10 запросов/минуту на пользователя
- DB-backed сессии (таблица owl_messages)
- Per-chat настройки API key и model

### 2.5 Crypto (crypto.go)
- AES-256-GCM для шифрования сообщений на сервере
- bcrypt для паролей
- E2EE: ECDH key exchange для секретных чатов (client-side)

### 2.6 HTTP Server (http_server.go)
- Порт 8082
- Upload: avatars, images, files, backgrounds, audio (10MB max)
- Serve: статика из ./uploads/
- Delete: удаление файлов

---

## 3. Ключевые компоненты Android клиента

### 3.1 RealGrpcClient — singleton, управление соединением
- OkHttpChannelBuilder с keepAlive (15s ping, 10s timeout)
- Автоматический reconnect (10s delay)
- CoroutineScope(Dispatchers.Main + SupervisorJob())
- Состояния: DISCONNECTED → CONNECTING → READY → FAILED
- Pending messages/reads для повторной отправки после reconnect

### 3.2 CallManager — управление звонками
- StateFlow<CallMessageProto?> для текущего звонка
- SharedFlow для incoming сигналов
- WebRTC signaling через gRPC CallSession
- Conference support

### 3.3 WebRtcClient — WebRTC реализация
- PeerConnectionFactory, PeerConnection
- ICE servers: только Google STUN (stun:stun.l.google.com:19302)
- Video: Camera2Enumerator, front-facing priority
- Audio: AudioSource → AudioTrack
- ICE candidate queuing (drain после remote description)

### 3.4 LavenderMessagingService — FCM handler
- onMessageReceived: разделение VOIP_CALL vs обычные сообщения
- onNewToken: синхронизация через SessionManager
- Notifications: 2 канала (lavender_calls, lavender_messages)
- FullScreenIntent для звонков

### 3.5 Локальная БД (Room)
- AppDatabase с DAO
- Кэширование сообщений, пользователей, настроек

---

### 3.6 Favorites — избранные сообщения
- Сохраняются в таблицу `messages` с `room_id = 'favorites_' + username`
- Дублирующая запись в таблицу `favorites` (user_id, message_id) для быстрого поиска
- При загрузке: `LEFT JOIN favorites` + `WHERE m.room_id = 'favorites_X' OR f.message_id IS NOT NULL`
- Сообщения зашифрованы как обычные (is_e2ee = false), расшифровываются сервером
- gRPC: `GetFavorites(uid)` → `SaveFavoriteMessage(msg)` → `RemoveFavorite(uid, mid)`
- Клиент: `FavoritesActivity.kt` — показывает сообщения из favorites room
- Важно: SELECT должен содержать ровно 16 полей, соответствующих Scan() — дубликат полей смещает все значения

## 4. Протокол (messenger.proto)

### Основные сервисы:
```
service ChatService {
  rpc Chat(stream Message) returns (stream Message);
  rpc Typing(stream TypingRequest) returns (stream TypingSignal);
  rpc CallSession(stream CallMessage) returns (stream CallMessage);
  rpc GetHistory(GetHistoryRequest) returns (GetHistoryResponse);
  // ... unary methods
}
```

### CallMessage типы:
```
enum Type {
  INITIATE = 0;  ACCEPT = 1;  REJECT = 2;  HANGUP = 3;
  OFFER = 4;  ANSWER = 5;  ICE_CANDIDATE = 6;
  INITIATE_CONFERENCE = 7;  JOIN_CONFERENCE = 8;
  LEAVE_CONFERENCE = 9;  END_CONFERENCE = 10;
}
```

---

## 5. Критические проблемы (по приоритету)

### 🔴 P0: FCM полностью сломан
**Симптом:** `[FCM ERROR] Invalid JWT Signature` на каждый push
**Причина:** Firebase service account key истёк или отозван
**Решение:** Сгенерировать новый key в Firebase Console → scp на сервер → restart

### 🔴 P0: WebRTC не соединяется (звонки не работают)
**Причины:**
1. **Нет TURN сервера** — только Google STUN
2. **Нет ICE candidate filtering**
3. **Нет signaling при ошибке** — нет retry

### 🟡 P1: CallActivity — множественные проблемы
1. receiverId по умолчанию — username, не UUID
2. Нет timeout на соединение
3. Нет обработки ICE connection state

### 🟡 P1: gRPC stream нестабильность
1. Keepalive failed при длительном простое
2. Нет graceful degradation при reconnect

### 🟠 P2: UI проблемы
1. OWL chat: `enableEdgeToEdge()` конфликтует с `adjustResize`
2. Keyboard overlapping navigation buttons

---

## 6. Проблемы безопасности

1. **password в Message proto** — plaintext через gRPC (без TLS на клиенте)
2. **No rate limiting** на сервере (кроме OWL)
3. **No message size limit** — gRPC max message size не настроен

---

## 7. Проблемы производительности

1. **GetUserChats** — сложный SQL с CTE, нет кэширования
2. **GetAllUsers** — вызывается для push на КАЖДОЕ сообщение
3. **Sync.Map** для `recentMsgs` — нет GC
4. **HTTP uploads** — 10MB limit, entire file loaded in memory

---

## 8. Архитектурные долги

1. Server version — hardcoded const, не из build system
2. No graceful shutdown
3. Global variables — `firebaseApp`, `owlRateLimiter`
4. No structured logging — `log.Printf` вместо zap/logrus
5. No metrics — latency, error rates
6. Mixed concerns — server.go = 2969 строк
7. No migration system — raw SQL in ConnectDB()
8. iOS клиент — многие методы stubs

---

## 9. Диаграмма потока звонка

```
Caller            Server              Listener
  │                  │                   │
  │──INITIATE──────►│                   │
  │                  │───FCM push──────►│
  │                  │──INITIATE──────►│
  │                  │                   │
  │                  │◄────ACCEPT───────│
  │◄──ACCEPT────────│                   │
  │──OFFER─────────►│──OFFER──────────►│
  │                  │                   │
  │  WebRTC p2p установление            │
  │◄═══════════════════════════════════►│
  │                  │                   │
  │──HANGUP────────►│──HANGUP─────────►│
```

---

## 10. Рекомендации

### Краткосрочные:
1. Исправить FCM (новый key)
2. Добавить HANGUP при abrupt disconnect
3. Добавить TURN сервер (coturn)
4. Исправить receiverId/username в CallActivity
5. Добавить timeout на WebRTC

### Среднесрочные:
1. Рефакторинг server.go → пакеты
2. Rate limiting
3. Graceful shutdown
4. ICE candidate filtering

### Долгосрочные:
1. Message queue для push
2. CDN для uploads
3. Load balancing
4. Automated testing

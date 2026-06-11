# Hermes Agent — Lavender Messenger Platform Adapter

## Конфигурация (.env)

```env
# Lavender gRPC server address
LAVENDER_SERVER=localhost:50052

# Ваш UUID user_id (берётся из базы или /whoami)
LAVENDER_USER_ID=your-uuid-here

# Имя пользователя для отображения
LAVENDER_USERNAME=hermes-agent

# Задержка переподключения (секунды)
LAVENDER_RECONNECT_DELAY=3.0

# Путь к директории с .proto и сгенерированными pb2 файлами
# По умолчанию: текущая директория плагина
# LAVENDER_PROTO_DIR=/path/to/proto/dir

# Hermes Agent model (передаётся в ChatWithAI если нужно)
HERMES_MODEL=openrouter/auto
HERMES_API_KEY=your-openrouter-key
```

## Компиляция .proto файлов

```bash
# Перейти в директорию с .proto файлами
cd /root/msg

# Сгенерировать Python-код из messenger.proto
python3 -m grpc_tools.protoc \
    --proto_path=. \
    --python_out=./hermes-agent/ \
    --grpc_python_out=./hermes-agent/ \
    messenger.proto

# Проверить что файлы созданы
ls -la ./hermes-agent/messenger_pb2.py ./hermes-agent/messenger_pb2_grpc.py
```

Если messenger.proto обновился на сервере — перекомпилируйте и перезапустите агента.

## Запуск

### Standalone (отладка)
```bash
cd /root/msg/hermes-agent
LAVENDER_SERVER=localhost:50052 \
LAVENDER_USER_ID=your-uuid \
LAVENDER_USERNAME=hermes-agent \
python3 adapter.py
```

### Как плагин Hermes Agent
```bash
hermes load-plugin /root/msg/hermes-agent/
hermes start
```

## Архитектура

```
┌──────────────┐    gRPC     ┌──────────────┐    RPC     ┌──────────────┐
│   Hermes     │◄───────────►│   Lavender   │◄──────────►│   Lavender   │
│   Agent      │  plugin API │   Adapter    │  streaming │   Server     │
│              │             │  (этот код)  │  Chat()    │   (Go)       │
└──────────────┘             └──────────────┘            └──────────────┘
```

## Методы

### inbound (входящие от Lavender → Hermes)
1. `handle_inbound_message(session_id, text, user_id)` — вызывается при получении сообщения
2. Если возвращает `str` — отправляется обратно в чат

### outbound (исходящие от Hermes → Lavender)
1. `send_message(session_id, text, **kwargs)` — отправляет сообщение в чат
2. Сообщения ставятся в очередь и отправляются через bidirectional stream

### Жизненный цикл
1. `start()` → подключается к серверу, запускает Chat stream
2. `stop()` → отменяется, закрывает gRPC-канал
3. Автоматическое переподключение при обрыве (exponential backoff)

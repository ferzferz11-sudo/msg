# Log Monitor — Lavender Messenger

Веб-интерфейс для просмотра логов сервера Lavender Messenger в реальном времени.

## Обзор

Log Monitor — это легковесный HTTP-сервер на Go, который:
- Отдаёт логи systemd-сервиса через `journalctl`
- Предоставляет веб-UI с автообновлением, фильтрацией и подсветкой
- Работает как отдельный сервис под systemd

## Два экземпляра

| Файл | Назначение | Порт | Пути | Сервис |
|------|-----------|------|------|--------|
| `log-monitor.go` | Prod | 8090 | `/server-logs/*` | `lavender-server` |
| `log-monitor-dev.go` | Dev | 8091 | `/server-logs-dev/*` | `lavender-server-dev` |

Различия между ними минимальны — только порт, префикс путей и имя сервиса.

## Сборка

```bash
# Prod
go build -o log-monitor log-monitor.go

# Dev
go build -o log-monitor-dev log-monitor-dev.go
```

## Переменные окружения

| Переменная | По умолчанию | Описание |
|-----------|-------------|----------|
| `LOG_PORT` | `8090` (prod) / `8091` (dev) | Порт HTTP-сервера |
| `LOG_SERVICE` | `lavender-server` / `lavender-server-dev` | Имя systemd-сервиса для journalctl |
| `LOG_TITLE` | `Lava Dev Server Logs` | Заголовок страницы (только dev) |
| `LOG_FILE` | `/root/LavenderMessenger/run/server.log` | Fallback-файл логов (только prod) |

## API Endpoints

| Метод | Путь | Описание |
|-------|------|----------|
| GET | `/server-logs` | HTML-страница с логами |
| GET | `/server-logs/raw` | Текст логов (последние 100 строк, 24h) |
| POST | `/server-logs/clear` | Очистка логов (rotate + vacuum) |
| GET | `/server-logs/health` | Health check (JSON) |

Для dev версии пути начинаются с `/server-logs-dev`.

## Веб-UI

- Автообновление каждые 1/3/5/10 секунд
- Пауза/возобновление
- Фильтрация по типу: Error, Warn, Info, OWL, gRPC, Other
- Текстовый фильтр
- Автопрокрутка
- Подсветка строк по типу (цветовая кодировка)
- Кнопка «Clear» — ротация journal + vacuum

## Развёртывание на новом сервере

См. [DEPLOY.md](DEPLOY.md)

## Системные требования

- Go 1.24+
- systemd + journald
- nginx (для проксирования с внешнего интерфейса)

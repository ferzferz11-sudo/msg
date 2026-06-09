# Lavender Messenger — Log Monitor

Документация по веб-интерфейсу для просмотра логов сервера в реальном времени.

**Обновлено:** 2026-06-09
**Версия:** 1.1.1

---

## Обзор

Log Monitor — легковесный HTTP-сервер на Go, который читает логи systemd-сервисов через `journalctl` и отдаёт их через веб-UI с автообновлением, фильтрацией и цветовой подсветкой.

Два экземпляра:

| | Файл исходника | Бинарник | Порт | URL путь | Сервис |
|---|---|---|---|---|---|
| **Prod** | `log-monitor/log-monitor.go` | `log-monitor` | 8090 | `/server-logs/*` | `lavender-server` |
| **Dev** | `log-monitor/log-monitor-dev.go` | `log-monitor-dev` | 8091 | `/server-logs-dev/*` | `lavender-server-dev` |

---

## Архитектура

```
Браузер ──► Nginx (80) ──┬── :8090 ──► log-monitor (prod)
                         └── :8091 ──► log-monitor-dev (dev)
                                              │
                                              ▼
                                        journalctl -u <service>
                                              │
                                              ▼
                                        systemd journal
```

Nginx проксирует HTTP-запросы на log-monitor, который через `journalctl` читает логи из systemd journal. Fallback: если journalctl недоступен (prod), читается файл `server.log`.

---

## API Endpoints

| Метод | Путь | Описание |
|-------|------|----------|
| GET | `/server-logs` | HTML-страница с логами |
| GET | `/server-logs/raw` | Текст логов (последние 100 строк) |
| POST | `/server-logs/clear` | Очистка логов (rotate + vacuum) |
| GET | `/server-logs/health` | Health check (JSON) |

Для dev версии пути начинаются с `/server-logs-dev`.

---

## Веб-UI

- **Автообновление** — интервал 1s / 3s / 5s / 10s
- **Пауза/возобновление** — без потери данных
- **Фильтрация по типу:** Error (красный), Warn (жёлтый), Info (синий), OWL (фиолетовый), gRPC (зелёный), Other
- **Текстовый фильтр** — поиск по подстроке
- **Автопрокрутка** — при включении скроллит к новым записям
- **Кнопка «Clear»** — ротация journal + vacuum

---

## Сборка

```bash
cd /root/msg/log-monitor
export PATH=$PATH:/usr/local/go/bin:~/go/bin

# Prod
go build -o /root/LavenderMessenger/run/log-monitor log-monitor.go

# Dev
go build -o /root/LavenderMessenger/run/log-monitor-dev log-monitor-dev.go
```

---

## Деплой

```bash
# Остановить, заменить бинарь, запустить
systemctl stop log-monitor
cp /tmp/log-monitor /root/LavenderMessenger/run/log-monitor
systemctl start log-monitor

systemctl stop log-monitor-dev
cp /tmp/log-monitor-dev /root/LavenderMessenger/run/log-monitor-dev
systemctl start log-monitor-dev
```

---

## Переменные окружения

| Переменная | По умолчанию | Описание |
|-----------|-------------|----------|
| `LOG_PORT` | `8090` (prod) / `8091` (dev) | Порт HTTP-сервера |
| `LOG_SERVICE` | `lavender-server` / `lavender-server-dev` | Имя systemd-сервиса для journalctl |
| `LOG_TITLE` | `Lava Dev Server Logs` | Заголовок страницы (только dev) |
| `LOG_FILE` | `/root/LavenderMessenger/run/server.log` | Fallback-файл (только prod) |

---

## Известные проблемы и решения

### Все логи красным (prod)
**Причина:** В Go raw string literal (`backtick`) последовательность `\\n` передаётся в браузер как literal `\n`, а не как newline character. JS `text.split('\\n')` не разбивал текст на строки — весь лог был одной строкой и классифицировался как "error".

**Решение:** В Go raw string использовать один бэкслеш: `\n`. Это передаётся в браузер как `\n` (newline character в JS).

### Логи не обновляются / показывают старые данные
**Причина:** `journalctl --since "24 hours ago" -n 100` сначала фильтрует по времени, потом берёт первые 100 строк — т.е. самые **старые** 100 записей из 24h диапазона.

**Решение:** Убран флаг `--since "24 hours ago"`. Теперь `journalctl -n 100` возвращает последние 100 записей.

### Пустой лог после перезапуска сервиса
**Причина:** При остановке сервиса его процесс завершается до того, как systemd перенаправит stdout в journal.

**Решение:** Это нормальное поведение systemd. Логи до перезапуска сохраняются в journal и доступны через `journalctl`.

---

## Связанные файлы

- Исходники: `/root/msg/log-monitor/log-monitor.go` (prod), `log-monitor-dev.go` (dev)
- Dev guide: `/root/msg/log-monitor/DEPLOY.md`
- Changelog: `/root/msg/log-monitor/CHANGELOG.md`
- Readme: `/root/msg/log-monitor/README.md`
- Nginx: `/etc/nginx/sites-enabled/lavender` (location `/server-logs` и `/server-logs-dev`)
- Systemd units: `/etc/systemd/system/log-monitor.service`, `log-monitor-dev.service`

# Changelog — Log Monitor

Все значимые изменения в log-monitor.

## [1.1.1] — 2026-06-09

### Исправлено
- **Prod:** JS `text.split('\\n')` — исправлено экранирование в Go raw string. Было `\\\\n` → в браузер приходил literal `\n`, split не работал, весь лог отображался одной строкой и был красным (class "error"). Стало `\\n` → в браузер приходит `\n` (newline character), split работает корректно.
- **Обе версии:** Убран `--since "24 hours ago"` из команды journalctl. `journalctl -n 100 --since 24h` брал первые (самые старые) 100 строк из 24h диапазона, а не последние. Теперь `-n 100` возвращает последние 100 записей.

### Изменено
- Prod `log-monitor.go`: строка `--since "24 hours ago"` убрана, комментарий обновлён
- Dev `log-monitor-dev.go`: строка `--since "24 hours ago"` убрана, комментарий обновлён

## [1.1.0] — 2026-06-07

### Добавлено (dev версия)
- Отдельный бинарник `log-monitor-dev` для dev сервера
- Переменная окружения `LOG_SERVICE` — имя systemd-сервиса (dev: `lavender-server-dev`)
- Переменная окружения `LOG_TITLE` — заголовок страницы
- Цветовая схема dev: оранжевая (`#d29922`) вместо синей
- Кнопка Clear с визуальной обратной связью (⏳ во время очистки)
- Endpoint `/clear` для dev: `systemctl kill -s USR1` + `journalctl --vacuum-size=1M`

### Изменено
- Dev версия слушает порт `8091` (prod: `8090`)
- Dev версия использует пути `/server-logs-dev/*` (prod: `/server-logs/*`)

## [1.0.0] — 2026-06-02

### Добавлено
- Первая версия log-monitor
- Веб-UI с автообновлением (1s/3s/5s/10s)
- Фильтрация по типу: Error, Warn, Info, OWL, gRPC, Other
- Текстовый фильтр
- Автопрокрутка
- Подсветка строк по типу
- Endpoint `/raw` — последние 100 строк через `journalctl`
- Endpoint `/clear` — `journalctl --rotate` + `--vacuum-time=1s`
- Endpoint `/health` — health check
- Fallback на файл логов если journalctl недоступен

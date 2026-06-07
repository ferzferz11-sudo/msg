# Changelog — Log Monitor

Все значимые изменения в log-monitor.

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
- Dev версия исходит пути `/server-logs-dev/*` (prod: `/server-logs/*`)

## [1.0.0] — 2026-06-02

### Добавлено
- Первая версия log-monitor
- Веб-UI с автообновлением (1s/3s/5s/10s)
- Фильтрация по типу: Error, Warn, Info, OWL, gRPC, Other
- Текстовый фильтр
- Автопрокрутка
- Подсветка строк по типу
- Endpoint `/raw` — последние 100 строк за 24h через `journalctl`
- Endpoint `/clear` — `journalctl --rotate` + `--vacuum-time=1s`
- Endpoint `/health` — health check
- Fallback на файл логов если journalctl недоступен

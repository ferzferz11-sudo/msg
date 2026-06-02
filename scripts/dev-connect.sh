#!/bin/bash
# dev-connect.sh — Подключиться к dev серверу как супер-админ
# Использование: ./dev-connect.sh [command]
# 
# Команды:
#   status    — статус dev сервера
#   logs      — логи dev сервера (последние 50 строк)
#   restart   — перезапустить dev сервер
#   stop      — остановить dev сервер
#   start     — запустить dev сервер
#   shell     — интерактивная оболочка (bash)
#   db        — подключиться к dev БД (psql)
#   deploy    — пересобрать и перезапустить (deploy-dev.sh)

set -e

COMMAND="${1:-status}"

case "$COMMAND" in
    status)
        echo "=== Dev Server Status ==="
        systemctl status lavender-server-dev --no-pager
        echo ""
        echo "=== Dev Log Monitor Status ==="
        systemctl status log-monitor-dev --no-pager
        ;;
    logs)
        echo "=== Dev Server Logs (last 50 lines) ==="
        journalctl -u lavender-server-dev -n 50 --no-pager
        ;;
    restart)
        systemctl restart lavender-server-dev
        systemctl restart log-monitor-dev
        echo "Dev server restarted"
        ;;
    stop)
        systemctl stop lavender-server-dev
        systemctl stop log-monitor-dev
        echo "Dev server stopped"
        ;;
    start)
        systemctl start lavender-server-dev
        systemctl start log-monitor-dev
        echo "Dev server started"
        ;;
    shell)
        bash
        ;;
    db)
        psql -h localhost -U lavender -d chat_db_dev
        ;;
    deploy)
        /root/msg/scripts/deploy-dev.sh
        ;;
    *)
        echo "Usage: $0 {status|logs|restart|stop|start|shell|db|deploy}"
        exit 1
        ;;
esac

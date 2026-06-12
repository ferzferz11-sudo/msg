#!/bin/bash
# deploy_agent.sh - install and manage Hermes Remote Agent as systemd service

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
AGENT_ID="${2:-agent-1}"
TOKEN="$3"

case "${1}" in
  install)
    if [ -z "$TOKEN" ]; then
      echo "Usage: $0 install <agent_id> <token>"
      exit 1
    fi
    if [ ! -f "$TEMPLATE" ]; then
      echo "ERROR: Template not found: $TEMPLATE"
      exit 1
    fi
    sed "s|%TOKEN%|${TOKEN}|g; s|%i|${AGENT_ID}|g" "$TEMPLATE" > "$SERVICE_FILE"
    systemctl daemon-reload
    systemctl enable "$SERVICE_NAME"
    systemctl start "$SERVICE_NAME"
    echo "Agent ${AGENT_ID} installed and started"
    ;;

  start)
    systemctl start "$SERVICE_NAME"
    echo "Agent ${AGENT_ID} started"
    ;;

  stop)
    systemctl stop "$SERVICE_NAME"
    echo "Agent ${AGENT_ID} stopped"
    ;;

  status)
    systemctl status "$SERVICE_NAME" --no-pager
    ;;

  logs)
    journalctl -u "$SERVICE_NAME" -f
    ;;

  uninstall)
    systemctl stop "$SERVICE_NAME" 2>/dev/null || true
    systemctl disable "$SERVICE_NAME" 2>/dev/null || true
    rm -f "$SERVICE_FILE"
    systemctl daemon-reload
    echo "Agent ${AGENT_ID} uninstalled"
    ;;

  *)
    echo "Usage: $0 {install|start|stop|status|logs|uninstall} <agent_id> [token]"
    exit 1
    ;;
esac

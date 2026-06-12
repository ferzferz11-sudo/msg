#!/bin/bash
# release.sh — выпуск нового релиза сервера Lavender Messenger
#
# Использование:
#   ./scripts/release.sh <version> [--deploy] [--remote]
#   Пример: ./scripts/release.sh 1.1.3.3
#   Пример: ./scripts/release.sh 1.1.3.3 --deploy --remote
#
# Режимы запуска:
#   С сервера (где OWL): ssh lava
#     ./scripts/release.sh 1.1.3.3 --deploy
#   С Mac (удалённо):
#     ./scripts/release.sh 1.1.3.3 --deploy --remote
#
# Требования для --remote:
#   - SSH alias "lava" в ~/.ssh/config
#   - GOOS=linux GOARCH=amd64 для cross-compile
#   - gh CLI для GitHub releases

set -e

VERSION="$1"
DEPLOY=false
REMOTE=false

if [ -z "$VERSION" ]; then
  echo "❌ Укажи версию: ./scripts/release.sh <version> [--deploy] [--remote]"
  exit 1
fi

for arg in "$@"; do
  case "$arg" in
    --deploy) DEPLOY=true ;;
    --remote) REMOTE=true ;;
  esac
done

REPO_DIR="$(cd "$(dirname "$0")/.." && pwd)"
SERVER="lava"
SERVER_DIR="/root/LavenderMessenger"

echo "🚀 Выпуск релиза v$VERSION"

# 1. Проверяем CHANGELOG
cd "$REPO_DIR"
if ! grep -q "^\## \[$VERSION\]" CHANGELOG.md 2>/dev/null; then
  echo "❌ CHANGELOG.md не содержит секцию [$VERSION]"
  exit 1
fi
echo "✅ CHANGELOG.md содержит [$VERSION]"

# 2. Git commit & push
if [ -n "$(git status --porcelain)" ]; then
  git add -A
  git commit -m "release: v$VERSION" --allow-empty
  git push origin "$(git branch --show-current)"
  echo "✅ Изменения запушены"
fi

# 3. Git tag
if git tag -l "v$VERSION" | grep -q "v$VERSION"; then
  echo "⚠️  Tag v$VERSION уже существует"
else
  git tag "v$VERSION"
  git push origin "v$VERSION"
  echo "✅ Tag v$VERSION создан"
fi

# 4. GitHub Release
if command -v gh &> /dev/null; then
  CHANGELOG=$(awk "/^## \[$VERSION\]/{flag=1; next} /^## \[/{flag=0} flag" CHANGELOG.md | sed '/^$/d' | head -50)
  [ -z "$CHANGELOG" ] && CHANGELOG="См. CHANGELOG.md"
  gh release create "v$VERSION" \
    --title "Lavender Server v$VERSION" \
    --notes "$CHANGELOG" 2>/dev/null || \
  gh release edit "v$VERSION" \
    --title "Lavender Server v$VERSION" \
    --notes "$CHANGELOG" 2>/dev/null || true
  echo "✅ GitHub Release v$VERSION"
fi

# 5. Deploy
if [ "$DEPLOY" = true ]; then
  if [ "$REMOTE" = true ]; then
    echo "→ Cross-compile для Linux..."
    GOOS=linux GOARCH=amd64 go build -o lavender-server-new .

    echo "→ Загрузка на сервер (ssh lava)..."
    scp lavender-server-new "${SERVER}:${SERVER_DIR}/run/"

    echo "→ Перезапуск..."
    ssh "$SERVER" "cd ${SERVER_DIR}/run && \
      cp lavender-server lavender-server-old && \
      cp lavender-server-new lavender-server && \
      pkill -f lavender-server; sleep 2; \
      nohup ./lavender-server >> logs.txt 2>&1 &"

    rm -f lavender-server-new
    echo "✅ Деплой завершён"
  else
    echo "→ Сборка..."
    go build -o lavender-server .

    echo "→ Копирование в run/..."
    cp -f lavender-server "${SERVER_DIR}/run/"

    echo "→ Перезапуск systemd..."
    systemctl restart lavender-server
    sleep 3

    if systemctl is-active lavender-server >/dev/null 2>&1; then
      echo "✅ Сервер запущен"
    else
      echo "❌ Сервер не запустился!"
      journalctl -u lavender-server --no-pager -n 20
      exit 1
    fi
  fi
fi

echo "🎉 v$VERSION готов!"

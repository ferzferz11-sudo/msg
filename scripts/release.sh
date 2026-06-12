#!/bin/bash
# release.sh — выпуск нового релиза сервера Lavender Messenger
#
# Использование:
#   ./scripts/release.sh <version> [--deploy]
#   Пример: ./scripts/release.sh 1.1.3.3
#   Пример: ./scripts/release.sh 1.1.3.3 --deploy
#
# Что делает:
# 1. Проверяет что CHANGELOG.md содержит секцию [version]
# 2. Коммитит и пушит все изменения
# 3. Создаёт git tag v<version>
# 4. Создаёт GitHub Release с changelog
# 5. Если --deploy: собирает, копирует в run/, перезапускает systemd
#
# Режимы запуска:
#   С сервера (где OWL): ./scripts/release.sh 1.1.3.3 --deploy
#   С Mac (удалённо):   ./scripts/release.sh 1.1.3.3 --deploy --remote
#
# Требования:
# - SSH доступ на сервер (для --remote)
# - gh CLI для GitHub releases
# - golang для сборки
#
# Ключ SSH:
#   С сервера: ssh-keygen, публичный ключ на GitHub
#   C Mac:     ~/.ssh/id_ed25519 (или другой, указанный в deploy.sh)

set -e

VERSION="$1"
DEPLOY=false
REMOTE=false

if [ -z "$VERSION" ]; then
  echo "❌ Укажи версию: ./scripts/release.sh <version> [--deploy] [--remote]"
  echo "   Пример: ./scripts/release.sh 1.1.3.3 --deploy"
  exit 1
fi

# Parse flags
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
echo ""

# 1. Проверяем CHANGELOG
cd "$REPO_DIR"
if ! grep -q "^\## \[$VERSION\]" CHANGELOG.md 2>/dev/null; then
  echo "❌ CHANGELOG.md не содержит секцию [$VERSION]"
  echo "   Добавь секцию перед релизом"
  exit 1
fi
echo "✅ CHANGELOG.md содержит [$VERSION]"

# 2. Git commit & push
if [ -n "$(git status --porcelain)" ]; then
  echo "→ Коммит незакоммиченных изменений..."
  git add -A
  git commit -m "release: v$VERSION" --allow-empty
  git push origin "$(git branch --show-current)"
  echo "✅ Изменения запушены"
else
  echo "✅ Нет незакоммиченных изменений"
fi

# 3. Git tag
if git tag -l "v$VERSION" | grep -q "v$VERSION"; then
  echo "⚠️  Tag v$VERSION уже существует, пропускаем"
else
  git tag "v$VERSION"
  git push origin "v$VERSION"
  echo "✅ Tag v$VERSION создан"
fi

# 4. GitHub Release
if command -v gh &> /dev/null; then
  echo "→ Создание GitHub Release..."

  # Извлекаем changelog для этой версии
  CHANGELOG=$(awk "/^## \\[$VERSION\\]/{flag=1; next} /^## \\[/{flag=0} flag" CHANGELOG.md | sed '/^$/d' | head -50)

  if [ -z "$CHANGELOG" ]; then
    CHANGELOG="См. CHANGELOG.md"
  fi

  gh release create "v$VERSION" \
    --title "Lavender Server v$VERSION" \
    --notes "$CHANGELOG" \
    2>/dev/null || {
      echo "  Release уже существует, обновляем..."
      gh release edit "v$VERSION" \
        --title "Lavender Server v$VERSION" \
        --notes "$CHANGELOG" 2>/dev/null || echo "⚠️  Не удалось обновить Release"
    }
  echo "✅ GitHub Release v$VERSION создан"
else
  echo "⚠️  gh CLI не найден, GitHub Release пропущен"
  echo "   Создай вручную: https://github.com/ferzferz11-sudo/msg/releases/new"
fi

# 5. Deploy
if [ "$DEPLOY" = true ]; then
  echo ""
  echo "→ Деплой на сервер..."

  if [ "$REMOTE" = true ]; then
    # С Mac — используем deploy.sh
    echo "  → Сборка для Linux (cross-compile)..."
    GOOS=linux GOARCH=amd64 go build -o lavender-server-new .

    echo "  → Загрузка на сервер..."
    scp lavender-server-new "${SERVER}:${SERVER_DIR}/run/"
    scp "${REPO_DIR}/.env" "${SERVER}:${SERVER_DIR}/.env"
    scp "${REPO_DIR}/config.yaml" "${SERVER}:${SERVER_DIR}/config.yaml"

    echo "  → Перезапуск..."
    ssh "$SERVER" "cd ${SERVER_DIR}/run && cp lavender-server lavender-server-old && cp lavender-server-new lavender-server && pkill -f lavender-server && sleep 2 && nohup ./lavender-server >> logs.txt 2>&1 &"

    rm -f lavender-server-new
    echo "  ✅ Деплой завершён"
  else
    # С сервера — используем build-server.sh
    echo "  → Сборка..."
    go build -o lavender-server .

    echo "  → Копирование в run/..."
    cp -f lavender-server "${SERVER_DIR}/run/"
    cp -f .env "${SERVER_DIR}/run/"
    cp -f config.yaml "${SERVER_DIR}/run/" 2>/dev/null || true

    echo "  → Перезапускаем через systemd..."
    systemctl restart lavender-server
    sleep 3

    if systemctl is-active lavender-server >/dev/null 2>&1; then
      echo "  ✅ Сервер запущен"
      journalctl -u lavender-server --no-pager -n 5
    else
      echo "  ❌ Сервер не запустился!"
      journalctl -u lavender-server --no-pager -n 20
      exit 1
    fi
  fi
fi

echo ""
echo "🎉 Релиз v$VERSION готов!"
echo "   Tag:    https://github.com/ferzferz11-sudo/msg/releases/tag/v$VERSION"
echo "   Сервер: grpc :50051, http :8082"

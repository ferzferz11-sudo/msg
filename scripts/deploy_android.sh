#!/bin/bash
# deploy_android.sh — деплой Android APK на сервер
# Использование:
#   С указанием пути:  ./deploy_android.sh /path/to/app-release.apk
#   Без аргументов:    ./deploy_android.sh  (ищет app/build/outputs/apk/release/app-release.apk)

set -e

SERVER="root@13.140.25.249"
SERVER_DIR="/var/www/lavender"
ARCHIVE_DIR="$SERVER_DIR/archive/android"
SSH_KEY="${SSH_KEY:-$HOME/.ssh/lava}"

# Определяем путь к APK
if [ -n "$1" ]; then
  APK_PATH="$1"
else
  APK_PATH="app/build/outputs/apk/release/app-release.apk"
fi

if [ ! -f "$APK_PATH" ]; then
  echo "❌ APK не найден: $APK_PATH"
  echo "Использование: $0 [путь_к_apk]"
  exit 1
fi

# Получаем версию из APK
AAPT=$(which aapt2 2>/dev/null || which aapt 2>/dev/null || find ~/Library/Android -name "aapt2" -type f 2>/dev/null | head -1)
if [ -z "$AAPT" ]; then
  echo "❌ aapt/aapt2 не найден. Укажи путь: export AAPT=/path/to/aapt2"
  exit 1
fi
VERSION=$($AAPT dump badging "$APK_PATH" 2>/dev/null | grep versionName | sed "s/.*versionName='\([^']*\)'.*/\1/")
if [ -z "$VERSION" ]; then
  # Fallback: читаем из version.txt рядом с APK
  VERSION_FILE="$(dirname $(dirname $(dirname "$APK_PATH")))/version.txt"
  if [ -f "$VERSION_FILE" ]; then
    VERSION=$(cat "$VERSION_FILE" | tr -d '[:space:]')
  fi
fi

if [ -z "$VERSION" ]; then
  echo "❌ Не удалось определить версию APK"
  exit 1
fi

APK_SIZE=$(du -h "$APK_PATH" | cut -f1)
echo "📦 Версия: $VERSION | Размер: $APK_SIZE"
echo "🚀 Деплой на $SERVER..."


# Дата для versions.json
TODAY=$(date +%Y-%m-%d)

# SCP парметры
SCP_OPTS=""
SSH_OPTS=""
if [ -f "$SSH_KEY" ]; then
  SCP_OPTS="-i $SSH_KEY"
  SSH_OPTS="-i $SSH_KEY"
fi


# 1. Копируем APK на сервер
echo "→ Копирование APK..."
scp $SCP_OPTS "$APK_PATH" "$SERVER:$SERVER_DIR/lavender.apk"

# 2. Копируем в архив и обновляем version.txt + changelog.txt
echo "→ Обновление сайта..."
ssh $SSH_OPTS "$SERVER" "VERSION='$VERSION' bash -c '
  mkdir -p $ARCHIVE_DIR/\$VERSION
  cp $SERVER_DIR/lavender.apk $ARCHIVE_DIR/\$VERSION/lavender.apk
  echo \$VERSION > $SERVER_DIR/version.txt
'"

# 3. Обновляем versions.json
echo "→ Обновление versions.json..."
ssh $SSH_OPTS "$SERVER" "VERSION='$VERSION' TODAY='$TODAY' python3 -c '
import json, os, sys
v = os.environ[\"VERSION\"]
t = os.environ[\"TODAY\"]
os.chdir(\"/var/www/lavender/archive\")
old = []
if os.path.exists(\"versions.json\"):
    with open(\"versions.json\",\"r\") as f:
        old = json.load(f)
entry = {\"version\": v, \"date\": t, \"client\": {\"android\": \"/archive/android/\" + v + \"/lavender.apk\"}}
old = [e for e in old if e[\"version\"] != v]
old.insert(0, entry)
with open(\"versions.json\",\"w\") as f:
    json.dump(old, f, indent=2, ensure_ascii=False)
print(\"OK:\", [e[\"version\"] for e in old])
'"

echo "✅ Готово!"
echo "   APK:   http://13.140.25.249/download"
echo "   Архив: http://13.140.25.249/archive"

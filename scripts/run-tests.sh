#!/bin/bash
# run-tests.sh — Запуск всех серверных тестов
# Использование: ./run-tests.sh [test_name]
#   Без аргументов — все тесты
#   С аргументом — конкретный тест (например: ./run-tests.sh TestStreamOpenRouter)

set -euo pipefail

cd "$(dirname "$0")/.."

export PATH="$PATH:/usr/local/go/bin:~/go/bin"

echo "=== Running Go tests ==="
echo "Working dir: $(pwd)"
echo "Go version: $(go version)"
echo ""

if [ -z "${1:-}" ]; then
    echo "--- All tests ---"
    go test ./... -count=1 -timeout 120s -v 2>&1 | tail -50
else
    echo "--- Single test: $1 ---"
    go test -run "$1" -count=1 -timeout 120s -v
fi

echo ""
echo "=== Tests complete ==="

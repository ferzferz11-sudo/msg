#!/bin/bash
# run-streaming-tests.sh — Запуск тестов стриминга (Remote Agent)
# Использование: ./run-streaming-tests.sh

set -euo pipefail

cd "$(dirname "$0")/.."

export PATH="$PATH:/usr/local/go/bin:~/go/bin"

echo "=== Running Streaming Tests ==="
echo ""

echo "--- Unit tests ---"
go test -run "TestDeployAgentTaskStream_Unit" -count=1 -timeout 60s -v
echo ""

echo "--- Integration tests ---"
go test -run "TestDeployAgentTaskStream_Integration" -count=1 -timeout 60s -v
echo ""

echo "--- All streaming tests ---"
go test -run "TestDeployAgentTaskStream" -count=1 -timeout 60s
echo ""

echo "=== Streaming tests complete ==="

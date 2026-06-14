#!/bin/bash
# run-unit-tests.sh — Запуск unit-тестов (без интеграционных)
# Использование: ./run-unit-tests.sh

set -euo pipefail

cd "$(dirname "$0")/.."

export PATH="$PATH:/usr/local/go/bin:~/go/bin"

echo "=== Running Unit Tests ==="
echo ""

echo "--- Auth Service ---"
go test -run "TestSignIn|TestSignUp" -count=1 -timeout 60s -v
echo ""

echo "--- OWL Streaming ---"
go test -run "TestStreamOpenRouter|TestOwlRateLimiter|TestOwlFullFlow|TestMockOpenRouterAPI" -count=1 -timeout 60s -v
echo ""

echo "--- Bot Commands ---"
go test -run "TestBot|TestHandleBot|TestDispatchBot" -count=1 -timeout 60s -v
echo ""

echo "--- Remote Agent (unit) ---"
go test -run "TestDeployAgentTaskStream_Unit" -count=1 -timeout 60s -v
echo ""

echo "=== Unit tests complete ==="

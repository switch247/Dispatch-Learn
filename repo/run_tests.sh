#!/bin/bash
# DispatchLearn - Test Execution Script
# Builds and runs all tests through Docker

set -euo pipefail

echo "=========================================="
echo "  DispatchLearn Test Suite"
echo "=========================================="
echo ""

UNIT_PASS=0
UNIT_FAIL=0
API_PASS=0
API_FAIL=0

# Step 0: Clean up any previous test state
echo "[test][cleanup] Cleaning previous state..."
ENABLE_TLS=false docker compose down -v 2>/dev/null || true

# Step 1: Build all services
echo "[test][build] Building Docker images..."
ENABLE_TLS=false docker compose --profile test build --quiet 2>&1

# Step 2: Start infrastructure
echo "[test][infra] Starting MySQL and application..."
ENABLE_TLS=false docker compose up -d mysql app
echo "[test][infra] Waiting for services to be healthy..."

# Wait for MySQL
for i in $(seq 1 30); do
  if ENABLE_TLS=false docker compose exec -T mysql mysqladmin ping -h localhost -u root -prootpassword --silent 2>/dev/null; then
    echo "[test][infra] MySQL is ready"
    break
  fi
  if [ "$i" -eq 30 ]; then
    echo "[test][infra] ERROR: MySQL failed to start"
    ENABLE_TLS=false docker compose logs mysql
    ENABLE_TLS=false docker compose down
    exit 1
  fi
  sleep 2
done

# Wait for app
for i in $(seq 1 30); do
  if ENABLE_TLS=false docker compose exec -T app wget -q -O /dev/null http://localhost:8080/health 2>/dev/null; then
    echo "[test][infra] Application is ready"
    break
  fi
  if [ "$i" -eq 30 ]; then
    echo "[test][infra] ERROR: Application failed to start"
    ENABLE_TLS=false docker compose logs app
    ENABLE_TLS=false docker compose down
    exit 1
  fi
  sleep 2
done

echo ""
echo "=========================================="
echo "  Running Unit Tests"
echo "=========================================="

# Step 3: Run unit tests inside the test container
UNIT_OUTPUT=$(ENABLE_TLS=false docker compose --profile test run --rm test go test -v -count=1 ./tests/unit/... 2>&1) || true
echo "$UNIT_OUTPUT"

UNIT_PASS=$(echo "$UNIT_OUTPUT" | grep -c "PASS:" || true)
UNIT_FAIL=$(echo "$UNIT_OUTPUT" | grep -c "FAIL:" || true)

echo ""
echo "=========================================="
echo "  Running API Tests"
echo "=========================================="

# Step 4: Run API integration tests
API_OUTPUT=$(ENABLE_TLS=false docker compose --profile test run --rm \
  -e APP_HOST=app \
  -e APP_PORT=8080 \
  test go test -v -count=1 ./tests/api/... 2>&1) || true
echo "$API_OUTPUT"

API_PASS=$(echo "$API_OUTPUT" | grep -c "PASS:" || true)
API_FAIL=$(echo "$API_OUTPUT" | grep -c "FAIL:" || true)

# Step 5: Summary
TOTAL_PASS=$((UNIT_PASS + API_PASS))
TOTAL_FAIL=$((UNIT_FAIL + API_FAIL))
TOTAL=$((TOTAL_PASS + TOTAL_FAIL))

echo ""
echo "=========================================="
echo "  Test Summary"
echo "=========================================="
echo "  Unit Tests:  ${UNIT_PASS} passed, ${UNIT_FAIL} failed"
echo "  API Tests:   ${API_PASS} passed, ${API_FAIL} failed"
echo "  ─────────────────────────────────"
echo "  Total:       ${TOTAL_PASS}/${TOTAL} passed"
echo "=========================================="

# Step 6: Cleanup
echo ""
echo "[test][cleanup] Stopping services and removing volumes..."
ENABLE_TLS=false docker compose down -v

if [ "$TOTAL_FAIL" -gt 0 ]; then
  echo ""
  echo "RESULT: FAIL (${TOTAL_FAIL} test(s) failed)"
  exit 1
else
  echo ""
  echo "RESULT: PASS (all ${TOTAL_PASS} tests passed)"
  exit 0
fi

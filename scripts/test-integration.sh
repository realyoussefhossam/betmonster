#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
export TEST_DATABASE_URL="${TEST_DATABASE_URL:-postgres://wallet:wallet@localhost:5433/wallet_test?sslmode=disable}"

cd "$ROOT"

# Ensure the test database exists and is reachable.
if command -v psql >/dev/null 2>&1; then
  psql "${TEST_DATABASE_URL}" -c 'SELECT 1' >/dev/null 2>&1 || {
    echo "Test database not reachable at ${TEST_DATABASE_URL}. Start Postgres and set TEST_DATABASE_URL if needed."
    exit 1
  }
else
  # Fall back to docker exec if psql is not installed locally. The container URL uses the
  # Postgres internal port (5432) while the host URL typically maps to 5433.
  INTERNAL_URL="${TEST_DATABASE_URL/:5433/:5432}"
  docker exec betmonster-postgres-1 psql "${INTERNAL_URL}" -c 'SELECT 1' >/dev/null 2>&1 || {
    echo "Test database not reachable at ${TEST_DATABASE_URL}. Start Postgres and set TEST_DATABASE_URL if needed."
    exit 1
  }
fi

# Run migrations against the test database.
DATABASE_URL="$TEST_DATABASE_URL" ./scripts/migrate.sh up

# Run only the wallet integration tests. They skip automatically if TEST_DATABASE_URL is unset.
TEST_DATABASE_URL="$TEST_DATABASE_URL" go test -tags integration -race ./internal/wallet/...

#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DATABASE_URL="${DATABASE_URL:-postgres://wallet:wallet@localhost:5433/wallet?sslmode=disable}"

cd "$ROOT"

case "${1:-up}" in
  up)
    go run -tags postgres github.com/golang-migrate/migrate/v4/cmd/migrate@latest \
      -database "${DATABASE_URL}" -path wallet/migrations up
    ;;
  down)
    go run -tags postgres github.com/golang-migrate/migrate/v4/cmd/migrate@latest \
      -database "${DATABASE_URL}" -path wallet/migrations down 1
    ;;
  create)
    go run -tags postgres github.com/golang-migrate/migrate/v4/cmd/migrate@latest \
      create -ext sql -dir wallet/migrations -seq "${2:-migration}"
    ;;
  *)
    echo "Usage: $0 [up|down|create name]"
    exit 1
    ;;
esac

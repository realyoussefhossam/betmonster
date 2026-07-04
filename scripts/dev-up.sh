#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"

cd "$ROOT"

docker compose up -d postgres redis nats

./scripts/migrate.sh up

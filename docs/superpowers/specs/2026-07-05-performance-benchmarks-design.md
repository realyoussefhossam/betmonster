# BetMonster Performance Benchmarks Design

**Scope:** add a reproducible, README-friendly benchmark suite for the v1 wallet microservice and gateway.

**Date:** 2026-07-05
**Status:** draft — pending implementation plan

## 1. Context

BetMonster v1 consists of a Next.js frontend, a Go gateway, a Go wallet service, Postgres, Redis, NATS, and xcash for deposits. Before the platform grows into sportsbook/casino workloads, we need baseline performance numbers that operators can trust and reproduce.

The benchmark suite will measure the hot paths of the v1 architecture: wallet writes, gateway reads, webhook ingestion, and the full deposit flow. The results will be published in `BENCHMARKS.md` and kept up to date by CI.

## 2. Goals

- Provide a **single-command benchmark** that runs against a real Docker Compose stack.
- Measure **throughput, latency percentiles, and error rates** for the core wallet/gateway operations.
- Produce **README-friendly Markdown output** that operators can compare against their own hardware.
- Keep benchmark runs **deterministic and isolated** from production data.

## 3. Non-Goals

- No comparison with other platforms or competitive claims.
- No long-running soak tests in v1 (target duration is 30 seconds per scenario).
- No benchmarking of external third-party services (e.g., real xcash SaaS, public RPC nodes).

## 4. Benchmark Scenarios

| Scenario | What it measures | Entry point |
|---|---|---|
| **Wallet Credit** | `CreditWallet` throughput and latency under concurrent deposits | `bench/wallet_credit_test.go` |
| **Gateway Read** | `GET /api/wallet/balance` and `GET /api/wallet/transactions` latency | `bench/gateway_read_test.go` |
| **Webhook Ingest** | `POST /webhooks/xcash/deposit` throughput and idempotency | `bench/webhook_ingest_test.go` |
| **End-to-End Deposit** | Full flow: get address → simulate xcash webhook → balance updated | `bench/e2e_deposit_test.go` |

### 4.1 Wallet Credit

Calls the wallet service's `CreditWallet` RPC directly to measure raw ledger write performance. This is the upper bound for deposit throughput because it bypasses the gateway and xcash webhook machinery.

- Each iteration credits a unique `user_id` / `reference_id` pair.
- Currency: `USDT`.
- Concurrency: 10, 50, 100 goroutines.
- Duration: 30 seconds per concurrency level.

### 4.2 Gateway Read

Measures the gateway HTTP latency for the two most common read endpoints.

- Pre-seeds a wallet with a balance and a small transaction history.
- Endpoints: `GET /api/wallet/balance?currency=USDT` and `GET /api/wallet/transactions`.
- Concurrency: 10, 50, 100 goroutines.
- Duration: 30 seconds per concurrency level.

### 4.3 Webhook Ingest

Measures the gateway's ability to accept and process xcash deposit webhooks.

- Generates valid HMAC-SHA256 signatures for each request.
- Uses unique `sys_no` values to avoid idempotency conflicts.
- Currency: `USDT`.
- Concurrency: 10, 50, 100 goroutines.
- Duration: 30 seconds per concurrency level.

### 4.4 End-to-End Deposit

Measures the complete deposit path as a user would experience it.

- Step 1: call `GET /api/wallet/deposit-address?currency=USDT&chain=anvil`.
- Step 2: send a signed `POST /webhooks/xcash/deposit` for the returned address.
- Step 3: verify the wallet balance increased.
- Concurrency: 10, 25 goroutines.
- Duration: 30 seconds per concurrency level.

## 5. Architecture

```text
┌─────────────┐      ┌──────────────┐      ┌──────────────┐
│   bench/    │──────▶│   Gateway    │──────▶│   Wallet     │
│   tests     │ HTTP  │   Service    │ gRPC  │   Service    │
│   (Go)      │       │              │       │  (Postgres)  │
└─────────────┘       └──────────────┘      └──────────────┘
                           │
                           ▼
                    ┌──────────────┐
                    │   xcash      │
                    │   (mocked)   │
                    └──────────────┘
```

Components:

- **`bench/runner.go`** — shared setup: connects to the Docker stack, creates test users, seeds wallets, and cleans up after the run.
- **`bench/wallet_client.go`** — gRPC client wrapper for the wallet service.
- **`bench/http_client.go`** — HTTP client wrapper for the gateway with connection pooling and timeout defaults.
- **`bench/reporter.go`** — collects `testing.B` results and prints a Markdown table.
- **`bench/xcash_webhook.go`** — helper to build valid xcash webhook payloads and HMAC signatures.

For the Docker stack, we add `docker-compose.bench.yml` that reuses the production service images but pins resources and uses a dedicated `bench_default` network. This avoids port collisions with a developer's running stack.

## 6. Metrics and Reporting

Each benchmark reports:

- **Throughput**: operations per second.
- **Latency**: p50, p95, p99 in milliseconds.
- **Errors**: error count and error rate.
- **Saturation**: the highest concurrency level where error rate stays below 1%.

The `bench/reporter.go` prints a Markdown table at the end of the run:

```markdown
| Scenario | Concurrency | RPS | p50 (ms) | p95 (ms) | p99 (ms) | Errors |
|---|---|---:|---:|---:|---:|---:|
| Wallet Credit | 100 | 1,250 | 2.1 | 4.3 | 8.7 | 0% |
| Gateway Balance | 100 | 3,400 | 1.2 | 2.8 | 5.1 | 0% |
| Gateway Transactions | 100 | 2,100 | 1.8 | 3.9 | 7.2 | 0% |
| Webhook Ingest | 100 | 980 | 2.8 | 6.2 | 12.4 | 0% |
| E2E Deposit | 25 | 45 | 18.0 | 32.0 | 55.0 | 0% |
```

`BENCHMARKS.md` will store the latest CI baseline. The README will link to it and show the `make bench` command.

## 7. Running the Benchmarks

Three ways to run:

1. **`make bench`** — starts the Docker Compose benchmark stack, runs all scenarios, prints the Markdown table, and tears the stack down. This is the README entry point.
2. **`make bench-against-running`** — skips stack startup and runs against an already-running `docker compose up` stack. Useful for iterative tuning.
3. **`go test ./bench/...`** — runs the Go benchmarks directly if `GATEWAY_ADDR` and `WALLET_GRPC_ADDR` env vars are set.

### Makefile targets

```makefile
bench:                      ## Start bench stack, run benchmarks, stop stack
	./scripts/bench.sh up run down

bench-against-running:      ## Run benchmarks against the current dev stack
	go test ./bench/... -bench=. -benchtime=30s

bench-report:               ## Print the latest BENCHMARKS.md
	cat BENCHMARKS.md
```

## 8. Error Handling

- If the Docker stack fails to start, `bench/runner.go` prints the failing container logs and exits non-zero.
- If a benchmark iteration errors, the error is counted but the run continues.
- The reporter flags any scenario where the error rate exceeds 1%.
- Optimistic locking failures during `CreditWallet` are retried inside the benchmark client to reflect realistic behavior.

## 9. CI Integration

- A new GitHub Actions workflow `.github/workflows/benchmark.yml` runs `make bench` on every push to `main`.
- The workflow runs on a fixed `ubuntu-latest` runner so the numbers are comparable across commits.
- On success, the workflow commits the updated `BENCHMARKS.md` with a `[bench]` prefix.
- The README links to `BENCHMARKS.md` so users can see the latest baseline without running anything.

## 10. Testing

- The benchmark suite itself uses Go `testing.B` and standard `benchstat` where possible.
- A smoke test `bench/smoke_test.go` verifies that all benchmark scenarios can complete at least one iteration against a real stack.
- The smoke test runs in CI before the full benchmark suite to catch regressions quickly.

## 11. Files Added

- `bench/runner.go`
- `bench/wallet_client.go`
- `bench/http_client.go`
- `bench/reporter.go`
- `bench/xcash_webhook.go`
- `bench/wallet_credit_test.go`
- `bench/gateway_read_test.go`
- `bench/webhook_ingest_test.go`
- `bench/e2e_deposit_test.go`
- `bench/smoke_test.go`
- `docker-compose.bench.yml`
- `scripts/bench.sh`
- `Makefile` updates
- `.github/workflows/benchmark.yml`
- `BENCHMARKS.md` (auto-generated)

## 12. Future Work

- Add throughput-vs-latency curves (variable concurrency levels) once the baseline is stable.
- Add stress tests that run for 5 minutes and measure memory/Goroutine growth.
- Add benchmarking for withdrawal flows once automated withdrawals are implemented.
- Add Prometheus scrape support for the benchmark stack to visualize resource usage.

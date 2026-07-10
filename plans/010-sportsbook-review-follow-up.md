# Plan 010: Address remaining sportsbook code-review findings

> **Executor instructions**: Follow this plan step by step. Run every verification command and confirm the expected result before moving to the next step. If anything in the "STOP conditions" section occurs, stop and report — do not improvise. When done, update the status row for this plan in `plans/README.md`.
>
> **Drift check (run first)**: `git diff --stat e6fa214..HEAD -- cmd/sportsbook/main.go internal/sportsbook/server.go internal/sportsbook/pgstore.go internal/sportsbook/memory_store.go internal/gateway/server.go`
> If any in-scope file changed since this plan was written, compare the "Current state" excerpts against the live code before proceeding; on a mismatch, treat it as a STOP condition.

## Status

- **Priority**: P1
- **Effort**: S
- **Risk**: MED
- **Depends on**: Plans 007, 008, 009
- **Category**: bug / correctness
- **Planned at**: commit `e6fa214`, 2026-07-11
- **Issue**: (none)

## Why this matters

A code review raised a HIGH-severity finding about missing auth on the sportsbook gRPC server. The core auth issue was already fixed in Plan 007 (`AuthInterceptor` + caller metadata). This plan addresses the still-valid follow-up items from that review: startup timeout, gRPC/HTTP error mapping, and store-layer consistency issues.

## Current state

- `cmd/sportsbook/main.go:33` — `db.Ping()` is unbounded; a hanging Postgres connection will block startup indefinitely.
- `internal/sportsbook/server.go:63-97` — handlers return raw Go errors, not gRPC status codes, so callers see `Internal` for validation, not-found, and conflict cases.
- `internal/gateway/server.go:569-654` — sportsbook REST handlers return HTTP 500 for every error, including client errors like `InvalidArgument` or `NotFound`.
- `internal/sportsbook/pgstore.go:142-158` — `UpdateBetStatus` always writes `settled_at`, even when `settledAt.IsZero()`; `InMemoryStore.UpdateBetStatus` only updates the field when non-zero.
- `internal/sportsbook/memory_store.go:169-197` — `ListPendingBets` sorts ascending; `PGStore.ListPendingBets` sorts ascending too (`ORDER BY created_at ASC`), which is correct for the scheduler, but the two implementations should be explicitly aligned and documented.
- `internal/sportsbook/pgstore.go:214-231` — same as above; ordering is ASC.

## Commands you will need

| Purpose | Command | Expected on success |
|---------|---------|---------------------|
| Build sportsbook | `go build ./cmd/sportsbook` | exit 0 |
| Run sportsbook tests | `go test -race ./internal/sportsbook/...` | exit 0 |
| Run gateway tests | `go test -race ./internal/gateway/...` | exit 0 |
| Run all tests | `go test -race ./...` | exit 0 |
| Build all services | `make build` | exit 0 |
| Format check | `gofmt -d $(find internal/sportsbook -name '*.go') internal/gateway/server.go cmd/sportsbook/main.go` | empty output |

## Scope

**In scope** (files you may modify):
- `cmd/sportsbook/main.go` — add bounded timeout to `db.Ping`.
- `internal/sportsbook/server.go` — add gRPC status mapping helper and use it in all four handlers.
- `internal/gateway/server.go` — map sportsbook gRPC status codes to HTTP statuses in the four sportsbook handlers.
- `internal/sportsbook/pgstore.go` — preserve existing `settled_at` when `settledAt.IsZero()` in `UpdateBetStatus` (and `UpdateBetStatusAndDebitTx`/`UpdateBetStatusAndOutcome` if they share the same issue).
- `internal/sportsbook/memory_store.go` — add a comment documenting that `ListPendingBets` returns oldest-first.
- `internal/sportsbook/server_test.go` — update or add tests for the new error mapping.
- `internal/gateway/server_test.go` — update or add tests for HTTP status mapping.

**Out of scope** (do NOT touch):
- Auth logic (already fixed in Plan 007).
- Wallet client metadata (already fixed in Plan 007).
- Sportsbook money-safety flow (already fixed in Plan 008).
- Docs (already fixed in Plan 009).
- Graceful shutdown, Dockerfile unprivileged user, settlement `reviewedBy` audit field, composite DB indexes, query refactoring — these are valid nitpicks but larger than this focused follow-up; defer to future plans.

## Git workflow

- Branch: `advisor/010-sportsbook-review-follow-up`
- Commit message style: `fix(sportsbook): ...`, `fix(gateway): ...`.
- Do NOT push or open a PR unless the operator instructed it.

## Steps

### Step 1: Add bounded timeout to sportsbook database ping

In `cmd/sportsbook/main.go`, replace:

```go
if err := db.Ping(); err != nil {
```

with:

```go
pingCtx, pingCancel := context.WithTimeout(context.Background(), 10*time.Second)
defer pingCancel()
if err := db.PingContext(pingCtx); err != nil {
```

**Verify**: `go build ./cmd/sportsbook` → compiles.

### Step 2: Add gRPC status mapping helper in sportsbook server

In `internal/sportsbook/server.go`, add a helper function:

```go
func mapServiceError(err error) error {
    if err == nil {
        return nil
    }
    switch {
    case errors.Is(err, ErrBetNotFound):
        return status.Error(codes.NotFound, err.Error())
    case errors.Is(err, ErrDuplicateReference): // if introduced; otherwise skip
        return status.Error(codes.AlreadyExists, err.Error())
    }
    // Treat explicit validation messages as InvalidArgument.
    msg := err.Error()
    if strings.HasPrefix(msg, "missing") || strings.HasPrefix(msg, "invalid") || strings.HasPrefix(msg, "stake") {
        return status.Error(codes.InvalidArgument, msg)
    }
    return status.Error(codes.Internal, msg)
}
```

Update all four `GRPCServer` handlers to wrap errors with `mapServiceError`:

```go
func (s *GRPCServer) PlaceBet(ctx context.Context, req *pb.PlaceBetRequest) (*pb.PlaceBetResponse, error) {
    bet, err := s.svc.PlaceBet(...)
    if err != nil {
        return nil, mapServiceError(err)
    }
    return &pb.PlaceBetResponse{Bet: betToProto(bet)}, nil
}
```

Do the same for `GetBet`, `ListBets`, `SettleBet`.

Add `errors` and `strings` imports if needed.

**Verify**: `go build ./internal/sportsbook/...` → compiles.

### Step 3: Map sportsbook gRPC errors to HTTP statuses in gateway

In `internal/gateway/server.go`, add or reuse a helper that converts gRPC status codes to HTTP status codes:

```go
func grpcStatusToHTTP(err error) int {
    if err == nil {
        return http.StatusOK
    }
    st, ok := status.FromError(err)
    if !ok {
        return http.StatusInternalServerError
    }
    switch st.Code() {
    case codes.InvalidArgument:
        return http.StatusBadRequest
    case codes.NotFound:
        return http.StatusNotFound
    case codes.AlreadyExists:
        return http.StatusConflict
    case codes.Unauthenticated:
        return http.StatusUnauthorized
    case codes.PermissionDenied:
        return http.StatusForbidden
    case codes.FailedPrecondition:
        return http.StatusConflict
    default:
        return http.StatusInternalServerError
    }
}
```

Update `handlePlaceBet`, `handleListBets`, `handleGetBet`, and `handleSettleBet` to call `s.writeError(w, grpcStatusToHTTP(err), err)` instead of always `http.StatusInternalServerError`.

**Verify**: `go build ./internal/gateway/...` → compiles.

### Step 4: Preserve existing `settled_at` when update passes zero time

In `internal/sportsbook/pgstore.go`, change `UpdateBetStatus` so a zero `settledAt` does not overwrite an existing value:

```go
func (s *PGStore) UpdateBetStatus(ctx context.Context, id, status string, settledAt time.Time) error {
    var q string
    var args []any
    if settledAt.IsZero() {
        q = `UPDATE bets SET status = $1 WHERE id = $2 RETURNING id`
        args = []any{status, id}
    } else {
        q = `UPDATE bets SET status = $1, settled_at = $2 WHERE id = $3 RETURNING id`
        args = []any{status, settledAt, id}
    }
    var returnedID string
    err := s.db.QueryRowContext(ctx, q, args...).Scan(&returnedID)
    // ... rest unchanged
}
```

Apply the same zero-time preservation to `UpdateBetStatusAndDebitTx` and `UpdateBetStatusAndOutcome`.

**Verify**: `go test -race ./internal/sportsbook/... -run TestPGStoreUpdateBetStatus` → passes.

### Step 5: Document ListPendingBets ordering

In `internal/sportsbook/memory_store.go`, add a comment above `ListPendingBets`:

```go
// ListPendingBets returns pending bets ordered oldest-first (created_at ASC),
// matching PGStore and the scheduler's expectations.
```

In `internal/sportsbook/pgstore.go`, add the same comment above `PGStore.ListPendingBets`.

No code change needed if both already order ASC (they do).

**Verify**: `go test -race ./internal/sportsbook/... -run TestSchedulerPaginatesPendingBets` → passes.

### Step 6: Update tests for error mapping

In `internal/sportsbook/server_test.go`, add or update tests that assert:
- Calling `GetBet` with a non-existent ID returns `codes.NotFound`.
- Calling `PlaceBet` with missing fields returns `codes.InvalidArgument`.
- Calling `SettleBet` on an already-settled bet returns `codes.FailedPrecondition` or `codes.AlreadyExists` (choose the one that matches the service error).

In `internal/gateway/server_test.go`, add or update tests that assert the gateway returns the correct HTTP status for the mapped gRPC codes.

**Verify**: `go test -race ./internal/sportsbook/... -run TestGRPC` → passes.

**Verify**: `go test -race ./internal/gateway/... -run TestGatewayPlaceBet|TestGatewayListBets|TestGatewaySettle` → passes.

### Step 7: Full verification

**Verify**: `go test -race ./internal/sportsbook/...` → passes.

**Verify**: `go test -race ./internal/gateway/...` → passes.

**Verify**: `go test -race ./...` → passes.

**Verify**: `make build` → exits 0.

**Verify**: `gofmt -d $(find internal/sportsbook -name '*.go') internal/gateway/server.go cmd/sportsbook/main.go` → empty output.

**Verify**: `git status --short` → only in-scope files modified.

## Test plan

- `TestSportsbookDBPingTimeout` (optional manual test via a blocked proxy is not required; compile verification suffices).
- `TestGRPCPlaceBetValidation` — `PlaceBet` with empty user id returns `codes.InvalidArgument`.
- `TestGRPCGetBetNotFound` — unknown bet id returns `codes.NotFound`.
- `TestGatewayPlaceBetReturnsBadRequest` — gateway maps `InvalidArgument` to 400.
- `TestGatewayGetBetReturnsNotFound` — gateway maps `NotFound` to 404.
- `TestPGStoreUpdateBetStatusPreservesSettledAt` — zero settledAt does not overwrite an existing value.

## Done criteria

- [ ] `cmd/sportsbook/main.go` uses `PingContext` with a bounded timeout.
- [ ] `internal/sportsbook/server.go` maps service errors to gRPC status codes.
- [ ] `internal/gateway/server.go` maps sportsbook gRPC status codes to HTTP statuses.
- [ ] `PGStore.UpdateBetStatus` preserves existing `settled_at` when passed a zero time.
- [ ] `ListPendingBets` ordering is documented in both store implementations.
- [ ] New/updated tests cover error mapping.
- [ ] `go test -race ./...` exits 0.
- [ ] `make build` exits 0.
- [ ] `gofmt` clean on modified files.
- [ ] Only in-scope files modified.
- [ ] `plans/README.md` status row updated to `DONE`.

## STOP conditions

Stop and report back (do not improvise) if:

- The current code already has gRPC status mapping (verify by reading `internal/sportsbook/server.go`).
- `db.Ping` already uses a timeout.
- Gateway sportsbook handlers already inspect gRPC status codes.
- A verification fails twice after a reasonable fix attempt.

## Maintenance notes

- The deferred larger items (graceful shutdown, Dockerfile unprivileged user, settlement `reviewedBy`, composite indexes, ListBets query consolidation) should be picked up in separate plans once this focused follow-up lands.
- Future changes to sportsbook service errors should route through `mapServiceError` so the gateway HTTP mapping stays consistent.

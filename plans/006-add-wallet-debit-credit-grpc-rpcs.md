# Plan 006: Add DebitWallet and CreditWallet gRPC RPCs to the wallet service

> **Executor instructions**: Follow this plan step by step. Run every verification command and confirm the expected result before moving to the next step. If anything in the "STOP conditions" section occurs, stop and report — do not improvise. When done, update the status row for this plan in `plans/README.md`.
>
> **Drift check (run first)**: `git diff --stat a4cb580..HEAD -- internal/proto/wallet.proto internal/proto/wallet.pb.go internal/proto/wallet_grpc.pb.go internal/wallet/server.go internal/wallet/service.go internal/wallet/store.go internal/wallet/memory_store.go internal/wallet/pgstore.go internal/wallet/server_test.go internal/gateway/wallet_client.go`
> If any in-scope file changed since this plan was written, compare the "Current state" excerpts against the live code before proceeding; on a mismatch, treat it as a STOP condition.

## Status

- **Priority**: P0
- **Effort**: M
- **Risk**: HIGH (handles real money)
- **Depends on**: none
- **Category**: feature
- **Planned at**: commit `a4cb580`, 2026-07-10
- **Issue**: (none)

## Why this matters

The Sportsbook service (Plan 005) needs to debit stakes and credit winnings via the Wallet service. Currently the Wallet only exposes `RequestWithdrawal` (which debits for a pending withdrawal) and `ProcessDepositWebhook` (which credits from xcash). There is no general-purpose, idempotent RPC to debit or credit a wallet for bet stakes and settlements. This plan adds `DebitWallet` and `CreditWallet` RPCs so downstream services can move money safely.

## Current state

- `internal/proto/wallet.proto` defines: `GetRates`, `GetBalance`, `ListTransactions`, `GetDepositAddress`, `RequestWithdrawal`, `ProcessDepositWebhook`, `ListPendingWithdrawals`, `ReviewWithdrawal`.
- `internal/wallet/service.go` already has internal `DebitWallet` and `CreditWallet` functions used by withdrawal and webhook flows.
- `internal/wallet/server.go` exposes the gRPC handlers but does not surface the debit/credit functions as RPCs.
- `internal/gateway/wallet_client.go` wraps the wallet gRPC client; it will need the new methods after this plan.

## Commands you will need

| Purpose | Command | Expected on success |
|---------|---------|---------------------|
| Regenerate protobuf | `make proto` | exit 0 |
| Run wallet tests | `go test -race ./internal/wallet/...` | exit 0 |
| Run gateway tests | `go test -race ./internal/gateway/...` | exit 0 |
| Run all tests | `go test -race ./...` | exit 0 |
| Build all services | `make build` | exit 0 |

## Scope

**In scope** (files you may modify):
- `internal/proto/wallet.proto` — add `DebitWallet` and `CreditWallet` RPCs + messages.
- `internal/proto/wallet*.pb.go` — regenerate via `make proto`.
- `internal/wallet/server.go` — implement the new gRPC handlers.
- `internal/wallet/server_test.go` — add gRPC contract tests for debit/credit.
- `internal/gateway/wallet_client.go` — add `DebitWallet` and `CreditWallet` methods.
- `Makefile` — ensure `make proto` includes `wallet.proto` (it likely already does).

**Out of scope** (do NOT touch):
- Internal wallet business logic in `internal/wallet/service.go` — reuse it, do not rewrite.
- Frontend code.
- Sportsbook code (that comes after this plan).

## Git workflow

- Branch: `advisor/006-add-wallet-debit-credit-grpc-rpcs`
- Commit message style: `feat(wallet): ...` (repo uses conventional commits).
- Do NOT push or open a PR unless the operator instructed it.

## Steps

### Step 1: Add DebitWallet and CreditWallet to the protobuf contract

In `internal/proto/wallet.proto`, add to the service:

```protobuf
  rpc DebitWallet(DebitWalletRequest) returns (DebitWalletResponse);
  rpc CreditWallet(CreditWalletRequest) returns (CreditWalletResponse);
```

Add messages:

```protobuf
message DebitWalletRequest {
  string user_id = 1;
  string currency = 2;
  string amount = 3;
  string reference_id = 4;
  string metadata = 5;
}
message DebitWalletResponse {
  string transaction_id = 1;
  string status = 2;
}

message CreditWalletRequest {
  string user_id = 1;
  string currency = 2;
  string amount = 3;
  string reference_id = 4;
  string metadata = 5;
}
message CreditWalletResponse {
  string transaction_id = 1;
  string status = 2;
}
```

Run `make proto` to regenerate.

**Verify**: `grep -q "DebitWallet\|CreditWallet" internal/proto/wallet.proto internal/proto/wallet_grpc.pb.go` → matches.

**Verify**: `go build ./internal/proto/...` → compiles.

### Step 2: Implement the gRPC handlers

In `internal/wallet/server.go`, add two methods to the gRPC server:

```go
func (s *GRPCServer) DebitWallet(ctx context.Context, req *pb.DebitWalletRequest) (*pb.DebitWalletResponse, error) {
    tx, err := s.svc.DebitWallet(ctx, req.UserId, req.Currency, req.Amount, req.ReferenceId, req.Metadata)
    if err != nil {
        return nil, fmt.Errorf("debit wallet: %w", err)
    }
    return &pb.DebitWalletResponse{TransactionId: tx.ID, Status: tx.Status}, nil
}

func (s *GRPCServer) CreditWallet(ctx context.Context, req *pb.CreditWalletRequest) (*pb.CreditWalletResponse, error) {
    tx, err := s.svc.CreditWallet(ctx, req.UserId, req.Currency, req.Amount, req.ReferenceId, req.Metadata)
    if err != nil {
        return nil, fmt.Errorf("credit wallet: %w", err)
    }
    return &pb.CreditWalletResponse{TransactionId: tx.ID, Status: tx.Status}, nil
}
```

The exact function signatures of `DebitWallet` and `CreditWallet` in `internal/wallet/service.go` may differ. Open the file and match the existing signatures. If they don't take a metadata string, either add it or pass an empty string. Do not change the existing call sites in `service.go` unless necessary.

**Verify**: `go build ./internal/wallet/...` → compiles.

**Verify**: `go test -race ./internal/wallet/...` → passes.

### Step 3: Add gateway client methods

In `internal/gateway/wallet_client.go`, add:

```go
func (c *WalletClient) DebitWallet(ctx context.Context, userID, currency, amount, referenceID, metadata string) (*pb.DebitWalletResponse, error) {
    return c.client.DebitWallet(ctx, &pb.DebitWalletRequest{UserId: userID, Currency: currency, Amount: amount, ReferenceId: referenceID, Metadata: metadata})
}

func (c *WalletClient) CreditWallet(ctx context.Context, userID, currency, amount, referenceID, metadata string) (*pb.CreditWalletResponse, error) {
    return c.client.CreditWallet(ctx, &pb.CreditWalletRequest{UserId: userID, Currency: currency, Amount: amount, ReferenceId: referenceID, Metadata: metadata})
}
```

Adjust to match the actual `WalletClient` struct and field names in the file.

**Verify**: `go build ./internal/gateway/...` → compiles.

**Verify**: `go test -race ./internal/gateway/...` → passes.

### Step 4: Add gRPC contract tests

In `internal/wallet/server_test.go`, add tests similar to the existing `TestGRPCServerGetBalance`:

1. `TestGRPCServerDebitWallet` — seed a wallet with balance, call `DebitWallet`, assert the response status and that a subsequent `GetBalance` shows the reduced balance.
2. `TestGRPCServerCreditWallet` — call `CreditWallet`, assert response status and increased balance.
3. `TestGRPCServerDebitWalletIdempotent` — call `DebitWallet` twice with the same `reference_id`; assert the second call succeeds without double-debiting.

Use the existing pattern of spinning up a `bufconn` gRPC server and a real Postgres test database (check how `TestPGStoreIntegration` or existing server tests set up the DB).

**Verify**: `go test -race ./internal/wallet/... -run TestGRPCServerDebitWallet` → passes.

**Verify**: `go test -race ./internal/wallet/... -run TestGRPCServerCreditWallet` → passes.

**Verify**: `go test -race ./internal/wallet/... -run TestGRPCServerDebitWalletIdempotent` → passes.

### Step 5: Run full verification suite

**Verify**: `go test -race ./internal/wallet/...` → passes.

**Verify**: `go test -race ./internal/gateway/...` → passes.

**Verify**: `go test -race ./...` → passes.

**Verify**: `make build` → exits 0.

**Verify**: `gofmt -d internal/proto/wallet.proto internal/wallet/server.go internal/wallet/server_test.go internal/gateway/wallet_client.go` → empty output.

**Verify**: `git status --short` → only in-scope files modified.

## Test plan

- New tests in `internal/wallet/server_test.go` for debit/credit gRPC handlers.
- Idempotency test for duplicate `reference_id` debits.
- Existing wallet tests must continue to pass.
- Verification: `go test -race ./...` passes.

## Done criteria

- [ ] `wallet.proto` has `DebitWallet` and `CreditWallet` RPCs.
- [ ] Generated `.pb.go` files include the new RPCs.
- [ ] `internal/wallet/server.go` implements the new handlers.
- [ ] `internal/gateway/wallet_client.go` exposes `DebitWallet` and `CreditWallet` methods.
- [ ] New tests for debit, credit, and idempotency pass.
- [ ] `go test -race ./...` exits 0.
- [ ] `make build` exits 0.
- [ ] `gofmt` clean on modified files.
- [ ] Only in-scope files modified.
- [ ] `plans/README.md` status row updated to `DONE` and Plan 005 dependency noted.

## STOP conditions

Stop and report back (do not improvise) if:

- The internal `DebitWallet`/`CreditWallet` signatures in `internal/wallet/service.go` cannot be called from a gRPC handler without changing existing behavior.
- `make proto` fails or produces broken generated code.
- Adding the RPCs requires changing the wallet database schema.
- A test fails twice after a reasonable fix attempt.
- You need to modify files outside the in-scope list.

## Maintenance notes

- These RPCs are intended for internal service use only (Sportsbook, Settlement, Casino). The gateway should expose them only through domain-specific routes (e.g., `POST /api/bets` places a bet, which internally debits the wallet), not as generic debit/credit endpoints.
- `reference_id` is the idempotency key. Callers must generate unique reference IDs per operation.
- When Plan 005 (Sportsbook) runs, it should depend on this plan being done.

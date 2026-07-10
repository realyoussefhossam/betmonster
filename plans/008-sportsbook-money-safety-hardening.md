# Plan 008: Harden sportsbook bet placement and settlement money safety

> **Executor instructions**: Follow this plan step by step. Run every verification command and confirm the expected result before moving to the next step. If anything in the "STOP conditions" section occurs, stop and report — do not improvise. When done, update the status row for this plan in `plans/README.md`.
>
> **Drift check (run first)**: `git diff --stat f3d0727..HEAD -- internal/sportsbook/`
> If any in-scope file changed since this plan was written, compare the "Current state" excerpts against the live code before proceeding; on a mismatch, treat it as a STOP condition.

## Status

- **Priority**: P1
- **Effort**: M
- **Risk**: HIGH (money safety)
- **Depends on**: Plan 007 (sportsbook gRPC auth must work before testing wallet debit/credit paths)
- **Category**: correctness / security
- **Planned at**: commit `f3d0727`, 2026-07-11
- **Issue**: (none)

## Why this matters

The current `PlaceBet` flow debits the wallet and only then inserts the bet record. If the insert fails (network blip, unique-constraint race, DB outage), the user's stake has been debited with no bet record to settle, violating the `AGENTS.md` reversibility guarantee. `SettleBet` credits the wallet and then updates bet status; if the status update fails, a retry will see the bet as still pending and credit again, creating a double-payout risk. Finally, the scheduler fetches every pending bet with no pagination or failure logging, so it will slow down and silently drop settlement errors as the book grows.

## Current state

- `internal/sportsbook/service.go:121-153` — `PlaceBet` calls `s.wallet.DebitWallet` first, then `s.store.CreateBet`.
- `internal/sportsbook/service.go:170-219` — `SettleBet` calls `s.wallet.CreditWallet` first, then `s.store.UpdateBetStatus`.
- `internal/sportsbook/pgstore.go:160-191` — `ListPendingBets` has no `LIMIT`/`OFFSET`.
- `internal/sportsbook/models.go` — the `Bet` model has no fields linking to the wallet ledger transactions.
- `internal/sportsbook/scheduler.go` — the scheduler swallows settlement errors.
- Wallet `DebitWallet`/`CreditWallet` RPCs return a `transaction_id` in their responses.

## Commands you will need

| Purpose | Command | Expected on success |
|---------|---------|---------------------|
| Build sportsbook | `go build ./cmd/sportsbook` | exit 0 |
| Run sportsbook tests | `go test -race ./internal/sportsbook/...` | exit 0 |
| Run all tests | `go test -race ./...` | exit 0 |
| Build all services | `make build` | exit 0 |
| Format check | `gofmt -d $(find internal/sportsbook -name '*.go')` | empty output |

## Scope

**In scope** (files you may modify):
- `internal/sportsbook/migrations/20260710120000_create_sportsbook_schema.up.sql` and `.down.sql`
- `internal/sportsbook/migrations/20260711120000_add_bet_wallet_tx_ids.up.sql` and `.down.sql` (new migration)
- `internal/sportsbook/models.go`
- `internal/sportsbook/store.go`
- `internal/sportsbook/memory_store.go`
- `internal/sportsbook/pgstore.go`
- `internal/sportsbook/service.go`
- `internal/sportsbook/clients.go` (to capture returned transaction ids)
- `internal/sportsbook/server.go` (if status enum changes affect proto mapping)
- `internal/sportsbook/scheduler.go`
- `internal/sportsbook/service_test.go`
- `internal/sportsbook/server_test.go`
- `internal/sportsbook/pgstore_integration_test.go`

**Out of scope** (do NOT touch):
- Wallet service business logic.
- Gateway REST routes or sportsbook gRPC auth (handled in Plan 007).
- Frontend code.

## Git workflow

- Branch: `advisor/008-sportsbook-money-safety-hardening`
- Commit message style: `fix(sportsbook): ...` or `feat(sportsbook): ...`.
- Do NOT push or open a PR unless the operator instructed it.

## Steps

### Step 1: Add wallet transaction id fields to the bet record

Add to `internal/sportsbook/models.go`:

```go
type Bet struct {
    ID                  string
    UserID              string
    EventID             string
    MarketID            string
    OutcomeID           string
    Odds                string
    Stake               string
    PotentialPayout     string
    Currency            string
    Status              string
    ReferenceID         string
    DebitTransactionID  string
    CreditTransactionID string
    CreatedAt           time.Time
    SettledAt           *time.Time
}
```

Add a new migration `internal/sportsbook/migrations/20260711120000_add_bet_wallet_tx_ids.up.sql`:

```sql
ALTER TABLE bets
    ADD COLUMN debit_transaction_id text,
    ADD COLUMN credit_transaction_id text;
```

And the matching `.down.sql` removing the columns.

Update `internal/sportsbook/pgstore.go` to scan and insert these columns. The existing `SELECT ... FROM bets` and `INSERT INTO bets` queries must include the two new columns. Update `memory_store.go` similarly.

Update `internal/sportsbook/server.go` `betToProto` to include the new fields only if they are non-empty (the proto may also need new fields; see Step 5).

**Verify**: `go build ./internal/sportsbook/...` → compiles.

### Step 2: Update the wallet client interface to return transaction ids

In `internal/sportsbook/clients.go`, change the local `WalletClient` interface so `DebitWallet` and `CreditWallet` return the wallet transaction id string:

```go
type WalletClient interface {
    DebitWallet(ctx context.Context, userID, currency, amount, referenceID string, metadata map[string]any) (string, error)
    CreditWallet(ctx context.Context, userID, currency, amount, referenceID string, metadata map[string]any) (string, error)
}
```

Update `GRPCWalletClient` to capture `resp.TransactionId` and return it. Update `noopWalletClient` to return an empty string.

**Verify**: `go build ./internal/sportsbook/...` → compiles.

### Step 3: Make `PlaceBet` record the bet before debiting and reverse on failure

Rewrite `internal/sportsbook/service.go` `PlaceBet` roughly as follows:

```go
const StatusDebitPending = "debit_pending"

func (s *Service) PlaceBet(...) (Bet, error) {
    // ... existing validation ...

    bet := Bet{
        UserID:          userID,
        EventID:         eventID,
        MarketID:        marketID,
        OutcomeID:       outcomeID,
        Odds:            odds,
        Stake:           stake,
        PotentialPayout: potentialPayout,
        Currency:        currency,
        Status:          StatusDebitPending,
        ReferenceID:     referenceID,
        CreatedAt:       time.Now(),
    }

    id, err := s.store.CreateBet(ctx, bet)
    if err != nil {
        if errors.Is(err, ErrBetNotFound) {
            // Existing bet with same reference; return it.
            existing, err := s.store.GetBetByReference(ctx, userID, referenceID)
            if err != nil {
                return Bet{}, fmt.Errorf("create bet conflict: %w", err)
            }
            return *existing, nil
        }
        return Bet{}, fmt.Errorf("create bet: %w", err)
    }
    bet.ID = id

    debitTxID, err := s.wallet.DebitWallet(ctx, userID, currency, stake, referenceID, metadata)
    if err != nil {
        // Mark as cancelled so the record reflects the failed stake hold.
        _ = s.store.UpdateBetStatus(ctx, bet.ID, StatusCancelled, time.Now())
        return Bet{}, fmt.Errorf("debit stake: %w", err)
    }

    bet.DebitTransactionID = debitTxID
    if err := s.store.UpdateBetStatusAndDebitTx(ctx, bet.ID, StatusPending, debitTxID, time.Time{}); err != nil {
        // The debit succeeded but we couldn't record it. Leave in debit_pending for operator review/retry.
        return Bet{}, fmt.Errorf("finalize bet after debit: %w", err)
    }

    bet.Status = StatusPending
    return bet, nil
}
```

Add `StatusDebitPending` to the `models.go` status constants. Add `UpdateBetStatusAndDebitTx` to the `Store` interface:

```go
UpdateBetStatusAndDebitTx(ctx context.Context, id, status, debitTxID string, settledAt time.Time) error
```

Implement it in `memory_store.go` and `pgstore.go` (PGStore updates both status and debit_transaction_id in one query). Also update the migration's `CHECK` constraint to include `'debit_pending'` if the schema uses a check constraint; otherwise the proto enum can remain unchanged.

Add a test that simulates `CreateBet` succeeding but `DebitWallet` failing, asserting the bet ends in `cancelled`.

**Verify**: `go test -race ./internal/sportsbook/... -run TestPlaceBet` → passes.

### Step 4: Make `SettleBet` update status before crediting and record the credit transaction

Rewrite `internal/sportsbook/service.go` `SettleBet` roughly as follows:

```go
func (s *Service) SettleBet(ctx context.Context, betID, outcome string) (Bet, error) {
    // ... validation ...

    bet, err := s.store.GetBet(ctx, betID)
    if err != nil {
        return Bet{}, fmt.Errorf("get bet: %w", err)
    }
    if bet.Status != StatusPending {
        return *bet, fmt.Errorf("bet cannot be settled: status=%s", bet.Status)
    }

    now := time.Now()
    switch outcome {
    case StatusWon:
        // Mark settled first so a retry cannot double-credit.
        if err := s.store.UpdateBetStatusAndOutcome(ctx, bet.ID, StatusWon, now); err != nil {
            return Bet{}, fmt.Errorf("mark bet won: %w", err)
        }
        creditTxID, err := s.wallet.CreditWallet(ctx, bet.UserID, bet.Currency, bet.PotentialPayout, "settle-"+bet.ReferenceID, metadata)
        if err != nil {
            // Bet is marked won but credit failed; leave credit_transaction_id empty for retry/review.
            return Bet{}, fmt.Errorf("credit winnings: %w", err)
        }
        if err := s.store.SetCreditTransactionID(ctx, bet.ID, creditTxID); err != nil {
            return Bet{}, fmt.Errorf("record credit tx: %w", err)
        }
    case StatusLost:
        if err := s.store.UpdateBetStatusAndOutcome(ctx, bet.ID, StatusLost, now); err != nil {
            return Bet{}, fmt.Errorf("mark bet lost: %w", err)
        }
    case StatusCancelled:
        if err := s.store.UpdateBetStatusAndOutcome(ctx, bet.ID, StatusCancelled, now); err != nil {
            return Bet{}, fmt.Errorf("mark bet cancelled: %w", err)
        }
        creditTxID, err := s.wallet.CreditWallet(ctx, bet.UserID, bet.Currency, bet.Stake, "cancel-"+bet.ReferenceID, metadata)
        if err != nil {
            return Bet{}, fmt.Errorf("credit refund: %w", err)
        }
        if err := s.store.SetCreditTransactionID(ctx, bet.ID, creditTxID); err != nil {
            return Bet{}, fmt.Errorf("record refund tx: %w", err)
        }
    }

    return *s.mustGetBet(ctx, bet.ID), nil
}
```

Add to the `Store` interface:

```go
UpdateBetStatusAndOutcome(ctx context.Context, id, status string, settledAt time.Time) error
SetCreditTransactionID(ctx context.Context, id, creditTxID string) error
```

Implement them in both stores.

Add a test that verifies settling a winning bet twice does not call `CreditWallet` twice (the second attempt should return the already-settled bet without a credit).

**Verify**: `go test -race ./internal/sportsbook/... -run TestSettle` → passes.

### Step 5: Bound `ListPendingBets` and improve scheduler logging

Add pagination to `ListPendingBets`:

```go
ListPendingBets(ctx context.Context, page, pageSize int) ([]Bet, error)
```

Implement in both stores with `LIMIT`/`OFFSET`.

Update `internal/sportsbook/scheduler.go` to:
- Call `ListPendingBets` in pages.
- Log errors from `resolveOutcomeStatus` and `SettleBet` instead of silently continuing.
- Optionally add a per-tick limit to avoid runaway settlement loops.

Example shape:

```go
const schedulerPageSize = 100

func (s *Scheduler) settlePending(ctx context.Context) {
    page := 1
    for {
        bets, err := s.svc.ListPendingBets(ctx, page, schedulerPageSize)
        if err != nil {
            s.logger.Error("list pending bets", slog.String("error", err.Error()))
            return
        }
        if len(bets) == 0 {
            return
        }
        for _, bet := range bets {
            if _, err := s.svc.SettleBet(ctx, bet.ID, outcome); err != nil {
                s.logger.Error("auto settle bet", slog.String("bet_id", bet.ID), slog.String("error", err.Error()))
            }
        }
        if len(bets) < schedulerPageSize {
            return
        }
        page++
    }
}
```

**Verify**: `go test -race ./internal/sportsbook/... -run TestAutoSettle` → passes.

### Step 6: Update proto and generated code if exposing new bet fields

If you added `debit_transaction_id` and `credit_transaction_id` to `internal/sportsbook/models.go`, also add them to `internal/proto/sportsbook.proto` `Bet` message, regenerate with `make proto`, and update `betToProto`.

**Verify**: `make proto` → exits 0.

**Verify**: `go build ./internal/proto/...` → compiles.

### Step 7: Full verification

**Verify**: `go test -race ./internal/sportsbook/...` → passes.

**Verify**: `go test -race ./...` → passes.

**Verify**: `make build` → exits 0.

**Verify**: `gofmt -d $(find internal/sportsbook -name '*.go')` → empty output.

**Verify**: `git status --short` → only in-scope files modified.

## Test plan

- `TestPlaceBetRecordsBetBeforeDebit` — verify bet exists in `debit_pending` before wallet call.
- `TestPlaceBetReversesWhenDebitFails` — verify a failed debit leaves the bet as `cancelled`.
- `TestPlaceBetIdempotency` — unchanged behavior, duplicate reference returns the same bet.
- `TestSettleWinMarksWonBeforeCredit` — verify status transition prevents double credit.
- `TestSettleWinIsIdempotent` — second settle call does not invoke `CreditWallet` again.
- `TestSchedulerPaginatesPendingBets` — verify `ListPendingBets` honors page size.

## Done criteria

- [ ] `Bet` model and `bets` table store `debit_transaction_id` and `credit_transaction_id`.
- [ ] `PlaceBet` creates the bet in `debit_pending`, debits the wallet, then transitions to `pending`.
- [ ] Failed stake debit leaves the bet record in a terminal `cancelled` state.
- [ ] `SettleBet` marks the bet won/lost/cancelled before calling `CreditWallet`, preventing double payout on retry.
- [ ] `ListPendingBets` supports pagination.
- [ ] Scheduler logs settlement failures.
- [ ] New tests cover debit failure and settlement idempotency.
- [ ] `go test -race ./...` exits 0.
- [ ] `make build` exits 0.
- [ ] `gofmt` clean on modified files.
- [ ] Only in-scope files modified.
- [ ] `plans/README.md` status row updated to `DONE`.

## STOP conditions

Stop and report back (do not improvise) if:

- The wallet `DebitWallet`/`CreditWallet` signatures or response shapes differ from the plan's assumption.
- The existing `bets` table has production data and the migration cannot be applied cleanly.
- Adding `debit_pending` to the status enum conflicts with the proto `Bet.status` field constraints.
- A verification fails twice after a reasonable fix attempt.

## Maintenance notes

- Bets stuck in `debit_pending` (debit succeeded but finalization failed) should be surfaced in an admin dashboard and/or auto-reconciled in a future plan.
- The `debit_transaction_id`/`credit_transaction_id` fields enable reconciliation between the sportsbook and wallet ledgers.
- Future live-betting or cash-out flows must preserve the same "record state transition before external money call" ordering.

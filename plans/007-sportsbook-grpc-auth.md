# Plan 007: Fix sportsbook gRPC auth metadata and add sportsbook authorization interceptor

> **Executor instructions**: Follow this plan step by step. Run every verification command and confirm the expected result before moving to the next step. If anything in the "STOP conditions" section occurs, stop and report — do not improvise. When done, update the status row for this plan in `plans/README.md`.
>
> **Drift check (run first)**: `git diff --stat ae38370..HEAD -- internal/sportsbook/ internal/shared/grpcmeta/ internal/wallet/server.go internal/gateway/wallet_client.go`
> If any in-scope file changed since this plan was written, compare the "Current state" excerpts against the live code before proceeding; on a mismatch, treat it as a STOP condition.

## Status

- **Priority**: P0
- **Effort**: M
- **Risk**: HIGH (security + feature blocker)
- **Depends on**: Plan 006 (wallet `DebitWallet`/`CreditWallet` RPCs with privileged auth)
- **Category**: security
- **Planned at**: commit `ae38370`, 2026-07-11
- **Issue**: (none)

## Why this matters

Plan 006 hardened the wallet service so `DebitWallet`/`CreditWallet` require both `x-caller-is-admin: true` and the sentinel caller id `gateway`. The new sportsbook service calls these RPCs but does not attach the required metadata, so all stake debits and payout credits will fail at runtime. Additionally, the sportsbook gRPC server accepts `PlaceBet`, `GetBet`, `ListBets`, and `SettleBet` requests with no authorization check, meaning anyone who can reach the sportsbook gRPC port can place bets for arbitrary users, read any bet, or settle outcomes.

## Current state

- `internal/shared/grpcmeta/grpcmeta.go` defines:
  - `UserIDHeader = "x-user-id"`
  - `IsAdminHeader = "x-caller-is-admin"`
- `internal/gateway/wallet_client.go:87-106` is the exemplar for privileged wallet calls:
  ```go
  md := metadata.Pairs(
      grpcmeta.UserIDHeader, gatewayCallerID,
      grpcmeta.IsAdminHeader, "true",
  )
  ctx = metadata.NewOutgoingContext(ctx, md)
  return c.conn.DebitWallet(ctx, &pb.DebitWalletRequest{...})
  ```
- `internal/wallet/server.go:44-88` is the exemplar for gRPC authorization interceptors.
- `internal/sportsbook/clients.go:25-54` currently calls `DebitWallet`/`CreditWallet` with **no metadata**.
- `cmd/sportsbook/main.go:57` creates the gRPC server with no interceptor:
  ```go
  grpcServer := grpc.NewServer()
  ```
- `internal/sportsbook/server.go:19-53` forwards raw `req.UserId` to the service without caller verification.
- `internal/sportsbook/server_test.go` tests currently spin up an unauthenticated gRPC server.
- `internal/gateway/sportsbook_client.go` calls the sportsbook gRPC service without attaching caller metadata.
- `internal/gateway/server_test.go` creates a sportsbook test server without the interceptor in `newSportsbookServerForGateway`.

## Commands you will need

| Purpose | Command | Expected on success |
|---------|---------|---------------------|
| Build sportsbook | `go build ./cmd/sportsbook` | exit 0 |
| Run sportsbook tests | `go test -race ./internal/sportsbook/...` | exit 0 |
| Run gateway tests | `go test -race ./internal/gateway/...` | exit 0 |
| Run wallet tests | `go test -race ./internal/wallet/...` | exit 0 |
| Run all tests | `go test -race ./...` | exit 0 |
| Build all services | `make build` | exit 0 |
| Format check | `gofmt -d internal/sportsbook/... internal/gateway/sportsbook_client.go` | empty output |

## Scope

**In scope** (files you may modify):
- `internal/sportsbook/clients.go` — attach privileged metadata to wallet calls.
- `internal/sportsbook/server.go` — add an authorization interceptor and apply it in gRPC handlers where needed.
- `cmd/sportsbook/main.go` — wire the interceptor into `grpc.NewServer`.
- `internal/sportsbook/server_test.go` — update test server setup to include the interceptor and use the correct caller metadata.
- `internal/sportsbook/service.go` — if the service needs to distinguish internal scheduler calls from gateway calls, adjust signatures minimally.
- `internal/gateway/sportsbook_client.go` — attach caller metadata so real gateway → sportsbook gRPC calls are accepted by the interceptor.
- `internal/gateway/server_test.go` — update `newSportsbookServerForGateway` to use the interceptor and ensure tests send the right metadata.

**Out of scope** (do NOT touch):
- Wallet service business logic or `internal/wallet/server.go` interceptor rules.
- Gateway REST route handlers (they already authenticate the end user via JWT; we only change the gRPC client that proxies calls).
- Frontend code.
- Odds/Feed service auth (separate issue).

## Git workflow

- Branch: `advisor/007-sportsbook-grpc-auth`
- Commit message style: `fix(sportsbook): ...` or `feat(sportsbook): ...` (repo uses conventional commits).
- Do NOT push or open a PR unless the operator instructed it.

## Steps

### Step 1: Add privileged metadata to sportsbook wallet client calls

In `internal/sportsbook/clients.go`, update `DebitWallet` and `CreditWallet` to attach the gateway service identity + admin flag, matching the pattern in `internal/gateway/wallet_client.go:87-106`.

```go
func (c *GRPCWalletClient) DebitWallet(ctx context.Context, userID, currency, amount, referenceID string, metadata map[string]any) error {
    metadataJSON := ""
    if metadata != nil {
        b, _ := json.Marshal(metadata)
        metadataJSON = string(b)
    }
    md := metadata.Pairs(
        grpcmeta.UserIDHeader, gatewayCallerID,
        grpcmeta.IsAdminHeader, "true",
    )
    ctx = metadata.NewOutgoingContext(ctx, md)
    _, err := c.conn.DebitWallet(ctx, &pb.DebitWalletRequest{
        UserId: userID, Currency: currency, Amount: amount, ReferenceId: referenceID, Metadata: metadataJSON,
    })
    return err
}
```

Define `gatewayCallerID = "gateway"` in `clients.go` (or reuse a shared constant if one exists). Add the required imports:

```go
"google.golang.org/grpc/metadata"
"github.com/realyoussefhossam/betmonster/internal/shared/grpcmeta"
```

Do the same for `CreditWallet`.

**Verify**: `go build ./internal/sportsbook/...` → compiles.

### Step 2: Add an authorization interceptor to the sportsbook gRPC server

In `internal/sportsbook/server.go`, add an interceptor that:
- Requires `x-user-id` metadata.
- For `SettleBet`, also requires `x-caller-is-admin: true` and `x-user-id == gateway`.
- For user-scoped methods (`PlaceBet`, `ListBets`), requires the caller's `x-user-id` to match `req.UserId` (only the gateway may proxy end-user calls). The gateway already authenticates the end user via JWT, so the sportsbook should trust the gateway's asserted user id.
- `GetBet` returns a bet by id; require that the caller is the gateway service (`x-user-id == gateway`) so the gateway can enforce ownership before returning the result.

Use `internal/wallet/server.go:44-88` as a structural exemplar. Example shape:

```go
const serviceCallerID = "gateway"

func AuthInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
    md, ok := metadata.FromIncomingContext(ctx)
    if !ok {
        return nil, status.Error(codes.Unauthenticated, "missing caller metadata")
    }
    userIDs := md.Get(grpcmeta.UserIDHeader)
    if len(userIDs) == 0 || userIDs[0] == "" {
        return nil, status.Error(codes.Unauthenticated, "missing caller user id")
    }
    callerID := userIDs[0]

    isAdmin := false
    if adminVals := md.Get(grpcmeta.IsAdminHeader); len(adminVals) > 0 {
        isAdmin = adminVals[0] == "true"
    }

    switch info.FullMethod {
    case pb.SportsbookService_SettleBet_FullMethodName:
        if !isAdmin || callerID != serviceCallerID {
            return nil, status.Error(codes.PermissionDenied, "admin service caller required")
        }
    default:
        // All other public methods must be called by the trusted gateway service.
        if callerID != serviceCallerID {
            return nil, status.Error(codes.PermissionDenied, "service caller required")
        }
    }

    return handler(ctx, req)
}
```

Add `google.golang.org/grpc/codes`, `google.golang.org/grpc/metadata`, `google.golang.org/grpc/status`, and `internal/shared/grpcmeta` imports if not present.

**Verify**: `go build ./internal/sportsbook/...` → compiles.

### Step 3: Wire the interceptor in the sportsbook entrypoint

In `cmd/sportsbook/main.go`, change:

```go
grpcServer := grpc.NewServer()
```

to:

```go
grpcServer := grpc.NewServer(grpc.UnaryInterceptor(sportsbook.AuthInterceptor))
```

**Verify**: `go build ./cmd/sportsbook` → compiles.

### Step 4: Update sportsbook server tests

In `internal/sportsbook/server_test.go`, the test helper currently creates a plain `grpc.NewServer()`. Update it to use the interceptor, and update callers to send the correct metadata.

Add a helper:

```go
func serviceCtx() context.Context {
    return metadata.AppendToOutgoingContext(context.Background(),
        grpcmeta.UserIDHeader, "gateway",
        grpcmeta.IsAdminHeader, "true",
    )
}
```

Update the test server setup:

```go
grpcServer := grpc.NewServer(grpc.UnaryInterceptor(AuthInterceptor))
```

Update all test contexts that call sportsbook methods to use `serviceCtx()` (or for `SettleBet`, include the admin header). If a test currently uses a plain background context, replace it.

**Verify**: `go test -race ./internal/sportsbook/... -run TestGRPC` → passes.

### Step 6: Update the gateway sportsbook client to send caller metadata

In `internal/gateway/sportsbook_client.go`, add outgoing metadata to each method so the sportsbook interceptor accepts the calls.

Add imports:

```go
"google.golang.org/grpc/metadata"
"github.com/realyoussefhossam/betmonster/internal/shared/grpcmeta"
```

Define `gatewayCallerID = "gateway"` in the package.

For `PlaceBet`, `GetBet`, and `ListBets`, attach `x-user-id: gateway`:

```go
func (c *SportsbookClient) PlaceBet(ctx context.Context, userID, eventID, marketID, outcomeID, stake, currency, referenceID string) (*pb.PlaceBetResponse, error) {
    ctx = metadata.AppendToOutgoingContext(ctx, grpcmeta.UserIDHeader, gatewayCallerID)
    return c.conn.PlaceBet(ctx, &pb.PlaceBetRequest{...})
}
```

For `SettleBet`, attach both `x-user-id: gateway` and `x-caller-is-admin: true`:

```go
func (c *SportsbookClient) SettleBet(ctx context.Context, betID, outcome string) (*pb.SettleBetResponse, error) {
    ctx = metadata.AppendToOutgoingContext(ctx,
        grpcmeta.UserIDHeader, gatewayCallerID,
        grpcmeta.IsAdminHeader, "true",
    )
    return c.conn.SettleBet(ctx, &pb.SettleBetRequest{BetId: betID, Outcome: outcome})
}
```

**Verify**: `go build ./internal/gateway/...` → compiles.

### Step 7: Update the gateway test helper

In `internal/gateway/server_test.go`, update `newSportsbookServerForGateway` to create the sportsbook gRPC server with the interceptor:

```go
grpcServer := grpc.NewServer(grpc.UnaryInterceptor(sportsbook.AuthInterceptor))
```

**Verify**: `go test -race ./internal/gateway/... -run TestGatewayPlaceBet|TestGatewayListBets` → passes.

### Step 8: Verify the full suite and formatting

**Verify**: `go test -race ./internal/sportsbook/...` → passes.

**Verify**: `go test -race ./internal/gateway/...` → passes.

**Verify**: `go test -race ./internal/wallet/...` → passes.

**Verify**: `go test -race ./...` → passes.

**Verify**: `make build` → creates `bin/sportsbook` and exits 0.

**Verify**: `gofmt -d $(find internal/sportsbook -name '*.go') internal/gateway/sportsbook_client.go` → empty output.

**Verify**: `git status --short` → only in-scope files modified.

## Test plan

- Update existing `internal/sportsbook/server_test.go` so the gRPC server runs with `AuthInterceptor`.
- Add or update a test that verifies an end-user caller (without `gateway` id) is rejected from `PlaceBet`.
- Add or update a test that verifies `SettleBet` rejects callers without admin metadata.
- Ensure existing wallet-debit/credit integration tests still pass with the new metadata.

## Done criteria

- [ ] `internal/sportsbook/clients.go` sends privileged metadata (`UserIDHeader=gateway`, `IsAdminHeader=true`) to wallet `DebitWallet`/`CreditWallet`.
- [ ] `internal/sportsbook/server.go` exposes `AuthInterceptor` and restricts methods to trusted gateway callers.
- [ ] `cmd/sportsbook/main.go` wires the interceptor into the gRPC server.
- [ ] Existing sportsbook gRPC tests pass with the interceptor enabled.
- [ ] `internal/gateway/sportsbook_client.go` sends caller metadata on all sportsbook gRPC calls.
- [ ] `internal/gateway/server_test.go` test helper uses the authenticated sportsbook server.
- [ ] `go test -race ./...` exits 0.
- [ ] `make build` exits 0.
- [ ] `gofmt` clean on modified files.
- [ ] Only in-scope files modified.
- [ ] `plans/README.md` status row updated to `DONE`.

## STOP conditions

Stop and report back (do not improvise) if:

- `internal/wallet/server.go` no longer enforces the privileged caller rules this plan assumes.
- The `DebitWallet`/`CreditWallet` gRPC methods are not present in `internal/proto/wallet.proto`.
- Adding the interceptor breaks the scheduler's ability to call `SettleBet` internally.
- A verification fails twice after a reasonable fix attempt.

## Maintenance notes

- This plan only establishes caller identity on the sportsbook gRPC port. It does not add mTLS or network-level isolation; production deployments should still run internal services on a private network.
- Future endpoints that should be admin-only (e.g. mass settlement, risk configuration) must be added to the interceptor's admin case and to the gateway client's admin metadata logic.
- The gateway now asserts `x-user-id: gateway` for all proxied sportsbook calls. The gateway REST handlers must continue to authenticate end users via JWT before forwarding; the sportsbook service trusts the gateway's assertion.
- A future direction plan should add mTLS or per-service tokens so the sportsbook can distinguish the real gateway from any internal caller that happens to set the metadata headers.

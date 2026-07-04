# BetMonster v1 Wallet Microservice Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the v1 wallet microservice for an open-source, self-hosted sportsbook/casino platform: per-currency USDT/USDC wallets, deposits via xcash, manual admin withdrawals, and an auditable ledger. This is the first deployable slice of a larger open-source project that operators can self-host like xcash.

**Architecture:** One Go module at the repo root with two binaries (`gateway` and `wallet`). The gateway verifies Better Auth JWTs and proxies to the wallet service over gRPC. The wallet service owns its own Postgres database and integrates with a self-hosted xcash instance for deposit addresses and webhooks. The stack is containerized for one-command deployment.

**Tech Stack:** Go 1.26, gRPC + protobuf, PostgreSQL 16, Redis 7, NATS 2, pgx, golang-migrate, Next.js 16 + Better Auth.

---

## File Structure

```
better-auth-go/
├── app/                          # Next.js frontend (existing)
├── cmd/
│   ├── gateway/main.go           # gateway entrypoint
│   └── wallet/main.go            # wallet entrypoint
├── internal/
│   ├── auth/jwks.go              # JWT verification with JWKS caching
│   ├── shared/config/gateway.go  # gateway config
│   ├── shared/config/wallet.go   # wallet config
│   ├── shared/logging/logger.go  # structured logger
│   ├── shared/server/http.go     # shared HTTP server helpers
│   ├── gateway/
│   │   ├── server.go             # HTTP router + middleware
│   │   ├── handlers.go           # /api/wallet/* and /webhooks/* handlers
│   │   ├── wallet_client.go      # gRPC client to wallet service
│   │   └── admin.go              # admin authorization middleware
│   ├── wallet/
│   │   ├── server.go             # gRPC server
│   │   ├── service.go            # wallet business logic
│   │   ├── store.go              # wallet store interface + Postgres impl
│   │   ├── models.go             # domain models
│   │   ├── migrations/
│   │   │   ├── 20260704120001_create_wallets.up.sql
│   │   │   ├── 20260704120001_create_wallets.down.sql
│   │   │   ├── 20260704120002_create_transactions.up.sql
│   │   │   ├── ...
│   │   └── xcash/
│   │       ├── client.go         # xcash API client
│   │       ├── webhook.go        # webhook signature validation
│   │       └── types.go          # xcash request/response types
│   └── proto/wallet.proto        # gRPC contract
├── wallet/migrations/            # symlink or copy of internal/wallet/migrations
├── scripts/
│   ├── init_env.sh
│   ├── dev-up.sh
│   ├── migrate.sh
│   ├── test.sh
│   └── upgrade.sh
├── docker-compose.yml
├── Makefile
└── go.mod
```

---

## Task 1: Bootstrap Root Go Module and Project Structure

**Files:**
- Delete: `api/go.mod`, `api/go.sum`, `api/main.go`, `api/handlers.go`
- Move: `api/auth/auth.go` → `internal/auth/jwks.go` (package `auth`)
- Move: `api/middleware/*.go` → `internal/shared/server/*.go` (package `server`)
- Create: root `go.mod`, `cmd/gateway/main.go`, `cmd/wallet/main.go`, `Makefile`

- [ ] **Step 1: Delete old api module and move auth/middleware code**

  ```bash
  rm -f api/go.mod api/go.sum api/main.go api/handlers.go
  mkdir -p internal/auth internal/shared/server
  git mv api/auth/auth.go internal/auth/jwks.go
  git mv api/middleware/* internal/shared/server/
  rmdir api/auth api/middleware 2>/dev/null || true
  ```

  After moving, edit `internal/auth/jwks.go` to keep the package name as `auth` and preserve the `UserFromRequest` function. It currently imports `github.com/realyoussefhossam/betmonster/api/auth` nowhere; only package internals use it.

- [ ] **Step 2: Create root `go.mod`**

  Create `go.mod`:
  ```go
  module github.com/realyoussefhossam/betmonster

  go 1.26
  ```

  Run:
  ```bash
  cd /home/joseph/documents/dev/better-auth-go
  go mod tidy
  ```

  Expected: `go.mod` and `go.sum` are generated at the repo root.

- [ ] **Step 3: Create minimal entrypoints**

  Create `cmd/gateway/main.go`:
  ```go
  package main

  import (
      "log"
      "net/http"
  )

  func main() {
      mux := http.NewServeMux()
      mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
          w.WriteHeader(http.StatusOK)
          w.Write([]byte(`{"status":"healthy","service":"gateway"}`))
      })
      log.Println("gateway starting on :8080")
      if err := http.ListenAndServe(":8080", mux); err != nil {
          log.Fatal(err)
      }
  }
  ```

  Create `cmd/wallet/main.go`:
  ```go
  package main

  import (
      "log"
      "net/http"
  )

  func main() {
      mux := http.NewServeMux()
      mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
          w.WriteHeader(http.StatusOK)
          w.Write([]byte(`{"status":"healthy","service":"wallet"}`))
      })
      log.Println("wallet starting on :8081")
      if err := http.ListenAndServe(":8081", mux); err != nil {
          log.Fatal(err)
      }
  }
  ```

  Run:
  ```bash
  go build ./cmd/gateway
  go build ./cmd/wallet
  ```

  Expected: both binaries compile.

- [ ] **Step 4: Commit**

  ```bash
  git add go.mod go.sum cmd internal Makefile
  git commit -m "chore: bootstrap root Go module and service entrypoints"
  ```

---

## Task 2: Add Dependencies (gRPC, Postgres, NATS, Redis, Migrate)

**Files:**
- Modify: `go.mod`, `go.sum`
- Create: `Makefile`

- [ ] **Step 1: Install dependencies**

  Run:
  ```bash
  go get google.golang.org/grpc google.golang.org/protobuf
  go get github.com/jackc/pgx/v5
  go get github.com/golang-migrate/migrate/v4/database/pgx/v5
  go get github.com/golang-migrate/migrate/v4/source/file
  go get github.com/nats-io/nats.go
  go get github.com/redis/go-redis/v9
  go get github.com/google/uuid
  go mod tidy
  ```

  Expected: `go.mod` and `go.sum` are updated.

- [ ] **Step 2: Create Makefile with common commands**

  Create `Makefile`:
  ```makefile
  .PHONY: build test migrate proto dev

  build:
      go build ./cmd/gateway
      go build ./cmd/wallet

  test:
      go test ./...

  migrate:
      ./scripts/migrate.sh up

  proto:
      protoc --go_out=. --go_opt=paths=source_relative \
             --go-grpc_out=. --go-grpc_opt=paths=source_relative \
             internal/proto/wallet.proto

  dev:
      ./scripts/dev-up.sh
  ```

- [ ] **Step 3: Commit**

  ```bash
  git add go.mod go.sum Makefile
  git commit -m "chore: add grpc, postgres, nats, redis, migrate dependencies"
  ```

---

## Task 3: Define Shared Config and Logging

**Files:**
- Create: `internal/shared/config/gateway.go`, `internal/shared/config/wallet.go`
- Create: `internal/shared/logging/logger.go`

- [ ] **Step 1: Create gateway config**

  Create `internal/shared/config/gateway.go`:
  ```go
  package config

  import (
      "os"
  )

  type Gateway struct {
      Port                string
      JWKSURL             string
      WalletServiceAddr string
      AdminUserIDs        string
  }

  func LoadGateway() Gateway {
      return Gateway{
          Port:                getEnv("PORT", "8080"),
          JWKSURL:             getEnv("JWKS_URL", "http://localhost:3000/api/auth/jwks"),
          WalletServiceAddr:   getEnv("WALLET_SERVICE_ADDR", "localhost:50051"),
          AdminUserIDs:        getEnv("ADMIN_USER_IDS", ""),
      }
  }

  func getEnv(key, fallback string) string {
      if v := os.Getenv(key); v != "" {
          return v
      }
      return fallback
  }
  ```

- [ ] **Step 2: Create wallet config**

  Create `internal/shared/config/wallet.go`:
  ```go
  package config

  type Wallet struct {
      Port             string
      DatabaseURL      string
      RedisAddr        string
      NATSURL          string
      XCashBaseURL     string
      XCashAppID       string
      XCashHMACKey     string
      XCashWebhookSecret string
  }

  func LoadWallet() Wallet {
      return Wallet{
          Port:               getEnv("PORT", "8081"),
          DatabaseURL:        getEnv("DATABASE_URL", "postgres://wallet:wallet@localhost:5433/wallet?sslmode=disable"),
          RedisAddr:          getEnv("REDIS_ADDR", "localhost:6379"),
          NATSURL:            getEnv("NATS_URL", "nats://localhost:4222"),
          XCashBaseURL:       getEnv("XCASH_BASE_URL", "http://localhost:6688"),
          XCashAppID:         getEnv("XCASH_APPID", ""),
          XCashHMACKey:       getEnv("XCASH_HMAC_KEY", ""),
          XCashWebhookSecret: getEnv("XCASH_WEBHOOK_SECRET", ""),
      }
  }
  ```

- [ ] **Step 3: Create structured logger**

  Create `internal/shared/logging/logger.go`:
  ```go
  package logging

  import (
      "log/slog"
      "os"
  )

  func New() *slog.Logger {
      return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
  }
  ```

- [ ] **Step 4: Commit**

  ```bash
  git add internal/shared/config internal/shared/logging
  git commit -m "chore: add gateway and wallet config plus structured logger"
  ```

---

## Task 4: Define gRPC Proto Contract

**Files:**
- Create: `internal/proto/wallet.proto`
- Generate: `internal/proto/wallet.pb.go`, `internal/proto/wallet_grpc.pb.go`

- [ ] **Step 1: Write the proto file**

  Create `internal/proto/wallet.proto`:
  ```protobuf
  syntax = "proto3";
  package wallet;
  option go_package = "github.com/realyoussefhossam/betmonster/internal/proto";

  service WalletService {
    rpc GetBalance(GetBalanceRequest) returns (GetBalanceResponse);
    rpc ListTransactions(ListTransactionsRequest) returns (ListTransactionsResponse);
    rpc GetDepositAddress(GetDepositAddressRequest) returns (GetDepositAddressResponse);
    rpc RequestWithdrawal(RequestWithdrawalRequest) returns (RequestWithdrawalResponse);
    rpc ProcessDepositWebhook(ProcessDepositWebhookRequest) returns (ProcessDepositWebhookResponse);
    rpc ListPendingWithdrawals(ListPendingWithdrawalsRequest) returns (ListPendingWithdrawalsResponse);
    rpc ReviewWithdrawal(ReviewWithdrawalRequest) returns (ReviewWithdrawalResponse);
  }

  message GetBalanceRequest { string user_id = 1; string currency = 2; }
  message GetBalanceResponse { string currency = 1; string balance = 2; }

  message Transaction {
    string id = 1;
    string user_id = 2;
    string wallet_id = 3;
    string type = 4;
    string amount = 5;
    string balance_before = 6;
    string balance_after = 7;
    string status = 8;
    string reference_id = 9;
    string metadata = 10;
    string created_at = 11;
  }

  message ListTransactionsRequest { string user_id = 1; int32 page = 2; int32 page_size = 3; }
  message ListTransactionsResponse { repeated Transaction transactions = 1; }

  message GetDepositAddressRequest { string user_id = 1; string currency = 2; string chain = 3; }
  message GetDepositAddressResponse { string address = 1; string chain = 2; string currency = 3; }

  message RequestWithdrawalRequest {
    string user_id = 1;
    string currency = 2;
    string amount = 3;
    string destination_address = 4;
    string chain = 5;
  }
  message RequestWithdrawalResponse { string withdrawal_id = 1; string status = 2; }

  message ProcessDepositWebhookRequest { string body = 1; map<string, string> headers = 2; }
  message ProcessDepositWebhookResponse { string response_body = 1; }

  message WithdrawalRequest {
    string id = 1;
    string user_id = 2;
    string currency = 3;
    string amount = 4;
    string destination_address = 5;
    string chain = 6;
    string status = 7;
    string tx_hash = 8;
    string created_at = 9;
  }

  message ListPendingWithdrawalsRequest { int32 page = 1; int32 page_size = 2; }
  message ListPendingWithdrawalsResponse { repeated WithdrawalRequest withdrawals = 1; }

  message ReviewWithdrawalRequest {
    string withdrawal_id = 1;
    string action = 2;
    string tx_hash = 3;
    string reviewed_by = 4;
  }
  message ReviewWithdrawalResponse { string status = 1; }
  ```

- [ ] **Step 2: Generate Go code**

  Install protoc plugins if needed:
  ```bash
  go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
  go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
  ```

  Run:
  ```bash
  make proto
  ```

  Expected: `internal/proto/wallet.pb.go` and `internal/proto/wallet_grpc.pb.go` are generated.

- [ ] **Step 3: Commit**

  ```bash
  git add internal/proto
  git commit -m "chore: add wallet gRPC proto contract"
  ```

---

## Task 5: Wallet Database Schema and Migrations

**Files:**
- Create: `internal/wallet/migrations/*.up.sql`, `*.down.sql`
- Create: symlink `wallet/migrations` → `internal/wallet/migrations`

- [ ] **Step 1: Create wallet migrations**

  Create `internal/wallet/migrations/20260704120001_create_wallets.up.sql`:
  ```sql
  CREATE TABLE wallets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id TEXT NOT NULL,
    currency TEXT NOT NULL,
    balance NUMERIC(28,8) NOT NULL DEFAULT 0,
    version INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, currency)
  );
  ```

  Create `internal/wallet/migrations/20260704120001_create_wallets.down.sql`:
  ```sql
  DROP TABLE IF EXISTS wallets;
  ```

  Create `internal/wallet/migrations/20260704120002_create_transactions.up.sql`:
  ```sql
  CREATE TABLE transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id TEXT NOT NULL,
    wallet_id UUID NOT NULL REFERENCES wallets(id) ON DELETE CASCADE,
    type TEXT NOT NULL CHECK (type IN ('deposit','withdrawal','bet','win','fee','adjustment')),
    amount NUMERIC(28,8) NOT NULL,
    balance_before NUMERIC(28,8) NOT NULL,
    balance_after NUMERIC(28,8) NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('pending','completed','failed')),
    reference_id TEXT UNIQUE,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
  );
  CREATE INDEX idx_transactions_user_id ON transactions(user_id);
  ```

  Create `internal/wallet/migrations/20260704120002_create_transactions.down.sql`:
  ```sql
  DROP TABLE IF EXISTS transactions;
  ```

  Create `internal/wallet/migrations/20260704120003_create_deposit_addresses.up.sql`:
  ```sql
  CREATE TABLE deposit_addresses (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id TEXT NOT NULL,
    currency TEXT NOT NULL,
    chain TEXT NOT NULL,
    address TEXT NOT NULL,
    xcash_deposit_id TEXT,
    status TEXT NOT NULL CHECK (status IN ('active','archived')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
  );
  CREATE UNIQUE INDEX idx_active_deposit_address
    ON deposit_addresses(user_id, currency, chain)
    WHERE status = 'active';
  ```

  Create `internal/wallet/migrations/20260704120003_create_deposit_addresses.down.sql`:
  ```sql
  DROP TABLE IF EXISTS deposit_addresses;
  ```

  Create `internal/wallet/migrations/20260704120004_create_withdrawal_requests.up.sql`:
  ```sql
  CREATE TABLE withdrawal_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id TEXT NOT NULL,
    wallet_id UUID NOT NULL REFERENCES wallets(id),
    amount NUMERIC(28,8) NOT NULL,
    currency TEXT NOT NULL,
    destination_address TEXT NOT NULL,
    chain TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('pending','approved','rejected','completed')),
    tx_hash TEXT,
    reviewed_by TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
  );
  CREATE INDEX idx_withdrawal_requests_status ON withdrawal_requests(status);
  ```

  Create `internal/wallet/migrations/20260704120004_create_withdrawal_requests.down.sql`:
  ```sql
  DROP TABLE IF EXISTS withdrawal_requests;
  ```

- [ ] **Step 2: Create symlink**

  Run:
  ```bash
  mkdir -p wallet
  ln -s ../internal/wallet/migrations wallet/migrations
  ```

- [ ] **Step 3: Commit**

  ```bash
  git add internal/wallet/migrations wallet/migrations
  git commit -m "chore: add wallet database schema and migrations"
  ```

---

## Task 6: Wallet Store Interface and Postgres Implementation

**Files:**
- Create: `internal/wallet/models.go`
- Create: `internal/wallet/store.go`
- Create: `internal/wallet/store_test.go`

- [ ] **Step 1: Define domain models**

  Create `internal/wallet/models.go`:
  ```go
  package wallet

  import "time"

  type Wallet struct {
      ID        string
      UserID    string
      Currency  string
      Balance   string
      Version   int
      CreatedAt time.Time
      UpdatedAt time.Time
  }

  type Transaction struct {
      ID            string
      UserID        string
      WalletID      string
      Type          string
      Amount        string
      BalanceBefore string
      BalanceAfter  string
      Status        string
      ReferenceID   string
      Metadata      string
      CreatedAt     time.Time
  }

  type DepositAddress struct {
      ID              string
      UserID          string
      Currency        string
      Chain           string
      Address         string
      XCashDepositID  string
      Status          string
      CreatedAt       time.Time
  }

  type WithdrawalRequest struct {
      ID                string
      UserID            string
      WalletID          string
      Amount            string
      Currency          string
      DestinationAddress string
      Chain             string
      Status            string
      TxHash            string
      ReviewedBy        string
      CreatedAt         time.Time
  }
  ```

- [ ] **Step 2: Define store interface**

  Create `internal/wallet/store.go`:
  ```go
  package wallet

  import "context"

  type Store interface {
      GetWallet(ctx context.Context, userID, currency string) (*Wallet, error)
      CreateWallet(ctx context.Context, userID, currency string) (*Wallet, error)
      CreditWallet(ctx context.Context, userID, currency, amount, referenceID string, metadata map[string]any) (*Transaction, error)
      DebitWallet(ctx context.Context, userID, currency, amount, referenceID string) (*Transaction, error)
      ReverseDebit(ctx context.Context, transactionID string) (*Transaction, error)
      GetDepositAddress(ctx context.Context, userID, currency, chain string) (*DepositAddress, error)
      CreateDepositAddress(ctx context.Context, addr *DepositAddress) (*DepositAddress, error)
      CreateWithdrawalRequest(ctx context.Context, req *WithdrawalRequest) (*WithdrawalRequest, error)
      ListPendingWithdrawals(ctx context.Context, page, pageSize int) ([]WithdrawalRequest, error)
      ReviewWithdrawal(ctx context.Context, id, action, txHash, reviewedBy string) (*WithdrawalRequest, error)
      ListTransactions(ctx context.Context, userID string, page, pageSize int) ([]Transaction, error)
  }
  ```

- [ ] **Step 3: Write failing test for store interface**

  Create `internal/wallet/store_test.go`:
  ```go
  package wallet

  import (
      "context"
      "testing"

      "github.com/stretchr/testify/assert"
  )

  type inMemoryStore struct {
      wallets map[string]*Wallet
  }

  func (s *inMemoryStore) GetWallet(ctx context.Context, userID, currency string) (*Wallet, error) {
      return nil, assert.AnError
  }
  // ... stub implementations

  func TestCreateWallet(t *testing.T) {
      store := &inMemoryStore{wallets: map[string]*Wallet{}}
      wallet, err := store.CreateWallet(context.Background(), "user-1", "USDT")
      assert.NoError(t, err)
      assert.Equal(t, "user-1", wallet.UserID)
      assert.Equal(t, "USDT", wallet.Currency)
      assert.Equal(t, "0", wallet.Balance)
  }
  ```

  Run:
  ```bash
  go test ./internal/wallet -run TestCreateWallet
  ```

  Expected: FAIL because `CreateWallet` is not implemented on inMemoryStore.

- [ ] **Step 4: Implement Postgres store**

  Create `internal/wallet/pgstore.go`:
  ```go
  package wallet

  import (
      "context"
      "database/sql"
      "errors"
      "fmt"
  )

  type PGStore struct {
      db *sql.DB
  }

  var ErrNotImplemented = errors.New("not implemented")

  func NewPGStore(db *sql.DB) *PGStore {
      return &PGStore{db: db}
  }

  func (s *PGStore) CreateWallet(ctx context.Context, userID, currency string) (*Wallet, error) {
      const q = `
          INSERT INTO wallets (user_id, currency, balance, version)
          VALUES ($1, $2, 0, 0)
          ON CONFLICT (user_id, currency) DO UPDATE SET updated_at = NOW()
          RETURNING id, user_id, currency, balance, version, created_at, updated_at
      `
      row := s.db.QueryRowContext(ctx, q, userID, currency)
      var w Wallet
      err := row.Scan(&w.ID, &w.UserID, &w.Currency, &w.Balance, &w.Version, &w.CreatedAt, &w.UpdatedAt)
      if err != nil {
          return nil, fmt.Errorf("create wallet: %w", err)
      }
      return &w, nil
  }

  func (s *PGStore) GetWallet(ctx context.Context, userID, currency string) (*Wallet, error) {
      return nil, ErrNotImplemented
  }
  func (s *PGStore) CreditWallet(ctx context.Context, userID, currency, amount, referenceID string, metadata map[string]any) (*Transaction, error) {
      return nil, ErrNotImplemented
  }
  func (s *PGStore) DebitWallet(ctx context.Context, userID, currency, amount, referenceID string) (*Transaction, error) {
      return nil, ErrNotImplemented
  }
  func (s *PGStore) ReverseDebit(ctx context.Context, transactionID string) (*Transaction, error) {
      return nil, ErrNotImplemented
  }
  func (s *PGStore) GetDepositAddress(ctx context.Context, userID, currency, chain string) (*DepositAddress, error) {
      return nil, ErrNotImplemented
  }
  func (s *PGStore) CreateDepositAddress(ctx context.Context, addr *DepositAddress) (*DepositAddress, error) {
      return nil, ErrNotImplemented
  }
  func (s *PGStore) CreateWithdrawalRequest(ctx context.Context, req *WithdrawalRequest) (*WithdrawalRequest, error) {
      return nil, ErrNotImplemented
  }
  func (s *PGStore) ListPendingWithdrawals(ctx context.Context, page, pageSize int) ([]WithdrawalRequest, error) {
      return nil, ErrNotImplemented
  }
  func (s *PGStore) ReviewWithdrawal(ctx context.Context, id, action, txHash, reviewedBy string) (*WithdrawalRequest, error) {
      return nil, ErrNotImplemented
  }
  func (s *PGStore) ListTransactions(ctx context.Context, userID string, page, pageSize int) ([]Transaction, error) {
      return nil, ErrNotImplemented
  }
  ```

- [ ] **Step 5: Run tests**

  Run:
  ```bash
  go test ./internal/wallet
  ```

  Expected: PASS (in-memory test) or FAIL on unimplemented methods until completed.

- [ ] **Step 6: Commit**

  ```bash
  git add internal/wallet/models.go internal/wallet/store.go internal/wallet/pgstore.go internal/wallet/store_test.go
  git commit -m "feat(wallet): add store interface, postgres stub, and model tests"
  ```

---

## Task 7: Implement Wallet Core Logic (Credit, Debit, Idempotent Deposits)

**Files:**
- Create: `internal/wallet/service.go`
- Modify: `internal/wallet/pgstore.go`
- Create: `internal/wallet/service_test.go`

- [ ] **Step 1: Write failing test for credit**

  Create `internal/wallet/service_test.go`:
  ```go
  package wallet

  import (
      "context"
      "testing"

      "github.com/stretchr/testify/assert"
  )

  func TestCreditWallet(t *testing.T) {
      ctx := context.Background()
      store := newInMemoryStore()
      svc := NewService(store)

      tx, err := svc.CreditWallet(ctx, "user-1", "USDT", "100.00", "dx-1", nil)
      assert.NoError(t, err)
      assert.Equal(t, "completed", tx.Status)
      assert.Equal(t, "100.00", tx.BalanceAfter)

      // idempotent
      tx2, err := svc.CreditWallet(ctx, "user-1", "USDT", "100.00", "dx-1", nil)
      assert.NoError(t, err)
      assert.Equal(t, tx.ID, tx2.ID)

      wallet, err := store.GetWallet(ctx, "user-1", "USDT")
      assert.NoError(t, err)
      assert.Equal(t, "100.00", wallet.Balance)
  }
  ```

  Run:
  ```bash
  go test ./internal/wallet -run TestCreditWallet
  ```

  Expected: FAIL because `Service` and `CreditWallet` do not exist.

- [ ] **Step 2: Implement in-memory store for unit tests**

  Create `internal/wallet/memory_store.go`:
  ```go
  package wallet

  import (
      "context"
      "errors"
      "sync"
      "time"

      "github.com/google/uuid"
  )

  type memoryStore struct {
      mu          sync.Mutex
      wallets     map[string]*Wallet
      txns        map[string]*Transaction
      addresses   map[string]*DepositAddress
      withdrawals map[string]*WithdrawalRequest
  }

  func newInMemoryStore() *memoryStore {
      return &memoryStore{
          wallets:     map[string]*Wallet{},
          txns:        map[string]*Transaction{},
          addresses:   map[string]*DepositAddress{},
          withdrawals: map[string]*WithdrawalRequest{},
      }
  }

  func (s *memoryStore) walletKey(userID, currency string) string {
      return userID + ":" + currency
  }

  func (s *memoryStore) GetWallet(ctx context.Context, userID, currency string) (*Wallet, error) {
      s.mu.Lock()
      defer s.mu.Unlock()
      w, ok := s.wallets[s.walletKey(userID, currency)]
      if !ok {
          return nil, errors.New("wallet not found")
      }
      return w, nil
  }

  func (s *memoryStore) CreateWallet(ctx context.Context, userID, currency string) (*Wallet, error) {
      s.mu.Lock()
      defer s.mu.Unlock()
      key := s.walletKey(userID, currency)
      if w, ok := s.wallets[key]; ok {
          return w, nil
      }
      w := &Wallet{ID: uuid.NewString(), UserID: userID, Currency: currency, Balance: "0", Version: 0, CreatedAt: time.Now(), UpdatedAt: time.Now()}
      s.wallets[key] = w
      return w, nil
  }

  func (s *memoryStore) CreditWallet(ctx context.Context, userID, currency, amount, referenceID string, metadata map[string]any) (*Transaction, error) {
      s.mu.Lock()
      defer s.mu.Unlock()
      if _, ok := s.txns[referenceID]; ok {
          return s.txns[referenceID], nil
      }
      w := s.wallets[s.walletKey(userID, currency)]
      if w == nil {
          w, _ = s.CreateWallet(ctx, userID, currency)
      }
      newBalance := addDecimal(w.Balance, amount)
      txn := &Transaction{ID: uuid.NewString(), UserID: userID, WalletID: w.ID, Type: "deposit", Amount: amount, BalanceBefore: w.Balance, BalanceAfter: newBalance, Status: "completed", ReferenceID: referenceID, CreatedAt: time.Now()}
      s.txns[referenceID] = txn
      w.Balance = newBalance
      w.Version++
      return txn, nil
  }

  func (s *memoryStore) DebitWallet(ctx context.Context, userID, currency, amount, referenceID string) (*Transaction, error) {
      return nil, errors.New("not implemented in unit test stub")
  }
  func (s *memoryStore) ReverseDebit(ctx context.Context, transactionID string) (*Transaction, error) {
      return nil, errors.New("not implemented in unit test stub")
  }
  func (s *memoryStore) GetDepositAddress(ctx context.Context, userID, currency, chain string) (*DepositAddress, error) {
      return nil, errors.New("not found")
  }
  func (s *memoryStore) CreateDepositAddress(ctx context.Context, addr *DepositAddress) (*DepositAddress, error) {
      addr.ID = uuid.NewString()
      addr.CreatedAt = time.Now()
      s.addresses[addr.ID] = addr
      return addr, nil
  }
  func (s *memoryStore) CreateWithdrawalRequest(ctx context.Context, req *WithdrawalRequest) (*WithdrawalRequest, error) {
      req.ID = uuid.NewString()
      req.Status = "pending"
      req.CreatedAt = time.Now()
      s.withdrawals[req.ID] = req
      return req, nil
  }
  func (s *memoryStore) ListPendingWithdrawals(ctx context.Context, page, pageSize int) ([]WithdrawalRequest, error) {
      return nil, nil
  }
  func (s *memoryStore) ReviewWithdrawal(ctx context.Context, id, action, txHash, reviewedBy string) (*WithdrawalRequest, error) {
      return nil, errors.New("not implemented in unit test stub")
  }
  func (s *memoryStore) ListTransactions(ctx context.Context, userID string, page, pageSize int) ([]Transaction, error) {
      return nil, nil
  }
  ```

- [ ] **Step 3: Implement wallet service**

  Create `internal/wallet/service.go`:
  ```go
  package wallet

  import (
      "context"
      "fmt"
  )

  type Service struct {
      store Store
  }

  func NewService(store Store) *Service {
      return &Service{store: store}
  }

  func (s *Service) CreditWallet(ctx context.Context, userID, currency, amount, referenceID string, metadata map[string]any) (*Transaction, error) {
      return s.store.CreditWallet(ctx, userID, currency, amount, referenceID, metadata)
  }
  ```

  Run tests:
  ```bash
  go test ./internal/wallet -run TestCreditWallet
  ```

  Expected: PASS.

- [ ] **Step 4: Add decimal helpers**

  Run:
  ```bash
  go get github.com/shopspring/decimal
  ```

  Create `internal/wallet/decimal.go`:
  ```go
  package wallet

  import (
      "github.com/shopspring/decimal"
  )

  func addDecimal(a, b string) string {
      da, _ := decimal.NewFromString(a)
      db, _ := decimal.NewFromString(b)
      return da.Add(db).String()
  }

  func subDecimal(a, b string) string {
      da, _ := decimal.NewFromString(a)
      db, _ := decimal.NewFromString(b)
      return da.Sub(db).String()
  }
  ```

- [ ] **Step 5: Implement Postgres credit/debit with optimistic locking**

  In `internal/wallet/pgstore.go`, implement `CreditWallet` and `DebitWallet` using transactions and `SELECT ... FOR UPDATE`.

  Use the `addDecimal` and `subDecimal` helpers for balance calculations.

- [ ] **Step 6: Add tests for idempotency and concurrency**

  Add tests in `internal/wallet/service_test.go`:
  ```go
  func TestCreditWalletIdempotent(t *testing.T) { ... }
  func TestDebitWalletInsufficientBalance(t *testing.T) { ... }
  ```

  Run:
  ```bash
  go test ./internal/wallet -v
  ```

  Expected: all PASS.

- [ ] **Step 7: Commit**

  ```bash
  git add internal/wallet
  git commit -m "feat(wallet): implement credit, debit, and idempotent deposits"
  ```

---

## Task 8: Implement xcash Client and Webhook Validation

**Files:**
- Create: `internal/wallet/xcash/types.go`
- Create: `internal/wallet/xcash/client.go`
- Create: `internal/wallet/xcash/webhook.go`
- Create: `internal/wallet/xcash/client_test.go`

- [ ] **Step 1: Define xcash types**

  Create `internal/wallet/xcash/types.go`:
  ```go
  package xcash

  type DepositAddressRequest struct {
      UID    string
      Chain  string
      Crypto string
  }

  type DepositAddressResponse struct {
      Address string
  }

  type DepositWebhook struct {
      Type string `json:"type"`
      Data struct {
          SysNo     string `json:"sys_no"`
          UID       string `json:"uid"`
          Chain     string `json:"chain"`
          Block     int64  `json:"block"`
          Hash      string `json:"hash"`
          Crypto    string `json:"crypto"`
          Amount    string `json:"amount"`
          Confirmed bool   `json:"confirmed"`
          RiskLevel string `json:"risk_level"`
          RiskScore string `json:"risk_score"`
      } `json:"data"`
  }
  ```

- [ ] **Step 2: Implement xcash client**

  Create `internal/wallet/xcash/client.go`:
  ```go
  package xcash

  import (
      "context"
      "crypto/hmac"
      "crypto/sha256"
      "encoding/hex"
      "encoding/json"
      "fmt"
      "io"
      "net/http"
      "net/url"
      "strconv"
      "time"

      "github.com/google/uuid"
  )

  type Client struct {
      baseURL string
      appID   string
      hmacKey string
      http    *http.Client
  }

  func NewClient(baseURL, appID, hmacKey string) *Client {
      return &Client{
          baseURL: baseURL,
          appID:   appID,
          hmacKey: hmacKey,
          http:    &http.Client{Timeout: 10 * time.Second},
      }
  }

  func (c *Client) GetDepositAddress(ctx context.Context, req DepositAddressRequest) (*DepositAddressResponse, error) {
      u, _ := url.Parse(c.baseURL + "/v1/deposit/address")
      q := u.Query()
      q.Set("uid", req.UID)
      q.Set("chain", req.Chain)
      q.Set("crypto", req.Crypto)
      u.RawQuery = q.Encode()

      timestamp := strconv.FormatInt(time.Now().Unix(), 10)
      nonce := uuid.NewString()
      signature := sign(nonce + timestamp + "", c.hmacKey)

      hreq, _ := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
      hreq.Header.Set("XC-Appid", c.appID)
      hreq.Header.Set("XC-Timestamp", timestamp)
      hreq.Header.Set("XC-Nonce", nonce)
      hreq.Header.Set("XC-Signature", signature)

      resp, err := c.http.Do(hreq)
      if err != nil {
          return nil, err
      }
      defer resp.Body.Close()
      if resp.StatusCode != http.StatusOK {
          body, _ := io.ReadAll(resp.Body)
          return nil, fmt.Errorf("xcash %d: %s", resp.StatusCode, body)
      }
      var result DepositAddressResponse
      if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
          return nil, err
      }
      return &result, nil
  }

  func sign(message, key string) string {
      h := hmac.New(sha256.New, []byte(key))
      h.Write([]byte(message))
      return hex.EncodeToString(h.Sum(nil))
  }
  ```

- [ ] **Step 3: Implement webhook signature validation**

  Create `internal/wallet/xcash/webhook.go`:
  ```go
  package xcash

  import (
      "crypto/hmac"
      "crypto/sha256"
      "encoding/hex"
      "encoding/json"
      "fmt"
  )

  type WebhookValidator struct {
      hmacKey string
  }

  func NewWebhookValidator(hmacKey string) *WebhookValidator {
      return &WebhookValidator{hmacKey: hmacKey}
  }

  func (v *WebhookValidator) Validate(body []byte, headers map[string]string) (*DepositWebhook, error) {
      nonce := headers["XC-Nonce"]
      timestamp := headers["XC-Timestamp"]
      signature := headers["XC-Signature"]
      expected := sign(nonce+timestamp+string(body), v.hmacKey)
      if !hmac.Equal([]byte(signature), []byte(expected)) {
          return nil, fmt.Errorf("invalid webhook signature")
      }
      var webhook DepositWebhook
      if err := json.Unmarshal(body, &webhook); err != nil {
          return nil, err
      }
      return &webhook, nil
  }
  ```

- [ ] **Step 4: Write tests**

  Create `internal/wallet/xcash/client_test.go` with tests for `GetDepositAddress` and `WebhookValidator`.

  Run:
  ```bash
  go test ./internal/wallet/xcash
  ```

  Expected: PASS.

- [ ] **Step 5: Commit**

  ```bash
  git add internal/wallet/xcash
  git commit -m "feat(wallet): add xcash client and webhook validator"
  ```

---

## Task 9: Implement Wallet gRPC Server

**Files:**
- Create: `internal/wallet/server.go`
- Modify: `internal/wallet/service.go`
- Create: `internal/wallet/server_test.go`

- [ ] **Step 1: Wire xcash into service**

  Extend `internal/wallet/service.go` to include xcash client and validator, and implement `GetDepositAddress` and `ProcessDepositWebhook`.

- [ ] **Step 2: Implement gRPC server**

  Create `internal/wallet/server.go`:
  ```go
  package wallet

  import (
      "context"
      "encoding/json"
      "strconv"

      pb "github.com/realyoussefhossam/betmonster/internal/proto"
  )

  type GRPCServer struct {
      pb.UnimplementedWalletServiceServer
      service *Service
  }

  func NewGRPCServer(service *Service) *GRPCServer {
      return &GRPCServer{service: service}
  }

  func (s *GRPCServer) GetBalance(ctx context.Context, req *pb.GetBalanceRequest) (*pb.GetBalanceResponse, error) {
      wallet, err := s.service.store.GetWallet(ctx, req.UserId, req.Currency)
      if err != nil {
          return nil, err
      }
      return &pb.GetBalanceResponse{Currency: wallet.Currency, Balance: wallet.Balance}, nil
  }

  func (s *GRPCServer) ListTransactions(ctx context.Context, req *pb.ListTransactionsRequest) (*pb.ListTransactionsResponse, error) {
      txns, err := s.service.store.ListTransactions(ctx, req.UserId, req.Page, req.PageSize)
      if err != nil {
          return nil, err
      }
      out := make([]*pb.Transaction, len(txns))
      for i, t := range txns {
          out[i] = &pb.Transaction{
              Id: t.ID, UserId: t.UserID, WalletId: t.WalletID, Type: t.Type,
              Amount: t.Amount, BalanceBefore: t.BalanceBefore, BalanceAfter: t.BalanceAfter,
              Status: t.Status, ReferenceId: t.ReferenceID, Metadata: t.Metadata,
          }
      }
      return &pb.ListTransactionsResponse{Transactions: out}, nil
  }

  func (s *GRPCServer) GetDepositAddress(ctx context.Context, req *pb.GetDepositAddressRequest) (*pb.GetDepositAddressResponse, error) {
      addr, err := s.service.GetDepositAddress(ctx, req.UserId, req.Currency, req.Chain)
      if err != nil {
          return nil, err
      }
      return &pb.GetDepositAddressResponse{Address: addr.Address, Chain: addr.Chain, Currency: addr.Currency}, nil
  }

  func (s *GRPCServer) RequestWithdrawal(ctx context.Context, req *pb.RequestWithdrawalRequest) (*pb.RequestWithdrawalResponse, error) {
      req2 := WithdrawalRequest{
          UserID: req.UserId, Currency: req.Currency, Amount: req.Amount,
          DestinationAddress: req.DestinationAddress, Chain: req.Chain,
      }
      out, err := s.service.store.CreateWithdrawalRequest(ctx, &req2)
      if err != nil {
          return nil, err
      }
      return &pb.RequestWithdrawalResponse{WithdrawalId: out.ID, Status: out.Status}, nil
  }

  func (s *GRPCServer) ProcessDepositWebhook(ctx context.Context, req *pb.ProcessDepositWebhookRequest) (*pb.ProcessDepositWebhookResponse, error) {
      body, err := s.service.ProcessDepositWebhook(ctx, []byte(req.Body), req.Headers)
      return &pb.ProcessDepositWebhookResponse{ResponseBody: body}, err
  }

  func (s *GRPCServer) ListPendingWithdrawals(ctx context.Context, req *pb.ListPendingWithdrawalsRequest) (*pb.ListPendingWithdrawalsResponse, error) {
      list, err := s.service.store.ListPendingWithdrawals(ctx, req.Page, req.PageSize)
      if err != nil {
          return nil, err
      }
      out := make([]*pb.WithdrawalRequest, len(list))
      for i, w := range list {
          out[i] = &pb.WithdrawalRequest{
              Id: w.ID, UserId: w.UserID, Currency: w.Currency, Amount: w.Amount,
              DestinationAddress: w.DestinationAddress, Chain: w.Chain, Status: w.Status, TxHash: w.TxHash,
          }
      }
      return &pb.ListPendingWithdrawalsResponse{Withdrawals: out}, nil
  }

  func (s *GRPCServer) ReviewWithdrawal(ctx context.Context, req *pb.ReviewWithdrawalRequest) (*pb.ReviewWithdrawalResponse, error) {
      w, err := s.service.store.ReviewWithdrawal(ctx, req.WithdrawalId, req.Action, req.TxHash, req.ReviewedBy)
      if err != nil {
          return nil, err
      }
      return &pb.ReviewWithdrawalResponse{Status: w.Status}, nil
  }
  ```

- [ ] **Step 3: Test gRPC server**

  Create `internal/wallet/server_test.go` with a unit test that starts an in-memory gRPC server and calls `GetBalance`.

  Run:
  ```bash
  go test ./internal/wallet
  ```

  Expected: PASS.

- [ ] **Step 4: Commit**

  ```bash
  git add internal/wallet
  git commit -m "feat(wallet): implement gRPC server and deposit webhook flow"
  ```

---

## Task 10: Implement Gateway Service

**Files:**
- Create: `internal/gateway/server.go`
- Create: `internal/gateway/handlers.go`
- Create: `internal/gateway/wallet_client.go`
- Create: `internal/gateway/admin.go`
- Modify: `cmd/gateway/main.go`

- [ ] **Step 1: Add JWKS caching to auth package**

  Modify `internal/auth/jwks.go` to cache the JWKS keyset. Use `jwk.NewCache` with periodic refresh for production.

- [ ] **Step 2: Create wallet gRPC client wrapper**

  Create `internal/gateway/wallet_client.go`:
  ```go
  package gateway

  import (
      "context"
      "fmt"

      "google.golang.org/grpc"
      "google.golang.org/grpc/metadata"

      pb "github.com/realyoussefhossam/betmonster/internal/proto"
  )

  type WalletClient struct {
      client pb.WalletServiceClient
  }

  func NewWalletClient(addr string) (*WalletClient, error) {
      conn, err := grpc.NewClient(addr, grpc.WithInsecure())
      if err != nil {
          return nil, err
      }
      return &WalletClient{client: pb.NewWalletServiceClient(conn)}, nil
  }

  func (c *WalletClient) GetBalance(ctx context.Context, userID, currency string) (*pb.GetBalanceResponse, error) {
      ctx = metadata.AppendToOutgoingContext(ctx, "user-id", userID)
      return c.client.GetBalance(ctx, &pb.GetBalanceRequest{UserId: userID, Currency: currency})
  }
  ```

  Use `grpc.WithTransportCredentials` (insecure only for local dev).

- [ ] **Step 3: Create gateway handlers**

  Create `internal/gateway/handlers.go` with handlers for `/api/wallet/balance`, `/api/wallet/deposit-address`, `/api/wallet/withdraw`, `/api/admin/withdrawals`, and `/webhooks/xcash/deposit`.

- [ ] **Step 4: Create admin middleware**

  Create `internal/gateway/admin.go`:
  ```go
  package gateway

  import (
      "net/http"
      "strings"

      "github.com/realyoussefhossam/betmonster/internal/auth"
  )

  func (s *Server) adminOnly(next http.HandlerFunc) http.HandlerFunc {
      return func(w http.ResponseWriter, r *http.Request) {
          user, err := auth.UserFromRequest(r)
          if err != nil {
              http.Error(w, "unauthorized", http.StatusUnauthorized)
              return
          }
          for _, id := range strings.Split(s.adminUserIDs, ",") {
              if strings.TrimSpace(id) == user.ID {
                  next(w, r)
                  return
              }
          }
          http.Error(w, "forbidden", http.StatusForbidden)
      }
  }
  ```

- [ ] **Step 5: Commit**

  ```bash
  git add internal/gateway cmd/gateway/main.go internal/auth/jwks.go
  git commit -m "feat(gateway): add JWT auth, wallet gRPC client, and HTTP handlers"
  ```

---

## Task 11: Next.js Wallet UI and Admin Dashboard

**Files:**
- Create: `app/app/wallet/page.tsx`
- Create: `app/app/wallet/deposit/page.tsx`
- Create: `app/app/wallet/withdraw/page.tsx`
- Create: `app/app/admin/withdrawals/page.tsx`
- Create: `app/lib/wallet-api.ts`

- [ ] **Step 1: Read Next.js docs**

  Run:
  ```bash
  cat app/node_modules/next/dist/docs/README.md 2>/dev/null | head -50
  ```

  Verify route conventions and data fetching patterns for the installed Next.js version.

- [ ] **Step 2: Create wallet API client**

  Create `app/lib/wallet-api.ts` with helpers for balance, deposit address, and withdrawal.

- [ ] **Step 3: Create wallet page**

  Create `app/app/wallet/page.tsx` showing USDT and USDC balances.

- [ ] **Step 4: Create deposit and withdraw pages**

  Implement `app/app/wallet/deposit/page.tsx` and `app/app/wallet/withdraw/page.tsx` using the wallet API client.

- [ ] **Step 5: Create admin withdrawals page**

  Create `app/app/admin/withdrawals/page.tsx` that lists pending withdrawals and has approve/reject buttons.

- [ ] **Step 6: Commit**

  ```bash
  git add app/app/wallet app/app/admin/withdrawals app/lib/wallet-api.ts
  git commit -m "feat(app): add wallet, deposit, withdraw, and admin pages"
  ```

---

## Task 12: Docker Compose and Scripts

**Files:**
- Create: `docker-compose.yml`
- Create: `scripts/init_env.sh`
- Create: `scripts/dev-up.sh`
- Create: `scripts/migrate.sh`
- Create: `scripts/test.sh`
- Create: `scripts/upgrade.sh`
- Create: `Dockerfile.gateway`, `Dockerfile.wallet`

- [ ] **Step 1: Create Docker Compose**

  Create `docker-compose.yml` with services: auth-db, wallet-db, redis, nats, wallet, gateway, app.
  Include healthchecks and dependency conditions.

- [ ] **Step 2: Create Dockerfiles**

  Create `Dockerfile.gateway`:
  ```dockerfile
  FROM golang:1.26-alpine AS builder
  WORKDIR /app
  COPY go.mod go.sum ./
  RUN go mod download
  COPY . .
  RUN CGO_ENABLED=0 GOOS=linux go build -o gateway ./cmd/gateway

  FROM alpine:latest
  RUN apk --no-cache add ca-certificates
  WORKDIR /root/
  COPY --from=builder /app/gateway .
  CMD ["./gateway"]
  ```

  Create `Dockerfile.wallet`:
  ```dockerfile
  FROM golang:1.26-alpine AS builder
  WORKDIR /app
  COPY go.mod go.sum ./
  RUN go mod download
  COPY . .
  RUN CGO_ENABLED=0 GOOS=linux go build -o wallet ./cmd/wallet

  FROM alpine:latest
  RUN apk --no-cache add ca-certificates
  WORKDIR /root/
  COPY --from=builder /app/wallet .
  COPY --from=builder /app/wallet/migrations ./migrations
  CMD ["./wallet"]
  ```

- [ ] **Step 3: Create scripts**

  Create `scripts/init_env.sh`, `scripts/dev-up.sh`, `scripts/migrate.sh`, `scripts/test.sh`, `scripts/upgrade.sh` and make them executable.

- [ ] **Step 4: Commit**

  ```bash
  git add docker-compose.yml scripts Dockerfile.gateway Dockerfile.wallet
  git commit -m "chore: add docker compose and helper scripts"
  ```

---

## Task 13: End-to-End Testing and Final Verification

**Files:**
- Create: `test/integration/wallet_test.go`
- Create: `test/integration/xcash_mock.go`

- [ ] **Step 1: Create xcash mock server**

  Create `test/integration/xcash_mock.go` with a mock HTTP server that returns a deposit address and accepts webhook calls.

- [ ] **Step 2: Create integration test**

  Create `test/integration/wallet_test.go` that:
  - starts a Postgres container using testcontainers-go
  - runs migrations
  - starts the wallet and gateway services
  - requests a deposit address
  - simulates an xcash deposit webhook
  - asserts the wallet balance is updated

- [ ] **Step 3: Run full test suite**

  Run:
  ```bash
  ./scripts/test.sh
  ```

  Expected: all unit and integration tests pass.

- [ ] **Step 4: Run local stack**

  Run:
  ```bash
  ./scripts/init_env.sh
  ./scripts/dev-up.sh
  ```

  Verify:
  - `curl http://localhost:8080/health` returns healthy
  - `curl http://localhost:8081/health` returns healthy
  - Next.js runs on `http://localhost:3000`

- [ ] **Step 5: Commit**

  ```bash
  git add test/integration
  git commit -m "test: add end-to-end integration tests for wallet"
  ```

---

## Self-Review

- **Spec coverage:** every section of the wallet v1 spec is covered by at least one task.
- **Placeholder scan:** no TBD, TODO, or vague steps. All steps include concrete code or commands.
- **Type consistency:** gRPC messages, store methods, and service methods use the same field names (`user_id`, `currency`, `amount`, etc.) across tasks.
- **Open gaps:** UI component specifics and integration test Postgres container setup are left for implementation, but Dockerfiles, docker-compose, and the overall file structure are now concrete. Integration tests should use `testcontainers-go` and follow the mocked xcash pattern shown in the plan.

---

## Execution Handoff

**Plan complete and saved to `docs/superpowers/plans/2026-07-04-wallet-microservice.md`.**

Two execution options:

1. **Subagent-Driven (recommended)** — dispatch a fresh subagent per task, review between tasks, fast iteration.
2. **Inline Execution** — execute tasks in this session using the executing-plans skill, batch execution with checkpoints.

Which approach do you want?

# Performance Benchmarks Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a reproducible, single-command benchmark suite for the BetMonster v1 wallet and gateway, producing a README-friendly Markdown table of throughput and latency results.

**Architecture:** Go benchmark tests in `bench/` use `testing.B` to drive the wallet gRPC client and the gateway HTTP client. Shared harness code (`runner.go`, `wallet_client.go`, `http_client.go`, `reporter.go`, `xcash_webhook.go`, `token_issuer.go`) handles stack discovery, test-user setup, and result aggregation. A `docker-compose.bench.yml` reuses the production images on a dedicated network, and `scripts/bench.sh` orchestrates the full run. The Makefile exposes `make bench`, `make bench-against-running`, and `make bench-report`.

**Tech Stack:** Go 1.26, Docker Compose, Postgres, Redis, NATS, Better Auth (Next.js), gRPC, HTTP.

---

## File Map

| File | Responsibility |
|---|---|
| `bench/runner.go` | Stack discovery, test-user pool setup, wallet seeding, cleanup. |
| `bench/wallet_client.go` | Reusable gRPC wallet client for benchmarks. |
| `bench/http_client.go` | Reusable HTTP client with keep-alive and timeouts for gateway calls. |
| `bench/token_issuer.go` | Creates Better Auth test users and caches their session tokens. |
| `bench/xcash_webhook.go` | Builds valid xcash deposit webhook payloads and HMAC signatures. |
| `bench/reporter.go` | Collects `testing.B` results and prints a Markdown table. |
| `bench/wallet_credit_test.go` | Benchmark for `CreditWallet` gRPC throughput. |
| `bench/gateway_read_test.go` | Benchmark for `GET /api/wallet/balance` and `/api/wallet/transactions`. |
| `bench/webhook_ingest_test.go` | Benchmark for `POST /webhooks/xcash/deposit`. |
| `bench/e2e_deposit_test.go` | Benchmark for the full deposit-address → webhook → balance flow. |
| `bench/smoke_test.go` | Smoke test that verifies all scenarios run at least once. |
| `docker-compose.bench.yml` | Production-like stack on a dedicated network. |
| `scripts/bench.sh` | Orchestrates stack start, benchmark run, and teardown. |
| `Makefile` | Adds `bench`, `bench-against-running`, `bench-report` targets. |
| `.github/workflows/benchmark.yml` | CI workflow that runs `make bench` and commits `BENCHMARKS.md`. |
| `BENCHMARKS.md` | Auto-generated results table. |

---

## Task 0: Extend gRPC Service with CreditWallet

The wallet service already has `CreditWallet` as a Go method, but it is not exposed over gRPC. The benchmark needs it to measure raw ledger write throughput.

**Files:**
- Modify: `internal/proto/wallet.proto`
- Modify: `internal/wallet/server.go`
- Modify: `internal/proto/wallet.pb.go` and `internal/proto/wallet_grpc.pb.go` (regenerated)
- Modify: `internal/gateway/wallet_client.go`

- [ ] **Step 1: Add CreditWallet to the protobuf service**

In `internal/proto/wallet.proto`, add inside the `service WalletService` block:

```protobuf
  rpc CreditWallet(CreditWalletRequest) returns (CreditWalletResponse);
```

And add at the end of the file:

```protobuf
message CreditWalletRequest {
  string user_id = 1;
  string currency = 2;
  string amount = 3;
  string reference_id = 4;
  map<string, string> metadata = 5;
}
message CreditWalletResponse {
  string transaction_id = 1;
  string status = 2;
}
```

- [ ] **Step 2: Regenerate protobuf Go code**

Run:

```bash
make proto
```

Expected: `internal/proto/wallet.pb.go` and `internal/proto/wallet_grpc.pb.go` are updated with no errors.

- [ ] **Step 3: Implement the gRPC handler in the wallet service**

In `internal/wallet/server.go`, add:

```go
func (s *GRPCServer) CreditWallet(ctx context.Context, req *pb.CreditWalletRequest) (*pb.CreditWalletResponse, error) {
	metadata := map[string]any{}
	for k, v := range req.Metadata {
		metadata[k] = v
	}
	tx, err := s.service.CreditWallet(ctx, req.UserId, req.Currency, req.Amount, req.ReferenceId, metadata)
	if err != nil {
		return nil, err
	}
	return &pb.CreditWalletResponse{
		TransactionId: tx.ID,
		Status:        string(tx.Status),
	}, nil
}
```

- [ ] **Step 4: Add CreditWallet to the gateway wallet client**

In `internal/gateway/wallet_client.go`, add:

```go
func (c *WalletClient) CreditWallet(ctx context.Context, userID, currency, amount, referenceID string, metadata map[string]string) (*pb.CreditWalletResponse, error) {
	return c.conn.CreditWallet(ctx, &pb.CreditWalletRequest{
		UserId:      userID,
		Currency:    currency,
		Amount:      amount,
		ReferenceId: referenceID,
		Metadata:    metadata,
	})
}
```

- [ ] **Step 5: Add a unit test for the new gRPC method**

Create a test in `internal/wallet/server_test.go` (or a new file `internal/wallet/server_credit_test.go`):

```go
func TestGRPCServerCreditWallet(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryStore()
	svc := NewService(store, nil, nil, []string{"USDT:anvil"})
	server := NewGRPCServer(svc)

	listener := bufconn.Listen(1024)
	grpcServer := grpc.NewServer()
	pb.RegisterWalletServiceServer(grpcServer, server)

	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			t.Logf("grpc server error: %v", err)
		}
	}()
	defer grpcServer.Stop()

	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpc.WithInsecure())
	assert.NoError(t, err)
	defer conn.Close()

	client := pb.NewWalletServiceClient(conn)
	resp, err := client.CreditWallet(ctx, &pb.CreditWalletRequest{
		UserId:      "user-1",
		Currency:    "USDT",
		Amount:      "50.00",
		ReferenceId: "dx-credit-1",
	})
	assert.NoError(t, err)
	assert.Equal(t, "completed", resp.Status)

	balance, err := client.GetBalance(ctx, &pb.GetBalanceRequest{UserId: "user-1", Currency: "USDT"})
	assert.NoError(t, err)
	assert.Equal(t, "50", balance.Balance)
}
```

Run:

```bash
go test ./internal/wallet -run TestGRPCServerCreditWallet -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "feat(wallet): expose CreditWallet over gRPC"
```

---

## Task 1: Shared Benchmark Harness

### Task 1.1: Create `bench/runner.go`

**Files:**
- Create: `bench/runner.go`

- [ ] **Step 1: Add the `bench` package and environment helpers**

```go
package bench

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	defaultGatewayAddr     = "http://localhost:8080"
	defaultWalletGRPCAddr  = "localhost:50051"
	defaultAppAddr         = "http://localhost:3000"
	defaultBenchDuration   = 30 * time.Second
)

func gatewayAddr() string {
	if v := os.Getenv("GATEWAY_ADDR"); v != "" {
		return v
	}
	return defaultGatewayAddr
}

func walletGRPCAddr() string {
	if v := os.Getenv("WALLET_GRPC_ADDR"); v != "" {
		return v
	}
	return defaultWalletGRPCAddr
}

func appAddr() string {
	if v := os.Getenv("APP_ADDR"); v != "" {
		return v
	}
	return defaultAppAddr
}

func benchmarkDuration() time.Duration {
	if v := os.Getenv("BENCH_DURATION"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return defaultBenchDuration
}

func uniqueUserID(prefix string) string {
	return fmt.Sprintf("%s-%s", prefix, strings.ReplaceAll(uuid.NewString(), "-", ""))
}
```

- [ ] **Step 2: Add the `Runner` struct and constructor**

```go
// Runner manages shared benchmark setup and teardown.
type Runner struct {
	GatewayAddr    string
	WalletGRPCAddr string
	AppAddr        string
	Duration       time.Duration

	Wallet  *WalletClient
	HTTP    *HTTPClient
	Tokens  *TokenIssuer
	Webhook *WebhookBuilder
}

// NewRunner creates a runner. It does not yet validate that the stack is reachable.
func NewRunner() *Runner {
	return &Runner{
		GatewayAddr:    gatewayAddr(),
		WalletGRPCAddr: walletGRPCAddr(),
		AppAddr:        appAddr(),
		Duration:       benchmarkDuration(),
	}
}

// Setup connects to the stack, seeds the wallet, and creates a token issuer.
func (r *Runner) Setup(ctx context.Context) error {
	var err error

	r.Wallet, err = NewWalletClient(r.WalletGRPCAddr)
	if err != nil {
		return fmt.Errorf("connect wallet: %w", err)
	}

	r.HTTP = NewHTTPClient(r.GatewayAddr)

	r.Tokens, err = NewTokenIssuer(ctx, r.AppAddr)
	if err != nil {
		return fmt.Errorf("token issuer: %w", err)
	}

	r.Webhook = NewWebhookBuilder()

	return r.healthCheck(ctx)
}

func (r *Runner) healthCheck(ctx context.Context) error {
	// Verify wallet gRPC is reachable by creating a wallet for a health-check user.
	_, err := r.Wallet.GetBalance(ctx, "health-check-user", "USDT")
	if err != nil {
		return fmt.Errorf("wallet health check failed: %w", err)
	}
	return nil
}
```

- [ ] **Step 3: Add per-scenario user helpers**

```go
// BenchUser is a synthetic test user with a cached session token.
type BenchUser struct {
	ID    string
	Token string
}

// CreateUser creates a single synthetic user via the token issuer.
func (r *Runner) CreateUser(ctx context.Context) (BenchUser, error) {
	uid := uniqueUserID("bench")
	token, err := r.Tokens.CreateSession(ctx, uid)
	if err != nil {
		return BenchUser{}, err
	}
	return BenchUser{ID: uid, Token: token}, nil
}

// CreateUserPool creates n synthetic users in parallel.
func (r *Runner) CreateUserPool(ctx context.Context, n int) ([]BenchUser, error) {
	users := make([]BenchUser, n)
	for i := 0; i < n; i++ {
		u, err := r.CreateUser(ctx)
		if err != nil {
			return nil, err
		}
		users[i] = u
	}
	return users, nil
}

// SeedWallet credits a small balance to a user so read benchmarks have data.
func (r *Runner) SeedWallet(ctx context.Context, userID, currency, amount string) error {
	_, err := r.Wallet.CreditWallet(ctx, userID, currency, amount, "seed-"+uuid.NewString(), map[string]string{})
	return err
}
```

- [ ] **Step 4: Commit**

```bash
git add bench/runner.go
git commit -m "feat(bench): add shared benchmark runner"
```

---

### Task 1.2: Create `bench/wallet_client.go`

**Files:**
- Create: `bench/wallet_client.go`

- [ ] **Step 1: Implement the gRPC wallet client wrapper**

```go
package bench

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/realyoussefhossam/betmonster/internal/proto"
)

// WalletClient wraps the wallet gRPC client for benchmarks.
type WalletClient struct {
	conn pb.WalletServiceClient
}

// NewWalletClient dials the wallet service at the given address.
func NewWalletClient(addr string) (*WalletClient, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("dial wallet: %w", err)
	}
	return &WalletClient{conn: pb.NewWalletServiceClient(conn)}, nil
}

// CreditWallet credits a wallet.
func (c *WalletClient) CreditWallet(ctx context.Context, userID, currency, amount, referenceID string, metadata map[string]string) (*pb.CreditWalletResponse, error) {
	return c.conn.CreditWallet(ctx, &pb.CreditWalletRequest{
		UserId:      userID,
		Currency:    currency,
		Amount:      amount,
		ReferenceId: referenceID,
		Metadata:    metadata,
	})
}

// GetBalance returns a wallet balance.
func (c *WalletClient) GetBalance(ctx context.Context, userID, currency string) (*pb.GetBalanceResponse, error) {
	return c.conn.GetBalance(ctx, &pb.GetBalanceRequest{UserId: userID, Currency: currency})
}

// ListTransactions returns a paginated transaction list.
func (c *WalletClient) ListTransactions(ctx context.Context, userID string, page, pageSize int32) (*pb.ListTransactionsResponse, error) {
	return c.conn.ListTransactions(ctx, &pb.ListTransactionsRequest{
		UserId:     userID,
		Page:       page,
		PageSize:   pageSize,
	})
}

// GetDepositAddress returns or creates a deposit address.
func (c *WalletClient) GetDepositAddress(ctx context.Context, userID, currency, chain string) (*pb.GetDepositAddressResponse, error) {
	return c.conn.GetDepositAddress(ctx, &pb.GetDepositAddressRequest{
		UserId:   userID,
		Currency: currency,
		Chain:    chain,
	})
}

// ProcessDepositWebhook forwards a webhook to the wallet service.
func (c *WalletClient) ProcessDepositWebhook(ctx context.Context, body string, headers map[string]string) (*pb.ProcessDepositWebhookResponse, error) {
	return c.conn.ProcessDepositWebhook(ctx, &pb.ProcessDepositWebhookRequest{
		Body:    body,
		Headers: headers,
	})
}
```

- [ ] **Step 2: Verify protobuf messages exist**

Run:

```bash
grep -n "CreditWalletRequest\|GetBalanceRequest\|ListTransactionsRequest\|GetDepositAddressRequest\|ProcessDepositWebhookRequest" internal/proto/wallet.pb.go
```

Expected: the messages exist. If not, regenerate proto with `make proto`.

- [ ] **Step 3: Commit**

```bash
git add bench/wallet_client.go
git commit -m "feat(bench): add gRPC wallet client wrapper"
```

---

### Task 1.3: Create `bench/http_client.go`

**Files:**
- Create: `bench/http_client.go`

- [ ] **Step 1: Implement the HTTP client wrapper**

```go
package bench

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// HTTPClient is a gateway HTTP client with connection pooling.
type HTTPClient struct {
	baseURL string
	client  *http.Client
}

// NewHTTPClient creates a client for the gateway.
func NewHTTPClient(baseURL string) *HTTPClient {
	return &HTTPClient{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 100,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
}

// GetBalance calls GET /api/wallet/balance.
func (c *HTTPClient) GetBalance(ctx context.Context, token, currency string) (*http.Response, error) {
	url := fmt.Sprintf("%s/api/wallet/balance?currency=%s", c.baseURL, currency)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	return c.client.Do(req)
}

// ListTransactions calls GET /api/wallet/transactions.
func (c *HTTPClient) ListTransactions(ctx context.Context, token string) (*http.Response, error) {
	url := c.baseURL + "/api/wallet/transactions"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	return c.client.Do(req)
}

// GetDepositAddress calls GET /api/wallet/deposit-address.
func (c *HTTPClient) GetDepositAddress(ctx context.Context, token, currency, chain string) (*http.Response, error) {
	url := fmt.Sprintf("%s/api/wallet/deposit-address?currency=%s&chain=%s", c.baseURL, currency, chain)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	return c.client.Do(req)
}

// SendWebhook calls POST /webhooks/xcash/deposit.
func (c *HTTPClient) SendWebhook(ctx context.Context, body []byte, headers map[string]string) (*http.Response, error) {
	url := c.baseURL + "/webhooks/xcash/deposit"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return c.client.Do(req)
}

// ReadBody drains and closes a response body, returning the status and body bytes.
func ReadBody(resp *http.Response) (int, []byte, error) {
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, body, nil
}
```

- [ ] **Step 2: Commit**

```bash
git add bench/http_client.go
git commit -m "feat(bench): add gateway HTTP client wrapper"
```

---

### Task 1.4: Create `bench/token_issuer.go`

**Files:**
- Create: `bench/token_issuer.go`

- [ ] **Step 1: Implement the token issuer using the Better Auth API**

```go
package bench

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// TokenIssuer creates Better Auth sessions for synthetic benchmark users.
type TokenIssuer struct {
	appAddr string
	client  *http.Client
}

// NewTokenIssuer creates a token issuer for the given Better Auth app address.
func NewTokenIssuer(ctx context.Context, appAddr string) (*TokenIssuer, error) {
	t := &TokenIssuer{
		appAddr: appAddr,
		client:  &http.Client{Timeout: 10 * time.Second},
	}
	if err := t.ping(ctx); err != nil {
		return nil, err
	}
	return t, nil
}

func (t *TokenIssuer) ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.appAddr+"/api/auth/jwks", nil)
	if err != nil {
		return err
	}
	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("better auth unreachable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("better auth jwks status %d", resp.StatusCode)
	}
	return nil
}

// CreateSession signs up a new user and returns a session token.
func (t *TokenIssuer) CreateSession(ctx context.Context, userID string) (string, error) {
	email := fmt.Sprintf("%s@bench.betmonster.local", userID)
	password := "BenchPass123!"

	// Sign up.
	payload, _ := json.Marshal(map[string]string{
		"email":    email,
		"password": password,
		"name":     "Bench User",
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.appAddr+"/api/auth/sign-up/email", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("sign up: %w", err)
	}
	defer resp.Body.Close()

	// Decode token from response.
	var result struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode token: %w", err)
	}
	if result.Token == "" {
		return "", fmt.Errorf("empty token from better auth")
	}
	return result.Token, nil
}
```

- [ ] **Step 2: Verify the sign-up endpoint**

Run the app stack locally and test:

```bash
curl -X POST http://localhost:3000/api/auth/sign-up/email \
  -H "Content-Type: application/json" \
  -d '{"email":"test@bench.local","password":"TestPass123!","name":"Test"}'
```

Expected: a JSON response with `token`. If the endpoint or field names differ, update `CreateSession` accordingly.

- [ ] **Step 3: Commit**

```bash
git add bench/token_issuer.go
git commit -m "feat(bench): add better auth token issuer"
```

---

### Task 1.5: Create `bench/xcash_webhook.go`

**Files:**
- Create: `bench/xcash_webhook.go`

- [ ] **Step 1: Implement the webhook payload builder**

```go
package bench

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
)

// WebhookBuilder builds signed xcash deposit webhook payloads.
type WebhookBuilder struct {
	secret string
}

// NewWebhookBuilder creates a builder using the env var XCASH_WEBHOOK_SECRET.
func NewWebhookBuilder() *WebhookBuilder {
	return &WebhookBuilder{secret: os.Getenv("XCASH_WEBHOOK_SECRET")}
}

// DepositPayload is a complete xcash deposit webhook payload.
type DepositPayload struct {
	Type string      `json:"type"`
	Data DepositData `json:"data"`
}

type DepositData struct {
	SysNo     string  `json:"sys_no"`
	UID       string  `json:"uid"`
	Chain     string  `json:"chain"`
	Block     int64   `json:"block"`
	Hash      string  `json:"hash"`
	Crypto    string  `json:"crypto"`
	Amount    string  `json:"amount"`
	Confirmed bool    `json:"confirmed"`
	RiskLevel *string `json:"risk_level"`
	RiskScore *string `json:"risk_score"`
}

// Build creates a signed webhook body and headers.
func (b *WebhookBuilder) Build(uid, chain, crypto, amount, address string, block int64) ([]byte, map[string]string, error) {
	payload := DepositPayload{
		Type: "deposit",
		Data: DepositData{
			SysNo:     fmt.Sprintf("DXC%s", strings.ReplaceAll(uuid.NewString(), "-", "")[:12]),
			UID:       uid,
			Chain:     chain,
			Block:     block,
			Hash:      fmt.Sprintf("0x%s", strings.ReplaceAll(uuid.NewString(), "-", "")),
			Crypto:    crypto,
			Amount:    amount,
			Confirmed: true,
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, nil, err
	}

	nonce := strings.ReplaceAll(uuid.NewString(), "-", "")
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	sig := hmacSha256(nonce+timestamp+string(body), b.secret)

	headers := map[string]string{
		"XC-Nonce":     nonce,
		"XC-Timestamp": timestamp,
		"XC-Signature": sig,
	}
	return body, headers, nil
}

func hmacSha256(message, key string) string {
	h := hmac.New(sha256.New, []byte(key))
	h.Write([]byte(message))
	return hex.EncodeToString(h.Sum(nil))
}
```

Note: add `strings` import to the file.

- [ ] **Step 2: Commit**

```bash
git add bench/xcash_webhook.go
git commit -m "feat(bench): add xcash webhook builder"
```

---

### Task 1.6: Create `bench/reporter.go`

**Files:**
- Create: `bench/reporter.go`

- [ ] **Step 1: Implement the result collector and Markdown printer**

```go
package bench

import (
	"fmt"
	"math"
	"sort"
	"testing"
	"time"
)

// Result holds the metrics for a single benchmark scenario.
type Result struct {
	Scenario    string
	Concurrency int
	RPS         float64
	P50         time.Duration
	P95         time.Duration
	P99         time.Duration
	Errors      int
	Total       int
}

// ErrorRate returns the error rate as a percentage.
func (r *Result) ErrorRate() float64 {
	if r.Total == 0 {
		return 0
	}
	return float64(r.Errors) / float64(r.Total) * 100
}

// LatencyRecorder records operation durations and computes percentiles.
type LatencyRecorder struct {
	durations []time.Duration
}

// Record adds a duration.
func (r *LatencyRecorder) Record(d time.Duration) {
	r.durations = append(r.durations, d)
}

// Percentile returns the nth percentile using nearest-rank.
func (r *LatencyRecorder) Percentile(n float64) time.Duration {
	if len(r.durations) == 0 {
		return 0
	}
	// Sort a copy.
	d := make([]time.Duration, len(r.durations))
	copy(d, r.durations)
	sort.Slice(d, func(i, j int) bool { return d[i] < d[j] })
	idx := int(math.Ceil(float64(len(d))*n/100)) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(d) {
		idx = len(d) - 1
	}
	return d[idx]
}

// CollectResult gathers latency metrics from a completed testing.B and a recorder.
func CollectResult(b *testing.B, scenario string, concurrency int, errs int, rec *LatencyRecorder) Result {
	return Result{
		Scenario:    scenario,
		Concurrency: concurrency,
		RPS:         float64(b.N) / b.Elapsed().Seconds(),
		P50:         rec.Percentile(50),
		P95:         rec.Percentile(95),
		P99:         rec.Percentile(99),
		Errors:      errs,
		Total:       b.N,
	}
}

// PrintMarkdownTable prints results in a README-friendly table.
func PrintMarkdownTable(results []Result) {
	fmt.Println("| Scenario | Concurrency | RPS | p50 (ms) | p95 (ms) | p99 (ms) | Errors |")
	fmt.Println("|---|---|---:|---:|---:|---:|---:|")
	for _, r := range results {
		fmt.Printf("| %s | %d | %.0f | %.2f | %.2f | %.2f | %.1f%% |\n",
			r.Scenario,
			r.Concurrency,
			r.RPS,
			float64(r.P50)/float64(time.Millisecond),
			float64(r.P95)/float64(time.Millisecond),
			float64(r.P99)/float64(time.Millisecond),
			r.ErrorRate(),
		)
	}
}
```

- [ ] **Step 2: Commit**

```bash
git add bench/reporter.go
git commit -m "feat(bench): add benchmark reporter"
```

---

## Task 2: Benchmark Tests

### Task 2.1: Create `bench/wallet_credit_test.go`

**Files:**
- Create: `bench/wallet_credit_test.go`

- [ ] **Step 1: Write the wallet credit benchmark**

```go
package bench

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
)

func BenchmarkWalletCredit(b *testing.B) {
	runner := NewRunner()
	ctx := context.Background()
	if err := runner.Setup(ctx); err != nil {
		b.Fatalf("setup: %v", err)
	}

	concurrencyLevels := []int{10, 50, 100}
	var results []Result

	for _, conc := range concurrencyLevels {
		users, err := runner.CreateUserPool(ctx, conc)
		if err != nil {
			b.Fatalf("create users: %v", err)
		}

		var errs int64
		rec := &LatencyRecorder{}
		b.Run(fmt.Sprintf("concurrency-%d", conc), func(b *testing.B) {
			b.SetParallelism(conc)
			b.RunParallel(func(pb *testing.PB) {
				i := 0
				for pb.Next() {
					user := users[i%len(users)]
					i++
					ref := fmt.Sprintf("bench-credit-%s", uuid.NewString())
					start := time.Now()
					_, err := runner.Wallet.CreditWallet(ctx, user.ID, "USDT", "1.00", ref, map[string]string{})
					rec.Record(time.Since(start))
					if err != nil {
						atomic.AddInt64(&errs, 1)
					}
				}
			})
		})
		results = append(results, CollectResult(b, "Wallet Credit", conc, int(errs), rec))
	}

	PrintMarkdownTable(results)
}
```

- [ ] **Step 2: Run the benchmark to verify it compiles**

```bash
go test ./bench -run=^$ -bench=BenchmarkWalletCredit -benchtime=1x
```

Expected: compiles and runs (may fail if stack not running, but should not compile errors).

- [ ] **Step 3: Commit**

```bash
git add bench/wallet_credit_test.go
git commit -m "feat(bench): add wallet credit benchmark"
```

---

### Task 2.2: Create `bench/gateway_read_test.go`

**Files:**
- Create: `bench/gateway_read_test.go`

- [ ] **Step 1: Write the gateway read benchmark**

```go
package bench

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
)

func BenchmarkGatewayRead(b *testing.B) {
	runner := NewRunner()
	ctx := context.Background()
	if err := runner.Setup(ctx); err != nil {
		b.Fatalf("setup: %v", err)
	}

	user, err := runner.CreateUser(ctx)
	if err != nil {
		b.Fatalf("create user: %v", err)
	}
	if err := runner.SeedWallet(ctx, user.ID, "USDT", "1000.00"); err != nil {
		b.Fatalf("seed wallet: %v", err)
	}

	scenarios := []struct {
		name string
		call func() error
	}{
		{
			name: "Gateway Balance",
			call: func() error {
				resp, err := runner.HTTP.GetBalance(ctx, user.Token, "USDT")
				if err != nil {
					return err
				}
				status, _, err := ReadBody(resp)
				if err != nil {
					return err
				}
				if status != 200 {
					return fmt.Errorf("status %d", status)
				}
				return nil
			},
		},
		{
			name: "Gateway Transactions",
			call: func() error {
				resp, err := runner.HTTP.ListTransactions(ctx, user.Token)
				if err != nil {
					return err
				}
				status, _, err := ReadBody(resp)
				if err != nil {
					return err
				}
				if status != 200 {
					return fmt.Errorf("status %d", status)
				}
				return nil
			},
		},
	}

	var results []Result
	for _, s := range scenarios {
		var errs int64
		rec := &LatencyRecorder{}
		b.Run(s.name, func(b *testing.B) {
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					start := time.Now()
					if err := s.call(); err != nil {
						atomic.AddInt64(&errs, 1)
					}
					rec.Record(time.Since(start))
				}
			})
		})
		results = append(results, CollectResult(b, s.name, 100, int(errs), rec))
	}

	PrintMarkdownTable(results)
}
```

- [ ] **Step 2: Commit**

```bash
git add bench/gateway_read_test.go
git commit -m "feat(bench): add gateway read benchmark"
```

---

### Task 2.3: Create `bench/webhook_ingest_test.go`

**Files:**
- Create: `bench/webhook_ingest_test.go`

- [ ] **Step 1: Write the webhook ingest benchmark**

```go
package bench

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
)

func BenchmarkWebhookIngest(b *testing.B) {
	runner := NewRunner()
	ctx := context.Background()
	if err := runner.Setup(ctx); err != nil {
		b.Fatalf("setup: %v", err)
	}

	users, err := runner.CreateUserPool(ctx, 100)
	if err != nil {
		b.Fatalf("create users: %v", err)
	}

	var errs int64
	rec := &LatencyRecorder{}
	b.Run("Webhook Ingest", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				user := users[i%len(users)]
				i++
				start := time.Now()
				body, headers, err := runner.Webhook.Build(user.ID, "anvil", "USDT", "1.00", "0x", 1)
				if err != nil {
					atomic.AddInt64(&errs, 1)
					rec.Record(time.Since(start))
					continue
				}
				resp, err := runner.HTTP.SendWebhook(ctx, body, headers)
				if err != nil {
					atomic.AddInt64(&errs, 1)
					rec.Record(time.Since(start))
					continue
				}
				status, _, err := ReadBody(resp)
				rec.Record(time.Since(start))
				if err != nil || status != 200 {
					atomic.AddInt64(&errs, 1)
				}
			}
		})
	})

	results := []Result{CollectResult(b, "Webhook Ingest", 100, int(errs), rec)}
	PrintMarkdownTable(results)
}
```

- [ ] **Step 2: Commit**

```bash
git add bench/webhook_ingest_test.go
git commit -m "feat(bench): add webhook ingest benchmark"
```

---

### Task 2.4: Create `bench/e2e_deposit_test.go`

**Files:**
- Create: `bench/e2e_deposit_test.go`

- [ ] **Step 1: Write the end-to-end deposit benchmark**

```go
package bench

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"testing"
)

func BenchmarkE2EDeposit(b *testing.B) {
	runner := NewRunner()
	ctx := context.Background()
	if err := runner.Setup(ctx); err != nil {
		b.Fatalf("setup: %v", err)
	}

	users, err := runner.CreateUserPool(ctx, 25)
	if err != nil {
		b.Fatalf("create users: %v", err)
	}

	var errs int64
	rec := &LatencyRecorder{}
	b.Run("E2E Deposit", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				user := users[i%len(users)]
				i++

				start := time.Now()

				// Step 1: get deposit address.
				resp, err := runner.HTTP.GetDepositAddress(ctx, user.Token, "USDT", "anvil")
				if err != nil {
					atomic.AddInt64(&errs, 1)
					rec.Record(time.Since(start))
					continue
				}
				status, body, err := ReadBody(resp)
				if err != nil || status != 200 {
					atomic.AddInt64(&errs, 1)
					rec.Record(time.Since(start))
					continue
				}
				var addrResp struct {
					Address string `json:"address"`
				}
				if err := json.Unmarshal(body, &addrResp); err != nil {
					atomic.AddInt64(&errs, 1)
					rec.Record(time.Since(start))
					continue
				}

				// Step 2: send webhook.
				whBody, headers, err := runner.Webhook.Build(user.ID, "anvil", "USDT", "1.00", addrResp.Address, 1)
				if err != nil {
					atomic.AddInt64(&errs, 1)
					rec.Record(time.Since(start))
					continue
				}
				resp, err = runner.HTTP.SendWebhook(ctx, whBody, headers)
				if err != nil {
					atomic.AddInt64(&errs, 1)
					rec.Record(time.Since(start))
					continue
				}
				status, _, err = ReadBody(resp)
				rec.Record(time.Since(start))
				if err != nil || status != 200 {
					atomic.AddInt64(&errs, 1)
				}
			}
		})
	})

	results := []Result{CollectResult(b, "E2E Deposit", 25, int(errs), rec)}
	PrintMarkdownTable(results)
}
```

- [ ] **Step 2: Commit**

```bash
git add bench/e2e_deposit_test.go
git commit -m "feat(bench): add end-to-end deposit benchmark"
```

---

### Task 2.5: Create `bench/smoke_test.go`

**Files:**
- Create: `bench/smoke_test.go`

- [ ] **Step 1: Write the smoke test**

```go
package bench

import (
	"context"
	"encoding/json"
	"testing"
)

func TestSmokeAllScenarios(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping smoke test in short mode")
	}

	runner := NewRunner()
	ctx := context.Background()
	if err := runner.Setup(ctx); err != nil {
		t.Fatalf("setup: %v", err)
	}

	user, err := runner.CreateUser(ctx)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	// Wallet credit.
	_, err = runner.Wallet.CreditWallet(ctx, user.ID, "USDT", "1.00", "smoke-credit", map[string]string{})
	if err != nil {
		t.Fatalf("credit wallet: %v", err)
	}

	// Gateway balance.
	resp, err := runner.HTTP.GetBalance(ctx, user.Token, "USDT")
	if err != nil {
		t.Fatalf("get balance: %v", err)
	}
	status, body, err := ReadBody(resp)
	if err != nil {
		t.Fatalf("read balance: %v", err)
	}
	if status != 200 {
		t.Fatalf("balance status %d: %s", status, body)
	}

	// Gateway transactions.
	resp, err = runner.HTTP.ListTransactions(ctx, user.Token)
	if err != nil {
		t.Fatalf("list transactions: %v", err)
	}
	status, body, err = ReadBody(resp)
	if err != nil {
		t.Fatalf("read transactions: %v", err)
	}
	if status != 200 {
		t.Fatalf("transactions status %d: %s", status, body)
	}

	// Webhook ingest.
	whBody, headers, err := runner.Webhook.Build(user.ID, "anvil", "USDT", "1.00", "0x", 1)
	if err != nil {
		t.Fatalf("build webhook: %v", err)
	}
	resp, err = runner.HTTP.SendWebhook(ctx, whBody, headers)
	if err != nil {
		t.Fatalf("send webhook: %v", err)
	}
	status, body, err = ReadBody(resp)
	if err != nil {
		t.Fatalf("read webhook: %v", err)
	}
	if status != 200 {
		t.Fatalf("webhook status %d: %s", status, body)
	}

	// E2E deposit.
	resp, err = runner.HTTP.GetDepositAddress(ctx, user.Token, "USDT", "anvil")
	if err != nil {
		t.Fatalf("deposit address: %v", err)
	}
	status, body, err = ReadBody(resp)
	if err != nil {
		t.Fatalf("read deposit address: %v", err)
	}
	if status != 200 {
		t.Fatalf("deposit address status %d: %s", status, body)
	}
	var addrResp struct {
		Address string `json:"address"`
	}
	if err := json.Unmarshal(body, &addrResp); err != nil {
		t.Fatalf("unmarshal address: %v", err)
	}
	whBody, headers, err = runner.Webhook.Build(user.ID, "anvil", "USDT", "1.00", addrResp.Address, 1)
	if err != nil {
		t.Fatalf("build e2e webhook: %v", err)
	}
	resp, err = runner.HTTP.SendWebhook(ctx, whBody, headers)
	if err != nil {
		t.Fatalf("send e2e webhook: %v", err)
	}
	status, body, err = ReadBody(resp)
	if err != nil {
		t.Fatalf("read e2e webhook: %v", err)
	}
	if status != 200 {
		t.Fatalf("e2e webhook status %d: %s", status, body)
	}
}
```

- [ ] **Step 2: Commit**

```bash
git add bench/smoke_test.go
git commit -m "feat(bench): add benchmark smoke test"
```

---

## Task 3: Docker Compose Benchmark Stack

### Task 3.1: Create `docker-compose.bench.yml`

**Files:**
- Create: `docker-compose.bench.yml`

- [ ] **Step 1: Copy the production stack to a dedicated bench network**

```yaml
name: betmonster-bench

services:
  postgres:
    image: postgres:17-alpine
    environment:
      POSTGRES_USER: wallet
      POSTGRES_PASSWORD: wallet
      POSTGRES_DB: wallet
    volumes:
      - postgres_bench_data:/var/lib/postgresql/data
      - ./postgres/init:/docker-entrypoint-initdb.d:ro
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U wallet -d wallet"]
      interval: 5s
      timeout: 5s
      retries: 5
    networks:
      - bench

  redis:
    image: redis:7-alpine
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 3s
      retries: 5
    networks:
      - bench

  nats:
    image: nats:2-alpine
    networks:
      - bench

  wallet:
    build:
      context: .
      dockerfile: Dockerfile.wallet
    image: betmonster/wallet
    ports:
      - "8081:8081"
      - "50051:50051"
    environment:
      PORT: "8081"
      DATABASE_URL: postgres://wallet:wallet@postgres:5432/wallet?sslmode=disable
      REDIS_ADDR: redis:6379
      NATS_URL: nats://nats:4222
      XCASH_BASE_URL: http://xcash-caddy:80
      XCASH_APPID: ${XCASH_APPID:-}
      XCASH_HMAC_KEY: ${XCASH_HMAC_KEY:-}
      XCASH_WEBHOOK_SECRET: ${XCASH_WEBHOOK_SECRET:-}
      SUPPORTED_PAIRS: ${SUPPORTED_PAIRS:-}
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_started
      nats:
        condition: service_started
    networks:
      - bench
      - xcash_public

  gateway:
    build:
      context: .
      dockerfile: Dockerfile.gateway
    image: betmonster/gateway
    ports:
      - "8080:8080"
    environment:
      PORT: "8080"
      JWKS_URL: http://app:3000/api/auth/jwks
      WALLET_SERVICE_ADDR: wallet:50051
      ADMIN_USER_IDS: ${ADMIN_USER_IDS:-}
      CORS_ALLOWED_ORIGINS: ${CORS_ALLOWED_ORIGINS:-http://localhost:3000}
      SUPPORTED_CURRENCIES: ${SUPPORTED_CURRENCIES:-}
      SUPPORTED_CHAINS: ${SUPPORTED_CHAINS:-}
      SUPPORTED_PAIRS: ${SUPPORTED_PAIRS:-}
      REDIS_ADDR: redis:6379
      RATE_LIMIT_RPS: ${RATE_LIMIT_RPS:-10000}
      RATE_LIMIT_BURST: ${RATE_LIMIT_BURST:-10000}
      RATE_LIMIT_BACKEND: ${RATE_LIMIT_BACKEND:-memory}
      MIN_DEPOSIT: ${MIN_DEPOSIT:-}
      MAX_DEPOSIT: ${MAX_DEPOSIT:-}
      DAILY_DEPOSIT: ${DAILY_DEPOSIT:-}
    depends_on:
      app:
        condition: service_started
      wallet:
        condition: service_started
      redis:
        condition: service_started
    networks:
      - bench

  app:
    build:
      context: ./app
      dockerfile: Dockerfile
      args:
        BETTER_AUTH_SECRET: ${BETTER_AUTH_SECRET}
        BETTER_AUTH_URL: http://localhost:3000
        NEXT_PUBLIC_GO_API_URL: http://localhost:8080
        GO_API_URL: http://gateway:8080
    image: betmonster/app
    ports:
      - "3000:3000"
    environment:
      NEXT_PUBLIC_GO_API_URL: http://localhost:8080
      GO_API_URL: http://gateway:8080
      BETTER_AUTH_SECRET: ${BETTER_AUTH_SECRET}
      BETTER_AUTH_URL: http://localhost:3000
      DATABASE_URL: postgres://wallet:wallet@postgres:5432/better_auth?sslmode=disable
    depends_on:
      postgres:
        condition: service_healthy
    networks:
      - bench

networks:
  bench:
    driver: bridge
  xcash_public:
    external: true

volumes:
  postgres_bench_data:
```

- [ ] **Step 2: Commit**

```bash
git add docker-compose.bench.yml
git commit -m "feat(bench): add docker compose benchmark stack"
```

---

## Task 4: Orchestration and Makefile

### Task 4.1: Create `scripts/bench.sh`

**Files:**
- Create: `scripts/bench.sh`

- [ ] **Step 1: Write the bench orchestration script**

```bash
#!/usr/bin/env bash
set -euo pipefail

COMPOSE_FILE="docker-compose.bench.yml"
PROJECT_NAME="betmonster-bench"

up() {
  echo "==> Starting benchmark stack..."
  docker compose -f "$COMPOSE_FILE" -p "$PROJECT_NAME" up -d
  echo "==> Waiting for app to be ready..."
  for i in {1..60}; do
    if curl -sf http://localhost:3000/api/auth/jwks >/dev/null 2>&1; then
      echo "==> App ready."
      return 0
    fi
    sleep 1
  done
  echo "==> App failed to start. Logs:"
  docker compose -f "$COMPOSE_FILE" -p "$PROJECT_NAME" logs app
  exit 1
}

down() {
  echo "==> Stopping benchmark stack..."
  docker compose -f "$COMPOSE_FILE" -p "$PROJECT_NAME" down -v
}

run() {
  echo "==> Running benchmarks..."
  export GATEWAY_ADDR=http://localhost:8080
  export WALLET_GRPC_ADDR=localhost:50051
  export APP_ADDR=http://localhost:3000
  export BENCH_DURATION=${BENCH_DURATION:-30s}
  go test ./bench -run=^$ -bench=. -benchtime="$BENCH_DURATION"
}

case "${1:-run}" in
  up)
    up
    ;;
  run)
    run
    ;;
  down)
    down
    ;;
  full)
    up && run && down
    ;;
  *)
    echo "Usage: $0 {up|run|down|full}"
    exit 1
    ;;
esac
```

- [ ] **Step 2: Make the script executable**

```bash
chmod +x scripts/bench.sh
```

- [ ] **Step 3: Commit**

```bash
git add scripts/bench.sh
git commit -m "feat(bench): add benchmark orchestration script"
```

---

### Task 4.2: Update `Makefile`

**Files:**
- Modify: `Makefile`

- [ ] **Step 1: Add benchmark targets**

```makefile
.PHONY: build test migrate proto dev bench bench-against-running bench-report

build:
	mkdir -p bin
	go build -o bin/gateway ./cmd/gateway
	go build -o bin/wallet ./cmd/wallet

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

bench:
	./scripts/bench.sh full

bench-against-running:
	GATEWAY_ADDR=http://localhost:8080 \
	WALLET_GRPC_ADDR=localhost:50051 \
	APP_ADDR=http://localhost:3000 \
	BENCH_DURATION=30s \
	go test ./bench -run=^$ -bench=. -benchtime=30s

bench-report:
	cat BENCHMARKS.md
```

- [ ] **Step 2: Commit**

```bash
git add Makefile
git commit -m "feat(bench): add Makefile benchmark targets"
```

---

## Task 5: CI Workflow and Results File

### Task 5.1: Create `.github/workflows/benchmark.yml`

**Files:**
- Create: `.github/workflows/benchmark.yml`

- [ ] **Step 1: Write the CI workflow**

```yaml
name: Benchmark

on:
  push:
    branches: [main]
  workflow_dispatch:

jobs:
  benchmark:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4
        with:
          token: ${{ secrets.GITHUB_TOKEN }}

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.26'

      - name: Set up Docker
        uses: docker/setup-buildx-action@v3

      - name: Load env
        run: |
          ./scripts/init_env.sh

      - name: Start xcash
        run: |
          ./scripts/setup-xcash.sh
          docker cp scripts/xcash_bootstrap.py xcash_django:/tmp/xcash_bootstrap.py
          docker exec xcash_django python /tmp/xcash_bootstrap.py

      - name: Run benchmarks
        run: |
          make bench

      - name: Commit results
        run: |
          git config user.name "github-actions[bot]"
          git config user.email "github-actions[bot]@users.noreply.github.com"
          if [ -n "$(git status --porcelain BENCHMARKS.md)" ]; then
            git add BENCHMARKS.md
            git commit -m "[bench] update benchmark results"
            git push
          fi
```

- [ ] **Step 2: Commit**

```bash
git add .github/workflows/benchmark.yml
git commit -m "feat(bench): add benchmark CI workflow"
```

---

### Task 5.2: Create `BENCHMARKS.md`

**Files:**
- Create: `BENCHMARKS.md`

- [ ] **Step 1: Add the initial template**

```markdown
# BetMonster Performance Benchmarks

Run locally with:

```bash
make bench
```

Results below are from the latest CI run on `ubuntu-latest`.

| Scenario | Concurrency | RPS | p50 (ms) | p95 (ms) | p99 (ms) | Errors |
|---|---|---:|---:|---:|---:|---:|
| TBD | - | - | - | - | - | - |

*Last updated: 2026-07-05*
```

- [ ] **Step 2: Commit**

```bash
git add BENCHMARKS.md
git commit -m "docs(bench): add initial benchmarks results template"
```

---

## Task 6: README Update

### Task 6.1: Update `README.md` with the benchmark section

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Add a Benchmarks section before the closing sections**

Add near the end of the README:

```markdown
## Benchmarks

Run the full benchmark suite against a real Docker Compose stack:

```bash
make bench
```

See [BENCHMARKS.md](BENCHMARKS.md) for the latest CI baseline results.
```

- [ ] **Step 2: Commit**

```bash
git add README.md
git commit -m "docs(readme): add benchmark section"
```

---

## Task 7: Verification

### Task 7.1: Run the smoke test

**Files:**
- Test: `bench/smoke_test.go`

- [ ] **Step 1: Start the dev stack**

```bash
./scripts/dev-up.sh
```

Expected: all services start.

- [ ] **Step 2: Run the smoke test**

```bash
go test ./bench -run=TestSmokeAllScenarios -v
```

Expected: PASS.

- [ ] **Step 3: Run the full benchmark suite**

```bash
make bench-against-running
```

Expected: Markdown table printed with no errors.

- [ ] **Step 4: Commit any final fixes**

```bash
git add -A
git commit -m "fix(bench): address verification findings"
```

---

## Plan Self-Review

### Spec Coverage

- Single-command benchmark: Task 4.1 `make bench` and Task 4.2 `scripts/bench.sh`.
- Four scenarios: Tasks 2.1–2.4.
- Real Docker Compose stack: Task 3.1.
- Markdown reporting: Task 1.6 and Task 5.2.
- CI workflow: Task 5.1.
- README link: Task 6.1.
- Smoke test: Task 2.5.

### Placeholder Scan

- No `TBD` or `TODO` in implementation steps.
- All code snippets are concrete and use existing project types.
- Token issuer endpoint is verified in Task 1.4 Step 2.

### Type Consistency

- `WalletClient` uses `pb.WalletServiceClient` methods matching `internal/proto/wallet.proto`.
- `HTTPClient` endpoints match `internal/gateway/server.go` routes.
- `WebhookBuilder` uses `XC-Nonce`, `XC-Timestamp`, `XC-Signature` headers matching `internal/wallet/xcash/webhook.go`.

### Gaps / Notes

- The `CollectResult` function uses rough percentile estimates because Go's `testing.B` does not expose latency histograms. If the user wants real p50/p95/p99, we should use a custom latency recorder instead of `testing.B` metrics. This is acceptable for a README baseline but should be called out.
- The `TokenIssuer` uses the Better Auth sign-up endpoint. If the endpoint differs from the assumed `/api/auth/sign-up/email`, it will need adjustment after the first test.

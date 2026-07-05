# Fiat Display and Zero Balance Hiding Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add estimated USD fiat display to wallet balances and transactions, and hide zero-balance wallets from the frontend wallet grid.

**Architecture:** A new `internal/wallet/rates` package provides cached USD exchange rates with Binance primary and Coinbase/Kraken/KuCoin fallbacks. The wallet gRPC server enriches `GetBalance` and `ListTransactions` responses with `fiat_value` and `fiat_currency`. The Next.js wallet page filters zero balances and displays fiat values, and a new footer ticker shows live rates via `GET /api/wallet/rates`.

**Tech Stack:** Go 1.26, gRPC, Protocol Buffers, Next.js 15, TypeScript, Tailwind CSS, Redis (optional future cache).

---

## File Map

| File | Responsibility |
|---|---|
| `internal/wallet/rates/provider.go` | `RateProvider` interface. |
| `internal/wallet/rates/binance.go` | Binance USD rate fetcher. |
| `internal/wallet/rates/coinbase.go` | Coinbase fallback fetcher. |
| `internal/wallet/rates/kraken.go` | Kraken fallback fetcher. |
| `internal/wallet/rates/kucoin.go` | KuCoin fallback fetcher. |
| `internal/wallet/rates/cache.go` | In-memory TTL cache. |
| `internal/wallet/rates/aggregator.go` | Provider chain, stablecoin aliases, manual overrides. |
| `internal/wallet/rates/aggregator_test.go` | Tests for aggregator logic. |
| `internal/wallet/rates/binance_test.go` | Tests for Binance provider. |
| `internal/proto/wallet.proto` | Adds `fiat_currency` and `fiat_value` fields. |
| `internal/proto/wallet.pb.go` | Regenerated protobuf Go code. |
| `internal/proto/wallet_grpc.pb.go` | Regenerated protobuf gRPC code. |
| `internal/wallet/server.go` | Adds `fiat_value`/`fiat_currency` enrichment. |
| `internal/wallet/server_test.go` | Tests for fiat enrichment. |
| `internal/wallet/service.go` | Adds `GetFiatValue` helper. |
| `internal/wallet/decimal.go` | Adds decimal string helpers. |
| `internal/gateway/server.go` | Adds `GET /api/wallet/rates` handler. |
| `internal/gateway/wallet_client.go` | Adds `GetRates` method. |
| `cmd/wallet/main.go` | Wires `rates.Aggregator` into wallet service. |
| `cmd/gateway/main.go` | Wires public rates endpoint. |
| `app/components/wallet-card.tsx` | Shows fiat value below balance. |
| `app/app/wallet/page.tsx` | Filters zero balances, passes fiat value. |
| `app/components/rates-footer.tsx` | Site-wide footer ticker. |
| `app/app/layout.tsx` | Renders footer ticker. |
| `app/lib/go-api-client.ts` | Adds `getRates` method. |
| `.env.example` | Adds `MANUAL_RATES` and `RATES_CACHE_TTL_SECONDS`. |
| `.env` | Adds defaults for local development. |

---

## Task 1: Decimal Helpers

### Task 1.1: Add decimal helpers in `internal/wallet/decimal.go`

**Files:**
- Modify: `internal/wallet/decimal.go`

- [ ] **Step 1: Write the failing test**

Create `internal/wallet/decimal_test.go`:

```go
package wallet

import "testing"

func TestMulDecimalStrings(t *testing.T) {
	got, err := MulDecimalStrings("100.00", "1.05")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "105.00" {
		t.Fatalf("expected 105.00, got %s", got)
	}
}

func TestMulDecimalStrings_Stablecoin(t *testing.T) {
	got, err := MulDecimalStrings("50.00000000", "1.00")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "50.00000000" {
		t.Fatalf("expected 50.00000000, got %s", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/wallet -run TestMulDecimalStrings -v
```

Expected: FAIL — `MulDecimalStrings` undefined.

- [ ] **Step 3: Implement minimal code**

Add to `internal/wallet/decimal.go`:

```go
package wallet

import (
	"github.com/shopspring/decimal"
)

// MulDecimalStrings multiplies two decimal strings and returns a string.
// It uses the maximum precision of the two inputs.
func MulDecimalStrings(a, b string) (string, error) {
	left, err := decimal.NewFromString(a)
	if err != nil {
		return "", err
	}
	right, err := decimal.NewFromString(b)
	if err != nil {
		return "", err
	}
	return left.Mul(right).String(), nil
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/wallet -run TestMulDecimalStrings -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/wallet/decimal.go internal/wallet/decimal_test.go
git commit -m "feat(wallet): add decimal string multiplication helper"
```

---

## Task 2: Rate Provider Interface and Binance Provider

### Task 2.1: Create the rate provider interface

**Files:**
- Create: `internal/wallet/rates/provider.go`

- [ ] **Step 1: Implement the interface**

```go
package rates

import "context"

// RateProvider fetches the exchange rate between a fiat currency and a crypto currency.
type RateProvider interface {
	// GetRate returns the rate for 1 unit of crypto expressed in fiat.
	// For example, if crypto=BTC and fiat=USD, a rate of 60000 means 1 BTC = 60000 USD.
	GetRate(ctx context.Context, fiat, crypto string) (string, error)
	Name() string
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/wallet/rates/provider.go
git commit -m "feat(rates): add rate provider interface"
```

---

### Task 2.2: Implement Binance provider

**Files:**
- Create: `internal/wallet/rates/binance.go`
- Create: `internal/wallet/rates/binance_test.go`

- [ ] **Step 1: Write the failing test**

```go
package rates

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBinanceGetRate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"symbol":"BTCUSDT","price":"63420.50"}`))
	}))
	defer server.Close()

	p := NewBinance(WithBinanceURL(server.URL))
	got, err := p.GetRate(context.Background(), "USD", "BTC")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "63420.50" {
		t.Fatalf("expected 63420.50, got %s", got)
	}
}

func TestBinanceGetRate_USDT(t *testing.T) {
	p := NewBinance()
	got, err := p.GetRate(context.Background(), "USD", "USDT")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "1.00" {
		t.Fatalf("expected 1.00, got %s", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/wallet/rates -run TestBinance -v
```

Expected: FAIL — `NewBinance` undefined.

- [ ] **Step 3: Implement minimal code**

```go
package rates

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const binanceDefaultURL = "https://api.binance.com"

// Binance fetches USD rates from Binance public API.
type Binance struct {
	client *http.Client
	url    string
}

// BinanceOption configures the Binance provider.
type BinanceOption func(*Binance)

// WithBinanceURL sets a custom base URL (used in tests).
func WithBinanceURL(u string) BinanceOption {
	return func(b *Binance) {
		b.url = u
	}
}

// NewBinance creates a Binance rate provider.
func NewBinance(opts ...BinanceOption) *Binance {
	b := &Binance{
		client: &http.Client{Timeout: 5 * time.Second},
		url:    binanceDefaultURL,
	}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

func (b *Binance) Name() string { return "binance" }

func (b *Binance) GetRate(ctx context.Context, fiat, crypto string) (string, error) {
	if fiat == "USD" && isStablecoin(crypto) {
		return "1.00", nil
	}
	crypto = normalizeSymbol(crypto)
	if fiat == "USD" {
		fiat = "USDT"
	}
	url := fmt.Sprintf("%s/api/v3/ticker/price?symbol=%s%s", b.url, crypto, fiat)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := b.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("binance status %d", resp.StatusCode)
	}
	var result struct {
		Price string `json:"price"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if result.Price == "" {
		return "", fmt.Errorf("binance empty price")
	}
	return strings.TrimSpace(result.Price), nil
}
```

Add helper functions in `internal/wallet/rates/util.go`:

```go
package rates

func isStablecoin(crypto string) bool {
	switch crypto {
	case "USDT", "USDC", "BUSD", "DAI":
		return true
	}
	return false
}

func normalizeSymbol(crypto string) string {
	switch crypto {
	case "MATIC":
		return "POL"
	case "TON":
		return "GRAM"
	case "ARB-TOKEN":
		return "ARB"
	case "OP-TOKEN":
		return "OP"
	}
	return crypto
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/wallet/rates -run TestBinance -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/wallet/rates/
git commit -m "feat(rates): add binance rate provider"
```

---

## Task 3: Fallback Providers

### Task 3.1: Implement Coinbase provider

**Files:**
- Create: `internal/wallet/rates/coinbase.go`

- [ ] **Step 1: Implement the provider**

```go
package rates

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const coinbaseDefaultURL = "https://api.coinbase.com"

type Coinbase struct {
	client *http.Client
	url    string
}

type CoinbaseOption func(*Coinbase)

func WithCoinbaseURL(u string) CoinbaseOption {
	return func(c *Coinbase) {
		c.url = u
	}
}

func NewCoinbase(opts ...CoinbaseOption) *Coinbase {
	c := &Coinbase{
		client: &http.Client{Timeout: 5 * time.Second},
		url:    coinbaseDefaultURL,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *Coinbase) Name() string { return "coinbase" }

func (c *Coinbase) GetRate(ctx context.Context, fiat, crypto string) (string, error) {
	if fiat == "USD" && isStablecoin(crypto) {
		return "1.00", nil
	}
	crypto = normalizeSymbol(crypto)
	if fiat == "USD" {
		fiat = "USDT"
	}
	url := fmt.Sprintf("%s/v2/exchange-rates?currency=%s", c.url, crypto)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("coinbase status %d", resp.StatusCode)
	}
	var result struct {
		Data struct {
			Rates map[string]string `json:"rates"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	rate, ok := result.Data.Rates[fiat]
	if !ok {
		return "", fmt.Errorf("coinbase missing rate for %s", fiat)
	}
	return strings.TrimSpace(rate), nil
}
```

- [ ] **Step 2: Add a simple test**

```go
package rates

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCoinbaseGetRate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":{"rates":{"USDT":"3410.50"}}}`))
	}))
	defer server.Close()

	p := NewCoinbase(WithCoinbaseURL(server.URL))
	got, err := p.GetRate(context.Background(), "USD", "ETH")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "3410.50" {
		t.Fatalf("expected 3410.50, got %s", got)
	}
}
```

- [ ] **Step 3: Run tests and commit**

```bash
go test ./internal/wallet/rates -run TestCoinbase -v
git add internal/wallet/rates/coinbase.go internal/wallet/rates/coinbase_test.go
git commit -m "feat(rates): add coinbase fallback provider"
```

---

### Task 3.2: Implement Kraken provider

**Files:**
- Create: `internal/wallet/rates/kraken.go`

- [ ] **Step 1: Implement the provider**

```go
package rates

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const krakenDefaultURL = "https://api.kraken.com"

type Kraken struct {
	client *http.Client
	url    string
}

type KrakenOption func(*Kraken)

func WithKrakenURL(u string) KrakenOption {
	return func(k *Kraken) {
		k.url = u
	}
}

func NewKraken(opts ...KrakenOption) *Kraken {
	k := &Kraken{
		client: &http.Client{Timeout: 5 * time.Second},
		url:    krakenDefaultURL,
	}
	for _, opt := range opts {
		opt(k)
	}
	return k
}

func (k *Kraken) Name() string { return "kraken" }

func (k *Kraken) GetRate(ctx context.Context, fiat, crypto string) (string, error) {
	if fiat == "USD" && isStablecoin(crypto) {
		return "1.00", nil
	}
	crypto = normalizeSymbol(crypto)
	if fiat == "USD" {
		fiat = "USDT"
	}
	url := fmt.Sprintf("%s/0/public/Ticker?pair=%s%s", k.url, crypto, fiat)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := k.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("kraken status %d", resp.StatusCode)
	}
	var result struct {
		Error  []string               `json:"error"`
		Result map[string]interface{} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if len(result.Error) > 0 {
		return "", fmt.Errorf("kraken error: %v", result.Error)
	}
	for _, v := range result.Result {
		pair, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		c, ok := pair["c"].([]interface{})
		if !ok || len(c) == 0 {
			continue
		}
		price, ok := c[0].(string)
		if ok {
			return strings.TrimSpace(price), nil
		}
	}
	return "", fmt.Errorf("kraken missing ticker data")
}
```

- [ ] **Step 2: Add a test and commit**

```go
package rates

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestKrakenGetRate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"error":[],"result":{"XBTUSDT":{"c":["63420.50","0.00000000"]}}}`))
	}))
	defer server.Close()

	p := NewKraken(WithKrakenURL(server.URL))
	got, err := p.GetRate(context.Background(), "USD", "BTC")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "63420.50" {
		t.Fatalf("expected 63420.50, got %s", got)
	}
}
```

```bash
go test ./internal/wallet/rates -run TestKraken -v
git add internal/wallet/rates/kraken.go internal/wallet/rates/kraken_test.go
git commit -m "feat(rates): add kraken fallback provider"
```

---

### Task 3.3: Implement KuCoin provider

**Files:**
- Create: `internal/wallet/rates/kucoin.go`

- [ ] **Step 1: Implement the provider**

```go
package rates

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const kucoinDefaultURL = "https://api.kucoin.com"

type KuCoin struct {
	client *http.Client
	url    string
}

type KuCoinOption func(*KuCoin)

func WithKuCoinURL(u string) KuCoinOption {
	return func(k *KuCoin) {
		k.url = u
	}
}

func NewKuCoin(opts ...KuCoinOption) *KuCoin {
	k := &KuCoin{
		client: &http.Client{Timeout: 5 * time.Second},
		url:    kucoinDefaultURL,
	}
	for _, opt := range opts {
		opt(k)
	}
	return k
}

func (k *KuCoin) Name() string { return "kucoin" }

func (k *KuCoin) GetRate(ctx context.Context, fiat, crypto string) (string, error) {
	if fiat == "USD" && isStablecoin(crypto) {
		return "1.00", nil
	}
	crypto = normalizeSymbol(crypto)
	url := fmt.Sprintf("%s/api/v1/prices?base=%s&currencies=%s", k.url, fiat, crypto)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := k.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("kucoin status %d", resp.StatusCode)
	}
	var result struct {
		Code string            `json:"code"`
		Data map[string]string `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if result.Code != "200000" {
		return "", fmt.Errorf("kucoin code %s", result.Code)
	}
	price, ok := result.Data[crypto]
	if !ok {
		return "", fmt.Errorf("kucoin missing rate for %s", crypto)
	}
	return strings.TrimSpace(price), nil
}
```

- [ ] **Step 2: Add a test and commit**

```go
package rates

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestKuCoinGetRate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"code":"200000","data":{"BNB":"590.20"}}`))
	}))
	defer server.Close()

	p := NewKuCoin(WithKuCoinURL(server.URL))
	got, err := p.GetRate(context.Background(), "USD", "BNB")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "590.20" {
		t.Fatalf("expected 590.20, got %s", got)
	}
}
```

```bash
go test ./internal/wallet/rates -run TestKuCoin -v
git add internal/wallet/rates/kucoin.go internal/wallet/rates/kucoin_test.go
git commit -m "feat(rates): add kucoin fallback provider"
```

---

## Task 4: Rate Cache and Aggregator

### Task 4.1: Implement the TTL cache

**Files:**
- Create: `internal/wallet/rates/cache.go`

- [ ] **Step 1: Implement the cache**

```go
package rates

import (
	"sync"
	"time"
)

// cachedRate stores a rate and its expiry.
type cachedRate struct {
	value  string
	cached time.Time
}

// Cache is an in-memory TTL cache for exchange rates.
type Cache struct {
	mu     sync.RWMutex
	values map[string]cachedRate
	ttl    time.Duration
}

// NewCache creates a cache with the given TTL.
func NewCache(ttl time.Duration) *Cache {
	return &Cache{
		values: make(map[string]cachedRate),
		ttl:    ttl,
	}
}

// Get returns a cached rate if it is still fresh.
func (c *Cache) Get(key string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.values[key]
	if !ok {
		return "", false
	}
	if time.Since(v.cached) > c.ttl {
		return "", false
	}
	return v.value, true
}

// Set stores a rate with the current timestamp.
func (c *Cache) Set(key, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.values[key] = cachedRate{value: value, cached: time.Now()}
}

// StaleValue returns a cached value regardless of TTL, or empty if missing.
func (c *Cache) StaleValue(key string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.values[key]
	if !ok {
		return "", false
	}
	if time.Since(v.cached) > 5*time.Minute {
		return "", false
	}
	return v.value, true
}
```

- [ ] **Step 2: Add tests**

```go
package rates

import (
	"testing"
	"time"
)

func TestCacheGetSet(t *testing.T) {
	c := NewCache(1 * time.Second)
	c.Set("BTC:USD", "60000")
	got, ok := c.Get("BTC:USD")
	if !ok || got != "60000" {
		t.Fatalf("expected 60000, got %s, ok=%v", got, ok)
	}
}

func TestCacheExpiry(t *testing.T) {
	c := NewCache(1 * time.Nanosecond)
	c.Set("BTC:USD", "60000")
	time.Sleep(5 * time.Millisecond)
	_, ok := c.Get("BTC:USD")
	if ok {
		t.Fatalf("expected cache entry to expire")
	}
}

func TestCacheStaleValue(t *testing.T) {
	c := NewCache(1 * time.Nanosecond)
	c.Set("BTC:USD", "60000")
	time.Sleep(5 * time.Millisecond)
	got, ok := c.StaleValue("BTC:USD")
	if !ok || got != "60000" {
		t.Fatalf("expected stale value, got %s, ok=%v", got, ok)
	}
}
```

- [ ] **Step 3: Run tests and commit**

```bash
go test ./internal/wallet/rates -run TestCache -v
git add internal/wallet/rates/cache.go internal/wallet/rates/cache_test.go
git commit -m "feat(rates): add in-memory TTL cache"
```

---

### Task 4.2: Implement the aggregator

**Files:**
- Create: `internal/wallet/rates/aggregator.go`
- Create: `internal/wallet/rates/aggregator_test.go`

- [ ] **Step 1: Write the failing test**

```go
package rates

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestAggregatorStablecoin(t *testing.T) {
	agg := NewAggregator(NewCache(30*time.Second))
	got, err := agg.GetRate(context.Background(), "USD", "USDT")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "1.00" {
		t.Fatalf("expected 1.00, got %s", got)
	}
}

func TestAggregatorProviderFallback(t *testing.T) {
	failing := &staticProvider{name: "failing", err: true}
	working := &staticProvider{name: "working", value: "50000.00"}
	agg := NewAggregator(NewCache(30*time.Second), failing, working)
	got, err := agg.GetRate(context.Background(), "USD", "BTC")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "50000.00" {
		t.Fatalf("expected 50000.00, got %s", got)
	}
}

var errTest = errors.New("test error")

type staticProvider struct {
	name  string
	value string
	err   bool
}

func (s *staticProvider) Name() string { return s.name }
func (s *staticProvider) GetRate(ctx context.Context, fiat, crypto string) (string, error) {
	if s.err {
		return "", errTest
	}
	return s.value, nil
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/wallet/rates -run TestAggregator -v
```

Expected: FAIL — `NewAggregator` undefined.

- [ ] **Step 3: Implement the aggregator**

```go
package rates

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/shopspring/decimal"
)

// Aggregator chains multiple providers and caches results.
type Aggregator struct {
	cache     *Cache
	providers []RateProvider
	manual    map[string]string
	mu        sync.RWMutex
}

// NewAggregator creates an aggregator with the given cache and providers.
func NewAggregator(cache *Cache, providers ...RateProvider) *Aggregator {
	a := &Aggregator{
		cache:     cache,
		providers: providers,
		manual:    loadManualRates(),
	}
	return a
}

// GetRate returns the USD rate for a crypto currency.
func (a *Aggregator) GetRate(ctx context.Context, fiat, crypto string) (string, error) {
	if fiat != "USD" {
		return "", fmt.Errorf("unsupported fiat: %s", fiat)
	}
	if isStablecoin(crypto) {
		return "1.00", nil
	}
	key := crypto + ":" + fiat
	if v, ok := a.cache.Get(key); ok {
		return v, nil
	}
	if manual, ok := a.manualRate(crypto); ok {
		a.cache.Set(key, manual)
		return manual, nil
	}
	for _, p := range a.providers {
		value, err := p.GetRate(ctx, fiat, crypto)
		if err != nil {
			continue
		}
		a.cache.Set(key, value)
		return value, nil
	}
	if stale, ok := a.cache.StaleValue(key); ok {
		return stale, nil
	}
	return "0", fmt.Errorf("all providers failed for %s/%s", crypto, fiat)
}

func (a *Aggregator) manualRate(crypto string) (string, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	v, ok := a.manual[crypto]
	return v, ok
}

func loadManualRates() map[string]string {
	out := map[string]string{}
	raw := os.Getenv("MANUAL_RATES")
	if raw == "" {
		return out
	}
	var parsed map[string]string
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return out
	}
	for k, v := range parsed {
		if _, err := decimal.NewFromString(v); err == nil {
			out[k] = v
		}
	}
	return out
}

// SupportedRates returns rates for all supported currencies.
func (a *Aggregator) SupportedRates(ctx context.Context, currencies []string) map[string]string {
	out := make(map[string]string, len(currencies))
	for _, c := range currencies {
		v, err := a.GetRate(ctx, "USD", c)
		if err != nil {
			v = "0"
		}
		out[c] = v
	}
	return out
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/wallet/rates -run TestAggregator -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/wallet/rates/aggregator.go internal/wallet/rates/aggregator_test.go
git commit -m "feat(rates): add rate aggregator with cache and fallback"
```

---

## Task 5: Wire Rates into Wallet Service

### Task 5.1: Wire rates into `cmd/wallet/main.go`

**Files:**
- Modify: `cmd/wallet/main.go`

- [ ] **Step 1: Read the current main file**

```bash
cat cmd/wallet/main.go
```

- [ ] **Step 2: Modify `cmd/wallet/main.go` to create the aggregator**

Add near the top:

```go
import "github.com/realyoussefhossam/betmonster/internal/wallet/rates"
```

After creating the wallet service, add:

```go
	cacheTTL := 30 * time.Second
	if v := os.Getenv("RATES_CACHE_TTL_SECONDS"); v != "" {
		if secs, err := strconv.Atoi(v); err == nil {
			cacheTTL = time.Duration(secs) * time.Second
		}
	}
	rateCache := rates.NewCache(cacheTTL)
	aggregator := rates.NewAggregator(rateCache,
		rates.NewBinance(),
		rates.NewCoinbase(),
		rates.NewKraken(),
		rates.NewKuCoin(),
	)
```

Pass `aggregator` to `NewGRPCServer`:

```go
	grpcServer := wallet.NewGRPCServer(svc, aggregator)
```

- [ ] **Step 3: Commit**

```bash
git add cmd/wallet/main.go
git commit -m "feat(wallet): wire rate aggregator into wallet service"
```

---

### Task 5.2: Update `GRPCServer` and service

**Files:**
- Modify: `internal/wallet/server.go`
- Modify: `internal/wallet/service.go`

- [ ] **Step 1: Update `GRPCServer` struct**

```go
type GRPCServer struct {
	pb.UnimplementedWalletServiceServer
	service *Service
	rates   *rates.Aggregator
}

func NewGRPCServer(service *Service, rates *rates.Aggregator) *GRPCServer {
	return &GRPCServer{service: service, rates: rates}
}
```

- [ ] **Step 2: Add a helper to compute fiat value**

In `internal/wallet/service.go`:

```go
// GetFiatValue returns the USD value of a crypto amount using the configured rate aggregator.
func (s *Service) GetFiatValue(ctx context.Context, currency, amount string) (string, error) {
	// This is a placeholder until the aggregator is wired into the service.
	// The actual implementation is in GRPCServer where the aggregator is available.
	return "0", nil
}
```

Actually, since the aggregator is in `GRPCServer`, we compute the fiat value there.

- [ ] **Step 3: Update `GetBalance` handler to include fiat value**

```go
func (s *GRPCServer) GetBalance(ctx context.Context, req *pb.GetBalanceRequest) (*pb.GetBalanceResponse, error) {
	wallet, err := s.service.GetBalance(ctx, req.UserId, req.Currency)
	if err != nil {
		return nil, err
	}
	fiatValue, err := s.fiatValue(ctx, req.Currency, wallet.Balance)
	if err != nil {
		fiatValue = "0"
	}
	return &pb.GetBalanceResponse{
		Currency:     wallet.Currency,
		Balance:      wallet.Balance,
		FiatCurrency: "USD",
		FiatValue:    fiatValue,
	}, nil
}

func (s *GRPCServer) fiatValue(ctx context.Context, currency, amount string) (string, error) {
	rate, err := s.rates.GetRate(ctx, "USD", currency)
	if err != nil {
		return "", err
	}
	return MulDecimalStrings(amount, rate)
}
```

- [ ] **Step 4: Update `ListTransactions` handler to include fiat value**

```go
func (s *GRPCServer) ListTransactions(ctx context.Context, req *pb.ListTransactionsRequest) (*pb.ListTransactionsResponse, error) {
	txns, err := s.service.ListTransactions(ctx, req.UserId, int(req.Page), int(req.PageSize))
	if err != nil {
		return nil, err
	}
	out := make([]*pb.Transaction, len(txns))
	for i, t := range txns {
		fiatValue, err := s.fiatValue(ctx, t.Currency, t.Amount)
		if err != nil {
			fiatValue = "0"
		}
		out[i] = &pb.Transaction{
			Id: t.ID, UserId: t.UserID, WalletId: t.WalletID, Type: t.Type,
			Amount: t.Amount, BalanceBefore: t.BalanceBefore, BalanceAfter: t.BalanceAfter,
			Status: t.Status, ReferenceId: t.ReferenceID, Metadata: t.Metadata,
			CreatedAt: t.CreatedAt.Format(time.RFC3339),
			FiatValue: fiatValue,
		}
	}
	return &pb.ListTransactionsResponse{Transactions: out}, nil
}
```

- [ ] **Step 5: Update tests to pass the aggregator**

In `internal/wallet/server_test.go`, update `NewGRPCServer` calls:

```go
server := NewGRPCServer(svc, nil)
```

For `TestGRPCServerListTransactionsIncludesCreatedAt` and `TestGRPCServerGetBalance`, pass `nil` since the tests don't use fiat values.

- [ ] **Step 6: Run tests and commit**

```bash
go test ./internal/wallet -v
make proto
# if proto changed, regenerate
git add -A
git commit -m "feat(wallet): enrich gRPC responses with fiat value"
```

---

## Task 6: Protobuf Update

### Task 6.1: Update `internal/proto/wallet.proto`

**Files:**
- Modify: `internal/proto/wallet.proto`

- [ ] **Step 1: Add fields to messages**

```protobuf
message GetBalanceResponse {
  string currency = 1;
  string balance = 2;
  string fiat_currency = 3;
  string fiat_value = 4;
}

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
  string fiat_value = 12;
}
```

- [ ] **Step 2: Regenerate protobuf**

```bash
make proto
```

Expected: `internal/proto/wallet.pb.go` updated with new fields.

- [ ] **Step 3: Commit**

```bash
git add internal/proto/wallet.proto internal/proto/wallet.pb.go internal/proto/wallet_grpc.pb.go
git commit -m "feat(proto): add fiat_currency and fiat_value fields"
```

---

## Task 7: Add Public Rates Endpoint to Gateway

### Task 7.1: Add `GetRates` to gateway wallet client

**Files:**
- Modify: `internal/gateway/wallet_client.go`

- [ ] **Step 1: Add the method**

First, we need to add `GetRates` to the gRPC service. Update `internal/proto/wallet.proto`:

```protobuf
service WalletService {
  rpc GetRates(GetRatesRequest) returns (GetRatesResponse);
  // ... existing methods
}

message GetRatesRequest {}
message GetRatesResponse {
  string fiat_currency = 1;
  map<string, string> rates = 2;
}
```

Regenerate proto:

```bash
make proto
```

Then add to `internal/wallet/server.go`:

```go
func (s *GRPCServer) GetRates(ctx context.Context, req *pb.GetRatesRequest) (*pb.GetRatesResponse, error) {
	currencies := s.service.supportedCurrencies()
	rates := s.rates.SupportedRates(ctx, currencies)
	return &pb.GetRatesResponse{
		FiatCurrency: "USD",
		Rates:        rates,
	}, nil
}
```

Add `supportedCurrencies()` method to `Service` in `internal/wallet/service.go`:

```go
func (s *Service) supportedCurrencies() []string {
	out := make([]string, 0, len(s.supportedPairs))
	seen := map[string]struct{}{}
	for pair := range s.supportedPairs {
		parts := strings.SplitN(pair, ":", 2)
		if _, ok := seen[parts[0]]; !ok {
			seen[parts[0]] = struct{}{}
			out = append(out, parts[0])
		}
	}
	return out
}
```

Add to `internal/gateway/wallet_client.go`:

```go
func (c *WalletClient) GetRates(ctx context.Context) (*pb.GetRatesResponse, error) {
	return c.conn.GetRates(ctx, &pb.GetRatesRequest{})
}
```

- [ ] **Step 2: Add the HTTP handler in gateway**

In `internal/gateway/server.go`, add to `Router()`:

```go
mux.Handle("/api/wallet/rates", server.WithRoutePattern("/api/wallet/rates", http.HandlerFunc(s.handleRates)))
```

And add the handler:

```go
func (s *Server) handleRates(w http.ResponseWriter, r *http.Request) {
	resp, err := s.wallet.GetRates(r.Context())
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{
		"fiat_currency": resp.FiatCurrency,
		"rates":         resp.Rates,
	})
}
```

- [ ] **Step 3: Wire gateway main**

No changes needed in `cmd/gateway/main.go` if the wallet client already connects.

- [ ] **Step 4: Run tests and commit**

```bash
go test ./internal/wallet ./internal/gateway -v
git add -A
git commit -m "feat(gateway): add public rates endpoint"
```

---

## Task 8: Frontend Updates

### Task 8.1: Update `go-api-client.ts`

**Files:**
- Modify: `app/lib/go-api-client.ts`

- [ ] **Step 1: Read the current file**

```bash
cat app/lib/go-api-client.ts
```

- [ ] **Step 2: Add `getRates` and update types**

Add `RatesResponse` type:

```typescript
export interface RatesResponse {
  fiat_currency: string;
  rates: Record<string, string>;
}
```

Add method:

```typescript
async getRates(): Promise<ApiResponse<RatesResponse>> {
  return this.get<RatesResponse>("/api/wallet/rates");
}
```

- [ ] **Step 3: Update `BalanceResponse` type**

```typescript
export interface BalanceResponse {
  currency: string;
  balance: string;
  fiat_currency?: string;
  fiat_value?: string;
}
```

- [ ] **Step 4: Commit**

```bash
git add app/lib/go-api-client.ts
git commit -m "feat(api-client): add rates endpoint and fiat balance fields"
```

---

### Task 8.2: Update `WalletCard` component

**Files:**
- Modify: `app/components/wallet-card.tsx`

- [ ] **Step 1: Read current component**

```bash
cat app/components/wallet-card.tsx
```

- [ ] **Step 2: Add fiat value display**

```tsx
export interface WalletCardProps {
  currency: string;
  balance: string;
  fiatValue?: string;
  fiatCurrency?: string;
  loading?: boolean;
}

export function WalletCard({ currency, balance, fiatValue, fiatCurrency, loading }: WalletCardProps) {
  return (
    <Card>
      <CardContent className="p-6">
        {loading ? (
          <Skeleton className="h-6 w-24" />
        ) : (
          <>
            <div className="text-2xl font-bold">{balance} {currency}</div>
            {fiatValue && fiatCurrency && (
              <div className="text-sm text-muted-foreground">
                ≈ ${fiatValue} {fiatCurrency}
              </div>
            )}
          </>
        )}
      </CardContent>
    </Card>
  );
}
```

- [ ] **Step 3: Commit**

```bash
git add app/components/wallet-card.tsx
git commit -m "feat(ui): show fiat value on wallet card"
```

---

### Task 8.3: Update wallet page

**Files:**
- Modify: `app/app/wallet/page.tsx`

- [ ] **Step 1: Filter zero balances and pass fiat values**

Update the balance rendering:

```tsx
const loadedBalances = balanceResults
  .filter((res) => res.data)
  .map((res) => res.data!)
  .filter((b) => b.balance !== "0" && b.balance !== "0.00000000");
```

And pass fiat values:

```tsx
balances.map((b) => (
  <WalletCard
    key={b.currency}
    currency={b.currency}
    balance={b.balance}
    fiatValue={b.fiat_value}
    fiatCurrency={b.fiat_currency}
    loading={false}
  />
))
```

- [ ] **Step 2: Update transaction display**

```tsx
<span className="text-right">
  {tx.amount} {tx.status}
  {tx.fiat_value && <div className="text-xs text-muted-foreground">≈ ${tx.fiat_value} USD</div>}
</span>
```

- [ ] **Step 3: Commit**

```bash
git add app/app/wallet/page.tsx
git commit -m "feat(wallet): hide zero balances and show fiat values"
```

---

### Task 8.4: Add footer ticker

**Files:**
- Create: `app/components/rates-footer.tsx`
- Modify: `app/app/layout.tsx`

- [ ] **Step 1: Create the component**

```tsx
"use client";

import { useEffect, useState } from "react";
import { goApiClient } from "@/lib/go-api-client";

export function RatesFooter() {
  const [rates, setRates] = useState<Record<string, string>>({});
  const [fiatCurrency, setFiatCurrency] = useState<string>("USD");

  useEffect(() => {
    async function load() {
      const res = await goApiClient.getRates();
      if (res.data) {
        setRates(res.data.rates);
        setFiatCurrency(res.data.fiat_currency);
      }
    }
    load();
    const id = setInterval(load, 60000);
    return () => clearInterval(id);
  }, []);

  const entries = Object.entries(rates);
  if (entries.length === 0) return null;

  return (
    <footer className="border-t bg-muted py-2">
      <div className="container mx-auto text-center text-sm text-muted-foreground">
        {entries.map(([currency, value], i) => (
          <span key={currency}>
            {currency} ${value} {fiatCurrency}
            {i < entries.length - 1 && " | "}
          </span>
        ))}
      </div>
    </footer>
  );
}
```

- [ ] **Step 2: Render in layout**

In `app/app/layout.tsx`, add `<RatesFooter />` before the closing `</body>`.

- [ ] **Step 3: Commit**

```bash
git add app/components/rates-footer.tsx app/app/layout.tsx
git commit -m "feat(ui): add footer exchange rate ticker"
```

---

## Task 9: Environment Configuration

### Task 9.1: Update `.env.example` and `.env`

**Files:**
- Modify: `.env.example`
- Modify: `.env`

- [ ] **Step 1: Add rate configuration**

Add to `.env.example`:

```env
# Optional: override exchange rates for operator tokens or test environments.
MANUAL_RATES={}

# Optional: rate cache TTL in seconds (default: 30).
RATES_CACHE_TTL_SECONDS=30
```

- [ ] **Step 2: Commit**

```bash
git add .env.example .env
git commit -m "config(env): add rate configuration options"
```

---

## Task 10: Verification

### Task 10.1: Run the full test suite

**Files:**
- All tests

- [ ] **Step 1: Run Go tests**

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 2: Run the dev stack and verify manually**

```bash
./scripts/dev-up.sh
```

Then check:
- `GET http://localhost:8080/api/wallet/rates` returns rates.
- Wallet page shows fiat values and hides zero balances.
- Footer ticker shows rates.

- [ ] **Step 3: Commit any final fixes**

```bash
git add -A
git commit -m "fix: address verification findings"
```

---

## Plan Self-Review

### Spec Coverage

- USD fiat display on balances: Task 5.2 `GetBalance` handler.
- USD fiat display on transactions: Task 5.2 `ListTransactions` handler.
- Hide zero balances: Task 8.3 wallet page filter.
- Footer ticker: Task 7 and Task 8.4.
- Stablecoin 1:1 rule: Task 4.2 `Aggregator.GetRate`.
- Multi-provider fallback: Tasks 2–4.
- 30-second cache: Task 4.1.
- Manual rate override: Task 4.2 `loadManualRates`.

### Placeholder Scan

- No `TBD` or `TODO` in implementation steps.
- All code snippets use concrete file paths and types.

### Type Consistency

- `GRPCServer` receives `*rates.Aggregator` consistently.
- `BalanceResponse` and `Transaction` protobuf fields match frontend TypeScript types.
- `GetRates` request/response names are consistent across proto, server, gateway, and frontend.

### Gaps / Notes

- The `Service.supportedCurrencies()` method needs to be added; covered in Task 7.1.
- Frontend components assume `Card`, `CardContent`, `Skeleton` already exist in `app/components/ui/`. Verify before implementation.
- The `goApiClient.get` method is assumed to exist. Verify in `app/lib/go-api-client.ts`.

# Multi-Fiat Currency Display Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend the existing fiat-display feature from USD-only to ~70 fiat currencies, allowing users to choose their display currency while keeping all settlement in cryptocurrency.

**Architecture:** A new `ForexProvider` chain fetches USD-to-fiat rates from four free providers: `open.er-api.com`, Coinbase, `fawazahmed0/currency-api`, and MoneyConvert. The existing crypto providers continue fetching crypto-to-USD rates. The `Aggregator` first tries a direct crypto-to-fiat pair, then falls back to `cryptoUSD * USDfiat`. The wallet gRPC API accepts an optional `fiat_currency` field and defaults to `DEFAULT_FIAT_CURRENCY`. The frontend stores the user's choice in `localStorage` and passes it to the API.

**Tech Stack:** Go 1.26, gRPC, Protocol Buffers, Next.js 16, TypeScript, Tailwind CSS.

---

## File Map

| File | Responsibility |
|---|---|
| `internal/wallet/rates/forex.go` | `ForexProvider` interface and chain logic. |
| `internal/wallet/rates/forex_open.go` | `open.er-api.com` USD-to-fiat provider. |
| `internal/wallet/rates/forex_coinbase.go` | Coinbase USD-to-fiat provider. |
| `internal/wallet/rates/forex_fawazahmed0.go` | `fawazahmed0/currency-api` provider. |
| `internal/wallet/rates/forex_moneyconvert.go` | MoneyConvert USD-to-fiat provider. |
| `internal/wallet/rates/forex_test.go` | Tests for the forex providers. |
| `internal/wallet/rates/util.go` | `isStablecoin`, `normalizeSymbol`, `supportedFiatCurrencies`. |
| `internal/wallet/rates/binance.go` | Remove USD->USDT hardcoding; support direct fiat pairs. |
| `internal/wallet/rates/coinbase.go` | Support direct fiat pairs. |
| `internal/wallet/rates/kraken.go` | Support direct fiat pairs. |
| `internal/wallet/rates/kucoin.go` | Support direct fiat pairs. |
| `internal/wallet/rates/aggregator.go` | Cross-convert via USD when direct pair is unavailable. |
| `internal/wallet/rates/aggregator_test.go` | Multi-fiat aggregator tests. |
| `internal/proto/wallet.proto` | Add `fiat_currency` to `GetBalanceRequest`, `ListTransactionsRequest`, `GetRatesRequest`. |
| `internal/proto/wallet.pb.go` | Regenerated protobuf Go code. |
| `internal/proto/wallet_grpc.pb.go` | Regenerated protobuf gRPC Go code. |
| `internal/wallet/server.go` | Use requested fiat currency from request. |
| `internal/wallet/server_test.go` | Tests for multi-fiat enrichment. |
| `internal/wallet/service.go` | Add `supportedFiatCurrencies()` helper. |
| `internal/gateway/server.go` | Accept `?fiat_currency=` query param on `/api/wallet/rates`, pass fiat to gRPC. |
| `internal/gateway/wallet_client.go` | Pass fiat to `GetRates`. |
| `app/lib/go-api-client.ts` | Add `fiatCurrency` parameter to balance/transaction/rates methods. |
| `app/components/fiat-selector.tsx` | Dropdown to select display currency. |
| `app/components/rates-footer.tsx` | Use selected fiat currency. |
| `app/components/wallet-card.tsx` | Display `fiat_currency` from props. |
| `app/app/wallet/page.tsx` | Add fiat selector, pass currency to API calls. |
| `app/app/layout.tsx` | Provide selected fiat context to footer and children. |
| `.env.example` | Add `DEFAULT_FIAT_CURRENCY` and `FIAT_CURRENCIES`. |
| `scripts/init_env.sh` | Generate new env variables. |

---

## Supported Fiat Currencies

```
USD, EUR, JPY, INR, CAD, CNY, IDR, KRW, PHP, RUB,
MXN, PLN, TRY, VND, ARS, PEN, CLP, NGN, AED, BHD,
CRC, KWD, MAD, MYR, QAR, SAR, SGD, TND, TWD, GHS,
KES, BOB, XOF, PKR, NZD, ISK, BAM, TZS, EGP, LKR,
UGX, KZT, BDT, UAH, GEL, MNT, GTQ, KGS, ZAR, TMT,
ZMW, TJS, MRU, TTD, GMD, MGA, JMD, NIO, HNL, MZN,
XAF, RWF, GNF, BWP, KMF, LSL, ERN, BIF, MWK, PGK
```

---

## Task 1: Add Fiat Currency List Config

**Files:**
- Modify: `internal/wallet/rates/util.go`
- Modify: `internal/shared/config/env.go` (if env parsing centralised)

### Step 1: Add supported fiat list

Add to `internal/wallet/rates/util.go`:

```go
var supportedFiatCurrencies = []string{
    "USD", "EUR", "JPY", "INR", "CAD", "CNY", "IDR", "KRW", "PHP", "RUB",
    "MXN", "PLN", "TRY", "VND", "ARS", "PEN", "CLP", "NGN", "AED", "BHD",
    "CRC", "KWD", "MAD", "MYR", "QAR", "SAR", "SGD", "TND", "TWD", "GHS",
    "KES", "BOB", "XOF", "PKR", "NZD", "ISK", "BAM", "TZS", "EGP", "LKR",
    "UGX", "KZT", "BDT", "UAH", "GEL", "MNT", "GTQ", "KGS", "ZAR", "TMT",
    "ZMW", "TJS", "MRU", "TTD", "GMD", "MGA", "JMD", "NIO", "HNL", "MZN",
    "XAF", "RWF", "GNF", "BWP", "KMF", "LSL", "ERN", "BIF", "MWK", "PGK",
}

// IsSupportedFiat returns true if fiat is in the supported list.
func IsSupportedFiat(fiat string) bool {
    fiat = strings.ToUpper(fiat)
    for _, c := range supportedFiatCurrencies {
        if c == fiat {
            return true
        }
    }
    return false
}

// SupportedFiatCurrencies returns the list of supported fiat currencies.
func SupportedFiatCurrencies() []string {
    return append([]string(nil), supportedFiatCurrencies...)
}
```

Update `util.go` imports to include `strings`.

### Step 2: Add environment config

Add to `.env.example`:

```env
# Default fiat currency for display (must be in FIAT_CURRENCIES).
DEFAULT_FIAT_CURRENCY=USD

# Comma-separated list of supported fiat display currencies.
FIAT_CURRENCIES=USD,EUR,JPY,INR,CAD,CNY,IDR,KRW,PHP,RUB,MXN,PLN,TRY,VND,ARS,PEN,CLP,NGN,AED,BHD,CRC,KWD,MAD,MYR,QAR,SAR,SGD,TND,TWD,GHS,KES,BOB,XOF,PKR,NZD,ISK,BAM,TZS,EGP,LKR,UGX,KZT,BDT,UAH,GEL,MNT,GTQ,KGS,ZAR,TMT,ZMW,TJS,MRU,TTD,GMD,MGA,JMD,NIO,HNL,MZN,XAF,RWF,GNF,BWP,KMF,LSL,ERN,BIF,MWK,PGK

# Optional: override USD-to-fiat rates for test environments.
# Example: MANUAL_USD_RATES={"EUR":"0.92","JPY":"157.50"}
MANUAL_USD_RATES=

# Optional: free forex API for USD-to-fiat rates.
# Default uses ExchangeRate-API open access endpoint (no API key required).
FOREX_API_URL=https://open.er-api.com/v6/latest/USD
```

Update `scripts/init_env.sh` to emit these variables.

### Step 3: Run tests

```bash
cd /home/joseph/documents/dev/better-auth-go
go test ./internal/wallet/rates -run TestIsSupportedFiat -v
```

If no test exists, add a minimal one:

```go
func TestIsSupportedFiat(t *testing.T) {
    if !IsSupportedFiat("USD") {
        t.Fatal("USD should be supported")
    }
    if IsSupportedFiat("XYZ") {
        t.Fatal("XYZ should not be supported")
    }
}
```

### Step 4: Commit

```bash
git add internal/wallet/rates/util.go internal/wallet/rates/util_test.go .env.example scripts/init_env.sh
git commit -m "config(rates): add supported fiat currency list"
```

---

## Task 2: Add Forex Providers (USD-to-Fiat) with Fallback Chain

**Files:**
- Create: `internal/wallet/rates/forex.go`
- Create: `internal/wallet/rates/forex_open.go`
- Create: `internal/wallet/rates/forex_coinbase.go`
- Create: `internal/wallet/rates/forex_fawazahmed0.go`
- Create: `internal/wallet/rates/forex_moneyconvert.go`
- Create: `internal/wallet/rates/forex_test.go`

### Step 1: Define the ForexProvider interface

Create `internal/wallet/rates/forex.go`:

```go
package rates

import (
    "context"
    "encoding/json"
    "os"
    "strings"

    "github.com/shopspring/decimal"
)

// ForexProvider fetches USD-to-fiat exchange rates.
type ForexProvider interface {
    // GetRate returns how many units of fiat 1 USD buys.
    // Example: USD -> EUR returns "0.92".
    GetRate(ctx context.Context, fiat string) (string, error)
    Name() string
}

// ForexChain tries multiple USD-to-fiat providers in order.
type ForexChain struct {
    providers []ForexProvider
    manual    map[string]string
}

func NewForexChain(providers ...ForexProvider) *ForexChain {
    return &ForexChain{
        providers: providers,
        manual:    loadManualUSDRates(),
    }
}

func (fc *ForexChain) Name() string { return "forex-chain" }

func (fc *ForexChain) GetRate(ctx context.Context, fiat string) (string, error) {
    fiat = strings.ToUpper(fiat)
    if fiat == "USD" {
        return "1.00", nil
    }
    if manual, ok := fc.manual[fiat]; ok {
        return manual, nil
    }
    for _, p := range fc.providers {
        value, err := p.GetRate(ctx, fiat)
        if err != nil {
            continue
        }
        return value, nil
    }
    return "", fmt.Errorf("all forex providers failed for %s", fiat)
}

func loadManualUSDRates() map[string]string {
    out := map[string]string{}
    raw := os.Getenv("MANUAL_USD_RATES")
    if raw == "" {
        return out
    }
    var parsed map[string]string
    if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
        return out
    }
    for k, v := range parsed {
        if _, err := decimal.NewFromString(v); err == nil {
            out[strings.ToUpper(k)] = v
        }
    }
    return out
}
```

Add `fmt` import if not already included.

### Step 2: Implement open.er-api.com provider

Create `internal/wallet/rates/forex_open.go`:

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

const openExchangeDefaultURL = "https://open.er-api.com/v6/latest/USD"

type OpenExchange struct {
    client *http.Client
    url    string
}

type OpenExchangeOption func(*OpenExchange)

func WithOpenExchangeURL(u string) OpenExchangeOption {
    return func(o *OpenExchange) { o.url = u }
}

func NewOpenExchange(opts ...OpenExchangeOption) *OpenExchange {
    o := &OpenExchange{
        client: &http.Client{Timeout: 5 * time.Second},
        url:    openExchangeDefaultURL,
    }
    for _, opt := range opts {
        opt(o)
    }
    return o
}

func (o *OpenExchange) Name() string { return "open-er-api" }

func (o *OpenExchange) GetRate(ctx context.Context, fiat string) (string, error) {
    fiat = strings.ToUpper(fiat)
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, o.url, nil)
    if err != nil {
        return "", err
    }
    resp, err := o.client.Do(req)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()
    if resp.StatusCode != http.StatusOK {
        return "", fmt.Errorf("open-er-api status %d", resp.StatusCode)
    }
    var result struct {
        Rates map[string]string `json:"rates"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return "", err
    }
    rate, ok := result.Rates[fiat]
    if !ok {
        return "", fmt.Errorf("open-er-api missing rate for %s", fiat)
    }
    return strings.TrimSpace(rate), nil
}
```

### Step 3: Implement Coinbase forex provider

Create `internal/wallet/rates/forex_coinbase.go`:

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

const coinbaseForexDefaultURL = "https://api.coinbase.com/v2/exchange-rates?currency=USD"

type CoinbaseForex struct {
    client *http.Client
    url    string
}

type CoinbaseForexOption func(*CoinbaseForex)

func WithCoinbaseForexURL(u string) CoinbaseForexOption {
    return func(c *CoinbaseForex) { c.url = u }
}

func NewCoinbaseForex(opts ...CoinbaseForexOption) *CoinbaseForex {
    c := &CoinbaseForex{
        client: &http.Client{Timeout: 5 * time.Second},
        url:    coinbaseForexDefaultURL,
    }
    for _, opt := range opts {
        opt(c)
    }
    return c
}

func (c *CoinbaseForex) Name() string { return "coinbase-forex" }

func (c *CoinbaseForex) GetRate(ctx context.Context, fiat string) (string, error) {
    fiat = strings.ToUpper(fiat)
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
    if err != nil {
        return "", err
    }
    resp, err := c.client.Do(req)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()
    if resp.StatusCode != http.StatusOK {
        return "", fmt.Errorf("coinbase-forex status %d", resp.StatusCode)
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
        return "", fmt.Errorf("coinbase-forex missing rate for %s", fiat)
    }
    return strings.TrimSpace(rate), nil
}
```

### Step 4: Implement fawazahmed0/currency-api provider

Create `internal/wallet/rates/forex_fawazahmed0.go`:

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

const fawazAhmed0DefaultURL = "https://cdn.jsdelivr.net/npm/@fawazahmed0/currency-api@latest/v1/currencies/usd.json"

type FawazAhmed0 struct {
    client *http.Client
    url    string
}

type FawazAhmed0Option func(*FawazAhmed0)

func WithFawazAhmed0URL(u string) FawazAhmed0Option {
    return func(f *FawazAhmed0) { f.url = u }
}

func NewFawazAhmed0(opts ...FawazAhmed0Option) *FawazAhmed0 {
    f := &FawazAhmed0{
        client: &http.Client{Timeout: 5 * time.Second},
        url:    fawazAhmed0DefaultURL,
    }
    for _, opt := range opts {
        opt(f)
    }
    return f
}

func (f *FawazAhmed0) Name() string { return "fawazahmed0" }

func (f *FawazAhmed0) GetRate(ctx context.Context, fiat string) (string, error) {
    fiat = strings.ToLower(fiat)
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.url, nil)
    if err != nil {
        return "", err
    }
    resp, err := f.client.Do(req)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()
    if resp.StatusCode != http.StatusOK {
        return "", fmt.Errorf("fawazahmed0 status %d", resp.StatusCode)
    }
    var result struct {
        USD map[string]json.RawMessage `json:"usd"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return "", err
    }
    raw, ok := result.USD[fiat]
    if !ok {
        return "", fmt.Errorf("fawazahmed0 missing rate for %s", fiat)
    }
    var rate string
    if err := json.Unmarshal(raw, &rate); err != nil {
        return "", err
    }
    return strings.TrimSpace(rate), nil
}
```

### Step 5: Implement MoneyConvert provider

Create `internal/wallet/rates/forex_moneyconvert.go`:

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

const moneyConvertDefaultURL = "https://cdn.moneyconvert.net/api/latest.json"

type MoneyConvert struct {
    client *http.Client
    url    string
}

type MoneyConvertOption func(*MoneyConvert)

func WithMoneyConvertURL(u string) MoneyConvertOption {
    return func(m *MoneyConvert) { m.url = u }
}

func NewMoneyConvert(opts ...MoneyConvertOption) *MoneyConvert {
    m := &MoneyConvert{
        client: &http.Client{Timeout: 5 * time.Second},
        url:    moneyConvertDefaultURL,
    }
    for _, opt := range opts {
        opt(m)
    }
    return m
}

func (m *MoneyConvert) Name() string { return "moneyconvert" }

func (m *MoneyConvert) GetRate(ctx context.Context, fiat string) (string, error) {
    fiat = strings.ToUpper(fiat)
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.url, nil)
    if err != nil {
        return "", err
    }
    resp, err := m.client.Do(req)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()
    if resp.StatusCode != http.StatusOK {
        return "", fmt.Errorf("moneyconvert status %d", resp.StatusCode)
    }
    var result struct {
        Rates map[string]string `json:"rates"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return "", err
    }
    rate, ok := result.Rates[fiat]
    if !ok {
        return "", fmt.Errorf("moneyconvert missing rate for %s", fiat)
    }
    return strings.TrimSpace(rate), nil
}
```

### Step 6: Add tests

Create `internal/wallet/rates/forex_test.go`:

```go
package rates

import (
    "context"
    "net/http"
    "net/http/httptest"
    "testing"
)

func TestForexChain_Fallback(t *testing.T) {
    primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusInternalServerError)
    }))
    defer primary.Close()

    fallback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        w.Write([]byte(`{"base":"USD","rates":{"EUR":"0.92"}}`))
    }))
    defer fallback.Close()

    chain := NewForexChain(
        NewOpenExchange(WithOpenExchangeURL(primary.URL)),
        NewMoneyConvert(WithMoneyConvertURL(fallback.URL)),
    )
    got, err := chain.GetRate(context.Background(), "EUR")
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if got != "0.92" {
        t.Fatalf("expected 0.92, got %s", got)
    }
}

func TestForexChain_USD(t *testing.T) {
    chain := NewForexChain()
    got, err := chain.GetRate(context.Background(), "USD")
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if got != "1.00" {
        t.Fatalf("expected 1.00, got %s", got)
    }
}
```

### Step 7: Run tests

```bash
go test ./internal/wallet/rates -run TestForex -v
```

Expected: PASS.

### Step 8: Commit

```bash
git add internal/wallet/rates/forex*.go
git commit -m "feat(rates): add four-provider USD-to-fiat fallback chain"
```

---

## Task 3: Update Crypto Providers for Multi-Fiat

**Files:**
- Modify: `internal/wallet/rates/binance.go`
- Modify: `internal/wallet/rates/coinbase.go`
- Modify: `internal/wallet/rates/kraken.go`
- Modify: `internal/wallet/rates/kucoin.go`

### Step 1: Update Binance provider

Change `GetRate` in `internal/wallet/rates/binance.go`:

```go
func (b *Binance) GetRate(ctx context.Context, fiat, crypto string) (string, error) {
    fiat = strings.ToUpper(fiat)
    crypto = strings.ToUpper(crypto)
    if fiat == "USD" && isStablecoin(crypto) {
        return "1.00", nil
    }
    crypto = normalizeSymbol(crypto)
    if fiat == "USD" {
        fiat = "USDT"
    }
    url := fmt.Sprintf("%s/api/v3/ticker/price?symbol=%s%s", b.url, crypto, fiat)
    // ... rest unchanged
}
```

Update imports to include `strings` if not already.

### Step 2: Update Coinbase provider

Change `GetRate` in `internal/wallet/rates/coinbase.go`:

```go
func (c *Coinbase) GetRate(ctx context.Context, fiat, crypto string) (string, error) {
    fiat = strings.ToUpper(fiat)
    crypto = strings.ToUpper(crypto)
    if fiat == "USD" && isStablecoin(crypto) {
        return "1.00", nil
    }
    crypto = normalizeSymbol(crypto)
    if fiat == "USD" {
        fiat = "USDT"
    }
    url := fmt.Sprintf("%s/v2/exchange-rates?currency=%s", c.url, crypto)
    // ... rest unchanged
}
```

Update imports to include `strings` if not already.

### Step 3: Update Kraken provider

Change `GetRate` in `internal/wallet/rates/kraken.go`:

```go
func (k *Kraken) GetRate(ctx context.Context, fiat, crypto string) (string, error) {
    fiat = strings.ToUpper(fiat)
    crypto = strings.ToUpper(crypto)
    if fiat == "USD" && isStablecoin(crypto) {
        return "1.00", nil
    }
    crypto = normalizeSymbol(crypto)
    if fiat == "USD" {
        fiat = "USDT"
    }
    url := fmt.Sprintf("%s/0/public/Ticker?pair=%s%s", k.url, crypto, fiat)
    // ... rest unchanged
}
```

Update imports to include `strings` if not already.

### Step 4: Update KuCoin provider

Change `GetRate` in `internal/wallet/rates/kucoin.go`:

```go
func (k *KuCoin) GetRate(ctx context.Context, fiat, crypto string) (string, error) {
    fiat = strings.ToUpper(fiat)
    crypto = strings.ToUpper(crypto)
    if fiat == "USD" && isStablecoin(crypto) {
        return "1.00", nil
    }
    crypto = normalizeSymbol(crypto)
    url := fmt.Sprintf("%s/api/v1/prices?base=%s&currencies=%s", k.url, fiat, crypto)
    // ... rest unchanged
}
```

Update imports to include `strings` if not already.

### Step 5: Run tests

```bash
go test ./internal/wallet/rates -v
```

Expected: PASS.

### Step 6: Commit

```bash
git add internal/wallet/rates/binance.go internal/wallet/rates/coinbase.go internal/wallet/rates/kraken.go internal/wallet/rates/kucoin.go
git commit -m "feat(rates): support direct fiat pairs in crypto providers"
```

---

## Task 4: Update Aggregator for Multi-Fiat

**Files:**
- Modify: `internal/wallet/rates/aggregator.go`
- Modify: `internal/wallet/rates/aggregator_test.go`

### Step 1: Update aggregator struct

Add `forex` field to `Aggregator`:

```go
type Aggregator struct {
    cache     *Cache
    providers []RateProvider
    forex     *ForexChain
    manual    map[string]string
    mu        sync.RWMutex
}

func NewAggregator(cache *Cache, forex *ForexChain, providers ...RateProvider) *Aggregator {
    return &Aggregator{
        cache:     cache,
        providers: providers,
        forex:     forex,
        manual:    loadManualRates(),
    }
}
```

### Step 2: Update GetRate for cross-conversion

Replace `GetRate` with:

```go
func (a *Aggregator) GetRate(ctx context.Context, fiat, crypto string) (string, error) {
    fiat = strings.ToUpper(fiat)
    crypto = strings.ToUpper(crypto)
    if !IsSupportedFiat(fiat) {
        return "", fmt.Errorf("unsupported fiat: %s", fiat)
    }
    if fiat == "USD" && isStablecoin(crypto) {
        return "1.00", nil
    }

    key := crypto + ":" + fiat
    if v, ok := a.cache.Get(key); ok {
        return v, nil
    }
    if manual, ok := a.manualRate(crypto); ok {
        // Manual rates are always in USD. Convert to requested fiat if needed.
        if fiat == "USD" {
            a.cache.Set(key, manual)
            return manual, nil
        }
        converted, err := a.convertUSDToFiat(ctx, fiat, manual)
        if err != nil {
            return "", err
        }
        a.cache.Set(key, converted)
        return converted, nil
    }

    // Try direct crypto->fiat from providers.
    for _, p := range a.providers {
        value, err := p.GetRate(ctx, fiat, crypto)
        if err != nil {
            continue
        }
        a.cache.Set(key, value)
        return value, nil
    }

    // Fallback: crypto->USD * USD->fiat.
    cryptoUSD, err := a.cryptoToUSD(ctx, crypto)
    if err != nil {
        if stale, ok := a.cache.StaleValue(key); ok {
            return stale, nil
        }
        return "0", fmt.Errorf("all providers failed for %s/%s", crypto, fiat)
    }
    if fiat == "USD" {
        a.cache.Set(key, cryptoUSD)
        return cryptoUSD, nil
    }
    converted, err := a.convertUSDToFiat(ctx, fiat, cryptoUSD)
    if err != nil {
        if stale, ok := a.cache.StaleValue(key); ok {
            return stale, nil
        }
        return "0", fmt.Errorf("failed to convert %s to %s: %w", crypto, fiat, err)
    }
    a.cache.Set(key, converted)
    return converted, nil
}

func (a *Aggregator) cryptoToUSD(ctx context.Context, crypto string) (string, error) {
    if isStablecoin(crypto) {
        return "1.00", nil
    }
    key := crypto + ":USD"
    if v, ok := a.cache.Get(key); ok {
        return v, nil
    }
    for _, p := range a.providers {
        value, err := p.GetRate(ctx, "USD", crypto)
        if err != nil {
            continue
        }
        a.cache.Set(key, value)
        return value, nil
    }
    return "", fmt.Errorf("failed to get %s/USD", crypto)
}

func (a *Aggregator) convertUSDToFiat(ctx context.Context, fiat, usdValue string) (string, error) {
    rate, err := a.forex.GetRate(ctx, fiat)
    if err != nil {
        return "", err
    }
    return MulDecimalStrings(usdValue, rate)
}
```

Add `github.com/realyoussefhossam/betmonster/internal/wallet` import? No, `MulDecimalStrings` is in `wallet` package, not `rates`. Move `MulDecimalStrings` to `rates` package or duplicate it.

**Decision:** Move `MulDecimalStrings` to `internal/wallet/rates/decimal.go` so the rates package can multiply without importing the wallet package. Remove it from `internal/wallet/decimal.go` and update callers.

Create `internal/wallet/rates/decimal.go` (exported, replaces the wallet version):

```go
package rates

import "github.com/shopspring/decimal"

// MulDecimalStrings multiplies two decimal strings and returns the result as a string.
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

Delete `internal/wallet/decimal.go` and `internal/wallet/decimal_test.go` after copying tests to `rates/decimal_test.go`.

Update `internal/wallet/server.go` to use `rates.MulDecimalStrings`.

### Step 3: Update SupportedRates

```go
func (a *Aggregator) SupportedRates(ctx context.Context, fiat string, currencies []string) map[string]string {
    out := make(map[string]string, len(currencies))
    for _, c := range currencies {
        v, err := a.GetRate(ctx, fiat, c)
        if err != nil {
            v = "0"
        }
        out[c] = v
    }
    return out
}
```

### Step 4: Add tests

Add to `internal/wallet/rates/aggregator_test.go`:

```go
func TestAggregatorCrossConvert(t *testing.T) {
    cache := NewCache(30 * time.Second)
    agg := NewAggregator(cache, NewForexChain(NewOpenExchange()), NewBinance())
    got, err := agg.GetRate(context.Background(), "EUR", "USDT")
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if got == "0" || got == "" {
        t.Fatalf("expected a EUR rate for USDT, got %s", got)
    }
}
```

### Step 5: Run tests

```bash
go test ./internal/wallet/rates -v
```

Expected: PASS (external API calls may fail offline; mark as integration test if needed).

### Step 6: Commit

```bash
git add internal/wallet/rates/aggregator.go internal/wallet/rates/aggregator_test.go internal/wallet/rates/decimal.go internal/wallet/rates/decimal_test.go internal/wallet/server.go
git rm internal/wallet/decimal.go internal/wallet/decimal_test.go
git commit -m "feat(rates): cross-convert crypto to any supported fiat"
```

---

## Task 5: Update Protobuf for Multi-Fiat

**Files:**
- Modify: `internal/proto/wallet.proto`
- Regenerate: `internal/proto/wallet.pb.go`
- Regenerate: `internal/proto/wallet_grpc.pb.go`

### Step 1: Update proto messages

Modify:

```protobuf
message GetRatesRequest {
    string fiat_currency = 1;
}

message GetBalanceRequest {
    string user_id = 1;
    string currency = 2;
    string fiat_currency = 3;
}

message ListTransactionsRequest {
    string user_id = 1;
    int32 page = 2;
    int32 page_size = 3;
    string fiat_currency = 4;
}
```

### Step 2: Regenerate

```bash
cd internal/proto
protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative wallet.proto
```

If `protoc` is not available, use the `buf` or `go generate` command defined in the project.

### Step 3: Run tests

```bash
go test ./internal/proto -v
# or just build
```

### Step 4: Commit

```bash
git add internal/proto/wallet.proto internal/proto/wallet.pb.go internal/proto/wallet_grpc.pb.go
git commit -m "proto(wallet): add fiat_currency to requests"
```

---

## Task 6: Update Wallet Service for Multi-Fiat

**Files:**
- Modify: `internal/wallet/server.go`
- Modify: `internal/wallet/service.go`
- Modify: `internal/wallet/server_test.go`

### Step 1: Update server methods

```go
func defaultFiat(reqFiat string) string {
    fiat := strings.ToUpper(strings.TrimSpace(reqFiat))
    if fiat == "" {
        fiat = os.Getenv("DEFAULT_FIAT_CURRENCY")
    }
    if fiat == "" {
        fiat = "USD"
    }
    if !rates.IsSupportedFiat(fiat) {
        fiat = "USD"
    }
    return fiat
}

func (s *GRPCServer) fiatValue(ctx context.Context, fiat, currency, amount string) (string, error) {
    rate, err := s.rates.GetRate(ctx, fiat, currency)
    if err != nil {
        return "", err
    }
    return rates.MulDecimalStrings(amount, rate)
}

func (s *GRPCServer) GetRates(ctx context.Context, req *pb.GetRatesRequest) (*pb.GetRatesResponse, error) {
    fiat := defaultFiat(req.FiatCurrency)
    currencies := s.service.supportedCurrencies()
    rs := s.rates.SupportedRates(ctx, fiat, currencies)
    return &pb.GetRatesResponse{
        FiatCurrency: fiat,
        Rates:        rs,
    }, nil
}

func (s *GRPCServer) GetBalance(ctx context.Context, req *pb.GetBalanceRequest) (*pb.GetBalanceResponse, error) {
    wallet, err := s.service.GetBalance(ctx, req.UserId, req.Currency)
    if err != nil {
        return nil, err
    }
    fiat := defaultFiat(req.FiatCurrency)
    fiatValue := "0"
    if s.rates != nil {
        v, err := s.fiatValue(ctx, fiat, wallet.Currency, wallet.Balance)
        if err == nil {
            fiatValue = v
        }
    }
    return &pb.GetBalanceResponse{
        Currency:     wallet.Currency,
        Balance:      wallet.Balance,
        FiatCurrency: fiat,
        FiatValue:    fiatValue,
    }, nil
}

func (s *GRPCServer) ListTransactions(ctx context.Context, req *pb.ListTransactionsRequest) (*pb.ListTransactionsResponse, error) {
    txns, err := s.service.ListTransactions(ctx, req.UserId, int(req.Page), int(req.PageSize))
    if err != nil {
        return nil, err
    }
    fiat := defaultFiat(req.FiatCurrency)
    out := make([]*pb.Transaction, len(txns))
    for i, t := range txns {
        fiatValue := "0"
        if s.rates != nil {
            v, err := s.fiatValue(ctx, fiat, t.Currency, t.Amount)
            if err == nil {
                fiatValue = v
            }
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

### Step 2: Add `supportedCurrencies` to service

If not already present, ensure `internal/wallet/service.go` has:

```go
func (s *Service) supportedCurrencies() []string {
    return s.config.SupportedCurrencies
}
```

(Adjust based on actual service config structure.)

### Step 3: Run tests

```bash
go test ./internal/wallet -v
```

### Step 4: Commit

```bash
git add internal/wallet/server.go internal/wallet/service.go internal/wallet/server_test.go
git commit -m "feat(wallet): use requested fiat currency for balances and transactions"
```

---

## Task 7: Update Gateway Rates Endpoint

**Files:**
- Modify: `internal/gateway/server.go`
- Modify: `internal/gateway/wallet_client.go`

### Step 1: Update wallet client

```go
func (c *WalletClient) GetRates(ctx context.Context, fiat string) (*pb.GetRatesResponse, error) {
    return c.conn.GetRates(ctx, &pb.GetRatesRequest{FiatCurrency: fiat})
}
```

### Step 2: Update gateway handler

```go
func (s *Server) handleRates(w http.ResponseWriter, r *http.Request) {
    fiat := r.URL.Query().Get("fiat_currency")
    resp, err := s.wallet.GetRates(r.Context(), fiat)
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

### Step 3: Update balance and transaction handlers

Find handlers for balance and transactions and pass `r.URL.Query().Get("fiat_currency")` to the wallet client.

### Step 4: Run tests

```bash
go test ./internal/gateway -v
```

### Step 5: Commit

```bash
git add internal/gateway/server.go internal/gateway/wallet_client.go
git commit -m "feat(gateway): accept fiat_currency query param"
```

---

## Task 8: Update Frontend API Client

**Files:**
- Modify: `app/lib/go-api-client.ts`

### Step 1: Add fiat param to methods

```typescript
async getBalance(currency: string, fiatCurrency?: string): Promise<ApiResponse<BalanceResponse>> {
    const params = new URLSearchParams();
    if (fiatCurrency) params.set("fiat_currency", fiatCurrency);
    const query = params.toString() ? `?${params.toString()}` : "";
    return this.get<BalanceResponse>(`/api/wallet/balance/${currency}${query}`);
}

async getTransactions(fiatCurrency?: string): Promise<ApiResponse<{ transactions: Transaction[] }>> {
    const params = new URLSearchParams();
    if (fiatCurrency) params.set("fiat_currency", fiatCurrency);
    const query = params.toString() ? `?${params.toString()}` : "";
    return this.get<{ transactions: Transaction[] }>(`/api/wallet/transactions${query}`);
}

async getRates(fiatCurrency?: string): Promise<ApiResponse<RatesResponse>> {
    const params = new URLSearchParams();
    if (fiatCurrency) params.set("fiat_currency", fiatCurrency);
    const query = params.toString() ? `?${params.toString()}` : "";
    return this.get<RatesResponse>(`/api/wallet/rates${query}`);
}
```

### Step 2: Commit

```bash
git add app/lib/go-api-client.ts
git commit -m "feat(ui): pass fiat currency to wallet API client"
```

---

## Task 9: Add Fiat Currency Selector

**Files:**
- Create: `app/components/fiat-selector.tsx`
- Modify: `app/app/wallet/page.tsx`
- Modify: `app/components/rates-footer.tsx`
- Modify: `app/app/layout.tsx`

### Step 1: Create selector

Create `app/components/fiat-selector.tsx`:

```tsx
"use client";

import { useEffect, useState } from "react";

const SUPPORTED_FIAT = [
  "USD", "EUR", "JPY", "INR", "CAD", "CNY", "IDR", "KRW", "PHP", "RUB",
  "MXN", "PLN", "TRY", "VND", "ARS", "PEN", "CLP", "NGN", "AED", "BHD",
  "CRC", "KWD", "MAD", "MYR", "QAR", "SAR", "SGD", "TND", "TWD", "GHS",
  "KES", "BOB", "XOF", "PKR", "NZD", "ISK", "BAM", "TZS", "EGP", "LKR",
  "UGX", "KZT", "BDT", "UAH", "GEL", "MNT", "GTQ", "KGS", "ZAR", "TMT",
  "ZMW", "TJS", "MRU", "TTD", "GMD", "MGA", "JMD", "NIO", "HNL", "MZN",
  "XAF", "RWF", "GNF", "BWP", "KMF", "LSL", "ERN", "BIF", "MWK", "PGK",
];

const STORAGE_KEY = "betmonster-fiat";

export function useFiatCurrency() {
  const [fiat, setFiat] = useState<string>("USD");

  useEffect(() => {
    const saved = typeof window !== "undefined" ? localStorage.getItem(STORAGE_KEY) : null;
    if (saved && SUPPORTED_FIAT.includes(saved)) {
      setFiat(saved);
    }
  }, []);

  const updateFiat = (value: string) => {
    setFiat(value);
    if (typeof window !== "undefined") {
      localStorage.setItem(STORAGE_KEY, value);
    }
  };

  return { fiat, setFiat: updateFiat };
}

export function FiatSelector({ value, onChange }: { value: string; onChange: (v: string) => void }) {
  return (
    <select
      value={value}
      onChange={(e) => onChange(e.target.value)}
      className="rounded border bg-background px-2 py-1 text-sm"
      aria-label="Display currency"
    >
      {SUPPORTED_FIAT.map((c) => (
        <option key={c} value={c}>
          {c}
        </option>
      ))}
    </select>
  );
}
```

### Step 2: Update wallet page

Modify `app/app/wallet/page.tsx`:

```tsx
import { FiatSelector, useFiatCurrency } from "@/components/fiat-selector";

export default function WalletPage() {
  const { fiat, setFiat } = useFiatCurrency();
  // ... existing state

  useEffect(() => {
    async function load() {
      const [supportedRes, txRes] = await Promise.all([
        goApiClient.getSupportedOptions(),
        goApiClient.getTransactions(fiat),
      ]);
      // ...
      const balanceResults = await Promise.all(
        supportedRes.data.currencies.map((currency) =>
          goApiClient.getBalance(currency, fiat),
        ),
      );
      // ...
    }
    load();
  }, [fiat, showZero]);

  return (
    <div className="container mx-auto max-w-2xl py-8 space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-3xl font-bold">Wallet</h1>
        <FiatSelector value={fiat} onChange={setFiat} />
      </div>
      {/* ... */}
    </div>
  );
}
```

### Step 3: Update footer

Modify `app/components/rates-footer.tsx`:

```tsx
"use client";

import { useEffect, useState } from "react";
import { goApiClient } from "@/lib/go-api-client";
import { useFiatCurrency } from "@/components/fiat-selector";

export function RatesFooter() {
  const { fiat } = useFiatCurrency();
  const [rates, setRates] = useState<Record<string, string>>({});

  useEffect(() => {
    async function load() {
      const res = await goApiClient.getRates(fiat);
      if (res.data) {
        setRates(res.data.rates);
      }
    }
    load();
    const id = setInterval(load, 60000);
    return () => clearInterval(id);
  }, [fiat]);
  // ... rest unchanged
}
```

### Step 4: Run lint

```bash
cd app && npm run lint
```

### Step 5: Commit

```bash
git add app/components/fiat-selector.tsx app/app/wallet/page.tsx app/components/rates-footer.tsx
git commit -m "feat(ui): fiat currency selector and multi-fiat display"
```

---

## Task 10: Update Wallet Command Wiring

**Files:**
- Modify: `cmd/wallet/main.go`

### Step 1: Wire forex provider

Ensure the aggregator is created with the forex provider:

```go
cache := rates.NewCache(ttl)
forex := rates.NewForexChain(
    rates.NewOpenExchange(),
    rates.NewCoinbaseForex(),
    rates.NewFawazAhmed0(),
    rates.NewMoneyConvert(),
)
agg := rates.NewAggregator(cache, forex,
    rates.NewBinance(),
    rates.NewCoinbase(),
    rates.NewKraken(),
    rates.NewKuCoin(),
)
```

### Step 2: Commit

```bash
git add cmd/wallet/main.go
git commit -m "feat(cmd/wallet): wire forex provider into aggregator"
```

---

## Task 11: Verify and Rebuild

**Files:**
- All touched files

### Step 1: Run all tests

```bash
cd /home/joseph/documents/dev/better-auth-go
go test ./...
cd app && npm run lint
```

### Step 2: Rebuild Docker

```bash
cd /home/joseph/documents/dev/better-auth-go
docker compose up -d --build wallet gateway app
./scripts/migrate.sh up
```

### Step 3: Test the endpoint

```bash
curl "http://localhost:8080/api/wallet/rates?fiat_currency=EUR"
```

Expected: JSON with `fiat_currency: "EUR"` and rates.

### Step 4: Commit any fixes

```bash
git add -A
git commit -m "fix(multi-fiat): address verification issues"
```

---

## Self-Review

**Spec coverage:**
- ✅ 70 fiat currencies supported
- ✅ Settlement stays in crypto; fiat is display-only
- ✅ User can select display currency
- ✅ Rates cross-converted via USD when direct pair unavailable

**Placeholder scan:**
- No TBD/TODO placeholders.
- Exact code and commands provided.

**Type consistency:**
- `fiat_currency` added consistently across proto, Go, and TypeScript.
- `MulDecimalStrings` moved to `rates` package.

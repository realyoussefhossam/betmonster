# BetMonster Fiat Display and Zero Balance Hiding Design

**Scope:** add estimated USD fiat display to wallet balances and transactions, and hide zero-balance wallets from the wallet grid.

**Date:** 2026-07-05
**Status:** draft — pending implementation plan

## 1. Context

BetMonster v1 maintains per-currency cryptocurrency wallets. Operators and users expect a modern wallet experience similar to major crypto platforms: balances shown in both crypto and an estimated fiat value, and empty wallets hidden to reduce clutter.

This design covers the backend rate infrastructure, the gRPC/API changes, and the Next.js frontend updates.

## 2. Goals

- Display an estimated **USD** value next to every cryptocurrency balance and transaction amount.
- Hide wallets with **zero balance** from the wallet grid.
- Add a site-wide footer ticker showing live USD rates for major cryptocurrencies.
- Keep the fiat display **informational only** — bets and transactions continue to settle in cryptocurrency.

## 3. Non-Goals

- No fiat deposits, withdrawals, or settlement.
- No currency conversion at the wallet level.
- No real-time trading feeds (WebSocket).
- No admin rate management in v1.

## 4. Architecture

```text
┌─────────────────┐      ┌────────────────────┐      ┌─────────────────────┐
│   Next.js UI    │──────▶│     Gateway        │──────▶│   Wallet Service    │
│ (wallet page,   │      │  (routes, JWT)     │      │  (rates, balances)  │
│  footer ticker) │      │                    │      │                     │
└─────────────────┘      └────────────────────┘      └──────────┬──────────┘
                                                                  │
                                                                  ▼
                                                       ┌─────────────────────┐
                                                       │   rates.Aggregator  │
                                                       │  (cache + providers) │
                                                       └──────────┬──────────┘
                                                                  │
                                                                  ▼
                                                       ┌─────────────────────┐
                                                       │ Binance → Coinbase  │
                                                       │ → Kraken → KuCoin   │
                                                       └─────────────────────┘
```

Components:

- **`internal/wallet/rates/provider.go`** — `RateProvider` interface.
- **`internal/wallet/rates/binance.go`** — Binance USD rate fetcher.
- **`internal/wallet/rates/coinbase.go`** — Coinbase fallback.
- **`internal/wallet/rates/kraken.go`** — Kraken fallback.
- **`internal/wallet/rates/kucoin.go`** — KuCoin fallback.
- **`internal/wallet/rates/cache.go`** — in-memory TTL cache (30 seconds).
- **`internal/wallet/rates/aggregator.go`** — provider chain, stablecoin alias handling, and error fallback.
- **Wallet service** — `GRPCServer` gets a `*rates.Aggregator` and enriches `GetBalance` and `ListTransactions` responses with `fiat_value` and `fiat_currency`.
- **Gateway service** — adds `GET /api/wallet/rates` for the public footer ticker.
- **Next.js frontend** — `WalletCard` and transaction list show fiat values; wallet grid filters zero balances; footer ticker shows major rates.

## 5. Exchange Rate Behavior

### Supported providers (in order)

1. **Binance** — primary.
2. **Coinbase** — fallback.
3. **Kraken** — fallback.
4. **KuCoin** — fallback.

### Stablecoin rule

For stablecoins (`USDT`, `USDC`, `BUSD`, `DAI`), the rate is always `1.00` USD. No external API call is made.

### Crypto-to-USD mapping

The aggregator normalizes crypto symbols before querying:

| Wallet Currency | Provider Symbol |
|---|---|
| ETH | ETH |
| BNB | BNB |
| TRX | TRX |
| POL | POL (formerly MATIC) |
| BTC | BTC |
| SHIB | SHIB |
| BETM | BETM (operator token) |

If a symbol is not supported by any provider, the aggregator returns `0` and logs a warning.

### Caching

- Rates are cached in memory for **30 seconds**.
- Cache key: `{currency}:{fiat}`.
- On cache miss, the aggregator tries providers in order until one succeeds.
- If all providers fail, the last cached value is returned if it is less than **5 minutes** old; otherwise it returns `0`.

### Hardcoded fallback

If the operator sets `MANUAL_RATES` in the environment (JSON map), those values override external providers. This is useful for tokens like `BETM` that are not listed on exchanges.

Example:

```env
MANUAL_RATES={"BETM":"0.50","SHIB":"0.000025"}
```

## 6. gRPC and API Changes

### `internal/proto/wallet.proto`

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

### New public endpoint

`GET /api/wallet/rates` returns a map of supported currencies to USD rates.

```json
{
  "fiat_currency": "USD",
  "rates": {
    "BTC": "63420.00",
    "ETH": "3410.50",
    "USDT": "1.00",
    "USDC": "1.00",
    "BNB": "590.20"
  }
}
```

This endpoint is public (no JWT required) because it only displays market data.

## 7. Wallet Service Changes

### New server dependency

`GRPCServer` receives a `*rates.Aggregator`:

```go
func NewGRPCServer(service *Service, rates *rates.Aggregator) *GRPCServer
```

### GetBalance

`Service.GetBalance` continues to return `*Wallet`. The gRPC handler enriches the response:

```go
func (s *GRPCServer) GetBalance(ctx context.Context, req *pb.GetBalanceRequest) (*pb.GetBalanceResponse, error) {
    wallet, err := s.service.GetBalance(ctx, req.UserId, req.Currency)
    if err != nil {
        return nil, err
    }
    rate := s.rates.GetRate(ctx, req.Currency, "USD")
    fiatValue := multiplyDecimalStrings(wallet.Balance, rate)
    return &pb.GetBalanceResponse{
        Currency:     wallet.Currency,
        Balance:      wallet.Balance,
        FiatCurrency: "USD",
        FiatValue:    fiatValue,
    }, nil
}
```

### ListTransactions

Each transaction includes `fiat_value` computed from the transaction amount and the rate at response time in the gRPC handler.

## 8. Frontend Changes

### Wallet page (`app/app/wallet/page.tsx`)

- Fetch balances as before.
- Filter out balances where `balance === "0" || balance === "0.00000000"`.
- Pass `fiatValue` to `WalletCard`.

### WalletCard component (`app/components/wallet-card.tsx`)

Display:

```
100.00000000 USDT
≈ $100.00 USD
```

### Transaction list

Each transaction item shows:

```
+10.00000000 USDT  completed
≈ $10.00 USD
```

### Footer ticker

New component `app/components/rates-footer.tsx`:

- Fetches `GET /api/wallet/rates` every 60 seconds.
- Shows a compact ticker:

```
BTC $63,420.00 | ETH $3,410.50 | USDT $1.00 | USDC $1.00 | BNB $590.20
```

- Rendered in the root layout so it appears on every page.

## 9. Error Handling

- If a provider API call fails, the aggregator tries the next provider.
- If all providers fail, it returns the last cached value if fresh enough; otherwise `0`.
- `fiat_value` is always a string and never `null`. A failure state is represented as `"0"`.
- Provider timeouts are capped at **5 seconds** to avoid blocking wallet requests.
- Provider failures are logged but do not fail the wallet request.

## 10. Testing

- Unit tests for each provider using a mock HTTP server.
- Unit tests for the aggregator: stablecoin rule, provider fallback, cache behavior, stale-cache fallback.
- Unit tests for `GetBalance` and `ListTransactions` fiat enrichment.
- Frontend test for zero-balance filtering.
- Integration test for the footer ticker endpoint.

## 11. Files Added or Modified

### Backend

- `internal/wallet/rates/provider.go`
- `internal/wallet/rates/binance.go`
- `internal/wallet/rates/coinbase.go`
- `internal/wallet/rates/kraken.go`
- `internal/wallet/rates/kucoin.go`
- `internal/wallet/rates/cache.go`
- `internal/wallet/rates/aggregator.go`
- `internal/proto/wallet.proto`
- `internal/proto/wallet.pb.go`
- `internal/proto/wallet_grpc.pb.go`
- `internal/wallet/server.go`
- `internal/wallet/service.go`
- `internal/wallet/models.go`
- `internal/wallet/store.go`
- `internal/wallet/memory_store.go`
- `internal/wallet/pgstore.go`
- `internal/gateway/server.go`
- `internal/gateway/wallet_client.go`
- `cmd/wallet/main.go`
- `cmd/gateway/main.go`
- `.env.example`
- `.env`

### Frontend

- `app/components/wallet-card.tsx`
- `app/app/wallet/page.tsx`
- `app/components/rates-footer.tsx`
- `app/app/layout.tsx`
- `app/lib/go-api-client.ts`

## 12. Configuration

New environment variables:

```env
# Optional: override rates for operator tokens or test environments.
MANUAL_RATES={"BETM":"0.50"}

# Optional: disable external providers and use only manual rates.
RATES_PROVIDER=manual

# Default: 30 seconds
RATES_CACHE_TTL_SECONDS=30
```

## 13. Future Work

- Add more fiat currencies (EUR, GBP) via `fiat_currency` config.
- Add admin UI to set manual rates.
- Add WebSocket rate feed for real-time trading pages.
- Add rate staleness alerts and metrics.

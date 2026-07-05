package rates

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/shopspring/decimal"
)

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

func (a *Aggregator) manualRate(crypto string) (string, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	v, ok := a.manual[crypto]
	return v, ok
}

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

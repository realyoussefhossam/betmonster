package rates

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/shopspring/decimal"
)

type Aggregator struct {
	cache     *Cache
	providers []RateProvider
	manual    map[string]string
	mu        sync.RWMutex
}

func NewAggregator(cache *Cache, providers ...RateProvider) *Aggregator {
	return &Aggregator{
		cache:     cache,
		providers: providers,
		manual:    loadManualRates(),
	}
}

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

package rates

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/shopspring/decimal"
)

// ForexProvider fetches USD-to-fiat exchange rates.
type ForexProvider interface {
	// GetRate returns how many units of fiat 1 USD buys.
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

package rates

import (
	"context"
	"errors"
	"testing"
	"time"
)

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

func TestAggregatorStablecoin(t *testing.T) {
	agg := NewAggregator(NewCache(30*time.Second), NewForexChain())
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
	agg := NewAggregator(NewCache(30*time.Second), NewForexChain(), failing, working)
	got, err := agg.GetRate(context.Background(), "USD", "BTC")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "50000.00" {
		t.Fatalf("expected 50000.00, got %s", got)
	}
}

func TestAggregatorCrossConvert(t *testing.T) {
	t.Setenv("MANUAL_USD_RATES", `{"EUR":"0.92"}`)
	cache := NewCache(30 * time.Second)
	agg := NewAggregator(cache, NewForexChain(), NewBinance())
	got, err := agg.GetRate(context.Background(), "EUR", "USDT")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == "0" || got == "" {
		t.Fatalf("expected a EUR rate for USDT, got %s", got)
	}
}

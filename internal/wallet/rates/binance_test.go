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

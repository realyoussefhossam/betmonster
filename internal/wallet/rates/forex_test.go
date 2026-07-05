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

func TestForexChain_EachProvider(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/coinbase":
			w.Write([]byte(`{"data":{"rates":{"EUR":"0.92"}}}`))
		default:
			w.Write([]byte(`{"base":"USD","rates":{"EUR":"0.92"}}`))
		}
	}))
	defer server.Close()

	providers := []ForexProvider{
		NewOpenExchange(WithOpenExchangeURL(server.URL + "/open")),
		NewCoinbaseForex(WithCoinbaseForexURL(server.URL + "/coinbase")),
		NewMoneyConvert(WithMoneyConvertURL(server.URL + "/money")),
	}
	for _, p := range providers {
		got, err := p.GetRate(context.Background(), "EUR")
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", p.Name(), err)
		}
		if got != "0.92" {
			t.Fatalf("%s: expected 0.92, got %s", p.Name(), got)
		}
	}
}

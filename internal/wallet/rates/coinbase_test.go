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

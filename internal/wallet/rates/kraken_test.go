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

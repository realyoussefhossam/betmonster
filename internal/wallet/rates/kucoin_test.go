package rates

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestKuCoinGetRate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"code":"200000","data":{"BNB":"590.20"}}`))
	}))
	defer server.Close()

	p := NewKuCoin(WithKuCoinURL(server.URL))
	got, err := p.GetRate(context.Background(), "USD", "BNB")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "590.20" {
		t.Fatalf("expected 590.20, got %s", got)
	}
}

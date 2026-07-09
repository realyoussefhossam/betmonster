package azuro

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/realyoussefhossam/betmonster/internal/oddsfeed"
)

func TestProviderName(t *testing.T) {
	p := New("", "", "")
	if p.Name() != ProviderName {
		t.Fatalf("expected name %s, got %s", ProviderName, p.Name())
	}
}

func TestValidateConfigMissingGraphURL(t *testing.T) {
	p := New("", "", "")
	if err := p.ValidateConfig(oddsfeed.ProviderConfig{}); err == nil {
		t.Fatal("expected error for missing graph URL")
	}
}

func TestValidateConfigOK(t *testing.T) {
	p := New("https://graph.azuro.org", "", "")
	if err := p.ValidateConfig(oddsfeed.ProviderConfig{GraphURL: "https://graph.azuro.org"}); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestFetchSnapshotWithoutGraphURL(t *testing.T) {
	p := New("", "", "")
	_, err := p.FetchSnapshot(context.Background(), "", nil)
	if err == nil {
		t.Fatal("expected error when graph URL not configured")
	}
}

func TestFetchSnapshotEmptyWhenConfigured(t *testing.T) {
	p := New("https://graph.azuro.org", "", "")
	snap, err := p.FetchSnapshot(context.Background(), "", nil)
	if err == nil {
		t.Fatal("expected network error when hitting placeholder URL")
	}
	if snap != nil {
		t.Fatalf("expected nil snapshot on error, got %v", snap)
	}
}

func TestSubscribeLiveWithoutWSURL(t *testing.T) {
	p := New("https://graph.azuro.org", "", "")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := p.SubscribeLive(ctx, "", make(chan oddsfeed.Update)); err == nil {
		t.Fatal("expected error when websocket URL not configured")
	}
}

func TestAzuroHierarchyURL(t *testing.T) {
	var gotURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sportsResponse{Sports: []sport{}})
	}))
	defer srv.Close()

	p := New(srv.URL, "", "PolygonUSDT")
	if _, err := p.FetchHierarchy(context.Background(), "", nil); err != nil {
		t.Fatalf("fetch hierarchy: %v", err)
	}
	if !strings.Contains(gotURL, "numberOfGames=100") {
		t.Fatalf("expected URL to contain numberOfGames=100, got %s", gotURL)
	}
	if !strings.Contains(gotURL, "orderDirection=desc") {
		t.Fatalf("expected URL to contain orderDirection=desc, got %s", gotURL)
	}
	if strings.Contains(gotURL, "orderDirection=asc") {
		t.Fatalf("expected URL not to contain orderDirection=asc, got %s", gotURL)
	}
}

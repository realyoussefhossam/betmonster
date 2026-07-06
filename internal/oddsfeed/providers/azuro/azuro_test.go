package azuro

import (
	"context"
	"testing"

	"github.com/realyoussefhossam/betmonster/internal/oddsfeed"
)

func TestProviderName(t *testing.T) {
	p := New("", "")
	if p.Name() != ProviderName {
		t.Fatalf("expected name %s, got %s", ProviderName, p.Name())
	}
}

func TestValidateConfigMissingGraphURL(t *testing.T) {
	p := New("", "")
	if err := p.ValidateConfig(oddsfeed.ProviderConfig{}); err == nil {
		t.Fatal("expected error for missing graph URL")
	}
}

func TestValidateConfigOK(t *testing.T) {
	p := New("https://graph.azuro.org", "")
	if err := p.ValidateConfig(oddsfeed.ProviderConfig{GraphURL: "https://graph.azuro.org"}); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestFetchSnapshotWithoutGraphURL(t *testing.T) {
	p := New("", "")
	_, err := p.FetchSnapshot(context.Background(), "", nil)
	if err == nil {
		t.Fatal("expected error when graph URL not configured")
	}
}

func TestFetchSnapshotEmptyWhenConfigured(t *testing.T) {
	p := New("https://graph.azuro.org", "")
	snap, err := p.FetchSnapshot(context.Background(), "", nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if snap.Provider != ProviderName {
		t.Fatalf("expected provider %s, got %s", ProviderName, snap.Provider)
	}
}

func TestSubscribeLiveWithoutWSURL(t *testing.T) {
	p := New("https://graph.azuro.org", "")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := p.SubscribeLive(ctx, "", make(chan oddsfeed.Update)); err == nil {
		t.Fatal("expected error when websocket URL not configured")
	}
}

package oddsfeed_test

import (
	"context"
	"testing"
	"time"

	"github.com/realyoussefhossam/betmonster/internal/oddsfeed"
	"github.com/realyoussefhossam/betmonster/internal/oddsfeed/providers/mock"
)

func TestMockProviderFetchSnapshot(t *testing.T) {
	p := mock.New()
	if p.Name() != "mock" { t.Fatalf("expected name mock, got %s", p.Name()) }
	ctx := context.Background()
	snap, err := p.FetchSnapshot(ctx, "soccer", nil)
	if err != nil { t.Fatalf("fetch snapshot: %v", err) }
	if len(snap.Events) == 0 { t.Fatal("expected events") }
	if len(snap.Outcomes) != 3 { t.Fatalf("expected 3 outcomes, got %d", len(snap.Outcomes)) }
}

func TestMockProviderSubscribeLive(t *testing.T) {
	p := mock.New()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	updates := make(chan oddsfeed.Update, 10)
	go func() {
		if err := p.SubscribeLive(ctx, "soccer", updates); err != nil && err != context.Canceled {
			t.Errorf("subscribe live: %v", err)
		}
	}()

	select {
	case u := <-updates:
		if u.Provider != "mock" { t.Fatalf("expected provider mock, got %s", u.Provider) }
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for update")
	}

	cancel()
}

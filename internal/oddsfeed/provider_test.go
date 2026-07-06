package oddsfeed_test

import (
	"context"
	"testing"

	"github.com/realyoussefhossam/betmonster/internal/oddsfeed/providers/mock"
)

func TestMockProviderFetchSnapshot(t *testing.T) {
	p := mock.New()
	if p.Name() != "mock" { t.Fatalf("expected name mock, got %s", p.Name()) }
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	snap, err := p.FetchSnapshot(ctx, "soccer", nil)
	if err != nil { t.Fatalf("fetch snapshot: %v", err) }
	if len(snap.Events) == 0 { t.Fatal("expected events") }
	if len(snap.Outcomes) != 3 { t.Fatalf("expected 3 outcomes, got %d", len(snap.Outcomes)) }
}

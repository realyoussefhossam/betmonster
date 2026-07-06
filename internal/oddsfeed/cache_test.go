package oddsfeed

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
)

func TestCacheLiveOddsRoundTrip(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()
	cache := NewCache(s.Addr(), 0)
	defer cache.Close()

	ctx := context.Background()
	if err := cache.SetLiveOdds(ctx, "mk-1", map[string]string{"oc-1": "2.00", "oc-2": "3.50"}); err != nil {
		t.Fatalf("set live odds: %v", err)
	}
	got, err := cache.GetLiveOdds(ctx, "mk-1")
	if err != nil {
		t.Fatalf("get live odds: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 outcomes, got %d: %v", len(got), got)
	}
	if got["oc-1"] != "2.00" || got["oc-2"] != "3.50" {
		t.Fatalf("unexpected odds: %v", got)
	}
}

func TestCacheLiveScoreRoundTrip(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()
	cache := NewCache(s.Addr(), 0)
	defer cache.Close()

	ctx := context.Background()
	if err := cache.SetLiveScore(ctx, "ev-1", "2", "1", "live"); err != nil {
		t.Fatalf("set live score: %v", err)
	}
	got, err := cache.GetLiveScore(ctx, "ev-1")
	if err != nil {
		t.Fatalf("get live score: %v", err)
	}
	if got["home_score"] != "2" || got["away_score"] != "1" || got["status"] != "live" {
		t.Fatalf("unexpected score: %v", got)
	}
}

func TestCacheSetLiveEventIDs(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()
	cache := NewCache(s.Addr(), 0)
	defer cache.Close()

	ctx := context.Background()
	if err := cache.SetLiveEventIDs(ctx, "sp-1", []string{"ev-1", "ev-2", "ev-3"}); err != nil {
		t.Fatalf("set live event ids: %v", err)
	}
	members, err := s.SMembers("oddsfeed:live:events:sp-1")
	if err != nil {
		t.Fatalf("smembers: %v", err)
	}
	if len(members) != 3 {
		t.Fatalf("expected 3 event ids, got %d", len(members))
	}
}

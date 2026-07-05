package rates

import (
	"testing"
	"time"
)

func TestCacheGetSet(t *testing.T) {
	c := NewCache(1 * time.Second)
	c.Set("BTC:USD", "60000")
	got, ok := c.Get("BTC:USD")
	if !ok || got != "60000" {
		t.Fatalf("expected 60000, got %s, ok=%v", got, ok)
	}
}

func TestCacheExpiry(t *testing.T) {
	c := NewCache(1 * time.Nanosecond)
	c.Set("BTC:USD", "60000")
	time.Sleep(5 * time.Millisecond)
	_, ok := c.Get("BTC:USD")
	if ok {
		t.Fatalf("expected cache entry to expire")
	}
}

func TestCacheStaleValue(t *testing.T) {
	c := NewCache(1 * time.Nanosecond)
	c.Set("BTC:USD", "60000")
	time.Sleep(5 * time.Millisecond)
	got, ok := c.StaleValue("BTC:USD")
	if !ok || got != "60000" {
		t.Fatalf("expected stale value, got %s, ok=%v", got, ok)
	}
}

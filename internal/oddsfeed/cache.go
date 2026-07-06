package oddsfeed

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

// Cache stores live odds, scores, and event IDs in Redis with a default TTL.
type Cache struct {
	client *redis.Client
	ttl    time.Duration
}

// NewCache creates a Redis-backed cache. If ttl is <= 0, it defaults to 60 seconds.
func NewCache(addr string, ttl time.Duration) *Cache {
	if ttl <= 0 {
		ttl = 60 * time.Second
	}
	return &Cache{client: redis.NewClient(&redis.Options{Addr: addr}), ttl: ttl}
}

// SetLiveOdds stores outcome odds for a market as a Redis hash with TTL.
func (c *Cache) SetLiveOdds(ctx context.Context, marketID string, odds map[string]string) error {
	key := fmt.Sprintf("oddsfeed:live:odds:%s", marketID)
	pipe := c.client.Pipeline()
	pipe.HSet(ctx, key, odds)
	pipe.Expire(ctx, key, c.ttl)
	_, err := pipe.Exec(ctx)
	return err
}

// SetLiveScore stores the current score and status for an event as a Redis hash with TTL.
func (c *Cache) SetLiveScore(ctx context.Context, eventID, home, away, status string) error {
	key := fmt.Sprintf("oddsfeed:live:score:%s", eventID)
	pipe := c.client.Pipeline()
	pipe.HSet(ctx, key, map[string]string{"home_score": home, "away_score": away, "status": status})
	pipe.Expire(ctx, key, c.ttl)
	_, err := pipe.Exec(ctx)
	return err
}

// SetLiveEventIDs replaces the set of live event IDs for a sport, applying TTL.
func (c *Cache) SetLiveEventIDs(ctx context.Context, sportID string, ids []string) error {
	key := fmt.Sprintf("oddsfeed:live:events:%s", sportID)
	pipe := c.client.Pipeline()
	pipe.Del(ctx, key)
	for _, id := range ids {
		pipe.SAdd(ctx, key, id)
	}
	pipe.Expire(ctx, key, c.ttl)
	_, err := pipe.Exec(ctx)
	return err
}

// GetLiveOdds returns the live odds hash for a market.
func (c *Cache) GetLiveOdds(ctx context.Context, marketID string) (map[string]string, error) {
	return c.client.HGetAll(ctx, fmt.Sprintf("oddsfeed:live:odds:%s", marketID)).Result()
}

// GetLiveScore returns the live score hash for an event.
func (c *Cache) GetLiveScore(ctx context.Context, eventID string) (map[string]string, error) {
	return c.client.HGetAll(ctx, fmt.Sprintf("oddsfeed:live:score:%s", eventID)).Result()
}

// Close closes the Redis client connection.
func (c *Cache) Close() error {
	return c.client.Close()
}

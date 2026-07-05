package rates

import (
	"sync"
	"time"
)

type cachedRate struct {
	value  string
	cached time.Time
}

type Cache struct {
	mu     sync.RWMutex
	values map[string]cachedRate
	ttl    time.Duration
}

func NewCache(ttl time.Duration) *Cache {
	return &Cache{
		values: make(map[string]cachedRate),
		ttl:    ttl,
	}
}

func (c *Cache) Get(key string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.values[key]
	if !ok {
		return "", false
	}
	if time.Since(v.cached) > c.ttl {
		return "", false
	}
	return v.value, true
}

func (c *Cache) Set(key, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.values[key] = cachedRate{value: value, cached: time.Now()}
}

func (c *Cache) StaleValue(key string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.values[key]
	if !ok {
		return "", false
	}
	if time.Since(v.cached) > 5*time.Minute {
		return "", false
	}
	return v.value, true
}

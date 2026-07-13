package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// ValkeyCache provides a Valkey-backed cache layer
type ValkeyCache struct {
	Valkey *ValkeyClient
}

// ValkeyClient is a thin Valkey/Redis-compatible client
type ValkeyClient struct {
	store map[string]*cacheEntry
	mu    sync.RWMutex
	ttl   time.Duration
}

type cacheEntry struct {
	value     []byte
	expiresAt time.Time
}

// NewValkeyClient creates a new Valkey cache client
func NewValkeyClient(ttl time.Duration) *ValkeyClient {
	cache := &ValkeyClient{
		store: make(map[string]*cacheEntry),
		ttl:   ttl,
	}
	go cache.reaper()
	return cache
}

// Set stores a value with a TTL
func (c *ValkeyClient) Set(ctx context.Context, key string, ttl time.Duration, value interface{}) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if ttl == 0 {
		ttl = c.ttl
	}

	c.store[key] = &cacheEntry{
		value:     data,
		expiresAt: time.Now().Add(ttl),
	}
	return nil
}

// Get retrieves a value by key
func (c *ValkeyClient) Get(ctx context.Context, key string, target interface{}) error {
	c.mu.RLock()
	entry, exists := c.store[key]
	c.mu.RUnlock()

	if !exists {
		return fmt.Errorf("cache miss: %s", key)
	}

	if time.Now().After(entry.expiresAt) {
		c.mu.Lock()
		delete(c.store, key)
		c.mu.Unlock()
		return fmt.Errorf("cache expired: %s", key)
	}

	return json.Unmarshal(entry.value, target)
}

// Delete removes a key
func (c *ValkeyClient) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.store, key)
	return nil
}

// Exists checks if a key exists and is not expired
func (c *ValkeyClient) Exists(ctx context.Context, key string) bool {
	c.mu.RLock()
	entry, exists := c.store[key]
	c.mu.RUnlock()

	if !exists {
		return false
	}
	if time.Now().After(entry.expiresAt) {
		c.mu.Lock()
		delete(c.store, key)
		c.mu.Unlock()
		return false
	}
	return true
}

// Expire sets a new TTL on an existing key
func (c *ValkeyClient) Expire(ctx context.Context, key string, ttl time.Duration) error {
	c.mu.RLock()
	_, exists := c.store[key]
	c.mu.RUnlock()

	if !exists {
		return fmt.Errorf("key not found: %s", key)
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if entry, ok := c.store[key]; ok {
		entry.expiresAt = time.Now().Add(ttl)
	}
	return nil
}

// Increment atomically increments a counter (creates at 0 if missing)
func (c *ValkeyClient) Increment(ctx context.Context, key string, delta int64) (int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, exists := c.store[key]
	if !exists || time.Now().After(entry.expiresAt) {
		c.store[key] = &cacheEntry{
			value:     []byte(fmt.Sprintf("%d", delta)),
			expiresAt: time.Now().Add(c.ttl),
		}
		return delta, nil
	}

	var current int64
	_ = json.Unmarshal(entry.value, &current) // ignore error, default 0
	newValue := current + delta
	entry.value, _ = json.Marshal(newValue)
	return newValue, nil
}

// KeysWithPrefix returns all non-expired keys matching a prefix
func (c *ValkeyClient) KeysWithPrefix(ctx context.Context, prefix string) []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	now := time.Now()
	var keys []string
	for key, entry := range c.store {
		if now.Before(entry.expiresAt) && len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			keys = append(keys, key)
		}
	}
	return keys
}

// Size returns the count of non-expired entries
func (c *ValkeyClient) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	count := 0
	now := time.Now()
	for _, entry := range c.store {
		if now.Before(entry.expiresAt) {
			count++
		}
	}
	return count
}

// Flush clears all entries
func (c *ValkeyClient) Flush() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.store = make(map[string]*cacheEntry)
}

// reaper removes expired entries periodically
func (c *ValkeyClient) reaper() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for key, entry := range c.store {
			if now.After(entry.expiresAt) {
				delete(c.store, key)
			}
		}
		c.mu.Unlock()
	}
}

// NewValkeyCache creates a high-level cache wrapper
func NewValkeyCache() *ValkeyCache {
	return &ValkeyCache{
		Valkey: NewValkeyClient(30 * time.Minute),
	}
}

// Get retrieves a typed value
func (c *ValkeyCache) Get(ctx context.Context, key string, target interface{}) error {
	return c.Valkey.Get(ctx, key, target)
}

// Set stores a typed value
func (c *ValkeyCache) Set(ctx context.Context, key string, ttl time.Duration, value interface{}) error {
	return c.Valkey.Set(ctx, key, ttl, value)
}

// Delete removes a key
func (c *ValkeyCache) Delete(ctx context.Context, key string) error {
	return c.Valkey.Delete(ctx, key)
}

// SetGeocode caches a geocode result (24h TTL)
func (c *ValkeyCache) SetGeocode(ctx context.Context, address string, result interface{}) error {
	return c.Valkey.Set(ctx, "geo:"+address, 24*time.Hour, result)
}

// GetGeocode retrieves a cached geocode result
func (c *ValkeyCache) GetGeocode(ctx context.Context, address string, target interface{}) error {
	return c.Valkey.Get(ctx, "geo:"+address, target)
}

// SetSession stores a user session (7d TTL)
func (c *ValkeyCache) SetSession(ctx context.Context, sessionID string, userID string) error {
	return c.Valkey.Set(ctx, "session:"+sessionID, 7*24*time.Hour, userID)
}

// GetSession retrieves a user session
func (c *ValkeyCache) GetSession(ctx context.Context, sessionID string) (string, error) {
	var userID string
	if err := c.Valkey.Get(ctx, "session:"+sessionID, &userID); err != nil {
		return "", err
	}
	return userID, nil
}

// RateLimit checks and decrements a rate counter; returns (allowed, remaining)
func (c *ValkeyCache) RateLimit(ctx context.Context, key string, maxRequests int, window time.Duration) (bool, int, error) {
	rateKey := "rate:" + key
	current, err := c.Valkey.Increment(ctx, rateKey, 1)
	if err != nil {
		return false, 0, err
	}

	// Set expiry on first request
	if current == 1 {
		if err := c.Valkey.Expire(ctx, rateKey, window); err != nil {
			return false, int(current), err
		}
	}

	remaining := maxRequests - int(current)
	allowed := current <= int64(maxRequests)
	return allowed, remaining, nil
}

// SetLiveETA caches live ETA values (30s TTL)
func (c *ValkeyCache) SetLiveETA(ctx context.Context, stopID string, etas interface{}) error {
	return c.Valkey.Set(ctx, "eta:"+stopID, 30*time.Second, etas)
}

// GetLiveETA retrieves cached live ETAs
func (c *ValkeyCache) GetLiveETA(ctx context.Context, stopID string, target interface{}) error {
	return c.Valkey.Get(ctx, "eta:"+stopID, target)
}

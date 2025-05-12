package utils

import (
	"sync"
	"time"
)

type CacheItem struct {
	Value      any
	Expiration time.Time
}

type TenderdutyCache struct {
	data sync.Map
}

// NewCache creates a new Cache instance.
func NewCache() *TenderdutyCache {
	return &TenderdutyCache{}
}

// Set adds a value to the cache with an optional expiration duration.
func (c *TenderdutyCache) Set(key string, value any, ttl time.Duration) {
	expiration := time.Time{}
	if ttl > 0 {
		expiration = time.Now().Add(ttl)
	}
	c.data.Store(key, CacheItem{Value: value, Expiration: expiration})
}

// Get retrieves a value from the cache if it exists and is not expired.
func (c *TenderdutyCache) Get(key string) (any, bool) {
	item, ok := c.data.Load(key)
	if !ok {
		return nil, false
	}

	cacheItem := item.(CacheItem)
	if !cacheItem.Expiration.IsZero() && cacheItem.Expiration.Before(time.Now()) {
		c.data.Delete(key) // Clean up expired entry
		return nil, false
	}

	return cacheItem.Value, true
}

// Delete removes a value from the cache.
func (c *TenderdutyCache) Delete(key string) {
	c.data.Delete(key)
}

// Cleanup removes all expired items from the cache.
func (c *TenderdutyCache) Cleanup() {
	c.data.Range(func(key, value any) bool {
		cacheItem := value.(CacheItem)
		if !cacheItem.Expiration.IsZero() && cacheItem.Expiration.Before(time.Now()) {
			c.data.Delete(key)
		}
		return true
	})
}

// Size returns the number of active (non-expired) items in the cache.
func (c *TenderdutyCache) Size() int {
	count := 0
	c.data.Range(func(_, value any) bool {
		cacheItem := value.(CacheItem)
		if cacheItem.Expiration.IsZero() || cacheItem.Expiration.After(time.Now()) {
			count++
		}
		return true
	})
	return count
}

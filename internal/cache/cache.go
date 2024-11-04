// internal/cache/cache.go

package cache

import (
	"sync"
	"time"
)

// Cache represents a thread-safe in-memory cache.
type Cache struct {
	data  map[string]string
	mutex sync.RWMutex
}

// NewCache initializes and returns a new Cache instance.
func NewCache() *Cache {
	return &Cache{
		data: make(map[string]string),
	}
}

// Get retrieves the value associated with the given key.
func (c *Cache) Get(key string) (string, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	val, exists := c.data[key]
	return val, exists
}

// Set assigns a value to the given key in the cache.
func (c *Cache) Set(key, value string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.data[key] = value
}

// StartEviction periodically removes expired entries from the cache.
// Currently, it deletes all entries at each interval.
// Implement TTL checks or other eviction policies as needed.
func (c *Cache) StartEviction(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	go func() {
		for range ticker.C {
			c.mutex.Lock()
			for key := range c.data {
				// TODO: Implement TTL checks or other eviction policies
				// Example: delete all entries (replace with actual logic)
				delete(c.data, key)
			}
			c.mutex.Unlock()
		}
	}()
}

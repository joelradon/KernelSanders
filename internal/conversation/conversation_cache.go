// internal/conversation/conversation_cache.go

package conversation

import (
	"sync"
	"time"
)

// ConversationCache manages conversation contexts with expiration.
type ConversationCache struct {
	data      map[string]conversationEntry
	mutex     sync.RWMutex
	expiry    time.Duration
	cleanupCh chan struct{}
}

// conversationEntry stores conversation data along with the last updated timestamp.
type conversationEntry struct {
	data     string
	lastSeen time.Time
}

// NewConversationCache initializes a new ConversationCache.
func NewConversationCache() *ConversationCache {
	cc := &ConversationCache{
		data:      make(map[string]conversationEntry),
		expiry:    30 * time.Minute, // Context expires after 30 minutes of inactivity
		cleanupCh: make(chan struct{}),
	}
	go cc.cleanupExpiredContexts()
	return cc
}

// Set stores a conversation context with the current timestamp.
func (cc *ConversationCache) Set(key, value string) {
	cc.mutex.Lock()
	defer cc.mutex.Unlock()
	cc.data[key] = conversationEntry{
		data:     value,
		lastSeen: time.Now(),
	}
}

// Get retrieves a conversation context if it's not expired.
func (cc *ConversationCache) Get(key string) (string, bool) {
	cc.mutex.RLock()
	defer cc.mutex.RUnlock()
	entry, exists := cc.data[key]
	if !exists {
		return "", false
	}
	if time.Since(entry.lastSeen) > cc.expiry {
		return "", false
	}
	return entry.data, true
}

// cleanupExpiredContexts periodically removes expired contexts.
func (cc *ConversationCache) cleanupExpiredContexts() {
	ticker := time.NewTicker(cc.expiry)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			cc.mutex.Lock()
			for key, entry := range cc.data {
				if time.Since(entry.lastSeen) > cc.expiry {
					delete(cc.data, key)
				}
			}
			cc.mutex.Unlock()
		case <-cc.cleanupCh:
			return
		}
	}
}

// Close stops the cleanup goroutine.
func (cc *ConversationCache) Close() {
	close(cc.cleanupCh)
}

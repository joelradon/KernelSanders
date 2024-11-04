// internal/app/response_store.go

package app

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// ResponseStore manages stored responses with expiration tracking.
type ResponseStore struct {
	responses map[string]responseEntry
	mutex     sync.RWMutex
}

// responseEntry represents a response's content and expiration time.
type responseEntry struct {
	content   string
	expiresAt time.Time
}

// NewResponseStore initializes the ResponseStore and begins the cleanup routine.
func NewResponseStore() *ResponseStore {
	rs := &ResponseStore{
		responses: make(map[string]responseEntry),
	}
	go rs.cleanupExpiredResponses()
	return rs
}

// StoreResponse stores the response content and returns a unique ID for retrieval.
func (rs *ResponseStore) StoreResponse(content string) string {
	rs.mutex.Lock()
	defer rs.mutex.Unlock()

	id := uuid.New().String()
	rs.responses[id] = responseEntry{
		content:   content,
		expiresAt: time.Now().Add(4 * time.Hour), // Set expiration to 4 hours
	}
	return id
}

// GetResponse retrieves the response content by ID if it hasn't expired.
func (rs *ResponseStore) GetResponse(id string) (string, bool) {
	rs.mutex.RLock()
	defer rs.mutex.RUnlock()

	entry, exists := rs.responses[id]
	if !exists || time.Now().After(entry.expiresAt) {
		return "", false
	}
	return entry.content, true
}

// GetExpirationTime returns the expiration time of a stored response by ID.
func (rs *ResponseStore) GetExpirationTime(id string) (time.Time, bool) {
	rs.mutex.RLock()
	defer rs.mutex.RUnlock()

	entry, exists := rs.responses[id]
	if !exists {
		return time.Time{}, false
	}
	return entry.expiresAt, true
}

// cleanupExpiredResponses periodically removes expired responses from the store.
func (rs *ResponseStore) cleanupExpiredResponses() {
	ticker := time.NewTicker(10 * time.Minute)
	for range ticker.C {
		rs.mutex.Lock()
		for id, entry := range rs.responses {
			if time.Now().After(entry.expiresAt) {
				delete(rs.responses, id)
			}
		}
		rs.mutex.Unlock()
	}
}

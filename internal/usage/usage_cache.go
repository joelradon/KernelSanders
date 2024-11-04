// internal/usage/usage_cache.go

package usage

import (
	"sync"
	"time"
)

// UsageCache tracks user message usage for rate limiting.
type UsageCache struct {
	users    map[int][]time.Time
	mutex    sync.Mutex
	limit    int
	duration time.Duration
}

// NewUsageCache initializes a new UsageCache.
func NewUsageCache() *UsageCache {
	return &UsageCache{
		users:    make(map[int][]time.Time),
		limit:    10,               // Default limit of 10 messages per duration
		duration: 10 * time.Minute, // 10-minute window
	}
}

// CanUserChat checks if a user is allowed to send a message based on usage in the last duration.
func (u *UsageCache) CanUserChat(userID int) bool {
	u.mutex.Lock()
	defer u.mutex.Unlock()

	// Filter out old timestamps.
	validTimes := u.filterRecentMessages(userID)
	u.users[userID] = validTimes

	// Check if user has exceeded the limit.
	return len(validTimes) < u.limit
}

// AddUsage records a new message usage for the user.
func (u *UsageCache) AddUsage(userID int) {
	u.mutex.Lock()
	defer u.mutex.Unlock()

	u.users[userID] = append(u.users[userID], time.Now())
}

// TimeUntilLimitReset calculates the time remaining until the rate limit is lifted.
func (u *UsageCache) TimeUntilLimitReset(userID int) time.Duration {
	u.mutex.Lock()
	defer u.mutex.Unlock()

	validTimes := u.filterRecentMessages(userID)
	if len(validTimes) < u.limit {
		return 0 // No limit currently in place.
	}

	// Calculate time remaining until the oldest timestamp falls outside the duration window.
	oldestTime := validTimes[0]
	return u.duration - time.Since(oldestTime)
}

// filterRecentMessages filters messages within the allowed duration.
func (u *UsageCache) filterRecentMessages(userID int) []time.Time {
	if _, exists := u.users[userID]; !exists {
		u.users[userID] = []time.Time{}
		return u.users[userID]
	}

	validTimes := []time.Time{}
	for _, t := range u.users[userID] {
		if time.Since(t) <= u.duration {
			validTimes = append(validTimes, t)
		}
	}
	return validTimes
}

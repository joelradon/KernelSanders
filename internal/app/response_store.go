// internal/app/response_store.go

package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"
	"time"

	"KernelSandersBot/internal/s3client"
	"KernelSandersBot/internal/types"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/google/uuid"
)

// ResponseStore manages stored responses with expiration tracking and user associations.
type ResponseStore struct {
	responses map[string]responseEntry
	mutex     sync.RWMutex
	s3Client  s3client.S3ClientInterface
	bucket    string
}

// responseEntry represents a response's content, creation time, and expiration time.
type responseEntry struct {
	Content     string    `json:"content"`
	CreatedAt   time.Time `json:"created_at"`
	ExpiresAt   time.Time `json:"expires_at"`
	OwnerUserID int       `json:"owner_user_id"`
}

// NewResponseStore initializes the ResponseStore with S3Client and begins the cleanup routine.
func NewResponseStore(s3Client s3client.S3ClientInterface, bucket string) *ResponseStore {
	rs := &ResponseStore{
		responses: make(map[string]responseEntry),
		s3Client:  s3Client,
		bucket:    bucket,
	}
	go rs.cleanupExpiredResponses()
	return rs
}

// StoreResponseForUser stores the response content associated with a user in both memory and S3.
// Returns a unique ID for the response.
func (rs *ResponseStore) StoreResponseForUser(content string, userID int) string {
	rs.mutex.Lock()
	defer rs.mutex.Unlock()

	id := uuid.New().String()
	now := time.Now()
	entry := responseEntry{
		Content:     content,
		CreatedAt:   now,
		ExpiresAt:   now.Add(types.FileRetentionTime), // Set expiration to 4 hours
		OwnerUserID: userID,
	}
	rs.responses[id] = entry

	// Serialize the entry to JSON
	entryJSON, err := json.Marshal(entry)
	if err != nil {
		log.Printf("Failed to marshal response entry: %v", err)
		return ""
	}

	// Upload to S3
	objectKey := fmt.Sprintf("web_responses/%s.json", id)
	_, err = rs.s3Client.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(rs.bucket),
		Key:    aws.String(objectKey),
		Body:   bytes.NewReader(entryJSON),
	})
	if err != nil {
		log.Printf("Failed to upload response to S3: %v", err)
		return ""
	}

	return id
}

// LoadResponsesFromS3 loads all existing responses from the S3 bucket into the in-memory store.
func (rs *ResponseStore) LoadResponsesFromS3() error {
	rs.mutex.Lock()
	defer rs.mutex.Unlock()

	prefix := "web_responses/"
	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(rs.bucket),
		Prefix: aws.String(prefix),
	}

	err := rs.s3Client.ListObjectsV2Pages(input, func(page *s3.ListObjectsV2Output, lastPage bool) bool {
		for _, obj := range page.Contents {
			// Retrieve each response JSON from S3
			getInput := &s3.GetObjectInput{
				Bucket: aws.String(rs.bucket),
				Key:    obj.Key,
			}
			resp, err := rs.s3Client.GetObject(getInput)
			if err != nil {
				log.Printf("Failed to get object %s from S3: %v", *obj.Key, err)
				continue
			}
			bodyBytes, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				log.Printf("Failed to read object %s from S3: %v", *obj.Key, err)
				continue
			}

			var entry responseEntry
			if err := json.Unmarshal(bodyBytes, &entry); err != nil {
				log.Printf("Failed to unmarshal JSON for object %s: %v", *obj.Key, err)
				continue
			}

			// Check if the response has already expired
			if time.Now().After(entry.ExpiresAt) {
				// Optionally delete expired responses immediately
				rs.DeleteResponse(getResponseIDFromKey(*obj.Key))
				continue
			}

			// Extract the response ID from the object key
			id := getResponseIDFromKey(*obj.Key)

			// Add to in-memory store
			rs.responses[id] = entry
		}
		return true // Continue to next page
	})

	if err != nil {
		log.Printf("Error listing objects in S3: %v", err)
		return err
	}

	log.Println("Successfully loaded responses from S3 into memory.")
	return nil
}

// getResponseIDFromKey extracts the response ID from the S3 object key.
func getResponseIDFromKey(key string) string {
	// Assuming the key format is "web_responses/{id}.json"
	parts := strings.Split(key, "/")
	if len(parts) != 2 {
		return ""
	}
	idWithExt := parts[1]
	id := strings.TrimSuffix(idWithExt, ".json")
	return id
}

// GetResponse retrieves the response content by ID if it hasn't expired.
// It first checks the in-memory store, then attempts to retrieve from S3 if not found.
func (rs *ResponseStore) GetResponse(id string) (string, bool) {
	rs.mutex.RLock()
	entry, exists := rs.responses[id]
	rs.mutex.RUnlock()

	if exists {
		if time.Now().After(entry.ExpiresAt) {
			return "", false
		}
		return entry.Content, true
	}

	// Attempt to retrieve from S3
	objectKey := fmt.Sprintf("web_responses/%s.json", id)
	resp, err := rs.s3Client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(rs.bucket),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		log.Printf("Failed to retrieve response from S3 for ID %s: %v", id, err)
		return "", false
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Failed to read response body from S3 for ID %s: %v", id, err)
		return "", false
	}

	var s3Entry responseEntry
	if err := json.Unmarshal(bodyBytes, &s3Entry); err != nil {
		log.Printf("Failed to unmarshal response JSON from S3 for ID %s: %v", id, err)
		return "", false
	}

	// Check expiration
	if time.Now().After(s3Entry.ExpiresAt) {
		// Optionally, delete the expired response from S3
		rs.DeleteResponse(id)
		return "", false
	}

	// Update in-memory store
	rs.mutex.Lock()
	rs.responses[id] = s3Entry
	rs.mutex.Unlock()

	return s3Entry.Content, true
}

// GetCreationTime returns the creation time of a stored response by ID.
func (rs *ResponseStore) GetCreationTime(id string) (time.Time, bool) {
	rs.mutex.RLock()
	entry, exists := rs.responses[id]
	rs.mutex.RUnlock()

	if exists {
		return entry.CreatedAt, true
	}

	// Attempt to retrieve from S3
	objectKey := fmt.Sprintf("web_responses/%s.json", id)
	resp, err := rs.s3Client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(rs.bucket),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		log.Printf("Failed to retrieve creation time from S3 for ID %s: %v", id, err)
		return time.Time{}, false
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Failed to read creation time body from S3 for ID %s: %v", id, err)
		return time.Time{}, false
	}

	var s3Entry responseEntry
	if err := json.Unmarshal(bodyBytes, &s3Entry); err != nil {
		log.Printf("Failed to unmarshal creation time JSON from S3 for ID %s: %v", id, err)
		return time.Time{}, false
	}

	// Update in-memory store
	rs.mutex.Lock()
	rs.responses[id] = s3Entry
	rs.mutex.Unlock()

	return s3Entry.CreatedAt, true
}

// GetExpirationTime returns the expiration time of a stored response by ID.
func (rs *ResponseStore) GetExpirationTime(id string) (time.Time, bool) {
	rs.mutex.RLock()
	entry, exists := rs.responses[id]
	rs.mutex.RUnlock()

	if exists {
		return entry.ExpiresAt, true
	}

	// Attempt to retrieve from S3
	objectKey := fmt.Sprintf("web_responses/%s.json", id)
	resp, err := rs.s3Client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(rs.bucket),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		log.Printf("Failed to retrieve expiration time from S3 for ID %s: %v", id, err)
		return time.Time{}, false
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Failed to read expiration time body from S3 for ID %s: %v", id, err)
		return time.Time{}, false
	}

	var s3Entry responseEntry
	if err := json.Unmarshal(bodyBytes, &s3Entry); err != nil {
		log.Printf("Failed to unmarshal expiration time JSON from S3 for ID %s: %v", id, err)
		return time.Time{}, false
	}

	// Update in-memory store
	rs.mutex.Lock()
	rs.responses[id] = s3Entry
	rs.mutex.Unlock()

	return s3Entry.ExpiresAt, true
}

// GetUserResponsesByUserID retrieves all responses associated with a user from both memory and S3.
func (rs *ResponseStore) GetUserResponsesByUserID(userID int) ([]types.UserResponse, error) {
	rs.mutex.RLock()
	defer rs.mutex.RUnlock()

	var responses []types.UserResponse
	for id, entry := range rs.responses {
		if entry.OwnerUserID == userID && time.Now().Before(entry.ExpiresAt) {
			responses = append(responses, types.UserResponse{
				ID:              id,
				CreatedAtUTC:    entry.CreatedAt.UTC(),
				CreatedAtEDT:    entry.CreatedAt.In(time.FixedZone("EDT", -4*3600)),
				DeletionTimeUTC: entry.ExpiresAt.UTC(),
				DeletionTimeEDT: entry.ExpiresAt.In(time.FixedZone("EDT", -4*3600)),
			})
		}
	}

	// Optionally, retrieve from S3 if not present in memory
	// Implementation depends on how responses are indexed and stored in S3
	// For simplicity, this example assumes all relevant responses are loaded into memory

	return responses, nil
}

// DeleteResponse removes a response from both memory and S3.
func (rs *ResponseStore) DeleteResponse(id string) {
	rs.mutex.Lock()
	defer rs.mutex.Unlock()

	delete(rs.responses, id)

	// Delete from S3
	objectKey := fmt.Sprintf("web_responses/%s.json", id)
	_, err := rs.s3Client.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(rs.bucket),
		Key:    aws.String(objectKey),
	})

	if err != nil {
		log.Printf("Failed to delete response from S3 for ID %s: %v", id, err)
	} else {
		log.Printf("Successfully deleted response ID %s from S3.", id)
	}
}

// cleanupExpiredResponses periodically removes expired responses from both memory and S3.
func (rs *ResponseStore) cleanupExpiredResponses() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		rs.mutex.Lock()
		for id, entry := range rs.responses {
			if time.Now().After(entry.ExpiresAt) {
				delete(rs.responses, id)

				// Delete from S3
				objectKey := fmt.Sprintf("web_responses/%s.json", id)
				_, err := rs.s3Client.DeleteObject(&s3.DeleteObjectInput{
					Bucket: aws.String(rs.bucket),
					Key:    aws.String(objectKey),
				})
				if err != nil {
					log.Printf("Failed to delete expired response from S3 for ID %s: %v", id, err)
				} else {
					log.Printf("Successfully deleted expired response ID %s from S3.", id)
				}
			}
		}
		rs.mutex.Unlock()
	}
}

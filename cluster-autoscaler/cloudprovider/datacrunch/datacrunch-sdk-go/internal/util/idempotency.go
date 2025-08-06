package util

import (
	"context"
	"crypto/md5"
	"fmt"
	"sync"
	"time"
)

// IdempotencyKey represents a unique key for idempotent operations
type IdempotencyKey string

// IdempotencyResult represents the result of an idempotent operation
type IdempotencyResult struct {
	Value     interface{}
	Error     error
	CreatedAt time.Time
}

// IdempotencyManager manages idempotent operations to prevent duplicate requests
type IdempotencyManager struct {
	cache           map[IdempotencyKey]*IdempotencyResult
	mutex           sync.RWMutex
	ttl             time.Duration
	maxSize         int
	cleanupInterval time.Duration
	stopCleanup     chan struct{}
	cleanupOnce     sync.Once
}

// IdempotencyConfig holds configuration for the idempotency manager
type IdempotencyConfig struct {
	TTL             time.Duration // How long to keep results cached
	MaxSize         int           // Maximum number of cached results
	CleanupInterval time.Duration // How often to clean up expired entries
}

// NewIdempotencyManager creates a new idempotency manager
func NewIdempotencyManager(config *IdempotencyConfig) *IdempotencyManager {
	if config == nil {
		config = &IdempotencyConfig{
			TTL:             5 * time.Minute,
			MaxSize:         1000,
			CleanupInterval: 1 * time.Minute,
		}
	}

	// Set defaults
	if config.TTL == 0 {
		config.TTL = 5 * time.Minute
	}
	if config.MaxSize == 0 {
		config.MaxSize = 1000
	}
	if config.CleanupInterval == 0 {
		config.CleanupInterval = 1 * time.Minute
	}

	manager := &IdempotencyManager{
		cache:           make(map[IdempotencyKey]*IdempotencyResult),
		ttl:             config.TTL,
		maxSize:         config.MaxSize,
		cleanupInterval: config.CleanupInterval,
		stopCleanup:     make(chan struct{}),
	}

	// Start cleanup goroutine
	manager.startCleanup()

	return manager
}

// GenerateKey generates an idempotency key from the given parameters
func (im *IdempotencyManager) GenerateKey(operation string, params ...interface{}) IdempotencyKey {
	hash := md5.New()
	hash.Write([]byte(operation))

	for _, param := range params {
		if _, err := fmt.Fprintf(hash, "%v", param); err != nil {
			// Hash write errors are rare and would indicate a serious problem
			// We'll continue anyway as this is just for idempotency
			_ = err // Suppress unused variable warning
		}
	}

	return IdempotencyKey(fmt.Sprintf("%x", hash.Sum(nil)))
}

// Execute executes an operation with idempotency protection
func (im *IdempotencyManager) Execute(ctx context.Context, key IdempotencyKey, operation func() (interface{}, error)) (interface{}, error) {
	// Check if we already have a result for this key
	if result := im.get(key); result != nil {
		return result.Value, result.Error
	}

	// Execute the operation
	value, err := operation()

	// Store the result
	result := &IdempotencyResult{
		Value:     value,
		Error:     err,
		CreatedAt: time.Now(),
	}

	im.set(key, result)

	return value, err
}

// InvalidateKey removes a result from the cache
func (im *IdempotencyManager) InvalidateKey(key IdempotencyKey) {
	im.mutex.Lock()
	defer im.mutex.Unlock()

	delete(im.cache, key)
}

// Clear removes all cached results
func (im *IdempotencyManager) Clear() {
	im.mutex.Lock()
	defer im.mutex.Unlock()

	im.cache = make(map[IdempotencyKey]*IdempotencyResult)
}

// Size returns the number of cached results
func (im *IdempotencyManager) Size() int {
	im.mutex.RLock()
	defer im.mutex.RUnlock()

	return len(im.cache)
}

// Close stops the cleanup goroutine and clears the cache
func (im *IdempotencyManager) Close() {
	im.cleanupOnce.Do(func() {
		close(im.stopCleanup)
	})
	im.Clear()
}

// get retrieves a result from the cache if it exists and is not expired
func (im *IdempotencyManager) get(key IdempotencyKey) *IdempotencyResult {
	im.mutex.RLock()
	defer im.mutex.RUnlock()

	result, exists := im.cache[key]
	if !exists {
		return nil
	}

	// Check if expired
	if time.Now().After(result.CreatedAt.Add(im.ttl)) {
		return nil
	}

	return result
}

// set stores a result in the cache
func (im *IdempotencyManager) set(key IdempotencyKey, result *IdempotencyResult) {
	im.mutex.Lock()
	defer im.mutex.Unlock()

	// Check if we need to make room
	if len(im.cache) >= im.maxSize {
		// Remove oldest entry
		var oldestKey IdempotencyKey
		var oldestTime time.Time

		for k, v := range im.cache {
			if oldestTime.IsZero() || v.CreatedAt.Before(oldestTime) {
				oldestKey = k
				oldestTime = v.CreatedAt
			}
		}

		if oldestKey != "" {
			delete(im.cache, oldestKey)
		}
	}

	im.cache[key] = result
}

// startCleanup starts the cleanup goroutine
func (im *IdempotencyManager) startCleanup() {
	go func() {
		ticker := time.NewTicker(im.cleanupInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				im.cleanup()
			case <-im.stopCleanup:
				return
			}
		}
	}()
}

// cleanup removes expired entries from the cache
func (im *IdempotencyManager) cleanup() {
	im.mutex.Lock()
	defer im.mutex.Unlock()

	now := time.Now()
	var keysToDelete []IdempotencyKey

	for key, result := range im.cache {
		if now.After(result.CreatedAt.Add(im.ttl)) {
			keysToDelete = append(keysToDelete, key)
		}
	}

	for _, key := range keysToDelete {
		delete(im.cache, key)
	}
}

// DefaultIdempotencyManager is a global instance for convenience
var DefaultIdempotencyManager = NewIdempotencyManager(nil)

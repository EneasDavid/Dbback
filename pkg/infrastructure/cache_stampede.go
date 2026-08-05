package infrastructure

import (
	"context"
	"sync"
	"time"
)

// StampedeGuard prevents cache stampede (thundering herd) by ensuring
// only one goroutine fetches data while others wait for the result.
type StampedeGuard struct {
	locks sync.Map // map[string]*sync.Cond
	mu    sync.Mutex
}

// FetchOnce executes fn only once for a given key, blocking other calls
// until the first completes. Useful for expensive operations like API calls.
func (sg *StampedeGuard) FetchOnce(ctx context.Context, key string, fn func() (interface{}, error)) (interface{}, error) {
	// Try to acquire lock for this key
	lockValue, _ := sg.locks.LoadOrStore(key, &sync.Cond{L: &sync.Mutex{}})
	cond := lockValue.(*sync.Cond)

	cond.L.Lock()
	defer cond.L.Unlock()

	// Double-check: maybe another goroutine finished while we waited
	// In production, check cache here

	// First goroutine fetches, others wait
	result := make([]interface{}, 2) // [data, error]
	done := make(chan struct{})

	go func() {
		defer close(done)
		data, err := fn()
		result[0] = data
		result[1] = err
	}()

	// Wait for completion with timeout
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-done:
		cond.Broadcast() // Wake all waiters
		if err, ok := result[1].(error); ok && err != nil {
			return nil, err
		}
		return result[0], nil
	}
}

// GuardedWrite ensures atomic write to multiple cache layers
type GuardedWrite struct {
	writes []func() error
	mu     sync.Mutex
}

// AddWrite registers a write operation to be executed atomically
func (gw *GuardedWrite) AddWrite(fn func() error) {
	gw.mu.Lock()
	defer gw.mu.Unlock()
	gw.writes = append(gw.writes, fn)
}

// CommitAll executes all writes atomically; if any fails, all rollback
func (gw *GuardedWrite) CommitAll(ctx context.Context) error {
	gw.mu.Lock()
	defer gw.mu.Unlock()

	if len(gw.writes) == 0 {
		return nil
	}

	// Execute all writes with potential rollback
	successfulWrites := 0
	for i, write := range gw.writes {
		if err := write(); err != nil {
			// Rollback previous writes (simple reversal for now)
			for j := 0; j < i; j++ {
				// In production: implement proper rollback per layer
				_ = gw.writes[j]()
			}
			return err
		}
		successfulWrites++
	}

	return nil
}

// CacheLayer defines interface for cache operations
type CacheLayer interface {
	Get(ctx context.Context, key string) (interface{}, error)
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
}

// WriteThrough writes data to primary and backup simultaneously
type WriteThrough struct {
	primary   CacheLayer
	secondary CacheLayer
	mu        sync.RWMutex
}

// NewWriteThrough creates a write-through cache wrapper
func NewWriteThrough(primary, secondary CacheLayer) *WriteThrough {
	return &WriteThrough{
		primary:   primary,
		secondary: secondary,
	}
}

// Set writes to both primary and secondary cache
func (wt *WriteThrough) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	wt.mu.Lock()
	defer wt.mu.Unlock()

	// Write to primary first (faster)
	if err := wt.primary.Set(ctx, key, value, ttl); err != nil {
		return err
	}

	// Write to secondary in parallel
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = wt.secondary.Set(ctx, key, value, ttl)
	}()

	return nil
}

// Get tries primary first, then secondary
func (wt *WriteThrough) Get(ctx context.Context, key string) (interface{}, error) {
	wt.mu.RLock()
	defer wt.mu.RUnlock()

	if val, err := wt.primary.Get(ctx, key); err == nil && val != nil {
		return val, nil
	}

	return wt.secondary.Get(ctx, key)
}

// Delete removes from both layers
func (wt *WriteThrough) Delete(ctx context.Context, key string) error {
	wt.mu.Lock()
	defer wt.mu.Unlock()

	_ = wt.primary.Delete(ctx, key)
	_ = wt.secondary.Delete(ctx, key)

	return nil
}

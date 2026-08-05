package infrastructure

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"
)

// RetryPolicy defines retry behavior
type RetryPolicy struct {
	MaxAttempts      int           // Max number of attempts
	InitialDelay     time.Duration // Initial backoff delay
	MaxDelay         time.Duration // Maximum backoff delay
	BackoffMultiplier float64      // Multiplier for exponential backoff
	JitterFraction   float64       // Jitter as fraction of delay (0-1)
	IsRetryable      func(error) bool
}

// DefaultRetryPolicy provides sensible defaults
func DefaultRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxAttempts:       5,
		InitialDelay:      100 * time.Millisecond,
		MaxDelay:          30 * time.Second,
		BackoffMultiplier: 2.0,
		JitterFraction:    0.1,
		IsRetryable: func(err error) bool {
			// By default, retry on any error in production
			return err != nil
		},
	}
}

// RetryResult contains result of retry attempt
type RetryResult struct {
	Attempt      int
	Error        error
	LastError    error
	DelayBefore  time.Duration
	TotalDuration time.Duration
}

// RetryManager executes operations with exponential backoff
type RetryManager struct {
	policy *RetryPolicy
}

// NewRetryManager creates a new retry manager
func NewRetryManager(policy *RetryPolicy) *RetryManager {
	if policy == nil {
		policy = DefaultRetryPolicy()
	}
	return &RetryManager{policy: policy}
}

// Do executes function with retries
func (rm *RetryManager) Do(ctx context.Context, fn func() error) *RetryResult {
	result := &RetryResult{}
	startTime := time.Now()

	for attempt := 1; attempt <= rm.policy.MaxAttempts; attempt++ {
		result.Attempt = attempt

		// Execute function
		err := fn()
		result.Error = err
		result.LastError = err

		if err == nil {
			// Success
			result.TotalDuration = time.Since(startTime)
			return result
		}

		// Check if error is retryable
		if rm.policy.IsRetryable != nil && !rm.policy.IsRetryable(err) {
			result.TotalDuration = time.Since(startTime)
			return result
		}

		// Don't retry after last attempt
		if attempt >= rm.policy.MaxAttempts {
			result.TotalDuration = time.Since(startTime)
			return result
		}

		// Calculate backoff with exponential growth and jitter
		delay := rm.calculateBackoff(attempt)
		result.DelayBefore = delay

		// Wait with context
		select {
		case <-ctx.Done():
			result.Error = ctx.Err()
			result.TotalDuration = time.Since(startTime)
			return result
		case <-time.After(delay):
			// Continue to next attempt
		}
	}

	result.TotalDuration = time.Since(startTime)
	return result
}

// DoAsync executes function asynchronously with retries
func (rm *RetryManager) DoAsync(ctx context.Context, fn func() error) <-chan *RetryResult {
	resultChan := make(chan *RetryResult, 1)

	go func() {
		resultChan <- rm.Do(ctx, fn)
		close(resultChan)
	}()

	return resultChan
}

// calculateBackoff computes exponential backoff with jitter
func (rm *RetryManager) calculateBackoff(attempt int) time.Duration {
	// Exponential: initialDelay * (multiplier ^ (attempt - 1))
	exponentialDelay := float64(rm.policy.InitialDelay) * math.Pow(rm.policy.BackoffMultiplier, float64(attempt-1))

	// Cap at max delay
	delay := time.Duration(math.Min(exponentialDelay, float64(rm.policy.MaxDelay)))

	// Add jitter
	if rm.policy.JitterFraction > 0 {
		jitterAmount := float64(delay) * rm.policy.JitterFraction
		jitter := time.Duration(rand.Float64() * jitterAmount)
		delay += jitter
	}

	return delay
}

// RetryableError wraps an error to indicate it's retryable
type RetryableError struct {
	Err           error
	Retryable     bool
	NextRetryAt   time.Time
	AttemptNumber int
}

func (re *RetryableError) Error() string {
	return fmt.Sprintf("retryable error (attempt %d): %v", re.AttemptNumber, re.Err)
}

// RetryQueue manages multiple concurrent retries
type RetryQueue struct {
	queue    chan func()
	workers  int
	done     chan struct{}
	active   int
	mu       sync.Mutex
}

// NewRetryQueue creates a queue with worker goroutines
func NewRetryQueue(workers int) *RetryQueue {
	rq := &RetryQueue{
		queue:   make(chan func(), workers*10),
		workers: workers,
		done:    make(chan struct{}),
	}

	// Start workers
	for i := 0; i < workers; i++ {
		go rq.worker()
	}

	return rq
}

// Enqueue adds a function to retry queue
func (rq *RetryQueue) Enqueue(fn func()) {
	rq.mu.Lock()
	rq.active++
	rq.mu.Unlock()

	rq.queue <- fn
}

// worker processes queue items
func (rq *RetryQueue) worker() {
	for fn := range rq.queue {
		fn()
		rq.mu.Lock()
		rq.active--
		rq.mu.Unlock()
	}
}

// Wait blocks until all items processed
func (rq *RetryQueue) Wait() {
	for {
		rq.mu.Lock()
		if rq.active == 0 && len(rq.queue) == 0 {
			rq.mu.Unlock()
			break
		}
		rq.mu.Unlock()
		time.Sleep(10 * time.Millisecond)
	}
}

// Stop closes the queue
func (rq *RetryQueue) Stop() {
	close(rq.queue)
	close(rq.done)
}

// Fallback provides alternative data when primary fails
type Fallback struct {
	primary     func(context.Context) (interface{}, error)
	secondary   func(context.Context) (interface{}, error)
	maxStaleness time.Duration
	lastSuccess time.Time
	cachedData   interface{}
	mu           sync.RWMutex
}

// NewFallback creates a fallback strategy
func NewFallback(
	primary func(context.Context) (interface{}, error),
	secondary func(context.Context) (interface{}, error),
	maxStaleness time.Duration,
) *Fallback {
	return &Fallback{
		primary:       primary,
		secondary:     secondary,
		maxStaleness:  maxStaleness,
		lastSuccess:   time.Now(),
	}
}

// Get tries primary, falls back to secondary
func (f *Fallback) Get(ctx context.Context) (interface{}, bool, error) {
	// Try primary
	data, err := f.primary(ctx)
	if err == nil {
		f.mu.Lock()
		f.cachedData = data
		f.lastSuccess = time.Now()
		f.mu.Unlock()
		return data, true, nil // primary succeeded
	}

	// Primary failed, try secondary
	data, err = f.secondary(ctx)
	if err == nil {
		return data, false, nil // secondary succeeded
	}

	// Both failed, try stale cache
	f.mu.RLock()
	defer f.mu.RUnlock()

	if f.cachedData != nil && time.Since(f.lastSuccess) <= f.maxStaleness {
		return f.cachedData, false, fmt.Errorf("serving stale data: %v", err)
	}

	return nil, false, err
}

// Stats for retry attempts
type RetryStats struct {
	TotalAttempts   int64
	SuccessfulAttempts int64
	FailedAttempts  int64
	TotalRetries    int64
	AverageAttempts float64
	MaxBackoffReached int64
}

// StatsCollector collects retry statistics
type StatsCollector struct {
	stats *RetryStats
	mu    sync.RWMutex
}

// NewStatsCollector creates collector
func NewStatsCollector() *StatsCollector {
	return &StatsCollector{
		stats: &RetryStats{},
	}
}

// Record records a retry result
func (sc *StatsCollector) Record(result *RetryResult) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	sc.stats.TotalAttempts += int64(result.Attempt)
	if result.Error == nil {
		sc.stats.SuccessfulAttempts++
	} else {
		sc.stats.FailedAttempts++
		if result.Attempt > 1 {
			sc.stats.TotalRetries++
		}
	}
}

// GetStats returns current statistics
func (sc *StatsCollector) GetStats() *RetryStats {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	if sc.stats.SuccessfulAttempts+sc.stats.FailedAttempts > 0 {
		sc.stats.AverageAttempts = float64(sc.stats.TotalAttempts) / float64(sc.stats.SuccessfulAttempts+sc.stats.FailedAttempts)
	}

	return sc.stats
}

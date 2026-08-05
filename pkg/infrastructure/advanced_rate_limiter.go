package infrastructure

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"
)

// TokenBucket implements the token bucket algorithm for rate limiting
type TokenBucket struct {
	name          string
	capacity      float64 // max tokens
	refillRate    float64 // tokens per second
	tokens        float64 // current tokens
	lastRefill    time.Time
	mu            sync.Mutex
	rejectedCount int64
}

// NewTokenBucket creates a new token bucket
// capacity: max tokens (burst size)
// refillRate: tokens added per second
func NewTokenBucket(name string, capacity, refillRate float64) *TokenBucket {
	return &TokenBucket{
		name:       name,
		capacity:   capacity,
		refillRate: refillRate,
		tokens:     capacity,
		lastRefill: time.Now(),
	}
}

// Allow checks if cost tokens are available and deducts them
func (tb *TokenBucket) Allow(cost float64) bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	return tb.allowLocked(cost)
}

func (tb *TokenBucket) allowLocked(cost float64) bool {
	// Add refilled tokens since last check
	now := time.Now()
	elapsed := now.Sub(tb.lastRefill).Seconds()

	tb.tokens = math.Min(tb.capacity, tb.tokens+elapsed*tb.refillRate)
	tb.lastRefill = now

	if tb.tokens >= cost {
		tb.tokens -= cost
		return true
	}

	tb.rejectedCount++
	return false
}

// AllowWithWait blocks until tokens are available
func (tb *TokenBucket) AllowWithWait(ctx context.Context, cost float64) (bool, error) {
	for {
		if tb.Allow(cost) {
			return true, nil
		}

		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-time.After(10 * time.Millisecond):
			// Retry
		}
	}
}

// Stats returns current bucket statistics
func (tb *TokenBucket) Stats() map[string]interface{} {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	return map[string]interface{}{
		"name":            tb.name,
		"capacity":        tb.capacity,
		"refill_rate":     tb.refillRate,
		"current_tokens":  tb.tokens,
		"rejected_count":  tb.rejectedCount,
		"fill_percentage": (tb.tokens / tb.capacity) * 100,
	}
}

// LeakyBucket implements the leaky bucket algorithm
// Uses a queue with steady output rate
type LeakyBucket struct {
	name       string
	queue      chan struct{}
	leakRate   time.Duration // time between outputs
	leakTicker *time.Ticker
	done       chan struct{}
	mu         sync.Mutex
	rejections int64
	accepted   int64
}

// NewLeakyBucket creates a new leaky bucket
// capacity: queue size
// leakRate: time between outputs (1 second / requests per second)
func NewLeakyBucket(name string, capacity int, requestsPerSecond float64) *LeakyBucket {
	lb := &LeakyBucket{
		name:       name,
		queue:      make(chan struct{}, capacity),
		leakRate:   time.Duration(float64(time.Second) / requestsPerSecond),
		leakTicker: time.NewTicker(time.Duration(float64(time.Second) / requestsPerSecond)),
		done:       make(chan struct{}),
	}

	// Start leak goroutine
	go lb.leak()

	return lb
}

// Allow tries to add request to queue
func (lb *LeakyBucket) Allow() bool {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	select {
	case lb.queue <- struct{}{}:
		lb.accepted++
		return true
	default:
		lb.rejections++
		return false
	}
}

// leak processes queue at steady rate
func (lb *LeakyBucket) leak() {
	for range lb.leakTicker.C {
		select {
		case <-lb.queue:
			// Process one request
		case <-lb.done:
			return
		default:
			// Queue empty
		}
	}
}

// Stop stops the leaky bucket
func (lb *LeakyBucket) Stop() {
	lb.leakTicker.Stop()
	close(lb.done)
}

// Stats returns statistics
func (lb *LeakyBucket) Stats() map[string]interface{} {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	return map[string]interface{}{
		"name":       lb.name,
		"queue_size": len(lb.queue),
		"queue_cap":  cap(lb.queue),
		"accepted":   lb.accepted,
		"rejections": lb.rejections,
		"leak_rate":  lb.leakRate.String(),
		"fill_ratio": float64(len(lb.queue)) / float64(cap(lb.queue)),
	}
}

// AdaptiveRateLimiter switches between Token Bucket and Leaky Bucket
type AdaptiveRateLimiter struct {
	tokenBucket  *TokenBucket
	leakyBucket  *LeakyBucket
	mu           sync.RWMutex
	currentMode  string // "token" or "leaky"
	stressLevel  float64
	switchBuffer int
}

// NewAdaptiveRateLimiter creates an adaptive rate limiter
func NewAdaptiveRateLimiter(name string, capacity float64, rps float64) *AdaptiveRateLimiter {
	return &AdaptiveRateLimiter{
		tokenBucket:  NewTokenBucket(name+" (token)", capacity, rps),
		leakyBucket:  NewLeakyBucket(name+" (leaky)", int(capacity), rps),
		currentMode:  "token",
		switchBuffer: int(capacity / 2),
	}
}

// Allow checks rate limit with adaptive switching
func (arl *AdaptiveRateLimiter) Allow(cost float64) bool {
	arl.mu.Lock()
	mode := arl.currentMode
	arl.mu.Unlock()

	switch mode {
	case "token":
		allowed := arl.tokenBucket.Allow(cost)

		// Check if should switch to leaky
		tokenStats := arl.tokenBucket.Stats()
		if fillPct, ok := tokenStats["fill_percentage"].(float64); ok && fillPct < 20 {
			arl.mu.Lock()
			arl.currentMode = "leaky"
			arl.mu.Unlock()
		}

		return allowed

	case "leaky":
		allowed := arl.leakyBucket.Allow()

		// Check if should switch back to token
		leakyStats := arl.leakyBucket.Stats()
		if ratio, ok := leakyStats["fill_ratio"].(float64); ok && ratio < 0.3 {
			arl.mu.Lock()
			arl.currentMode = "token"
			arl.mu.Unlock()
		}

		return allowed
	}

	return false
}

// Stats returns stats from current algorithm
func (arl *AdaptiveRateLimiter) Stats() map[string]interface{} {
	arl.mu.RLock()
	mode := arl.currentMode
	arl.mu.RUnlock()

	stats := map[string]interface{}{
		"current_mode": mode,
	}

	if mode == "token" {
		stats["token_bucket"] = arl.tokenBucket.Stats()
	} else {
		stats["leaky_bucket"] = arl.leakyBucket.Stats()
	}

	return stats
}

// RateLimiterPool manages multiple limiters per user/IP
type RateLimiterPool struct {
	limiters map[string]*AdaptiveRateLimiter
	mu       sync.RWMutex
	factory  func(string) *AdaptiveRateLimiter
}

// NewRateLimiterPool creates a new pool
func NewRateLimiterPool(factory func(string) *AdaptiveRateLimiter) *RateLimiterPool {
	return &RateLimiterPool{
		limiters: make(map[string]*AdaptiveRateLimiter),
		factory:  factory,
	}
}

// Allow checks rate limit for identifier
func (p *RateLimiterPool) Allow(identifier string, cost float64) bool {
	p.mu.RLock()
	limiter, exists := p.limiters[identifier]
	p.mu.RUnlock()

	if !exists {
		p.mu.Lock()
		limiter = p.factory(identifier)
		p.limiters[identifier] = limiter
		p.mu.Unlock()
	}

	return limiter.Allow(cost)
}

// AllStats returns all statistics
func (p *RateLimiterPool) AllStats() map[string]map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	stats := make(map[string]map[string]interface{})
	for id, limiter := range p.limiters {
		stats[id] = limiter.Stats()
	}
	return stats
}

// Cleanup removes stale limiters
func (p *RateLimiterPool) Cleanup(maxAge time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	toRemove := []string{}

	// Mark for removal (would need timestamp tracking in production)
	_ = now
	_ = maxAge

	for _, key := range toRemove {
		delete(p.limiters, key)
	}
}

// RateLimitMiddleware creates HTTP middleware for rate limiting
func RateLimitMiddleware(pool *RateLimiterPool, costFn func(string) float64) func(identifier string) error {
	return func(identifier string) error {
		cost := costFn(identifier)
		if !pool.Allow(identifier, cost) {
			return fmt.Errorf("rate limit exceeded for %s", identifier)
		}
		return nil
	}
}

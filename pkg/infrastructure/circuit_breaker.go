package infrastructure

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// CircuitState represents the current state of the circuit breaker
type CircuitState string

const (
	StateClosed    CircuitState = "closed"    // Normal operation
	StateOpen      CircuitState = "open"      // Failing fast
	StateHalfOpen  CircuitState = "half_open" // Testing recovery
)

// CircuitBreaker implements the circuit breaker pattern to prevent cascading failures
type CircuitBreaker struct {
	name              string
	state             CircuitState
	failureCount      int
	successCount      int
	lastFailureTime   time.Time
	lastStateChange   time.Time
	mu                sync.RWMutex
	failureThreshold  int
	successThreshold  int
	timeout           time.Duration
	maxConcurrent     int
	activeRequests    int
	onStateChange     func(oldState, newState CircuitState)
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(name string, failureThreshold int, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		name:              name,
		state:             StateClosed,
		failureThreshold:  failureThreshold,
		successThreshold:  failureThreshold / 2, // Half the failures needed to recover
		timeout:           timeout,
		maxConcurrent:     100,
		lastStateChange:   time.Now(),
	}
}

// OnStateChange registers callback for state transitions
func (cb *CircuitBreaker) OnStateChange(fn func(oldState, newState CircuitState)) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.onStateChange = fn
}

// Call executes fn with circuit breaker protection
func (cb *CircuitBreaker) Call(ctx context.Context, fn func(context.Context) error) error {
	cb.mu.Lock()
	if cb.state == StateOpen {
		cb.mu.Unlock()

		// Check if timeout elapsed to try recovery
		if time.Since(cb.lastFailureTime) > cb.timeout {
			cb.mu.Lock()
			cb.changeState(StateHalfOpen)
			cb.mu.Unlock()
		} else {
			return fmt.Errorf("circuit breaker %s is open: failing fast", cb.name)
		}
	}

	// Check concurrent request limit
	if cb.activeRequests >= cb.maxConcurrent {
		cb.mu.Unlock()
		return fmt.Errorf("circuit breaker %s: max concurrent requests reached", cb.name)
	}

	cb.activeRequests++
	cb.mu.Unlock()

	defer func() {
		cb.mu.Lock()
		cb.activeRequests--
		cb.mu.Unlock()
	}()

	// Execute with timeout
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	err := fn(ctx)

	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.recordFailure()
	} else {
		cb.recordSuccess()
	}

	return err
}

// recordFailure handles failure in current state
func (cb *CircuitBreaker) recordFailure() {
	cb.failureCount++
	cb.successCount = 0
	cb.lastFailureTime = time.Now()

	switch cb.state {
	case StateClosed:
		if cb.failureCount >= cb.failureThreshold {
			cb.changeState(StateOpen)
		}
	case StateHalfOpen:
		// Failed in half-open, go back to open
		cb.changeState(StateOpen)
		cb.failureCount = 0
	}
}

// recordSuccess handles success in current state
func (cb *CircuitBreaker) recordSuccess() {
	cb.successCount++
	cb.failureCount = 0

	switch cb.state {
	case StateHalfOpen:
		if cb.successCount >= cb.successThreshold {
			cb.changeState(StateClosed)
		}
	}
}

// changeState transitions to new state and calls callback
func (cb *CircuitBreaker) changeState(newState CircuitState) {
	oldState := cb.state
	cb.state = newState
	cb.lastStateChange = time.Now()

	if cb.onStateChange != nil {
		go cb.onStateChange(oldState, newState)
	}
}

// State returns current state
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Stats returns current statistics
func (cb *CircuitBreaker) Stats() map[string]interface{} {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return map[string]interface{}{
		"name":              cb.name,
		"state":             cb.state,
		"failures":          cb.failureCount,
		"successes":         cb.successCount,
		"active_requests":   cb.activeRequests,
		"last_failure_ago":  time.Since(cb.lastFailureTime).String(),
		"state_changed_ago": time.Since(cb.lastStateChange).String(),
	}
}

// Reset manually resets circuit breaker to closed state
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.changeState(StateClosed)
	cb.failureCount = 0
	cb.successCount = 0
	cb.activeRequests = 0
}

// CircuitBreakerPool manages multiple circuit breakers
type CircuitBreakerPool struct {
	breakers map[string]*CircuitBreaker
	mu       sync.RWMutex
}

// NewCircuitBreakerPool creates a new pool
func NewCircuitBreakerPool() *CircuitBreakerPool {
	return &CircuitBreakerPool{
		breakers: make(map[string]*CircuitBreaker),
	}
}

// Get or create circuit breaker for a service
func (p *CircuitBreakerPool) GetOrCreate(name string, failureThreshold int, timeout time.Duration) *CircuitBreaker {
	p.mu.RLock()
	if cb, exists := p.breakers[name]; exists {
		p.mu.RUnlock()
		return cb
	}
	p.mu.RUnlock()

	p.mu.Lock()
	defer p.mu.Unlock()

	// Double-check
	if cb, exists := p.breakers[name]; exists {
		return cb
	}

	cb := NewCircuitBreaker(name, failureThreshold, timeout)
	p.breakers[name] = cb
	return cb
}

// AllStats returns stats for all circuit breakers
func (p *CircuitBreakerPool) AllStats() map[string]map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	stats := make(map[string]map[string]interface{})
	for name, cb := range p.breakers {
		stats[name] = cb.Stats()
	}
	return stats
}

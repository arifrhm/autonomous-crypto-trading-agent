package llm

import (
	"errors"
	"sync"
	"time"
)

var (
	ErrCircuitBreakerOpen = errors.New("circuit breaker is open: calls are blocked to prevent cascade failures")
)

// CircuitBreaker prevents cascade API failures by tripping if errors exceed threshold.
type CircuitBreaker struct {
	mu           sync.Mutex
	maxFailures  int
	failureCount int
	window       time.Duration
	coolingTime  time.Duration
	lastFailure  time.Time
	stateOpen    bool
	openUntil    time.Time
}

func NewCircuitBreaker(maxFailures int, window, coolingTime time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		maxFailures: maxFailures,
		window:      window,
		coolingTime: coolingTime,
	}
}

func (cb *CircuitBreaker) Allow() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now()

	// Check if in cooldown period
	if cb.stateOpen {
		if now.Before(cb.openUntil) {
			return ErrCircuitBreakerOpen
		}
		// Half-open: test call allowed
		cb.stateOpen = false
		cb.failureCount = 0
	}

	return nil
}

func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failureCount = 0
	cb.stateOpen = false
}

func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now()
	// Reset failure count if past sliding window
	if now.Sub(cb.lastFailure) > cb.window {
		cb.failureCount = 0
	}

	cb.lastFailure = now
	cb.failureCount++

	if cb.failureCount >= cb.maxFailures {
		cb.stateOpen = true
		cb.openUntil = now.Add(cb.coolingTime)
	}
}

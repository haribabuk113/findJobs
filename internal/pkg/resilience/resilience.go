package resilience

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"time"
)

// CircuitState represents the three states of a circuit breaker.
type CircuitState int

const (
	// Closed allows requests through and counts failures toward the threshold.
	Closed CircuitState = iota
	// Open rejects requests immediately until recovery timeout elapses.
	Open
	// HalfOpen allows probe requests to test whether recovery has occurred.
	HalfOpen
)

// CircuitBreaker is a thread-safe circuit breaker that prevents cascading failures
// against a downstream dependency. It opens after failureThreshold consecutive
// failures and stays open for recoveryTimeout before probing with successThreshold
// successes to close again.
type CircuitBreaker struct {
	mu               sync.Mutex
	state            CircuitState
	failureCount     int
	failureThreshold int
	recoveryTimeout  time.Duration
	successThreshold int
	lastFailure      time.Time
}

// NewCircuitBreaker creates a new breaker that opens after failureThreshold
// consecutive failures and probes recovery after recoveryTimeout. It accepts
// successThreshold successes in half-open before closing the circuit again.
func NewCircuitBreaker(failureThreshold int, recoveryTimeout time.Duration, successThreshold int) *CircuitBreaker {
	cb := &CircuitBreaker{
		state:            Closed,
		failureThreshold: failureThreshold,
		recoveryTimeout:  recoveryTimeout,
		successThreshold: successThreshold,
	}
	return cb
}

// defaultCircuitBreaker is the shared circuit breaker used by adapters.
// It can be configured once at startup via SetDefaultConfig().
var defaultCircuitBreaker *CircuitBreaker

// defaultRetryConfig is the shared retry config used by adapters.
var defaultRetryConfig *RetryConfig

// init sets the default circuit breaker and retry config to production-grade values.
func init() {
	defaultCircuitBreaker = NewCircuitBreaker(5, 30*time.Second, 3)
	rc := DefaultRetryConfig()
	defaultRetryConfig = &rc
}

// SetDefaultConfig configures the default circuit breaker and retry config
// used by all outbound HTTP adapters. It must be called before any adapter
// requests are made (e.g., during application startup).
func SetDefaultConfig(cfg ResilienceConfig) {
	defaultCircuitBreaker = &CircuitBreaker{
		state:            Closed,
		failureThreshold: cfg.CircuitFailureThreshold,
		recoveryTimeout:  cfg.CircuitRecoveryTimeout,
		successThreshold: cfg.CircuitSuccessThreshold,
	}
	defaultRetryConfig = &RetryConfig{
		MaxRetries:           cfg.RetryMaxRetries,
		BackoffBase:          cfg.RetryBackoffBase,
		BackoffMax:           cfg.RetryBackoffMax,
		JitterPercentage:     cfg.RetryJitterPercentage,
		RetryableStatusCodes: DefaultRetryConfig().RetryableStatusCodes,
		Timeout:              cfg.RetryTimeout,
	}
}

// WithCircuitBreaker returns a copy of the shared default circuit breaker that was
// configured via SetDefaultConfig(). If SetDefaultConfig has not been called,
// it returns a breaker with production-grade defaults (5 failures, 30s timeout,
// 3 successes to close).
func WithCircuitBreaker() *CircuitBreaker {
	if defaultCircuitBreaker != nil {
		// Return a copy of the state but reuse the same thresholds so
		// callers get a consistent breaker that reflects the current defaults.
		cb := *defaultCircuitBreaker
		return &cb
	}
	return NewCircuitBreaker(5, 30*time.Second, 3)
}

// State returns the current state of the circuit breaker.
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}

// Allow checks whether a request should be sent through the circuit.
// Returns true if the request is allowed; false if the circuit is open.
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case Closed:
		return true
	case Open:
		if time.Since(cb.lastFailure) < cb.recoveryTimeout {
			return false
		}
		// Transition to half-open to probe recovery.
		cb.state = HalfOpen
		cb.failureCount = 0
		return true
	case HalfOpen:
		return true
	default:
		return true
	}
}

// RecordSuccess records a successful request outcome.
// In half-open state, it counts toward closing the circuit; otherwise it resets
// the failure count.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case HalfOpen:
		cb.failureCount++
		if cb.failureCount >= cb.successThreshold {
			cb.state = Closed
			cb.failureCount = 0
		}
	case Closed:
		// Reset failure count on success.
		cb.failureCount = 0
	}
}

// RecordFailure records a failed request outcome.
// In closed state, it increments the failure count and may open the circuit.
// In half-open state, any failure reopens the circuit.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case Closed:
		cb.failureCount++
		if cb.failureCount >= cb.failureThreshold {
			cb.state = Open
			cb.lastFailure = time.Now()
		}
	case HalfOpen:
		// A failure in half-open reopens the circuit.
		cb.state = Open
		cb.lastFailure = time.Now()
		cb.failureCount = 0
	case Open:
		// Already open; just update last failure time.
		cb.lastFailure = time.Now()
	}
}

// RetryConfig configures exponential backoff and retry behavior for HTTP requests.
type RetryConfig struct {
	// MaxRetries is the maximum number of retry attempts (default: 3).
	MaxRetries int
	// BackoffBase is the base duration for exponential backoff (default: 100ms).
	BackoffBase time.Duration
	// BackoffMax is the maximum backoff duration (default: 5s).
	BackoffMax time.Duration
	// JitterPercentage adds randomness to avoid thundering herd (default: 15).
	JitterPercentage int
	// RetryableStatusCodes are HTTP status codes that trigger a retry.
	RetryableStatusCodes []int
	// Timeout is the per-request timeout; if exceeded the attempt is retried.
	Timeout time.Duration
}

// DefaultRetryConfig returns production-grade defaults for retry behavior.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:       3,
		BackoffBase:      100 * time.Millisecond,
		BackoffMax:       5 * time.Second,
		JitterPercentage: 15,
		RetryableStatusCodes: []int{
			http.StatusRequestTimeout,
			http.StatusTooManyRequests,
			http.StatusBadGateway,
			http.StatusServiceUnavailable,
			http.StatusGatewayTimeout,
		},
	}
}

// WithRetryConfig returns a copy of the shared default retry config that was
// configured via SetDefaultConfig(). If SetDefaultConfig has not been called,
// it returns a config with production-grade defaults.
func WithRetryConfig() *RetryConfig {
	if defaultRetryConfig != nil {
		rc := *defaultRetryConfig
		return &rc
	}
	rc := DefaultRetryConfig()
	return &rc
}

// Do executes an HTTP request with circuit breaker protection and retries.
// The client *http.Client is passed explicitly so it can be overridden in tests
// (e.g., httptest.NewServer). If cb is nil, requests always go through.
// If rcfg is nil, DefaultRetryConfig() is used.
//
// Returns the final *http.Response and any error. The caller must close the
// response body. The body is NOT read inside this function; the caller handles
// reading it after receiving the response.
func Do(ctx context.Context, client *http.Client, req *http.Request, cb *CircuitBreaker, rcfg *RetryConfig) (*http.Response, error) {
	if client == nil {
		client = http.DefaultClient
	}

	if rcfg == nil {
		rc := DefaultRetryConfig()
		rcfg = &rc
	}

	// Ensure context has a timeout; fall back to 30s if not set.
	var cancel context.CancelFunc
	if rcfg.Timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, rcfg.Timeout)
		defer cancel()
	} else {
		ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
	}

	// Circuit breaker gate.
	if cb != nil && !cb.Allow() {
		return nil, fmt.Errorf("circuit breaker is open: request rejected")
	}

	var lastErr error

	for attempt := 0; attempt <= rcfg.MaxRetries; attempt++ {
		// Perform the HTTP request.
		resp, err := client.Do(req.WithContext(ctx))
		if err != nil {
			// Context errors: deadline exceeded is retryable; canceled is not.
			if ctx.Err() == context.DeadlineExceeded {
				if attempt < rcfg.MaxRetries {
					duration := backoff(attempt, rcfg.BackoffBase, rcfg.BackoffMax, rcfg.JitterPercentage)
					time.Sleep(duration)
					cb.RecordFailure()
					lastErr = fmt.Errorf("request timeout (attempt %d/%d): %w", attempt+1, rcfg.MaxRetries+1, err)
					continue
				}
				return nil, fmt.Errorf("request timeout after %d attempts: %w", rcfg.MaxRetries+1, err)
			}
			if ctx.Err() == context.Canceled {
				return nil, fmt.Errorf("request canceled (attempt %d/%d): %w", attempt+1, rcfg.MaxRetries+1, err)
			}
			// Generic network error – retry if attempts remain.
			if attempt < rcfg.MaxRetries {
				duration := backoff(attempt, rcfg.BackoffBase, rcfg.BackoffMax, rcfg.JitterPercentage)
				time.Sleep(duration)
				cb.RecordFailure()
				lastErr = fmt.Errorf("request error (attempt %d/%d): %w", attempt+1, rcfg.MaxRetries+1, err)
				continue
			}
			return nil, fmt.Errorf("request failed after %d attempts: %w", rcfg.MaxRetries+1, err)
		}

		// Check for retryable HTTP status codes.
		if isStatusRetryable(resp.StatusCode, rcfg.RetryableStatusCodes) {
			resp.Body.Close()
			if attempt < rcfg.MaxRetries {
				duration := backoff(attempt, rcfg.BackoffBase, rcfg.BackoffMax, rcfg.JitterPercentage)
				time.Sleep(duration)
				cb.RecordFailure()
				lastErr = fmt.Errorf("retryable status %d (attempt %d/%d)", resp.StatusCode, attempt+1, rcfg.MaxRetries+1)
				continue
			}
			return nil, fmt.Errorf("retryable status %d after %d attempts", resp.StatusCode, rcfg.MaxRetries+1)
		}

		// Success: record in circuit breaker and return the response.
		cb.RecordSuccess()
		return resp, nil
	}

	// Exhausted all retries without success.
	cb.RecordFailure()
	return nil, fmt.Errorf("request failed after %d attempts: %w", rcfg.MaxRetries+1, lastErr)
}

// backoff calculates exponential backoff with optional jitter.
// attempt is 0-indexed (attempt=0 => base*1, attempt=1 => base*2, etc.).
func backoff(attempt int, base, max time.Duration, jitterPercent int) time.Duration {
	// Exponential: base * 2^attempt
	backoffDuration := base * time.Duration(1<<uint(attempt))
	if backoffDuration > max {
		backoffDuration = max
	}
	if jitterPercent > 0 {
		jitter := time.Duration(float64(backoffDuration) * (rand.Float64() * float64(jitterPercent) / 100.0))
		backoffDuration += jitter
	}
	return backoffDuration
}

// isStatusRetryable checks whether an HTTP status code should trigger a retry.
func isStatusRetryable(statusCode int, retryableStatusCodes []int) bool {
	for _, code := range retryableStatusCodes {
		if statusCode == code {
			return true
		}
	}
	return false
}

// ResilienceConfig holds options for circuit breakers and retries.
// It can be populated from env/YAML config.
type ResilienceConfig struct {
	// Circuit breaker settings
	CircuitFailureThreshold int
	CircuitRecoveryTimeout  time.Duration
	CircuitSuccessThreshold int

	// Retry settings
	RetryMaxRetries       int
	RetryBackoffBase      time.Duration
	RetryBackoffMax       time.Duration
	RetryJitterPercentage int
	RetryTimeout          time.Duration
}

// DefaultResilienceConfig returns a ResilienceConfig with production-grade defaults.
func DefaultResilienceConfig() ResilienceConfig {
	return ResilienceConfig{
		CircuitFailureThreshold: 5,
		CircuitRecoveryTimeout:  30 * time.Second,
		CircuitSuccessThreshold: 3,
		RetryMaxRetries:         3,
		RetryBackoffBase:        100 * time.Millisecond,
		RetryBackoffMax:         5 * time.Second,
		RetryJitterPercentage:   15,
		RetryTimeout:            30 * time.Second,
	}
}

// ToCircuitBreaker builds a CircuitBreaker from this config.
func (rc *ResilienceConfig) ToCircuitBreaker() *CircuitBreaker {
	if rc == nil {
		return WithCircuitBreaker()
	}
	return NewCircuitBreaker(
		rc.CircuitFailureThreshold,
		rc.CircuitRecoveryTimeout,
		rc.CircuitSuccessThreshold,
	)
}

// ToRetryConfig builds a RetryConfig from this config.
func (rc *ResilienceConfig) ToRetryConfig() *RetryConfig {
	if rc == nil {
		return WithRetryConfig()
	}
	return &RetryConfig{
		MaxRetries:           rc.RetryMaxRetries,
		BackoffBase:          rc.RetryBackoffBase,
		BackoffMax:           rc.RetryBackoffMax,
		JitterPercentage:     rc.RetryJitterPercentage,
		RetryableStatusCodes: DefaultRetryConfig().RetryableStatusCodes,
		Timeout:              rc.RetryTimeout,
	}
}

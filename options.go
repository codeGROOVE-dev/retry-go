package retry //nolint:revive // More than 5 public structs are necessary for API flexibility

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"time"
)

// IfFunc is the signature for functions that determine whether to retry after an error.
// It returns true if the error is retryable, false otherwise.
type IfFunc func(error) bool

// RetryIfFunc is an alias for IfFunc for backwards compatibility.
// Deprecated: Use IfFunc instead.
type RetryIfFunc = IfFunc //nolint:revive // Keeping RetryIfFunc name for backwards compatibility

// OnRetryFunc is the signature for functions called after each retry attempt.
// The attempt parameter is the zero-based index of the attempt.
type OnRetryFunc func(attempt uint, err error)

// DelayTypeFunc calculates the delay duration before the next retry attempt.
// The attempt parameter is the zero-based index of the attempt.
type DelayTypeFunc func(attempt uint, err error, config *Config) time.Duration

// Timer provides an interface for time operations in retry logic.
// This abstraction allows for mocking time in tests and implementing
// custom timing behaviors. The standard implementation uses time.After.
//
// Security note: Custom Timer implementations must return a valid channel
// that either receives a time value or blocks. Returning nil will cause
// the retry to fail immediately.
type Timer interface {
	// After returns a channel that sends the current time after the duration elapses.
	// It should behave like time.After.
	After(time.Duration) <-chan time.Time
}

// Config holds all configuration options for retry behavior.
// It is typically populated using Option functions and should not be
// constructed directly. Use the various Option functions like Attempts,
// Delay, and RetryIf to configure retry behavior.
//
//nolint:govet // Field alignment optimization would break API compatibility
type Config struct {
	attemptsForError map[error]uint
	context          context.Context //nolint:containedctx // Required for backwards compatibility - context is part of the public API
	delay            time.Duration
	maxDelay         time.Duration
	maxJitter        time.Duration
	onRetry          OnRetryFunc
	retryIf          IfFunc
	delayType        DelayTypeFunc
	timer            Timer
	attempts         uint
	lastErrorOnly    bool
	wrapLastErr      bool // wrap context error with last function error
}

// validate checks if the configuration is valid and safe to use.
func (c *Config) validate() error {
	// Ensure delay values are non-negative.
	if c.delay < 0 {
		return fmt.Errorf("retry: delay must be non-negative, got %v", c.delay)
	}
	if c.maxDelay < 0 {
		return fmt.Errorf("retry: maxDelay must be non-negative, got %v", c.maxDelay)
	}
	if c.maxJitter < 0 {
		return fmt.Errorf("retry: maxJitter must be non-negative, got %v", c.maxJitter)
	}

	// Ensure we have required functions.
	if c.retryIf == nil {
		return errors.New("retry: retryIf function cannot be nil")
	}
	if c.delayType == nil {
		return errors.New("retry: delayType function cannot be nil")
	}
	if c.timer == nil {
		return errors.New("retry: timer cannot be nil")
	}
	if c.context == nil {
		return errors.New("retry: context cannot be nil")
	}

	// Ensure map is initialized.
	if c.attemptsForError == nil {
		c.attemptsForError = make(map[error]uint)
	}

	return nil
}

// Option configures retry behavior. Options are applied in the order provided
// to Do or DoWithData. Later options override earlier ones if they modify the
// same configuration field.
type Option func(*Config)

// LastErrorOnly configures whether to return only the last error that occurred,
// or wrap all errors together. Default is false (return all errors).
func LastErrorOnly(lastErrorOnly bool) Option {
	return func(c *Config) {
		c.lastErrorOnly = lastErrorOnly
	}
}

// Attempts sets the maximum number of retry attempts.
// Setting to 0 enables infinite retries. Default is 10.
func Attempts(attempts uint) Option {
	return func(c *Config) {
		c.attempts = attempts
	}
}

// UntilSucceeded configures infinite retry attempts until success.
// Equivalent to Attempts(0).
func UntilSucceeded() Option {
	return func(c *Config) {
		c.attempts = 0
	}
}

// AttemptsForError sets a specific number of retry attempts for a particular error.
// These attempts are counted against the total retry limit.
// The retry stops when either the specific error limit or total limit is reached.
// Note: errors are compared using errors.Is for matching.
func AttemptsForError(attempts uint, err error) Option {
	return func(c *Config) {
		if err == nil {
			return // Ignore nil errors.
		}
		if c.attemptsForError == nil {
			c.attemptsForError = make(map[error]uint)
		}
		c.attemptsForError[err] = attempts
	}
}

// Delay sets the base delay duration between retry attempts.
// Default is 100ms. The actual delay may be modified by the DelayType function.
func Delay(delay time.Duration) Option {
	return func(c *Config) {
		c.delay = delay
	}
}

// MaxDelay sets the maximum delay duration between retry attempts.
// This caps the delay calculated by DelayType functions.
// By default, there is no maximum (0 means no limit).
func MaxDelay(maxDelay time.Duration) Option {
	return func(c *Config) {
		c.maxDelay = maxDelay
	}
}

// MaxJitter sets the maximum random jitter duration for RandomDelay.
// Default is 100ms.
func MaxJitter(maxJitter time.Duration) Option {
	return func(c *Config) {
		c.maxJitter = maxJitter
	}
}

// DelayType sets the delay calculation function between retries.
// Default is CombineDelay(BackOffDelay, RandomDelay).
func DelayType(delayType DelayTypeFunc) Option {
	return func(c *Config) {
		if delayType != nil {
			c.delayType = delayType
		}
	}
}

const maxShiftAttempts = 62 // Maximum bit shift to prevent overflow

// BackOffDelay implements exponential backoff delay strategy.
// Each retry attempt doubles the delay, up to a maximum.
func BackOffDelay(attempt uint, _ error, config *Config) time.Duration {
	// Adjust for zero-based indexing.
	if attempt > 0 {
		attempt--
	}

	// Cap attempt to prevent overflow.
	if attempt > maxShiftAttempts {
		attempt = maxShiftAttempts
	}

	// Simple exponential backoff.
	delay := config.delay
	if delay <= 0 {
		delay = 1
	}

	// Check if shift would overflow.
	if delay > time.Duration(math.MaxInt64)>>attempt {
		return time.Duration(math.MaxInt64)
	}

	return delay << attempt
}

// FixedDelay implements a constant delay strategy.
// The delay is always config.delay regardless of attempt number.
func FixedDelay(_ uint, _ error, config *Config) time.Duration {
	return config.delay
}

// RandomDelay implements a random delay strategy.
// Returns a random duration between 0 and config.maxJitter.
func RandomDelay(_ uint, _ error, config *Config) time.Duration {
	// Ensure maxJitter is positive to avoid panic.
	if config.maxJitter <= 0 {
		return 0
	}
	return time.Duration(secureRandomInt63n(int64(config.maxJitter)))
}

// secureRandomInt63n returns a non-negative pseudo-random number in [0,n) using crypto/rand.
func secureRandomInt63n(n int64) int64 {
	if n <= 0 {
		return 0
	}
	var b [8]byte
	for {
		if _, err := cryptorand.Read(b[:]); err != nil {
			// Fall back to 0 on error rather than panic
			return 0
		}
		// Clear sign bit to ensure non-negative
		val := int64(binary.BigEndian.Uint64(b[:]) & 0x7FFFFFFFFFFFFFFF) //nolint:gosec // Bitwise AND to clear sign bit is safe
		// Rejection sampling to avoid modulo bias
		const maxInt63 = 0x7FFFFFFFFFFFFFFF // max int63
		maxVal := int64(maxInt63)
		if val < maxVal-(maxVal%n) {
			return val % n
		}
	}
}

// CombineDelay creates a DelayTypeFunc that sums the delays from multiple strategies.
// The total delay is capped at math.MaxInt64 to prevent overflow.
func CombineDelay(delays ...DelayTypeFunc) DelayTypeFunc {
	const maxDuration = time.Duration(math.MaxInt64)

	return func(attempt uint, err error, config *Config) time.Duration {
		var total time.Duration
		for _, delayFunc := range delays {
			d := delayFunc(attempt, err, config)
			// Prevent overflow by checking if we can safely add.
			if d > 0 && total > maxDuration-d {
				return maxDuration
			}
			total += d
		}

		return total
	}
}

// FullJitterBackoffDelay implements exponential backoff with full jitter.
// It returns a random delay between 0 and min(maxDelay, baseDelay * 2^attempt).
func FullJitterBackoffDelay(attempt uint, _ error, config *Config) time.Duration {
	// Handle zero/negative base delay.
	base := config.delay
	if base <= 0 {
		return 0
	}

	// Cap attempt to prevent overflow.
	if attempt > maxShiftAttempts {
		attempt = maxShiftAttempts
	}

	// Calculate ceiling with overflow check.
	var ceiling time.Duration
	if base > time.Duration(math.MaxInt64)>>attempt {
		ceiling = time.Duration(math.MaxInt64)
	} else {
		ceiling = base << attempt
	}

	// Apply max delay cap if set.
	if config.maxDelay > 0 && ceiling > config.maxDelay {
		ceiling = config.maxDelay
	}

	// Return random delay up to ceiling.
	if ceiling <= 0 {
		return 0
	}
	return time.Duration(secureRandomInt63n(int64(ceiling)))
}

// OnRetry sets a callback function that is called after each failed attempt.
// This is useful for logging or other side effects.
//
// Example:
//
//	retry.Do(
//		func() error {
//			return errors.New("some error")
//		},
//		retry.OnRetry(func(attempt uint, err error) {
//			log.Printf("#%d: %s\n", attempt, err)
//		}),
//	)
func OnRetry(onRetry OnRetryFunc) Option {
	return func(c *Config) {
		if onRetry != nil {
			c.onRetry = onRetry
		}
	}
}

// RetryIf controls whether a retry should be attempted after an error
// (assuming there are any retry attempts remaining)
//
// skip retry if special error example:
//
//	retry.Do(
//		func() error {
//			return errors.New("special error")
//		},
//		retry.RetryIf(func(err error) bool {
//			if err.Error() == "special error" {
//				return false
//			}
//			return true
//		})
//	)
//
// By default RetryIf stops execution if the error is wrapped using `retry.Unrecoverable`,
// so above example may also be shortened to:
//
//	retry.Do(
//		func() error {
//			return retry.Unrecoverable(errors.New("special error"))
//		}
//	)
func RetryIf(retryIf IfFunc) Option { //nolint:revive // Keeping RetryIf name for backwards compatibility
	return func(c *Config) {
		if retryIf != nil {
			c.retryIf = retryIf
		}
	}
}

// Context sets the context for retry operations.
// The retry loop will stop if the context is cancelled or times out.
// Default is context.Background().
//
// Example with timeout:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
//	defer cancel()
//
//	retry.Do(
//		func() error {
//			return doSomething()
//		},
//		retry.Context(ctx),
//	)
func Context(ctx context.Context) Option {
	return func(c *Config) {
		c.context = ctx //nolint:fatcontext // Required for backwards compatibility - users expect to set context via option
	}
}

// WithTimer provides a way to swap out timer implementations.
// This is primarily useful for testing.
//
// Example:
//
//	type mockTimer struct{}
//
//	func (mockTimer) After(d time.Duration) <-chan time.Time {
//		return time.After(0) // immediate return for tests
//	}
//
//	retry.Do(
//		func() error { ... },
//		retry.WithTimer(mockTimer{}),
//	)
func WithTimer(t Timer) Option {
	return func(c *Config) {
		c.timer = t
	}
}

// WrapContextErrorWithLastError configures whether to wrap context errors with the last function error.
// This is useful when using infinite retries (Attempts(0)) with context cancellation,
// as it preserves information about what error was occurring when the context expired.
// Default is false.
//
//	ctx, cancel := context.WithCancel(context.Background())
//	defer cancel()
//
//	retry.Do(
//		func() error {
//			...
//		},
//		retry.Context(ctx),
//		retry.Attempts(0),
//		retry.WrapContextErrorWithLastError(true),
//	)
func WrapContextErrorWithLastError(wrap bool) Option {
	return func(c *Config) {
		c.wrapLastErr = wrap
	}
}

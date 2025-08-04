// Package retry provides a simple and flexible retry mechanism for Go.
// It allows executing functions with automatic retry on failure, with configurable
// backoff strategies, retry conditions, and error handling.
//
// The package is safe for concurrent use.
//
// The package is inspired by Try::Tiny::Retry from Perl.
//
// Basic usage:
//
//	url := "http://example.com"
//	var body []byte
//
//	err := retry.Do(
//		func() error {
//			resp, err := http.Get(url)
//			if err != nil {
//				return err
//			}
//			defer resp.Body.Close()
//			body, err = io.ReadAll(resp.Body)
//			if err != nil {
//				return err
//			}
//			return nil
//		},
//	)
//
// With data return:
//
//	body, err := retry.DoWithData(
//		func() ([]byte, error) {
//			resp, err := http.Get(url)
//			if err != nil {
//				return nil, err
//			}
//			defer resp.Body.Close()
//			return io.ReadAll(resp.Body)
//		},
//	)
package retry

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

// RetryableFunc is the function signature for retryable functions used with Do.
type RetryableFunc func() error

// RetryableFuncWithData is the function signature for retryable functions that return data.
// Used with DoWithData.
type RetryableFuncWithData[T any] func() (T, error)

type defaultTimer struct{}

func (defaultTimer) After(d time.Duration) <-chan time.Time {
	return time.After(d)
}

// Do executes the retryable function with the provided options.
// It returns nil if the function succeeds, or an error if all retry attempts fail.
//
// By default, it will retry up to 10 times with exponential backoff and jitter.
// The behavior can be customized using Option functions.
func Do(retryableFunc RetryableFunc, opts ...Option) error {
	retryableFuncWithData := func() (any, error) {
		return nil, retryableFunc()
	}

	_, err := DoWithData(retryableFuncWithData, opts...)
	return err
}

// DoWithData executes the retryable function with the provided options and returns the function's data.
// It returns the data and nil error if the function succeeds, or zero value and error if all retry attempts fail.
//
// By default, it will retry up to 10 times with exponential backoff and jitter.
// The behavior can be customized using Option functions.
//
//nolint:gocognit,gocyclo,revive // Complexity is necessary for retry logic
func DoWithData[T any](retryableFunc RetryableFuncWithData[T], opts ...Option) (T, error) {
	var attempt uint
	var emptyT T

	const (
		defaultAttempts = 10
		defaultDelay    = 100 * time.Millisecond
		defaultJitter   = 100 * time.Millisecond
	)

	// default config
	config := &Config{
		attempts:         defaultAttempts,
		attemptsForError: make(map[error]uint),
		delay:            defaultDelay,
		maxJitter:        defaultJitter,
		onRetry:          func(_ uint, _ error) {},
		retryIf:          IsRecoverable,
		delayType:        CombineDelay(BackOffDelay, RandomDelay),
		lastErrorOnly:    false,
		context:          context.Background(),
		timer:            defaultTimer{},
	}

	// apply opts
	for _, opt := range opts {
		opt(config)
	}

	// Validate configuration
	if err := config.validate(); err != nil {
		return emptyT, err
	}

	// Check if context is already done
	if err := context.Cause(config.context); err != nil {
		return emptyT, err
	}

	// Pre-allocate error slice with bounded capacity to prevent memory exhaustion.
	const maxErrors = 1000 // Limit to prevent DoS through memory consumption.
	capacity := config.attempts
	if capacity == 0 || capacity > maxErrors {
		capacity = maxErrors
	}
	errorLog := make(Error, 0, capacity)

	// Copy map to avoid modifying the original.
	attemptsForError := make(map[error]uint, len(config.attemptsForError))
	for err, attempts := range config.attemptsForError {
		attemptsForError[err] = attempts
	}

	for {
		// Check context before each attempt.
		if err := context.Cause(config.context); err != nil {
			if config.lastErrorOnly {
				return emptyT, err
			}
			if len(errorLog) < cap(errorLog) {
				errorLog = append(errorLog, err)
			}
			return emptyT, errorLog
		}

		t, err := retryableFunc()
		if err == nil {
			return t, nil
		}

		// Only append if we haven't hit the cap to prevent unbounded growth.
		if len(errorLog) < cap(errorLog) {
			// Unpack unrecoverable errors to store the underlying error.
			var u unrecoverableError
			if errors.As(err, &u) {
				errorLog = append(errorLog, u.error)
			} else {
				errorLog = append(errorLog, err)
			}
		}

		// Check if we should stop retrying.
		if !IsRecoverable(err) || !config.retryIf(err) {
			if config.lastErrorOnly {
				return emptyT, err
			}
			// For unrecoverable errors on first attempt, return the error directly.
			if len(errorLog) == 1 && !IsRecoverable(err) {
				return emptyT, err
			}
			return emptyT, errorLog
		}

		// Check error-specific attempt limits.
		stop := false
		for errToCheck, attempts := range attemptsForError {
			if errors.Is(err, errToCheck) {
				attempts--
				attemptsForError[errToCheck] = attempts
				if attempts <= 0 {
					stop = true
					break
				}
			}
		}
		if stop {
			break
		}

		// Check if this is the last attempt (when attempts > 0).
		if config.attempts > 0 && attempt >= config.attempts-1 {
			break
		}

		// Call onRetry callback.
		config.onRetry(attempt, err)

		// Increment attempt counter.
		if attempt == math.MaxUint {
			break
		}
		attempt++

		// Calculate delay.
		delay := config.delayType(attempt, err, config)
		if delay < 0 {
			delay = 0
		}
		if config.maxDelay > 0 && delay > config.maxDelay {
			delay = config.maxDelay
		}

		// Protect against timer implementations that might return nil channel.
		timer := config.timer.After(delay)
		if timer == nil {
			return emptyT, errors.New("retry: timer.After returned nil channel")
		}

		select {
		case <-timer:
		case <-config.context.Done():
			contextErr := context.Cause(config.context)
			if config.lastErrorOnly {
				if config.wrapLastErr {
					return emptyT, fmt.Errorf("%w: %w", contextErr, err)
				}
				return emptyT, contextErr
			}
			if len(errorLog) < cap(errorLog) {
				errorLog = append(errorLog, contextErr)
			}
			return emptyT, errorLog
		}
	}

	if config.lastErrorOnly {
		if len(errorLog) > 0 {
			return emptyT, errorLog.Unwrap()
		}
		return emptyT, nil
	}
	return emptyT, errorLog
}

// Error represents a collection of errors that occurred during retry attempts.
// It implements the error interface and provides compatibility with errors.Is,
// errors.As, and errors.Unwrap.
type Error []error

// Error returns a string representation of all errors that occurred during retry attempts.
// Each error is prefixed with its attempt number.
func (e Error) Error() string {
	if len(e) == 0 {
		return "retry: all attempts failed"
	}

	// Pre-size builder for better performance
	// Estimate: prefix (30) + each error (~50 chars avg)
	var b strings.Builder
	b.Grow(30 + len(e)*50)
	b.WriteString("retry: all attempts failed:")

	for i, err := range e {
		if err != nil {
			fmt.Fprintf(&b, "\n#%d: %s", i+1, err.Error())
		}
	}

	return b.String()
}

// Is reports whether any error in e matches target.
// It implements support for errors.Is.
func (e Error) Is(target error) bool {
	for _, err := range e {
		if errors.Is(err, target) {
			return true
		}
	}
	return false
}

// As finds the first error in e that matches target, and if so,
// sets target to that error value and returns true.
// It implements support for errors.As.
func (e Error) As(target any) bool {
	for _, err := range e {
		if errors.As(err, target) {
			return true
		}
	}
	return false
}

// Unwrap returns the last error for compatibility with errors.Unwrap.
// When you need to unwrap all errors, you should use WrappedErrors instead.
//
// Example:
//
//	err := Do(
//		func() error {
//			return errors.New("original error")
//		},
//		Attempts(1),
//	)
//	fmt.Println(errors.Unwrap(err)) // "original error" is printed
func (e Error) Unwrap() error {
	return e[len(e)-1]
}

// WrappedErrors returns the list of errors that this Error is wrapping.
// It is an implementation of the `errwrap.Wrapper` interface
// in package [errwrap](https://github.com/hashicorp/errwrap) so that
// `retry.Error` can be used with that library.
func (e Error) WrappedErrors() []error {
	return e
}

type unrecoverableError struct {
	error
}

func (ue unrecoverableError) Error() string {
	if ue.error == nil {
		return "retry: unrecoverable error with nil value"
	}
	return ue.error.Error()
}

func (ue unrecoverableError) Unwrap() error {
	return ue.error
}

// Unrecoverable wraps an error to mark it as unrecoverable.
// When an unrecoverable error is returned, the retry mechanism will stop immediately.
func Unrecoverable(err error) error {
	return unrecoverableError{err}
}

// IsRecoverable reports whether err is recoverable.
// It returns false if err is or wraps an unrecoverable error.
func IsRecoverable(err error) bool {
	return !errors.Is(err, unrecoverableError{})
}

// Is implements error matching for unrecoverableError.
func (unrecoverableError) Is(err error) bool {
	_, ok := err.(unrecoverableError)
	return ok
}

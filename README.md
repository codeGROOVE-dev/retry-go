# retry-go: Production Retry Logic for Go

[![Release](https://img.shields.io/github/release/codeGROOVE-dev/retry-go.svg?style=flat-square)](https://github.com/codeGROOVE-dev/retry-go/releases/latest)
[![Software License](https://img.shields.io/badge/license-MIT-brightgreen.svg?style=flat-square)](LICENSE.md)
[![Go Report Card](https://goreportcard.com/badge/github.com/codeGROOVE-dev/retry-go?style=flat-square)](https://goreportcard.com/report/github.com/codeGROOVE-dev/retry-go)
[![Go Reference](https://pkg.go.dev/badge/github.com/codeGROOVE-dev/retry-go.svg)](https://pkg.go.dev/github.com/codeGROOVE-dev/retry-go)

**Zero dependencies. Memory-bounded. No goroutine leaks. No panics.**

*Hard guarantees: Bounded memory (1000 errors max). No allocations in hot path. Context-aware cancellation.*

Actively maintained fork of [avast/retry-go](https://github.com/avast/retry-go) focused on correctness and resource efficiency. 100% API compatible drop-in replacement.

**Production guarantees:**
- Memory bounded: Max 1000 errors stored (configurable via maxErrors constant)
- No goroutine leaks: Uses caller's goroutine exclusively
- Integer overflow safe: Backoff capped at 2^62 to prevent wraparound
- Context-aware: Cancellation checked before each attempt
- No panics: All edge cases return errors
- Predictable jitter: Uses math/rand/v2 for consistent performance
- Zero allocations after init in success path

## Quick Start

### Simple Retry

```go
// Retry a flaky operation up to 10 times (default)
err := retry.Do(func() error {
    return doSomethingFlaky()
})
```

### Retry with Custom Attempts

```go
// Retry up to 5 times with exponential backoff
err := retry.Do(
    func() error {
        resp, err := http.Get("https://api.example.com/data")
        if err != nil {
            return err
        }
        defer resp.Body.Close()
        return nil
    },
    retry.Attempts(5),
)
```

### Overly-complicated production configuration

```go

// Overly-complex production pattern: bounded retries with circuit breaking
err := retry.Do(
    func() error {
        return processPayment(ctx, req)
    },
    retry.Attempts(3),                        // Hard limit
    retry.Context(ctx),                       // Respect cancellation
    retry.MaxDelay(10*time.Second),           // Cap backoff
    retry.AttemptsForError(0, ErrRateLimit),  // Stop on rate limit
    retry.OnRetry(func(n uint, err error) {
        log.Printf("retry attempt %d: %v", n, err)
    }),
    retry.RetryIf(func(err error) bool {
        // Only retry on network errors
        var netErr net.Error
        return errors.As(err, &netErr) && netErr.Temporary()
    }),
)
```

### Preventing Cascading Failures

```go
// Stop retry storms with Unrecoverable
if errors.Is(err, context.DeadlineExceeded) {
    return retry.Unrecoverable(err) // Don't retry timeouts
}

// Per-error type limits prevent thundering herd
retry.AttemptsForError(0, ErrCircuitOpen)    // Fail fast on circuit breaker
retry.AttemptsForError(1, sql.ErrTxDone)     // One retry for tx errors
retry.AttemptsForError(5, ErrServiceUnavailable) // More retries for 503s
```

## Failure Modes & Limits

| Scenario | Behavior | Limit |
|----------|----------|-------|
| Error accumulation | Old errors dropped after limit | 1000 errors |
| Attempt overflow | Stops retrying | uint max (~4B) |
| Backoff overflow | Capped at max duration | 2^62 ns |
| Context cancelled | Returns immediately | No retries |
| Timer returns nil | Returns error | Fail safe |
| Panic in retryable func | Propagates panic | No swallowing |

## Installation

Requires Go 1.22 or higher.

```bash
go get github.com/codeGROOVE-dev/retry-go
```

## Documentation

- [API Docs](https://pkg.go.dev/github.com/codeGROOVE-dev/retry-go)
- [Examples](https://github.com/codeGROOVE-dev/retry-go/tree/master/examples)
- [Tests](https://github.com/codeGROOVE-dev/retry-go/tree/master/retry_test.go)

---

*Production retry logic that just works.*

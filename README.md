# retry-go: Production Retry Logic for Go

[![Release](https://img.shields.io/github/release/codeGROOVE-dev/retry-go.svg?style=flat-square)](https://github.com/codeGROOVE-dev/retry-go/releases/latest)
[![Software License](https://img.shields.io/badge/license-MIT-brightgreen.svg?style=flat-square)](LICENSE.md)
[![Go Report Card](https://goreportcard.com/badge/github.com/codeGROOVE-dev/retry-go?style=flat-square)](https://goreportcard.com/report/github.com/codeGROOVE-dev/retry-go)
[![Go Reference](https://pkg.go.dev/badge/github.com/codeGROOVE-dev/retry-go.svg)](https://pkg.go.dev/github.com/codeGROOVE-dev/retry-go)

**Zero dependencies. Memory-bounded. Uptime-focused.**

*Because 99.99% uptime means your retry logic can't be the failure point.*

Actively maintained fork of [avast/retry-go](https://github.com/avast/retry-go) focused on correctness and resource efficiency. 100% API compatible drop-in replacement.

**Key improvements:**
- ⚡ Zero external dependencies
- 🔒 Memory-bounded error accumulation  
- 🛡️ Integer overflow protection in backoff
- 🎯 Enhanced readability and debuggability
- 📊 Predictable behavior under load

## Quick Start

### Basic Retry with Error Handling

```go
import (
    "net/http"
    "github.com/codeGROOVE-dev/retry-go"
)

// Retry API call with exponential backoff + jitter
err := retry.Do(
    func() error {
        resp, err := http.Get("https://api.stripe.com/v1/charges")
        if err != nil {
            return err
        }
        defer resp.Body.Close()
        
        if resp.StatusCode >= 500 {
            return fmt.Errorf("server error: %d", resp.StatusCode)
        }
        return nil
    },
    retry.Attempts(5),
    retry.DelayType(retry.CombineDelay(retry.BackOffDelay, retry.RandomDelay)),
)
```

### Retry with Data Return (Generics)

```go
import (
    "context"
    "encoding/json"
    "github.com/codeGROOVE-dev/retry-go"
)

// Database query with timeout and retry
users, err := retry.DoWithData(
    func() ([]User, error) {
        return db.FindActiveUsers(ctx)
    },
    retry.Attempts(3),
    retry.Context(ctx),
    retry.DelayType(retry.BackOffDelay),
)
```

### Production Configuration

```go
// Payment processing with comprehensive retry logic
err := retry.Do(
    func() error { return paymentGateway.Charge(ctx, amount) },
    retry.Attempts(5),
    retry.Context(ctx),
    retry.DelayType(retry.FullJitterBackoffDelay),
    retry.MaxDelay(30*time.Second),
    retry.OnRetry(func(n uint, err error) {
        log.Warn("Payment retry", "attempt", n+1, "error", err)
    }),
    retry.RetryIf(func(err error) bool {
        return !isAuthError(err) // Don't retry 4xx errors
    }),
)
```

## Key Features

**Unrecoverable Errors** - Stop immediately for certain errors:
```go
if resp.StatusCode == 401 {
    return retry.Unrecoverable(errors.New("auth failed"))
}
```

**Context Integration** - Timeout and cancellation:
```go
ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
retry.Do(dbQuery, retry.Context(ctx))
```

**Error-Specific Limits** - Different retry counts per error type:
```go
retry.AttemptsForError(1, sql.ErrTxDone)  // Don't retry transaction errors
```

## Performance & Scale

| Metric | Value |
|--------|-------|
| Memory overhead | ~200 bytes per operation |
| Error accumulation | Bounded at 1000 errors (DoS protection) |
| Goroutines | Uses calling goroutine only |
| High-throughput safe | No hidden allocations or locks |

## Library Comparison

**[cenkalti/backoff](https://github.com/cenkalti/backoff)** - Complex interface, requires manual retry loops, no error accumulation.

**[sethgrid/pester](https://github.com/sethgrid/pester)** - HTTP-only, lacks general-purpose retry logic.

**[matryer/try](https://github.com/matryer/try)** - Popular but non-standard API, missing production features.

**[rafaeljesus/retry-go](https://github.com/rafaeljesus/retry-go)** - Similar design but lacks error-specific limits and comprehensive context handling.

**This fork** builds on avast/retry-go's solid foundation with correctness fixes and resource optimizations.

## Installation

```bash
go get github.com/codeGROOVE-dev/retry-go
```

## Documentation

- [API Docs](https://pkg.go.dev/github.com/codeGROOVE-dev/retry-go)
- [Examples](https://github.com/codeGROOVE-dev/retry-go/tree/master/examples)
- [Tests](https://github.com/codeGROOVE-dev/retry-go/tree/master/retry_test.go)

---

*Production retry logic that just works.*
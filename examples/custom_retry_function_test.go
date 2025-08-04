package retry_test

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/codeGROOVE-dev/retry"
)

// RetriableError is a custom error that contains a positive duration for the next retry.
type RetriableError struct {
	Err        error
	RetryAfter time.Duration
}

// Error returns error message and a Retry-After duration.
func (e *RetriableError) Error() string {
	return fmt.Sprintf("%s (retry after %v)", e.Err.Error(), e.RetryAfter)
}

var _ error = (*RetriableError)(nil)

// TestCustomRetryFunction shows how to use a custom retry function.
func TestCustomRetryFunction(t *testing.T) {
	attempts := 5 // server succeeds after 5 attempts
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if attempts > 0 {
			// inform the client to retry after one second using standard
			// HTTP 429 status code with Retry-After header in seconds
			w.Header().Add("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte("Server limit reached"))
			attempts--
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("hello"))
	}))
	defer ts.Close()

	var body []byte

	err := retry.Do(
		func() error {
			resp, err := http.Get(ts.URL)

			if err == nil {
				defer func() {
					if err := resp.Body.Close(); err != nil {
						panic(err)
					}
				}()
				body, err = io.ReadAll(resp.Body)
				if resp.StatusCode != http.StatusOK {
					err = fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
					if resp.StatusCode == http.StatusTooManyRequests {
						// check Retry-After header if it contains seconds to wait for the next retry
						if retryAfter, e := strconv.ParseInt(resp.Header.Get("Retry-After"), 10, 32); e == nil {
							// the server returns 0 to inform that the operation cannot be retried
							if retryAfter <= 0 {
								return retry.Unrecoverable(err)
							}
							return &RetriableError{
								Err:        err,
								RetryAfter: time.Duration(retryAfter) * time.Second,
							}
						}
						// A real implementation should also try to http.Parse the retryAfter response header
						// to conform with HTTP specification. Herein we know here that we return only seconds.
					}
				}
			}

			return err
		},
		retry.DelayType(func(n uint, err error, config *retry.Config) time.Duration {
			// Server fails with: <error message>
			var retriable *RetriableError
			if errors.As(err, &retriable) {
				// Client follows server recommendation to retry after retriable.RetryAfter
				return retriable.RetryAfter
			}
			// apply a default exponential back off strategy
			return retry.BackOffDelay(n, err, config)
		}),
	)
	// Server responds with: <body content>
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(body) != "hello" {
		t.Errorf("expected %v, got %v", "hello", string(body))
	}
}

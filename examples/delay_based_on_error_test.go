// This test delay is based on kind of error
// e.g. HTTP response [Retry-After](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Retry-After)
package retry_test

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/codeGROOVE-dev/retry-go"
)

type RetryAfterError struct {
	response http.Response
}

func (err RetryAfterError) Error() string {
	return fmt.Sprintf(
		"Request to %s fail %s (%d)",
		err.response.Request.RequestURI,
		err.response.Status,
		err.response.StatusCode,
	)
}

type SomeOtherError struct {
	err        string
	retryAfter time.Duration
}

func (err SomeOtherError) Error() string {
	return err.err
}

func TestCustomRetryFunctionBasedOnKindOfError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintln(w, "hello")
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
			}

			return err
		},
		retry.DelayType(func(n uint, err error, config *retry.Config) time.Duration {
			var retryAfterErr RetryAfterError
			if errors.As(err, &retryAfterErr) {
				t := parseRetryAfter(retryAfterErr.response.Header.Get("Retry-After"))
				return time.Until(t)
			}
			var someOtherErr SomeOtherError
			if errors.As(err, &someOtherErr) {
				return someOtherErr.retryAfter
			}

			// default is backoffdelay
			return retry.BackOffDelay(n, err, config)
		}),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(body) == 0 {
		t.Error("expected non-empty value")
	}
}

// use https://github.com/aereal/go-httpretryafter instead.
func parseRetryAfter(_ string) time.Time {
	return time.Now().Add(1 * time.Second)
}

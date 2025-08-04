package retry_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/codeGROOVE-dev/retry"
)

// TestErrorHistory shows an example of how to get all the previous errors when
// retry.Do ends in success.
func TestErrorHistory(t *testing.T) {
	attempts := 3 // server succeeds after 3 attempts
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if attempts > 0 {
			attempts--
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	var allErrors []error
	err := retry.Do(
		func() error {
			resp, err := http.Get(ts.URL)
			if err != nil {
				return err
			}
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("failed HTTP - %d", resp.StatusCode)
			}
			return nil
		},
		retry.OnRetry(func(n uint, err error) {
			allErrors = append(allErrors, err)
		}),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(allErrors) != 3 {
		t.Errorf("expected len(allErrors) = 3, got %d", len(allErrors))
	}
}

package retry

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"reflect"
	"testing"
	"time"
)

func TestDoWithDataAllFailed(t *testing.T) {
	var retrySum uint
	v, err := DoWithData(
		func() (int, error) { return 7, errors.New("test") },
		OnRetry(func(n uint, err error) { retrySum += n }),
		Delay(time.Nanosecond),
	)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if v != 0 {
		t.Errorf("got v=%d, want 0", v)
	}

	expectedErrorFormat := `All attempts fail:
#1: test
#2: test
#3: test
#4: test
#5: test
#6: test
#7: test
#8: test
#9: test
#10: test`
	if retryErr, ok := err.(Error); ok {
		if len(retryErr) != 10 {
			t.Errorf("error count: got %d, want 10", len(retryErr))
		}
	} else {
		t.Fatalf("expected Error type, got %T", err)
	}
	fmt.Println(err.Error())
	if err.Error() != expectedErrorFormat {
		t.Errorf("error message: got %q, want %q", err.Error(), expectedErrorFormat)
	}
	if retrySum != uint(36) {
		t.Errorf("retry sum: got %d, want 36", retrySum)
	}
}

func TestDoFirstOk(t *testing.T) {
	var retrySum uint
	err := Do(
		func() error { return nil },
		OnRetry(func(n uint, err error) { retrySum += n }),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if uint(0) != retrySum {
		t.Errorf("retrySum (expected no retries): got %d, want 0", retrySum)
	}
}

func TestDoWithDataFirstOk(t *testing.T) {
	returnVal := 1

	var retrySum uint
	val, err := DoWithData(
		func() (int, error) { return returnVal, nil },
		OnRetry(func(n uint, err error) { retrySum += n }),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if returnVal != val {
		t.Errorf("return value: got %d, want %d", val, returnVal)
	}
	if uint(0) != retrySum {
		t.Errorf("retrySum (expected no retries): got %d, want 0", retrySum)
	}
}

func TestRetryIf(t *testing.T) {
	var retryCount uint
	err := Do(
		func() error {
			if retryCount >= 2 {
				return errors.New("special")
			} else {
				return errors.New("test")
			}
		},
		OnRetry(func(n uint, err error) { retryCount++ }),
		RetryIf(func(err error) bool {
			return err.Error() != "special"
		}),
		Delay(time.Nanosecond),
	)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	expectedErrorFormat := `All attempts fail:
#1: test
#2: test
#3: special`
	if retryErr, ok := err.(Error); ok {
		if len(retryErr) != 3 {
			t.Errorf("error count: got %d, want 3", len(retryErr))
		}
	} else {
		t.Fatalf("expected Error type, got %T", err)
	}
	if err.Error() != expectedErrorFormat {
		t.Errorf("error message: got %q, want %q", err.Error(), expectedErrorFormat)
	}
	if uint(2) != retryCount {
		t.Errorf("retry count: got %d, want 2", retryCount)
	}
}

func TestRetryIf_ZeroAttempts(t *testing.T) {
	var retryCount, onRetryCount uint
	err := Do(
		func() error {
			if retryCount >= 2 {
				return errors.New("special")
			} else {
				retryCount++
				return errors.New("test")
			}
		},
		OnRetry(func(n uint, err error) { onRetryCount = n }),
		RetryIf(func(err error) bool {
			return err.Error() != "special"
		}),
		Delay(time.Nanosecond),
		LastErrorOnly(true),
		Attempts(0),
	)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if "special" != err.Error() {
		t.Errorf("error message: got %q, want %q", err.Error(), "special")
	}
	if retryCount != onRetryCount+1 {
		t.Errorf("retry count vs onRetry count: got retryCount=%d, onRetryCount=%d, want retryCount=onRetryCount+1", retryCount, onRetryCount)
	}
}

func TestZeroAttemptsWithError(t *testing.T) {
	const maxErrors = 999
	count := 0

	err := Do(
		func() error {
			if count < maxErrors {
				count += 1
				return errors.New("test")
			}

			return nil
		},
		Attempts(0),
		MaxDelay(time.Nanosecond),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if maxErrors != count {
		t.Errorf("execution count: got %d, want %d", count, maxErrors)
	}
}

func TestZeroAttemptsWithoutError(t *testing.T) {
	count := 0

	err := Do(
		func() error {
			count++

			return nil
		},
		Attempts(0),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if 1 != count {
		t.Errorf("execution count: got %d, want 1", count)
	}
}

func TestZeroAttemptsWithUnrecoverableError(t *testing.T) {
	err := Do(
		func() error {
			return Unrecoverable(errors.New("test error"))
		},
		Attempts(0),
		MaxDelay(time.Nanosecond),
	)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	expectedErr := Unrecoverable(errors.New("test error"))
	if err.Error() != expectedErr.Error() {
		t.Errorf("got %v, want %v", err, expectedErr)
	}
}

func TestAttemptsForError(t *testing.T) {
	count := uint(0)
	testErr := os.ErrInvalid
	attemptsForTestError := uint(3)
	err := Do(
		func() error {
			count++
			return testErr
		},
		AttemptsForError(attemptsForTestError, testErr),
		Attempts(5),
	)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if count != attemptsForTestError {
		t.Errorf("attempt count: got %d, want %d", count, attemptsForTestError)
	}
}

func TestDefaultSleep(t *testing.T) {
	start := time.Now()
	err := Do(
		func() error { return errors.New("test") },
		Attempts(3),
	)
	dur := time.Since(start)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if dur <= 300*time.Millisecond {
		t.Errorf("retry duration too short: got %v, want >300ms (3 retries with default delay)", dur)
	}
}

func TestFixedSleep(t *testing.T) {
	start := time.Now()
	err := Do(
		func() error { return errors.New("test") },
		Attempts(3),
		DelayType(FixedDelay),
	)
	dur := time.Since(start)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if dur >= 500*time.Millisecond {
		t.Errorf("retry duration too long: got %v, want <500ms (3 retries with fixed delay)", dur)
	}
}

func TestLastErrorOnly(t *testing.T) {
	var retrySum uint
	err := Do(
		func() error { return fmt.Errorf("%d", retrySum) },
		OnRetry(func(n uint, err error) { retrySum += 1 }),
		Delay(time.Nanosecond),
		LastErrorOnly(true),
	)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "9" {
		t.Errorf("error message: got %q, want %q", err.Error(), "9")
	}
}

func TestUnrecoverableError(t *testing.T) {
	attempts := 0
	testErr := errors.New("error")
	err := Do(
		func() error {
			attempts++
			return Unrecoverable(testErr)
		},
		Attempts(2),
	)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	expectedErr := Unrecoverable(testErr)
	if err.Error() != expectedErr.Error() {
		t.Errorf("error: got %v, want %v", err, expectedErr)
	}
	if attempts != 1 {
		t.Errorf("attempts with unrecoverable error: got %d, want 1", attempts)
	}
}

func TestCombineFixedDelays(t *testing.T) {
	if os.Getenv("OS") == "macos-latest" {
		t.Skip("Skipping testing in MacOS GitHub actions - too slow, duration is wrong")
	}

	start := time.Now()
	err := Do(
		func() error { return errors.New("test") },
		Attempts(3),
		DelayType(CombineDelay(FixedDelay, FixedDelay)),
	)
	dur := time.Since(start)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if dur <= 400*time.Millisecond {
		t.Errorf("combined delay duration too short: got %v, want >400ms", dur)
	}
	if dur >= 500*time.Millisecond {
		t.Errorf("combined delay duration too long: got %v, want <500ms", dur)
	}
}

func TestRandomDelay(t *testing.T) {
	if os.Getenv("OS") == "macos-latest" {
		t.Skip("Skipping testing in MacOS GitHub actions - too slow, duration is wrong")
	}

	start := time.Now()
	err := Do(
		func() error { return errors.New("test") },
		Attempts(3),
		DelayType(RandomDelay),
		MaxJitter(50*time.Millisecond),
	)
	dur := time.Since(start)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if dur <= 2*time.Millisecond {
		t.Errorf("random delay duration too short: got %v, want >2ms", dur)
	}
	if dur >= 150*time.Millisecond {
		t.Errorf("random delay duration too long: got %v, want <150ms", dur)
	}
}

func TestMaxDelay(t *testing.T) {
	if os.Getenv("OS") == "macos-latest" {
		t.Skip("Skipping testing in MacOS GitHub actions - too slow, duration is wrong")
	}

	start := time.Now()
	err := Do(
		func() error { return errors.New("test") },
		Attempts(5),
		Delay(10*time.Millisecond),
		MaxDelay(50*time.Millisecond),
	)
	dur := time.Since(start)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if dur <= 120*time.Millisecond {
		t.Errorf("max delay duration too short: got %v, want >120ms (5 retries with max delay)", dur)
	}
	if dur >= 275*time.Millisecond {
		t.Errorf("max delay duration too long: got %v, want <275ms (5 retries with max delay)", dur)
	}
}

func TestBackOffDelay(t *testing.T) {
	for _, c := range []struct {
		label         string
		delay         time.Duration
		expectedMaxN  uint
		n             uint
		expectedDelay time.Duration
	}{
		{
			label:         "negative-delay",
			delay:         -1,
			expectedMaxN:  62,
			n:             2,
			expectedDelay: 2,
		},
		{
			label:         "zero-delay",
			delay:         0,
			expectedMaxN:  62,
			n:             65,
			expectedDelay: 1 << 62,
		},
		{
			label:         "one-second",
			delay:         time.Second,
			expectedMaxN:  33,
			n:             62,
			expectedDelay: time.Second << 33,
		},
		{
			label:         "one-second-n",
			delay:         time.Second,
			expectedMaxN:  33,
			n:             1,
			expectedDelay: time.Second,
		},
	} {
		t.Run(
			c.label,
			func(t *testing.T) {
				config := Config{
					delay: c.delay,
				}
				delay := BackOffDelay(c.n, nil, &config)
				if c.expectedMaxN != config.maxBackOffN {
					t.Errorf("max n mismatch: got %v, want %v", config.maxBackOffN, c.expectedMaxN)
				}
				if c.expectedDelay != delay {
					t.Errorf("delay duration mismatch: got %v, want %v", delay, c.expectedDelay)
				}
			},
		)
	}
}

func TestCombineDelay(t *testing.T) {
	f := func(d time.Duration) DelayTypeFunc {
		return func(_ uint, _ error, _ *Config) time.Duration {
			return d
		}
	}
	const max = time.Duration(1<<63 - 1)
	for _, c := range []struct {
		label    string
		delays   []time.Duration
		expected time.Duration
	}{
		{
			label: "empty",
		},
		{
			label: "single",
			delays: []time.Duration{
				time.Second,
			},
			expected: time.Second,
		},
		{
			label: "negative",
			delays: []time.Duration{
				time.Second,
				-time.Millisecond,
			},
			expected: time.Second - time.Millisecond,
		},
		{
			label: "overflow",
			delays: []time.Duration{
				max,
				time.Second,
				time.Millisecond,
			},
			expected: max,
		},
	} {
		t.Run(
			c.label,
			func(t *testing.T) {
				funcs := make([]DelayTypeFunc, len(c.delays))
				for i, d := range c.delays {
					funcs[i] = f(d)
				}
				actual := CombineDelay(funcs...)(0, nil, nil)
				if c.expected != actual {
					t.Errorf("delay duration mismatch: got %v, want %v", actual, c.expected)
				}
			},
		)
	}
}

func TestContext(t *testing.T) {
	const defaultDelay = 100 * time.Millisecond
	t.Run("cancel before", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		retrySum := 0
		start := time.Now()
		err := Do(
			func() error { return errors.New("test") },
			OnRetry(func(n uint, err error) { retrySum += 1 }),
			Context(ctx),
		)
		dur := time.Since(start)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !(dur < defaultDelay) {
			t.Errorf("cancellation timing: got duration=%v, want <defaultDelay=%v", dur, defaultDelay)
		}
		if retrySum != 0 {
			t.Errorf("retry count: got %d, want 0", retrySum)
		}
	})

	t.Run("cancel in retry progress", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		retrySum := 0
		err := Do(
			func() error { return errors.New("test") },
			OnRetry(func(n uint, err error) {
				retrySum += 1
				if retrySum > 1 {
					cancel()
				}
			}),
			Context(ctx),
		)
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		expectedErrorFormat := `All attempts fail:
#1: test
#2: test
#3: context canceled`
		if retryErr, ok := err.(Error); ok {
			if len(retryErr) != 3 {
				t.Errorf("expected len=%d, got %d", 3, len(retryErr))
			}
		} else {
			t.Fatalf("expected Error type, got %T", err)
		}
		if expectedErrorFormat != err.Error() {
			t.Errorf("error message: got %q, want %q", err.Error(), expectedErrorFormat)
		}
		if retrySum != 2 {
			t.Errorf("retry count: got %d, want 2", retrySum)
		}
	})

	t.Run("cancel in retry progress - last error only", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		retrySum := 0
		err := Do(
			func() error { return errors.New("test") },
			OnRetry(func(n uint, err error) {
				retrySum += 1
				if retrySum > 1 {
					cancel()
				}
			}),
			Context(ctx),
			LastErrorOnly(true),
		)
		if context.Canceled != err {
			t.Errorf("error: got %v, want %v", err, context.Canceled)
		}

		if retrySum != 2 {
			t.Errorf("retry count: got %d, want 2", retrySum)
		}
	})

	t.Run("cancel in retry progress - infinite attempts", func(t *testing.T) {
		go func() {
			ctx, cancel := context.WithCancel(context.Background())

			retrySum := 0
			err := Do(
				func() error { return errors.New("test") },
				OnRetry(func(n uint, err error) {
					fmt.Println(n)
					retrySum += 1
					if retrySum > 1 {
						cancel()
					}
				}),
				LastErrorOnly(true),
				Context(ctx),
				Attempts(0),
			)

			if context.Canceled != err {
				t.Errorf("error: got %v, want %v", err, context.Canceled)
			}

			if retrySum != 2 {
				t.Errorf("retry count: got %d, want 2", retrySum)
			}
		}()
	})

	t.Run("cancelled on retry infinte attempts - wraps context error with last retried function error", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		retrySum := 0
		err := Do(
			func() error { return fooErr{str: fmt.Sprintf("error %d", retrySum+1)} },
			OnRetry(func(n uint, err error) {
				retrySum += 1
				if retrySum == 2 {
					cancel()
				}
			}),
			Context(ctx),
			Attempts(0),
			WrapContextErrorWithLastError(true),
		)
		if !errors.Is(err, context.Canceled) {
			t.Errorf("errors.Is(err, context.Canceled): got false, want true")
		}
		if !errors.Is(err, fooErr{str: "error 2"}) {
			t.Errorf("errors.Is(err, last function error): got false, want true")
		}
	})

	t.Run("timed out on retry infinte attempts - wraps context error with last retried function error", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*200)
		defer cancel()

		retrySum := 0
		err := Do(
			func() error { return fooErr{str: fmt.Sprintf("error %d", retrySum+1)} },
			OnRetry(func(n uint, err error) {
				retrySum += 1
			}),
			Context(ctx),
			Attempts(0),
			WrapContextErrorWithLastError(true),
		)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("errors.Is(err, context.DeadlineExceeded): got false, want true")
		}
		if !errors.Is(err, fooErr{str: "error 2"}) {
			t.Errorf("errors.Is(err, last function error): got false, want true")
		}
	})
}

type testTimer struct {
	called bool
}

func (t *testTimer) After(d time.Duration) <-chan time.Time {
	t.called = true
	return time.After(d)
}

func TestTimerInterface(t *testing.T) {
	var timer testTimer
	err := Do(
		func() error { return errors.New("test") },
		Attempts(1),
		Delay(10*time.Millisecond),
		MaxDelay(50*time.Millisecond),
		WithTimer(&timer),
	)

	if err == nil {
		t.Fatal("expected error, got nil")
	}

}

func TestErrorIs(t *testing.T) {
	var e Error
	expectErr := errors.New("error")
	closedErr := os.ErrClosed
	e = append(e, expectErr)
	e = append(e, closedErr)

	if !errors.Is(e, expectErr) {
		t.Error("IsRecoverable(err) = false, want true")
	}
	if !errors.Is(e, closedErr) {
		t.Error("IsRecoverable(err) = false, want true")
	}
	if errors.Is(e, errors.New("error")) {
		t.Error("IsRecoverable(err) = true, want false")
	}
}

type fooErr struct{ str string }

func (e fooErr) Error() string {
	return e.str
}

type barErr struct{ str string }

func (e barErr) Error() string {
	return e.str
}

func TestErrorAs(t *testing.T) {
	var e Error
	fe := fooErr{str: "foo"}
	e = append(e, fe)

	var tf fooErr
	var tb barErr

	if !errors.As(e, &tf) {
		t.Error("IsRecoverable(err) = false, want true")
	}
	if errors.As(e, &tb) {
		t.Error("IsRecoverable(err) = true, want false")
	}
	if "foo" != tf.str {
		t.Errorf("fooErr.str: got %q, want %q", tf.str, "foo")
	}
}

func TestUnwrap(t *testing.T) {
	testError := errors.New("test error")
	err := Do(
		func() error {
			return testError
		},
		Attempts(1),
	)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if testError != errors.Unwrap(err) {
		t.Errorf("unwrapped error: got %v, want %v", errors.Unwrap(err), testError)
	}
}

func BenchmarkDo(b *testing.B) {
	testError := errors.New("test error")

	for i := 0; i < b.N; i++ {
		_ = Do(
			func() error {
				return testError
			},
			Attempts(10),
			Delay(0),
		)
	}
}

func BenchmarkDoWithData(b *testing.B) {
	testError := errors.New("test error")

	for i := 0; i < b.N; i++ {
		_, _ = DoWithData(
			func() (int, error) {
				return 0, testError
			},
			Attempts(10),
			Delay(0),
		)
	}
}

func BenchmarkDoNoErrors(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = Do(
			func() error {
				return nil
			},
			Attempts(10),
			Delay(0),
		)
	}
}

func BenchmarkDoWithDataNoErrors(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = DoWithData(
			func() (int, error) {
				return 0, nil
			},
			Attempts(10),
			Delay(0),
		)
	}
}

type attemptsForErrorTestError struct{}

func (attemptsForErrorTestError) Error() string { return "test error" }

func TestAttemptsForErrorNoDelayAfterFinalAttempt(t *testing.T) {
	var count uint64
	var timestamps []time.Time

	startTime := time.Now()

	err := Do(
		func() error {
			count++
			timestamps = append(timestamps, time.Now())
			return attemptsForErrorTestError{}
		},
		Attempts(3),
		Delay(200*time.Millisecond),
		DelayType(FixedDelay),
		AttemptsForError(2, attemptsForErrorTestError{}),
		LastErrorOnly(true),
		Context(context.Background()),
	)

	endTime := time.Now()

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if uint64(2) != count {
		t.Errorf("attempt count: got %d, want 2", count)
	}
	if len(timestamps) != 2 {
		t.Errorf("timestamp count: got %d, want 2", len(timestamps))
	}

	// Verify timing: first attempt at ~0ms, second at ~200ms, end immediately after second attempt
	firstAttemptTime := timestamps[0].Sub(startTime)
	secondAttemptTime := timestamps[1].Sub(startTime)
	totalTime := endTime.Sub(startTime)

	// First attempt should be immediate
	if firstAttemptTime >= 50*time.Millisecond {
		t.Errorf("first attempt timing: got %v, want <50ms (should be immediate)", firstAttemptTime)
	}

	// Second attempt should be after delay
	if secondAttemptTime <= 150*time.Millisecond {
		t.Errorf("second attempt timing: got %v, want >150ms (should be after 200ms delay)", secondAttemptTime)
	}
	if secondAttemptTime >= 250*time.Millisecond {
		t.Errorf("second attempt timing: got %v, want <250ms", secondAttemptTime)
	}

	// Total time should not include delay after final attempt
	if totalTime >= 300*time.Millisecond {
		t.Errorf("total duration: got %v, want <300ms (no delay after final attempt)", totalTime)
	}
}

func TestOnRetryNotCalledOnLastAttempt(t *testing.T) {
	callCount := 0
	onRetryCalls := make([]uint, 0)

	err := Do(
		func() error {
			callCount++
			return errors.New("test error")
		},
		Attempts(3),
		OnRetry(func(n uint, err error) {
			onRetryCalls = append(onRetryCalls, n)
		}),
		Delay(time.Nanosecond),
	)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if callCount != 3 {
		t.Errorf("function call count: got %d, want 3", callCount)
	}
	if !reflect.DeepEqual(onRetryCalls, []uint{0, 1}) {
		t.Errorf("onRetry calls: got %v, want %v (should not be called on final attempt)", onRetryCalls, []uint{0, 1})
	}
	if len(onRetryCalls) != 2 {
		t.Errorf("onRetry call count: got %d, want 2 (not called on last attempt)", len(onRetryCalls))
	}
}

func TestIsRecoverable(t *testing.T) {
	err := errors.New("err")
	if !IsRecoverable(err) {
		t.Error("IsRecoverable(err) = false, want true")
	}

	err = Unrecoverable(err)
	if IsRecoverable(err) {
		t.Error("IsRecoverable(err) = true, want false")
	}

	err = fmt.Errorf("wrapping: %w", err)
	if IsRecoverable(err) {
		t.Error("IsRecoverable(err) = true, want false")
	}
}

func TestFullJitterBackoffDelay(t *testing.T) {
	// Seed for predictable randomness in tests
	// In real usage, math/rand is auto-seeded in Go 1.20+ or should be seeded once at program start.
	// For library test predictability, local seeding is fine.
	// However, retry-go's RandomDelay uses global math/rand without explicit seeding in tests.
	// Let's follow the existing pattern of not explicitly seeding in each test for now,
	// assuming test runs are isolated enough or that exact delay values aren't asserted,
	// but rather ranges or properties.

	baseDelay := 50 * time.Millisecond
	maxDelay := 500 * time.Millisecond

	config := &Config{
		delay:    baseDelay,
		maxDelay: maxDelay,
		// other fields can be zero/default for this test
	}

	attempts := []uint{0, 1, 2, 3, 4, 5, 6, 10}

	for _, n := range attempts {
		delay := FullJitterBackoffDelay(n, errors.New("test error"), config)

		expectedMaxCeiling := float64(baseDelay) * math.Pow(2, float64(n))
		if expectedMaxCeiling > float64(maxDelay) {
			expectedMaxCeiling = float64(maxDelay)
		}

		if delay < 0 {
			t.Errorf("Delay should be non-negative. Got: %v for attempt %d", delay, n)
		}
		if delay > time.Duration(expectedMaxCeiling) {
			t.Errorf("Delay %v should be less than or equal to current backoff ceiling %v for attempt %d", delay, time.Duration(expectedMaxCeiling), n)
		}

		t.Logf("Attempt %d: BaseDelay=%v, MaxDelay=%v, Calculated Ceiling=~%v, Actual Delay=%v",
			n, baseDelay, maxDelay, time.Duration(expectedMaxCeiling), delay)

		// Test with MaxDelay disabled (0)
		configNoMax := &Config{delay: baseDelay, maxDelay: 0}
		delayNoMax := FullJitterBackoffDelay(n, errors.New("test error"), configNoMax)
		expectedCeilingNoMax := float64(baseDelay) * math.Pow(2, float64(n))
		if expectedCeilingNoMax > float64(10*time.Minute) { // Avoid overflow for very large N
			expectedCeilingNoMax = float64(10 * time.Minute)
		}
		if delayNoMax < 0 {
			t.Errorf("Delay (no max) should be non-negative. Got: %v for attempt %d", delayNoMax, n)
		}
		if delayNoMax > time.Duration(expectedCeilingNoMax) {
			t.Errorf("Delay (no max) %v should be less than or equal to current backoff ceiling %v for attempt %d", delayNoMax, time.Duration(expectedCeilingNoMax), n)
		}
	}

	// Test case where baseDelay might be zero
	configZeroBase := &Config{delay: 0, maxDelay: maxDelay}
	delayZeroBase := FullJitterBackoffDelay(0, errors.New("test error"), configZeroBase)
	if time.Duration(0) != delayZeroBase {
		t.Errorf("delay with zero base: got %v, want 0", delayZeroBase)
	}

	delayZeroBaseAttempt1 := FullJitterBackoffDelay(1, errors.New("test error"), configZeroBase)
	if time.Duration(0) != delayZeroBaseAttempt1 {
		t.Errorf("delay with zero base (attempt>0): got %v, want 0", delayZeroBaseAttempt1)
	}

	// Test with very small base delay
	smallBaseDelay := 1 * time.Nanosecond
	configSmallBase := &Config{delay: smallBaseDelay, maxDelay: 100 * time.Nanosecond}
	for i := uint(0); i < 5; i++ {
		d := FullJitterBackoffDelay(i, errors.New("test"), configSmallBase)
		ceil := float64(smallBaseDelay) * math.Pow(2, float64(i))
		if ceil > 100 {
			ceil = 100
		}
		if d > time.Duration(ceil) {
			t.Errorf("delay ceiling: got %v, want <=%v", d, time.Duration(ceil))
		}
	}
}

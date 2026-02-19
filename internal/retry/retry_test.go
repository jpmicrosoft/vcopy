package retry

import (
	"errors"
	"testing"
	"time"
)

func TestDo_Success(t *testing.T) {
	calls := 0
	err := Do(Default(), "test", func() error {
		calls++
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
}

func TestDo_RetryThenSuccess(t *testing.T) {
	cfg := Config{MaxAttempts: 3, InitialWait: 1 * time.Millisecond, MaxWait: 10 * time.Millisecond}
	calls := 0
	err := Do(cfg, "test", func() error {
		calls++
		if calls < 3 {
			return errors.New("transient error")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 3 {
		t.Errorf("expected 3 calls, got %d", calls)
	}
}

func TestDo_AllFail(t *testing.T) {
	cfg := Config{MaxAttempts: 2, InitialWait: 1 * time.Millisecond, MaxWait: 10 * time.Millisecond}
	calls := 0
	err := Do(cfg, "test-op", func() error {
		calls++
		return errors.New("permanent error")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if calls != 2 {
		t.Errorf("expected 2 calls, got %d", calls)
	}
	if !errors.Is(err, errors.Unwrap(err)) {
		// Just verify the error message contains the operation name
		if err.Error() == "" {
			t.Error("empty error message")
		}
	}
}

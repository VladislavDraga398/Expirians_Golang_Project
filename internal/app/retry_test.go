package app

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRetryWithDeadline_SucceedsAfterRetries(t *testing.T) {
	t.Parallel()

	attempts := 0
	err := retryWithDeadline(
		context.Background(),
		nil,
		"test-op",
		100*time.Millisecond,
		5*time.Millisecond,
		func() error {
			attempts++
			if attempts < 3 {
				return errors.New("temporary failure")
			}
			return nil
		},
	)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

func TestRetryWithDeadline_Timeout(t *testing.T) {
	t.Parallel()

	attempts := 0
	err := retryWithDeadline(
		context.Background(),
		nil,
		"timeout-op",
		20*time.Millisecond,
		5*time.Millisecond,
		func() error {
			attempts++
			return errors.New("always failing")
		},
	)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if attempts < 2 {
		t.Fatalf("expected at least 2 attempts before timeout, got %d", attempts)
	}
}

func TestRetryWithDeadline_ContextCanceledBeforeStart(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	called := 0
	err := retryWithDeadline(
		ctx,
		nil,
		"canceled-op",
		time.Second,
		10*time.Millisecond,
		func() error {
			called++
			return nil
		},
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
	if called != 0 {
		t.Fatalf("expected fn not to be called, got %d calls", called)
	}
}

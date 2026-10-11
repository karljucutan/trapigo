package database

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestPingWithRetryRetriesUntilSuccess(t *testing.T) {
	attempts := 0
	ping := func(context.Context) error {
		attempts++
		if attempts < 3 {
			return errors.New("database unavailable")
		}
		return nil
	}

	if err := pingWithRetry(context.Background(), ping, 0); err != nil {
		t.Fatalf("pingWithRetry() error = %v", err)
	}
	if attempts != 3 {
		t.Fatalf("ping attempts = %d, want 3", attempts)
	}
}

func TestPingWithRetryStopsWhenContextIsCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	attempts := 0
	ping := func(context.Context) error {
		attempts++
		return errors.New("database unavailable")
	}

	if err := pingWithRetry(ctx, ping, time.Hour); err == nil {
		t.Fatal("pingWithRetry() error = nil, want error")
	}
	if attempts != 1 {
		t.Fatalf("ping attempts = %d, want 1", attempts)
	}
}

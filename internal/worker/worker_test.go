package worker_test

import (
	"context"
	"testing"
	"time"

	"github.com/Sprained/notify-system/internal/message"
	"github.com/Sprained/notify-system/internal/notifier/lognotifier"
	"github.com/Sprained/notify-system/internal/subscriber"
	"github.com/Sprained/notify-system/internal/worker"
)

func noSleep(time.Duration) {}

func TestDeliver_SucceedsOnFirstAttempt(t *testing.T) {
	n := lognotifier.New(0)
	attempts, err := worker.Deliver(context.Background(), n, subscriber.Subscriber{}, message.Message{}, noSleep)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if attempts != 1 {
		t.Errorf("expected 1 attempt, got %d", attempts)
	}
}

func TestDeliver_RetriesAndEventuallySucceeds(t *testing.T) {
	n := lognotifier.New(2) // falha 2x, sucede na 3ª
	attempts, err := worker.Deliver(context.Background(), n, subscriber.Subscriber{}, message.Message{}, noSleep)
	if err != nil {
		t.Fatalf("expected eventual success, got error: %v", err)
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestDeliver_GivesUpAfterMaxAttempts(t *testing.T) {
	n := lognotifier.New(10) // falha mais vezes do que o worker vai tentar
	attempts, err := worker.Deliver(context.Background(), n, subscriber.Subscriber{}, message.Message{}, noSleep)
	if err == nil {
		t.Fatal("expected error after exhausting attempts, got nil")
	}
	if attempts != worker.MaxAttempts {
		t.Errorf("expected %d attempts, got %d", worker.MaxAttempts, attempts)
	}
}

func TestDeliver_BackoffGrowsExponentially(t *testing.T) {
	n := lognotifier.New(10)
	var waits []time.Duration
	sleep := func(d time.Duration) { waits = append(waits, d) }

	worker.Deliver(context.Background(), n, subscriber.Subscriber{}, message.Message{}, sleep)

	want := []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second}
	if len(waits) != len(want) {
		t.Fatalf("expected %d waits, got %d: %v", len(want), len(waits), waits)
	}
	for i := range want {
		if waits[i] != want[i] {
			t.Errorf("wait %d: expected %v, got %v", i, want[i], waits[i])
		}
	}
}

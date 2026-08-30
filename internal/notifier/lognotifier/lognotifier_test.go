package lognotifier_test

import (
	"context"
	"testing"

	"github.com/Sprained/notify-system/internal/message"
	"github.com/Sprained/notify-system/internal/notifier/lognotifier"
	"github.com/Sprained/notify-system/internal/subscriber"
)

func TestKind(t *testing.T) {
	n := lognotifier.New(0)
	if n.Kind() != "log" {
		t.Errorf("expected kind %q, got %q", "log", n.Kind())
	}
}

func TestSend_SucceedsImmediatelyWhenFailTimesIsZero(t *testing.T) {
	n := lognotifier.New(0)
	if err := n.Send(context.Background(), subscriber.Subscriber{}, message.Message{}); err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
}

func TestSend_FailsConfiguredNumberOfTimesThenSucceeds(t *testing.T) {
	n := lognotifier.New(2)

	if err := n.Send(context.Background(), subscriber.Subscriber{}, message.Message{}); err == nil {
		t.Error("expected failure on 1st call")
	}
	if err := n.Send(context.Background(), subscriber.Subscriber{}, message.Message{}); err == nil {
		t.Error("expected failure on 2nd call")
	}
	if err := n.Send(context.Background(), subscriber.Subscriber{}, message.Message{}); err != nil {
		t.Errorf("expected success on 3rd call, got error: %v", err)
	}
}

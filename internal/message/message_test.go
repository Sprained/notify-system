package message_test

import (
	"testing"

	"github.com/Sprained/notify-system/internal/message"
)

func TestNew_ValidMessage(t *testing.T) {
	msg, err := message.New("alerts", "Title", "corpo da mensagem", 3, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.Topic != "alerts" {
		t.Errorf("expected topic %q, got %q", "alerts", msg.Topic)
	}
	if msg.Body != "corpo da mensagem" {
		t.Errorf("expected body preserved, got %q", msg.Body)
	}
	if msg.Priority != 3 {
		t.Errorf("expected priority 3, got %d", msg.Priority)
	}
}

func TestNew_GeneratesValidULID(t *testing.T) {
	msg, err := message.New("alerts", "", "corpo", 3, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msg.ID) != 26 {
		t.Errorf("expected ULID with 26 chars, got %d chars (%q)", len(msg.ID), msg.ID)
	}
}

func TestNew_EmptyBodyIsRejected(t *testing.T) {
	_, err := message.New("alerts", "Title", "", 3, nil, "")
	if err == nil {
		t.Fatal("expected error for empty body, got nil")
	}
}

func TestNew_BlankBodyIsRejected(t *testing.T) {
	_, err := message.New("alerts", "Title", "   ", 3, nil, "")
	if err == nil {
		t.Fatal("expected error for whitespace-only body, got nil")
	}
}

func TestNew_PriorityBelowRangeClampsToMinimum(t *testing.T) {
	msg, err := message.New("alerts", "", "corpo", 0, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.Priority != 1 {
		t.Errorf("expected priority clamped to 1, got %d", msg.Priority)
	}
}

func TestNew_PriorityAboveRangeClampsToMaximum(t *testing.T) {
	msg, err := message.New("alerts", "", "corpo", 9, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.Priority != 5 {
		t.Errorf("expected priority clamped to 5, got %d", msg.Priority)
	}
}

func TestNew_TagsPreserved(t *testing.T) {
	msg, err := message.New("alerts", "", "corpo", 3, []string{"urgente", "casa"}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msg.Tags) != 2 || msg.Tags[0] != "urgente" || msg.Tags[1] != "casa" {
		t.Errorf("expected tags preserved, got %v", msg.Tags)
	}
}

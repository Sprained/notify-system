package postgres_test

import (
	"context"
	"testing"

	"github.com/Sprained/notify-system/internal/message"
	"github.com/Sprained/notify-system/internal/postgres"
)

func TestStore_ListPending_OnlyReturnsPendingDeliveries(t *testing.T) {
	db := migratedDB(t)
	insertTopic(t, db, "topico")
	insertMessage(t, db, "msg-1", "topico")
	insertMessage(t, db, "msg-2", "topico")
	insertMessage(t, db, "msg-3", "topico")
	subID := insertSubscriber(t, db, "telegram")

	pendingID := insertDelivery(t, db, "msg-1", subID, "pending")
	insertDelivery(t, db, "msg-2", subID, "sent")
	insertDelivery(t, db, "msg-3", subID, "failed")

	store := postgres.NewStore(db)
	got, err := store.ListPending(context.Background())
	if err != nil {
		t.Fatalf("ListPending: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("expected 1 pending delivery, got %d: %+v", len(got), got)
	}
	if got[0].ID != pendingID {
		t.Errorf("expected pending delivery id %d, got %d", pendingID, got[0].ID)
	}
	if got[0].Status != "pending" {
		t.Errorf("expected status pending, got %q", got[0].Status)
	}
}

func TestStore_InsertAndGet_RoundTripsTagsCorrectly(t *testing.T) {
	db := migratedDB(t)
	store := postgres.NewStore(db)

	msg, err := message.New("topico", "Título", "corpo", 3, []string{"casa", "urgente"}, "")
	if err != nil {
		t.Fatalf("message.New: %v", err)
	}

	if err := store.Insert(context.Background(), msg); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	got, err := store.Get(context.Background(), msg.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if len(got.Tags) != 2 || got.Tags[0] != "casa" || got.Tags[1] != "urgente" {
		t.Errorf("expected tags [casa urgente], got %v", got.Tags)
	}
}

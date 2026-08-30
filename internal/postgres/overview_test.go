package postgres_test

import (
	"context"
	"testing"

	"github.com/Sprained/notify-system/internal/postgres"
)

func TestStore_CountSentToday_CountsOnlyDeliveriesSentToday(t *testing.T) {
	db := migratedDB(t)
	insertTopic(t, db, "topico")
	insertMessage(t, db, "msg-1", "topico")
	insertMessage(t, db, "msg-2", "topico")
	subID := insertSubscriber(t, db, "telegram")

	store := postgres.NewStore(db)

	sentToday := insertDelivery(t, db, "msg-1", subID, "pending")
	if err := store.MarkSent(context.Background(), sentToday); err != nil {
		t.Fatalf("MarkSent: %v", err)
	}

	sentYesterday := insertDelivery(t, db, "msg-2", subID, "pending")
	if err := store.MarkSent(context.Background(), sentYesterday); err != nil {
		t.Fatalf("MarkSent: %v", err)
	}
	if _, err := db.Exec(`UPDATE delivery SET sent_at = now() - interval '1 day' WHERE id = $1`, sentYesterday); err != nil {
		t.Fatalf("backdate sent_at: %v", err)
	}

	count, err := store.CountSentToday(context.Background())
	if err != nil {
		t.Fatalf("CountSentToday: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 sent today, got %d", count)
	}
}

func TestStore_CountSentToday_ZeroWhenNoneSent(t *testing.T) {
	db := migratedDB(t)
	store := postgres.NewStore(db)

	count, err := store.CountSentToday(context.Background())
	if err != nil {
		t.Fatalf("CountSentToday: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0, got %d", count)
	}
}

func TestStore_CountFailedDeliveries(t *testing.T) {
	db := migratedDB(t)
	insertTopic(t, db, "topico")
	insertMessage(t, db, "msg-1", "topico")
	insertMessage(t, db, "msg-2", "topico")
	insertMessage(t, db, "msg-3", "topico")
	subID := insertSubscriber(t, db, "telegram")

	insertDelivery(t, db, "msg-1", subID, "failed")
	insertDelivery(t, db, "msg-2", subID, "failed")
	insertDelivery(t, db, "msg-3", subID, "sent")

	store := postgres.NewStore(db)
	count, err := store.CountFailedDeliveries(context.Background())
	if err != nil {
		t.Fatalf("CountFailedDeliveries: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 failed, got %d", count)
	}
}

func TestSubscriberStore_CountActive_IgnoresDisabled(t *testing.T) {
	db := migratedDB(t)
	insertSubscriber(t, db, "telegram")
	disabledID := insertSubscriber(t, db, "telegram")
	if _, err := db.Exec(`UPDATE subscriber SET enabled = false WHERE id = $1`, disabledID); err != nil {
		t.Fatalf("disable subscriber: %v", err)
	}

	store := postgres.NewSubscriberStore(db)
	count, err := store.CountActive(context.Background())
	if err != nil {
		t.Fatalf("CountActive: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 active, got %d", count)
	}
}

func TestStore_Recent_ReturnsLatestMessagesAcrossTopics(t *testing.T) {
	db := migratedDB(t)
	insertTopic(t, db, "topico-a")
	insertTopic(t, db, "topico-b")
	insertMessage(t, db, "01J0000000000000000000001", "topico-a")
	insertMessage(t, db, "01J0000000000000000000002", "topico-b")
	insertMessage(t, db, "01J0000000000000000000003", "topico-a")

	store := postgres.NewStore(db)
	got, err := store.Recent(context.Background(), 2)
	if err != nil {
		t.Fatalf("Recent: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(got))
	}
	if got[0].ID != "01J0000000000000000000003" || got[1].ID != "01J0000000000000000000002" {
		t.Errorf("expected most recent first, got %v", []string{got[0].ID, got[1].ID})
	}
}

func TestStore_Recent_EmptyWhenNoMessages(t *testing.T) {
	db := migratedDB(t)
	store := postgres.NewStore(db)

	got, err := store.Recent(context.Background(), 5)
	if err != nil {
		t.Fatalf("Recent: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected no messages, got %d", len(got))
	}
}

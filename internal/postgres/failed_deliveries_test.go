package postgres_test

import (
	"context"
	"testing"

	"github.com/Sprained/notify-system/internal/postgres"
)

func TestStore_ListFailed_ReturnsMessageTitleAndSubscriberLabel(t *testing.T) {
	db := migratedDB(t)
	insertTopic(t, db, "topico")
	insertMessage(t, db, "msg-1", "topico")
	if _, err := db.Exec(`UPDATE message SET title = $1 WHERE id = $2`, "Disco cheio", "msg-1"); err != nil {
		t.Fatalf("set title: %v", err)
	}
	subID := insertSubscriber(t, db, "telegram")
	if _, err := db.Exec(`UPDATE subscriber SET label = $1 WHERE id = $2`, "Gabriel", subID); err != nil {
		t.Fatalf("set label: %v", err)
	}

	failedID := insertDelivery(t, db, "msg-1", subID, "pending")
	if _, err := db.Exec(
		`UPDATE delivery SET status = 'failed', attempts = 4, last_error = $1 WHERE id = $2`,
		"telegram: 403 forbidden", failedID,
	); err != nil {
		t.Fatalf("update to failed: %v", err)
	}

	store := postgres.NewStore(db)
	got, err := store.ListFailed(context.Background())
	if err != nil {
		t.Fatalf("ListFailed: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 failed delivery, got %d", len(got))
	}
	fd := got[0]
	if fd.MessageTitle != "Disco cheio" {
		t.Errorf("expected message title %q, got %q", "Disco cheio", fd.MessageTitle)
	}
	if fd.SubscriberLabel != "Gabriel" {
		t.Errorf("expected subscriber label %q, got %q", "Gabriel", fd.SubscriberLabel)
	}
	if fd.Attempts != 4 {
		t.Errorf("expected attempts 4, got %d", fd.Attempts)
	}
	if fd.LastError != "telegram: 403 forbidden" {
		t.Errorf("expected last_error %q, got %q", "telegram: 403 forbidden", fd.LastError)
	}
}

func TestStore_ListFailed_IgnoresPendingAndSent(t *testing.T) {
	db := migratedDB(t)
	insertTopic(t, db, "topico")
	insertMessage(t, db, "msg-1", "topico")
	insertMessage(t, db, "msg-2", "topico")
	subID := insertSubscriber(t, db, "telegram")

	insertDelivery(t, db, "msg-1", subID, "pending")
	insertDelivery(t, db, "msg-2", subID, "sent")

	store := postgres.NewStore(db)
	got, err := store.ListFailed(context.Background())
	if err != nil {
		t.Fatalf("ListFailed: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected no failed deliveries, got %d", len(got))
	}
}

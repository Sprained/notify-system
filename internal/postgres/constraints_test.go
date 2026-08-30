package postgres_test

import (
	"database/sql"
	"testing"
)

func insertTopic(t *testing.T, db *sql.DB, name string) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO topic (name) VALUES ($1)`, name)
	if err != nil {
		t.Fatalf("insert topic: %v", err)
	}
}

func insertMessage(t *testing.T, db *sql.DB, id, topic string) {
	t.Helper()
	_, err := db.Exec(
		`INSERT INTO message (id, topic, body, priority) VALUES ($1, $2, $3, $4)`,
		id, topic, "body", 3,
	)
	if err != nil {
		t.Fatalf("insert message: %v", err)
	}
}

func insertSubscriber(t *testing.T, db *sql.DB, kind string) int64 {
	t.Helper()
	var id int64
	err := db.QueryRow(
		`INSERT INTO subscriber (kind, config) VALUES ($1, '{}') RETURNING id`,
		kind,
	).Scan(&id)
	if err != nil {
		t.Fatalf("insert subscriber: %v", err)
	}
	return id
}

func migratedDB(t *testing.T) *sql.DB {
	t.Helper()
	db := setupPostgres(t)
	m := newMigrator(t, db)
	if err := m.Up(); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	return db
}

func TestDelivery_ForeignKey_MessageMustExist(t *testing.T) {
	db := migratedDB(t)
	subID := insertSubscriber(t, db, "telegram")

	_, err := db.Exec(
		`INSERT INTO delivery (message_id, subscriber_id) VALUES ($1, $2)`,
		"nonexistent-message", subID,
	)
	if err == nil {
		t.Fatal("expected error inserting delivery with nonexistent message_id, got nil")
	}
}

func TestDelivery_UniqueIndex_MessageSubscriberPair(t *testing.T) {
	db := migratedDB(t)
	insertTopic(t, db, "topico")
	insertMessage(t, db, "msg-1", "topico")
	subID := insertSubscriber(t, db, "telegram")

	_, err := db.Exec(
		`INSERT INTO delivery (message_id, subscriber_id) VALUES ($1, $2)`,
		"msg-1", subID,
	)
	if err != nil {
		t.Fatalf("first insert should succeed: %v", err)
	}

	_, err = db.Exec(
		`INSERT INTO delivery (message_id, subscriber_id) VALUES ($1, $2)`,
		"msg-1", subID,
	)
	if err == nil {
		t.Fatal("expected error inserting duplicate (message_id, subscriber_id), got nil")
	}
}

func TestSubscriber_Kind_CheckConstraint(t *testing.T) {
	db := migratedDB(t)

	for _, kind := range []string{"telegram", "fcm", "webpush"} {
		_, err := db.Exec(`INSERT INTO subscriber (kind, config) VALUES ($1, '{}')`, kind)
		if err != nil {
			t.Errorf("expected kind %q to be accepted, got error: %v", kind, err)
		}
	}

	_, err := db.Exec(`INSERT INTO subscriber (kind, config) VALUES ($1, '{}')`, "carrier-pigeon")
	if err == nil {
		t.Fatal("expected error inserting subscriber with invalid kind, got nil")
	}
}

func TestDelivery_Status_CheckConstraint(t *testing.T) {
	db := migratedDB(t)
	insertTopic(t, db, "topico")
	insertMessage(t, db, "msg-1", "topico")
	subID := insertSubscriber(t, db, "telegram")

	for _, status := range []string{"pending", "sent", "failed"} {
		_, err := db.Exec(
			`INSERT INTO delivery (message_id, subscriber_id, status) VALUES ($1, $2, $3)`,
			"msg-1", subID, status,
		)
		if err != nil {
			t.Errorf("expected status %q to be accepted, got error: %v", status, err)
		}
		_, _ = db.Exec(`DELETE FROM delivery WHERE message_id = $1 AND subscriber_id = $2`, "msg-1", subID)
	}

	_, err := db.Exec(
		`INSERT INTO delivery (message_id, subscriber_id, status) VALUES ($1, $2, $3)`,
		"msg-1", subID, "in-orbit",
	)
	if err == nil {
		t.Fatal("expected error inserting delivery with invalid status, got nil")
	}
}

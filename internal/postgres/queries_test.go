package postgres_test

import (
	"database/sql"
	"testing"
)

func insertDelivery(t *testing.T, db *sql.DB, messageID string, subscriberID int64, status string) int64 {
	t.Helper()
	var id int64
	err := db.QueryRow(
		`INSERT INTO delivery (message_id, subscriber_id, status) VALUES ($1, $2, $3) RETURNING id`,
		messageID, subscriberID, status,
	).Scan(&id)
	if err != nil {
		t.Fatalf("insert delivery: %v", err)
	}
	return id
}

func TestQuery_MessagesSinceCursor(t *testing.T) {
	db := migratedDB(t)
	insertTopic(t, db, "topico-a")
	insertTopic(t, db, "topico-b")
	insertMessage(t, db, "msg-0001", "topico-a")
	insertMessage(t, db, "msg-0002", "topico-a")
	insertMessage(t, db, "msg-0003", "topico-a")
	insertMessage(t, db, "msg-0004", "topico-b") // outro tópico, não deve aparecer

	rows, err := db.Query(
		`SELECT id FROM message WHERE topic = $1 AND id > $2 ORDER BY id ASC LIMIT $3`,
		"topico-a", "msg-0001", 10,
	)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()

	var got []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got = append(got, id)
	}

	want := []string{"msg-0002", "msg-0003"}
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("expected %v, got %v", want, got)
			break
		}
	}
}

func TestQuery_FailedDeliveries(t *testing.T) {
	db := migratedDB(t)
	insertTopic(t, db, "topico")
	insertMessage(t, db, "msg-1", "topico")
	insertMessage(t, db, "msg-2", "topico")
	subID := insertSubscriber(t, db, "telegram")

	insertDelivery(t, db, "msg-1", subID, "sent")
	failedID := insertDelivery(t, db, "msg-2", subID, "pending") // vira failed abaixo
	_, err := db.Exec(
		`UPDATE delivery SET status = 'failed', last_error = $1 WHERE id = $2`,
		"telegram: 403 forbidden", failedID,
	)
	if err != nil {
		t.Fatalf("update to failed: %v", err)
	}

	rows, err := db.Query(
		`SELECT d.id, d.last_error FROM delivery d WHERE d.status = 'failed'`,
	)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var id int64
		var lastErr string
		if err := rows.Scan(&id, &lastErr); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if id != failedID {
			t.Errorf("expected failed delivery id %d, got %d", failedID, id)
		}
		if lastErr != "telegram: 403 forbidden" {
			t.Errorf("unexpected last_error: %q", lastErr)
		}
		count++
	}
	if count != 1 {
		t.Fatalf("expected 1 failed delivery, got %d", count)
	}
}

func TestQuery_UnreadCount(t *testing.T) {
	db := migratedDB(t)
	insertTopic(t, db, "topico")
	insertMessage(t, db, "msg-1", "topico")
	insertMessage(t, db, "msg-2", "topico")
	insertMessage(t, db, "msg-3", "topico")
	subID := insertSubscriber(t, db, "telegram")

	insertDelivery(t, db, "msg-1", subID, "sent")
	insertDelivery(t, db, "msg-2", subID, "sent")
	insertDelivery(t, db, "msg-3", subID, "pending") // não conta, ainda não foi entregue

	_, err := db.Exec(
		`INSERT INTO message_read (message_id, subscriber_id) VALUES ($1, $2)`,
		"msg-1", subID,
	)
	if err != nil {
		t.Fatalf("insert message_read: %v", err)
	}

	var count int
	err = db.QueryRow(`
		SELECT count(*)
		FROM delivery d
		WHERE d.subscriber_id = $1
		  AND d.status = 'sent'
		  AND NOT EXISTS (
		      SELECT 1 FROM message_read mr
		      WHERE mr.message_id = d.message_id AND mr.subscriber_id = d.subscriber_id
		  )`,
		subID,
	).Scan(&count)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 unread, got %d", count)
	}
}

func matchingSubscribers(t *testing.T, db *sql.DB, topic string, priority int) []int64 {
	t.Helper()
	rows, err := db.Query(`
		SELECT s.id
		FROM route r
		JOIN subscriber s ON s.id = r.subscriber_id
		WHERE r.topic = $1
		  AND r.enabled = true
		  AND s.enabled = true
		  AND r.min_priority <= $2`,
		topic, priority,
	)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()

	var got []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got = append(got, id)
	}
	return got
}

func TestRouting_MatchesEnabledRouteWithinPriority(t *testing.T) {
	db := migratedDB(t)
	insertTopic(t, db, "topico")
	subID := insertSubscriber(t, db, "telegram")
	_, err := db.Exec(
		`INSERT INTO route (topic, subscriber_id, min_priority, enabled) VALUES ($1, $2, $3, true)`,
		"topico", subID, 3,
	)
	if err != nil {
		t.Fatalf("insert route: %v", err)
	}

	got := matchingSubscribers(t, db, "topico", 5)
	if len(got) != 1 || got[0] != subID {
		t.Fatalf("expected [%d], got %v", subID, got)
	}
}

func TestRouting_NoMatchWhenNoRoute(t *testing.T) {
	db := migratedDB(t)
	insertTopic(t, db, "topico-sem-rota")

	got := matchingSubscribers(t, db, "topico-sem-rota", 5)
	if len(got) != 0 {
		t.Fatalf("expected no matches, got %v", got)
	}
}

func TestRouting_NoMatchWhenPriorityBelowMinimum(t *testing.T) {
	db := migratedDB(t)
	insertTopic(t, db, "topico")
	subID := insertSubscriber(t, db, "telegram")
	_, err := db.Exec(
		`INSERT INTO route (topic, subscriber_id, min_priority, enabled) VALUES ($1, $2, $3, true)`,
		"topico", subID, 4,
	)
	if err != nil {
		t.Fatalf("insert route: %v", err)
	}

	got := matchingSubscribers(t, db, "topico", 2)
	if len(got) != 0 {
		t.Fatalf("expected no matches, got %v", got)
	}
}

func TestRouting_NoMatchWhenRouteDisabled(t *testing.T) {
	db := migratedDB(t)
	insertTopic(t, db, "topico")
	subID := insertSubscriber(t, db, "telegram")
	_, err := db.Exec(
		`INSERT INTO route (topic, subscriber_id, min_priority, enabled) VALUES ($1, $2, $3, false)`,
		"topico", subID, 1,
	)
	if err != nil {
		t.Fatalf("insert route: %v", err)
	}

	got := matchingSubscribers(t, db, "topico", 5)
	if len(got) != 0 {
		t.Fatalf("expected no matches, got %v", got)
	}
}

func TestRouting_NoMatchWhenSubscriberDisabled(t *testing.T) {
	db := migratedDB(t)
	insertTopic(t, db, "topico")
	subID := insertSubscriber(t, db, "telegram")
	_, err := db.Exec(`UPDATE subscriber SET enabled = false WHERE id = $1`, subID)
	if err != nil {
		t.Fatalf("disable subscriber: %v", err)
	}
	_, err = db.Exec(
		`INSERT INTO route (topic, subscriber_id, min_priority, enabled) VALUES ($1, $2, $3, true)`,
		"topico", subID, 1,
	)
	if err != nil {
		t.Fatalf("insert route: %v", err)
	}

	got := matchingSubscribers(t, db, "topico", 5)
	if len(got) != 0 {
		t.Fatalf("expected no matches, got %v", got)
	}
}

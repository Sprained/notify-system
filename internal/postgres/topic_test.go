package postgres_test

import (
	"context"
	"testing"

	"github.com/Sprained/notify-system/internal/postgres"
	"github.com/Sprained/notify-system/internal/topic"
)

func topicByName(topics []topic.Topic, name string) (topic.Topic, bool) {
	for _, tp := range topics {
		if tp.Name == name {
			return tp, true
		}
	}
	return topic.Topic{}, false
}

func TestTopicStore_List_ReturnsCountsPerTopic(t *testing.T) {
	db := migratedDB(t)
	insertTopic(t, db, "a")
	insertTopic(t, db, "b")
	insertTopic(t, db, "c")
	insertMessage(t, db, "msg-a1", "a")
	insertMessage(t, db, "msg-a2", "a")
	insertMessage(t, db, "msg-c1", "c")
	subID := insertSubscriber(t, db, "telegram")
	if _, err := db.Exec(
		`INSERT INTO route (topic, subscriber_id, min_priority) VALUES ($1, $2, $3)`,
		"a", subID, 3,
	); err != nil {
		t.Fatalf("insert route: %v", err)
	}

	store := postgres.NewTopicStore(db)
	got, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 topics, got %d", len(got))
	}

	a, ok := topicByName(got, "a")
	if !ok || a.MessageCount != 2 || a.RouteCount != 1 {
		t.Errorf("unexpected topic a: %+v (found=%v)", a, ok)
	}
	b, ok := topicByName(got, "b")
	if !ok || b.MessageCount != 0 || b.RouteCount != 0 {
		t.Errorf("unexpected topic b: %+v (found=%v)", b, ok)
	}
	c, ok := topicByName(got, "c")
	if !ok || c.MessageCount != 1 || c.RouteCount != 0 {
		t.Errorf("unexpected topic c: %+v (found=%v)", c, ok)
	}
}

func TestTopicStore_List_EmptyWhenNoTopics(t *testing.T) {
	db := migratedDB(t)
	store := postgres.NewTopicStore(db)

	got, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected no topics, got %d", len(got))
	}
}

func TestTopicStore_Delete_RemovesEmptyTopic(t *testing.T) {
	db := migratedDB(t)
	insertTopic(t, db, "descartavel")

	store := postgres.NewTopicStore(db)
	if err := store.Delete(context.Background(), "descartavel"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	got, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected topic to be gone, got %+v", got)
	}
}

func TestTopicStore_Delete_BlockedWhenMessageExists(t *testing.T) {
	db := migratedDB(t)
	insertTopic(t, db, "com-mensagem")
	insertMessage(t, db, "msg-1", "com-mensagem")

	store := postgres.NewTopicStore(db)
	if err := store.Delete(context.Background(), "com-mensagem"); err == nil {
		t.Fatal("expected error deleting topic with existing message, got nil")
	}
}

func TestTopicStore_Delete_BlockedWhenRouteExists(t *testing.T) {
	db := migratedDB(t)
	insertTopic(t, db, "com-rota")
	subID := insertSubscriber(t, db, "telegram")
	if _, err := db.Exec(
		`INSERT INTO route (topic, subscriber_id, min_priority) VALUES ($1, $2, $3)`,
		"com-rota", subID, 3,
	); err != nil {
		t.Fatalf("insert route: %v", err)
	}

	store := postgres.NewTopicStore(db)
	if err := store.Delete(context.Background(), "com-rota"); err == nil {
		t.Fatal("expected error deleting topic with existing route, got nil")
	}
}

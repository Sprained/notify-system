package postgres_test

import (
	"context"
	"testing"

	"github.com/Sprained/notify-system/internal/postgres"
)

func TestStore_ListRoutes_ReturnsSubscriberLabel(t *testing.T) {
	db := migratedDB(t)
	insertTopic(t, db, "alertas")
	subID := insertSubscriber(t, db, "telegram")
	if _, err := db.Exec(`UPDATE subscriber SET label = $1 WHERE id = $2`, "Gabriel", subID); err != nil {
		t.Fatalf("set label: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO route (topic, subscriber_id, min_priority, enabled) VALUES ($1, $2, $3, true)`,
		"alertas", subID, 3,
	); err != nil {
		t.Fatalf("insert route: %v", err)
	}

	store := postgres.NewStore(db)
	got, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 route, got %d", len(got))
	}
	r := got[0]
	if r.Topic != "alertas" || r.SubscriberLabel != "Gabriel" || r.MinPriority != 3 || !r.Enabled {
		t.Errorf("unexpected route: %+v", r)
	}
}

func TestStore_ListRoutes_EmptyWhenNoRoutes(t *testing.T) {
	db := migratedDB(t)
	store := postgres.NewStore(db)

	got, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected no routes, got %d", len(got))
	}
}

func TestStore_CreateRoute_PersistsAndListable(t *testing.T) {
	db := migratedDB(t)
	insertTopic(t, db, "alertas")
	subID := insertSubscriber(t, db, "telegram")

	store := postgres.NewStore(db)
	if err := store.Create(context.Background(), "alertas", subID, 4); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 route after create, got %d", len(got))
	}
	if got[0].Topic != "alertas" || got[0].SubscriberID != subID || got[0].MinPriority != 4 {
		t.Errorf("unexpected route: %+v", got[0])
	}
}

func TestStore_CreateRoute_NewTopic_CreatesTopicImplicitly(t *testing.T) {
	db := migratedDB(t)
	subID := insertSubscriber(t, db, "telegram")

	store := postgres.NewStore(db)
	if err := store.Create(context.Background(), "topico-novo", subID, 3); err != nil {
		t.Fatalf("Create: %v", err)
	}

	var exists bool
	if err := db.QueryRow(`SELECT EXISTS (SELECT 1 FROM topic WHERE name = $1)`, "topico-novo").Scan(&exists); err != nil {
		t.Fatalf("query topic: %v", err)
	}
	if !exists {
		t.Error("expected topic to be created implicitly")
	}
}

func TestStore_CreateRoute_TwoRoutesSameNewTopic_BothSucceed(t *testing.T) {
	db := migratedDB(t)
	sub1 := insertSubscriber(t, db, "telegram")
	sub2 := insertSubscriber(t, db, "telegram")

	store := postgres.NewStore(db)
	if err := store.Create(context.Background(), "topico-compartilhado", sub1, 3); err != nil {
		t.Fatalf("Create (sub1): %v", err)
	}
	if err := store.Create(context.Background(), "topico-compartilhado", sub2, 4); err != nil {
		t.Fatalf("Create (sub2): %v", err)
	}

	got, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(got))
	}
}

func TestSubscriberStore_List_ReturnsAllSubscribers(t *testing.T) {
	db := migratedDB(t)
	insertSubscriber(t, db, "telegram")
	insertSubscriber(t, db, "telegram")

	store := postgres.NewSubscriberStore(db)
	got, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 subscribers, got %d", len(got))
	}
}

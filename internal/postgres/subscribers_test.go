package postgres_test

import (
	"context"
	"testing"

	"github.com/Sprained/notify-system/internal/postgres"
)

func TestSubscriberStore_Create_PersistsAndListable(t *testing.T) {
	db := migratedDB(t)
	store := postgres.NewSubscriberStore(db)

	err := store.Create(context.Background(), "telegram", map[string]string{"chat_id": "123456789"}, "Gabriel")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 subscriber, got %d", len(got))
	}
	sub := got[0]
	if sub.Kind != "telegram" || sub.Label != "Gabriel" || sub.Config["chat_id"] != "123456789" {
		t.Errorf("unexpected subscriber: %+v", sub)
	}
	if !sub.Enabled {
		t.Errorf("expected subscriber to be enabled by default")
	}
}

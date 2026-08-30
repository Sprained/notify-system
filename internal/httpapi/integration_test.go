package httpapi_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	pgmigrate "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/Sprained/notify-system/internal/httpapi"
	"github.com/Sprained/notify-system/internal/postgres"
)

func setupIntegrationDB(t *testing.T) *sql.DB {
	t.Helper()
	ctx := context.Background()

	container, err := tcpostgres.Run(ctx, "postgres:17-alpine",
		tcpostgres.WithDatabase("notify"),
		tcpostgres.WithUsername("notify"),
		tcpostgres.WithPassword("notify"),
		testcontainers.WithWaitStrategy(wait.ForListeningPort("5432/tcp")),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() {
		if err := container.Terminate(ctx); err != nil {
			t.Logf("terminate container: %v", err)
		}
	})

	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}

	db, err := sql.Open("pgx", connStr)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	driver, err := pgmigrate.WithInstance(db, &pgmigrate.Config{})
	if err != nil {
		t.Fatalf("migrate driver: %v", err)
	}
	m, err := migrate.NewWithDatabaseInstance("file://../postgres/migrations", "postgres", driver)
	if err != nil {
		t.Fatalf("migrate instance: %v", err)
	}
	if err := m.Up(); err != nil {
		t.Fatalf("migrate up: %v", err)
	}

	return db
}

func TestIntegration_PublishEndToEnd(t *testing.T) {
	db := setupIntegrationDB(t)

	_, err := db.Exec(`INSERT INTO topic (name) VALUES ($1)`, "alerts")
	if err != nil {
		t.Fatalf("insert topic: %v", err)
	}
	var subID int64
	err = db.QueryRow(
		`INSERT INTO subscriber (kind, config) VALUES ('telegram', '{}') RETURNING id`,
	).Scan(&subID)
	if err != nil {
		t.Fatalf("insert subscriber: %v", err)
	}
	_, err = db.Exec(
		`INSERT INTO route (topic, subscriber_id, min_priority, enabled) VALUES ($1, $2, 1, true)`,
		"alerts", subID,
	)
	if err != nil {
		t.Fatalf("insert route: %v", err)
	}

	store := postgres.NewStore(db)
	router := httpapi.NewRouter(&httpapi.Handler{
		Messages:   store,
		Routes:     store,
		Deliveries: store,
	})

	req := httptest.NewRequest(http.MethodPost, "/alerts", strings.NewReader("corpo real"))
	req.Header.Set("X-Title", "Alerta de teste")
	req.Header.Set("X-Priority", "5")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var body struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	var dbBody, dbTitle string
	var dbPriority int
	err = db.QueryRow(
		`SELECT body, title, priority FROM message WHERE id = $1`, body.ID,
	).Scan(&dbBody, &dbTitle, &dbPriority)
	if err != nil {
		t.Fatalf("query message: %v", err)
	}
	if dbBody != "corpo real" || dbTitle != "Alerta de teste" || dbPriority != 5 {
		t.Errorf("unexpected message in db: body=%q title=%q priority=%d", dbBody, dbTitle, dbPriority)
	}

	var deliveryCount int
	err = db.QueryRow(
		`SELECT count(*) FROM delivery WHERE message_id = $1 AND subscriber_id = $2 AND status = 'pending'`,
		body.ID, subID,
	).Scan(&deliveryCount)
	if err != nil {
		t.Fatalf("query delivery: %v", err)
	}
	if deliveryCount != 1 {
		t.Fatalf("expected 1 pending delivery, got %d", deliveryCount)
	}
}

func TestIntegration_TopicCreatedImplicitly(t *testing.T) {
	db := setupIntegrationDB(t)
	store := postgres.NewStore(db)
	router := httpapi.NewRouter(&httpapi.Handler{
		Messages:   store,
		Routes:     store,
		Deliveries: store,
	})

	req := httptest.NewRequest(http.MethodPost, "/topico-novo", strings.NewReader("corpo"))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var exists bool
	err := db.QueryRow(
		`SELECT EXISTS (SELECT 1 FROM topic WHERE name = $1)`, "topico-novo",
	).Scan(&exists)
	if err != nil {
		t.Fatalf("check topic: %v", err)
	}
	if !exists {
		t.Error("expected topic to be created implicitly")
	}
}

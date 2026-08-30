package postgres_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	pgmigrate "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setupPostgres(t *testing.T) *sql.DB {
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

	return db
}

func newMigrator(t *testing.T, db *sql.DB) *migrate.Migrate {
	t.Helper()

	driver, err := pgmigrate.WithInstance(db, &pgmigrate.Config{})
	if err != nil {
		t.Fatalf("migrate driver: %v", err)
	}

	m, err := migrate.NewWithDatabaseInstance("file://migrations", "postgres", driver)
	if err != nil {
		t.Fatalf("migrate instance: %v", err)
	}
	return m
}

func TestMigrations_UpOnEmptyDatabase(t *testing.T) {
	db := setupPostgres(t)
	m := newMigrator(t, db)

	if err := m.Up(); err != nil {
		t.Fatalf("migrate up: %v", err)
	}

	tables := []string{"topic", "message", "subscriber", "route", "delivery", "message_read"}
	for _, table := range tables {
		var exists bool
		err := db.QueryRow(
			`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = $1)`,
			table,
		).Scan(&exists)
		if err != nil {
			t.Fatalf("check table %s: %v", table, err)
		}
		if !exists {
			t.Errorf("expected table %q to exist after migration", table)
		}
	}
}

func TestMigrations_DownUndoesCleanly(t *testing.T) {
	db := setupPostgres(t)
	m := newMigrator(t, db)

	if err := m.Up(); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	if err := m.Down(); err != nil {
		t.Fatalf("migrate down: %v", err)
	}

	tables := []string{"topic", "message", "subscriber", "route", "delivery", "message_read"}
	for _, table := range tables {
		var exists bool
		err := db.QueryRow(
			`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = $1)`,
			table,
		).Scan(&exists)
		if err != nil {
			t.Fatalf("check table %s: %v", table, err)
		}
		if exists {
			t.Errorf("expected table %q to not exist after down migration", table)
		}
	}
}

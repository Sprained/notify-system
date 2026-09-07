package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/golang-migrate/migrate/v4"
	pgmigrate "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/Sprained/notify-system/internal/httpapi"
	"github.com/Sprained/notify-system/internal/notifier"
	"github.com/Sprained/notify-system/internal/notifier/lognotifier"
	"github.com/Sprained/notify-system/internal/notifier/telegram"
	"github.com/Sprained/notify-system/internal/postgres"
	"github.com/Sprained/notify-system/internal/webui"
	"github.com/Sprained/notify-system/internal/worker"
)

const workerPollInterval = 10 * time.Second

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://notify:notify@localhost:5432/notify?sslmode=disable"
	}

	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		log.Fatalf("notify: open db: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("notify: ping db: %v", err)
	}

	if err := runMigrations(db); err != nil {
		log.Fatalf("notify: run migrations: %v", err)
	}

	store := postgres.NewStore(db)
	subscribers := postgres.NewSubscriberStore(db)
	topics := postgres.NewTopicStore(db)

	handler := &httpapi.Handler{
		Messages:   store,
		Routes:     store,
		Deliveries: store,
	}
	mux := http.NewServeMux()
	httpapi.RegisterRoutes(mux, handler)
	webui.RegisterRoutes(mux, &webui.Handler{
		Messages:    store,
		Deliveries:  store,
		Subscribers: subscribers,
		Routes:      store,
		Topics:      topics,
	})

	notifiers := map[string]notifier.Notifier{
		"log": lognotifier.New(0),
	}
	if token := os.Getenv("TELEGRAM_BOT_TOKEN"); token != "" {
		notifiers["telegram"] = &telegram.Notifier{Token: token}
	}

	w := &worker.Worker{
		Messages:    store,
		Subscribers: subscribers,
		Deliveries:  store,
		Notifiers:   notifiers,
		Sleep:       time.Sleep,
	}

	go runWorkerLoop(w)

	log.Printf("notify: listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("notify: server: %v", err)
	}
}

func runMigrations(db *sql.DB) error {
	sourceDriver, err := iofs.New(postgres.MigrationsFS, "migrations")
	if err != nil {
		return err
	}

	dbDriver, err := pgmigrate.WithInstance(db, &pgmigrate.Config{})
	if err != nil {
		return err
	}

	m, err := migrate.NewWithInstance("iofs", sourceDriver, "postgres", dbDriver)
	if err != nil {
		return err
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}
	return nil
}

func runWorkerLoop(w *worker.Worker) {
	for {
		w.ProcessPending(context.Background())
		time.Sleep(workerPollInterval)
	}
}

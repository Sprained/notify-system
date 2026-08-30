package postgres

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/Sprained/notify-system/internal/subscriber"
)

type SubscriberStore struct {
	db *sql.DB
}

func NewSubscriberStore(db *sql.DB) *SubscriberStore {
	return &SubscriberStore{db: db}
}

func (s *SubscriberStore) Get(ctx context.Context, id int64) (subscriber.Subscriber, error) {
	var sub subscriber.Subscriber
	var configJSON []byte
	err := s.db.QueryRowContext(ctx,
		`SELECT id, kind, config, COALESCE(label, ''), enabled FROM subscriber WHERE id = $1`,
		id,
	).Scan(&sub.ID, &sub.Kind, &configJSON, &sub.Label, &sub.Enabled)
	if err != nil {
		return subscriber.Subscriber{}, err
	}
	if err := json.Unmarshal(configJSON, &sub.Config); err != nil {
		return subscriber.Subscriber{}, err
	}
	return sub, nil
}

func (s *SubscriberStore) List(ctx context.Context) ([]subscriber.Subscriber, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, kind, config, COALESCE(label, ''), enabled FROM subscriber ORDER BY id`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []subscriber.Subscriber
	for rows.Next() {
		var sub subscriber.Subscriber
		var configJSON []byte
		if err := rows.Scan(&sub.ID, &sub.Kind, &configJSON, &sub.Label, &sub.Enabled); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(configJSON, &sub.Config); err != nil {
			return nil, err
		}
		result = append(result, sub)
	}
	return result, rows.Err()
}

func (s *SubscriberStore) Create(ctx context.Context, kind string, config map[string]string, label string) error {
	configJSON, err := json.Marshal(config)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO subscriber (kind, config, label) VALUES ($1, $2, $3)`,
		kind, configJSON, label,
	)
	return err
}

func (s *SubscriberStore) CountActive(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT count(*) FROM subscriber WHERE enabled = true`,
	).Scan(&count)
	return count, err
}

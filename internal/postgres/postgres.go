package postgres

import (
	"context"
	"database/sql"

	"github.com/lib/pq"

	"github.com/Sprained/notify-system/internal/delivery"
	"github.com/Sprained/notify-system/internal/message"
	"github.com/Sprained/notify-system/internal/route"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// message.Repository

func (s *Store) Insert(ctx context.Context, msg message.Message) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO topic (name) VALUES ($1) ON CONFLICT (name) DO NOTHING`,
		msg.Topic,
	)
	if err != nil {
		return err
	}

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO message (id, topic, title, body, priority, tags, click_url, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		msg.ID, msg.Topic, msg.Title, msg.Body, msg.Priority, pq.Array(msg.Tags), msg.ClickURL, msg.CreatedAt,
	)
	return err
}

func (s *Store) Get(ctx context.Context, id string) (message.Message, error) {
	var msg message.Message
	err := s.db.QueryRowContext(ctx, `
		SELECT id, topic, COALESCE(title, ''), body, priority,
		       COALESCE(tags, '{}'), COALESCE(click_url, ''), created_at
		FROM message WHERE id = $1`,
		id,
	).Scan(&msg.ID, &msg.Topic, &msg.Title, &msg.Body, &msg.Priority, pq.Array(&msg.Tags), &msg.ClickURL, &msg.CreatedAt)
	if err != nil {
		return message.Message{}, err
	}
	return msg, nil
}

func (s *Store) Recent(ctx context.Context, limit int) ([]message.Message, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, topic, COALESCE(title, ''), body, priority,
		       COALESCE(tags, '{}'), COALESCE(click_url, ''), created_at
		FROM message ORDER BY id DESC LIMIT $1`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []message.Message
	for rows.Next() {
		var msg message.Message
		if err := rows.Scan(&msg.ID, &msg.Topic, &msg.Title, &msg.Body, &msg.Priority, pq.Array(&msg.Tags), &msg.ClickURL, &msg.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, msg)
	}
	return result, rows.Err()
}

// route.Repository

func (s *Store) List(ctx context.Context) ([]route.Route, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT r.id, r.topic, r.subscriber_id, COALESCE(s.label, ''), r.min_priority, r.enabled
		FROM route r
		JOIN subscriber s ON s.id = r.subscriber_id
		ORDER BY r.id DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []route.Route
	for rows.Next() {
		var r route.Route
		if err := rows.Scan(&r.ID, &r.Topic, &r.SubscriberID, &r.SubscriberLabel, &r.MinPriority, &r.Enabled); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

func (s *Store) Create(ctx context.Context, topic string, subscriberID int64, minPriority int) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO route (topic, subscriber_id, min_priority) VALUES ($1, $2, $3)`,
		topic, subscriberID, minPriority,
	)
	return err
}

func (s *Store) MatchingSubscribers(ctx context.Context, topic string, priority int) ([]int64, error) {
	rows, err := s.db.QueryContext(ctx, `
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
		return nil, err
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// delivery.Repository

func (s *Store) InsertPending(ctx context.Context, messageID string, subscriberID int64) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO delivery (message_id, subscriber_id) VALUES ($1, $2)`,
		messageID, subscriberID,
	)
	return err
}

func (s *Store) ListPending(ctx context.Context) ([]delivery.Delivery, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, message_id, subscriber_id, status, attempts FROM delivery WHERE status = 'pending'`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []delivery.Delivery
	for rows.Next() {
		var d delivery.Delivery
		if err := rows.Scan(&d.ID, &d.MessageID, &d.SubscriberID, &d.Status, &d.Attempts); err != nil {
			return nil, err
		}
		result = append(result, d)
	}
	return result, rows.Err()
}

func (s *Store) MarkSent(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE delivery SET status = 'sent', sent_at = now() WHERE id = $1`,
		id,
	)
	return err
}

func (s *Store) MarkFailed(ctx context.Context, id int64, attempts int, lastError string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE delivery SET status = 'failed', attempts = $2, last_error = $3 WHERE id = $1`,
		id, attempts, lastError,
	)
	return err
}

func (s *Store) CountSentToday(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT count(*) FROM delivery WHERE status = 'sent' AND sent_at::date = current_date`,
	).Scan(&count)
	return count, err
}

func (s *Store) CountFailedDeliveries(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT count(*) FROM delivery WHERE status = 'failed'`,
	).Scan(&count)
	return count, err
}

func (s *Store) ListFailed(ctx context.Context) ([]delivery.FailedDelivery, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT d.id, COALESCE(m.title, ''), COALESCE(s.label, ''), d.attempts, COALESCE(d.last_error, '')
		FROM delivery d
		JOIN message m ON m.id = d.message_id
		JOIN subscriber s ON s.id = d.subscriber_id
		WHERE d.status = 'failed'
		ORDER BY d.id DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []delivery.FailedDelivery
	for rows.Next() {
		var fd delivery.FailedDelivery
		if err := rows.Scan(&fd.ID, &fd.MessageTitle, &fd.SubscriberLabel, &fd.Attempts, &fd.LastError); err != nil {
			return nil, err
		}
		result = append(result, fd)
	}
	return result, rows.Err()
}

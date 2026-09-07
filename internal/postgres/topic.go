package postgres

import (
	"context"
	"database/sql"

	"github.com/Sprained/notify-system/internal/topic"
)

type TopicStore struct {
	db *sql.DB
}

func NewTopicStore(db *sql.DB) *TopicStore {
	return &TopicStore{db: db}
}

func (s *TopicStore) List(ctx context.Context) ([]topic.Topic, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT t.name,
		       COUNT(DISTINCT m.id) AS message_count,
		       COUNT(DISTINCT r.id) AS route_count
		FROM topic t
		LEFT JOIN message m ON m.topic = t.name
		LEFT JOIN route r ON r.topic = t.name
		GROUP BY t.name
		ORDER BY t.name`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []topic.Topic
	for rows.Next() {
		var t topic.Topic
		if err := rows.Scan(&t.Name, &t.MessageCount, &t.RouteCount); err != nil {
			return nil, err
		}
		result = append(result, t)
	}
	return result, rows.Err()
}

func (s *TopicStore) Delete(ctx context.Context, name string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM topic WHERE name = $1`, name)
	return err
}

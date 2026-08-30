package subscriber

import "context"

type Repository interface {
	Get(ctx context.Context, id int64) (Subscriber, error)
	CountActive(ctx context.Context) (int, error)
	List(ctx context.Context) ([]Subscriber, error)
	Create(ctx context.Context, kind string, config map[string]string, label string) error
}

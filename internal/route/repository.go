package route

import "context"

type Repository interface {
	MatchingSubscribers(ctx context.Context, topic string, priority int) ([]int64, error)
	List(ctx context.Context) ([]Route, error)
	Create(ctx context.Context, topic string, subscriberID int64, minPriority int) error
}

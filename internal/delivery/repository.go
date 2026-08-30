package delivery

import "context"

type Repository interface {
	InsertPending(ctx context.Context, messageID string, subscriberID int64) error
	ListPending(ctx context.Context) ([]Delivery, error)
	MarkSent(ctx context.Context, id int64) error
	MarkFailed(ctx context.Context, id int64, attempts int, lastError string) error
	CountSentToday(ctx context.Context) (int, error)
	CountFailedDeliveries(ctx context.Context) (int, error)
	ListFailed(ctx context.Context) ([]FailedDelivery, error)
}

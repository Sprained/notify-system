package message

import "context"

type Repository interface {
	Insert(ctx context.Context, msg Message) error
	Get(ctx context.Context, id string) (Message, error)
	Recent(ctx context.Context, limit int) ([]Message, error)
}

package topic

import "context"

type Repository interface {
	List(ctx context.Context) ([]Topic, error)
	Delete(ctx context.Context, name string) error
}

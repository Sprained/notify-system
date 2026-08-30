package notifier

import (
	"context"

	"github.com/Sprained/notify-system/internal/message"
	"github.com/Sprained/notify-system/internal/subscriber"
)

type Notifier interface {
	Kind() string
	Send(ctx context.Context, sub subscriber.Subscriber, msg message.Message) error
}

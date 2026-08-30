package lognotifier

import (
	"context"
	"errors"
	"log"

	"github.com/Sprained/notify-system/internal/message"
	"github.com/Sprained/notify-system/internal/subscriber"
)

type Notifier struct {
	failTimes int
	calls     int
}

func New(failTimes int) *Notifier {
	return &Notifier{failTimes: failTimes}
}

func (n *Notifier) Kind() string {
	return "log"
}

func (n *Notifier) Send(ctx context.Context, sub subscriber.Subscriber, msg message.Message) error {
	n.calls++
	if n.calls <= n.failTimes {
		return errors.New("lognotifier: simulated failure")
	}
	log.Printf("lognotifier: to subscriber %d: %s", sub.ID, msg.Body)
	return nil
}

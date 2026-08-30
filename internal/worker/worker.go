package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/Sprained/notify-system/internal/delivery"
	"github.com/Sprained/notify-system/internal/message"
	"github.com/Sprained/notify-system/internal/notifier"
	"github.com/Sprained/notify-system/internal/subscriber"
)

const MaxAttempts = 4 // 1 tentativa inicial + 3 retries

func backoff(attempt int) time.Duration {
	return time.Duration(1<<(attempt-1)) * time.Second
}

// Deliver tenta enviar msg pro sub via n, com retry e backoff exponencial em memória.
// Retorna quantas tentativas foram feitas e o erro da última (nil se alguma teve sucesso).
func Deliver(ctx context.Context, n notifier.Notifier, sub subscriber.Subscriber, msg message.Message, sleep func(time.Duration)) (int, error) {
	var lastErr error
	for attempt := 1; attempt <= MaxAttempts; attempt++ {
		lastErr = n.Send(ctx, sub, msg)
		if lastErr == nil {
			return attempt, nil
		}
		if attempt < MaxAttempts {
			sleep(backoff(attempt))
		}
	}
	return MaxAttempts, lastErr
}

type Worker struct {
	Messages    message.Repository
	Subscribers subscriber.Repository
	Deliveries  delivery.Repository
	Notifiers   map[string]notifier.Notifier
	Sleep       func(time.Duration)
}

func (w *Worker) ProcessPending(ctx context.Context) {
	pending, err := w.Deliveries.ListPending(ctx)
	if err != nil {
		return
	}
	for _, d := range pending {
		w.processOne(ctx, d)
	}
}

func (w *Worker) processOne(ctx context.Context, d delivery.Delivery) {
	sub, err := w.Subscribers.Get(ctx, d.SubscriberID)
	if err != nil {
		w.Deliveries.MarkFailed(ctx, d.ID, 0, err.Error())
		return
	}

	msg, err := w.Messages.Get(ctx, d.MessageID)
	if err != nil {
		w.Deliveries.MarkFailed(ctx, d.ID, 0, err.Error())
		return
	}

	n, ok := w.Notifiers[sub.Kind]
	if !ok {
		w.Deliveries.MarkFailed(ctx, d.ID, 0, fmt.Sprintf("no notifier registered for kind: %s", sub.Kind))
		return
	}

	attempts, err := Deliver(ctx, n, sub, msg, w.Sleep)
	if err != nil {
		w.Deliveries.MarkFailed(ctx, d.ID, attempts, err.Error())
		return
	}

	w.Deliveries.MarkSent(ctx, d.ID)
}

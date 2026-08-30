package worker_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Sprained/notify-system/internal/delivery"
	"github.com/Sprained/notify-system/internal/message"
	"github.com/Sprained/notify-system/internal/notifier"
	"github.com/Sprained/notify-system/internal/notifier/lognotifier"
	"github.com/Sprained/notify-system/internal/subscriber"
	"github.com/Sprained/notify-system/internal/worker"
)

type fakeMessageRepo struct {
	messages map[string]message.Message
}

func (f *fakeMessageRepo) Insert(ctx context.Context, msg message.Message) error {
	return errors.New("not implemented in this fake")
}

func (f *fakeMessageRepo) Get(ctx context.Context, id string) (message.Message, error) {
	msg, ok := f.messages[id]
	if !ok {
		return message.Message{}, errors.New("message not found")
	}
	return msg, nil
}

func (f *fakeMessageRepo) Recent(ctx context.Context, limit int) ([]message.Message, error) {
	return nil, nil
}

type fakeSubscriberRepo struct {
	subscribers map[int64]subscriber.Subscriber
}

func (f *fakeSubscriberRepo) Get(ctx context.Context, id int64) (subscriber.Subscriber, error) {
	sub, ok := f.subscribers[id]
	if !ok {
		return subscriber.Subscriber{}, errors.New("subscriber not found")
	}
	return sub, nil
}

func (f *fakeSubscriberRepo) CountActive(ctx context.Context) (int, error) {
	return 0, nil
}

func (f *fakeSubscriberRepo) List(ctx context.Context) ([]subscriber.Subscriber, error) {
	return nil, nil
}

func (f *fakeSubscriberRepo) Create(ctx context.Context, kind string, config map[string]string, label string) error {
	return nil
}

type markCall struct {
	id        int64
	attempts  int
	lastError string
	sent      bool
}

type fakeDeliveryRepo struct {
	pending []delivery.Delivery
	marks   []markCall
}

func (f *fakeDeliveryRepo) InsertPending(ctx context.Context, messageID string, subscriberID int64) error {
	return errors.New("not implemented in this fake")
}

func (f *fakeDeliveryRepo) ListPending(ctx context.Context) ([]delivery.Delivery, error) {
	return f.pending, nil
}

func (f *fakeDeliveryRepo) MarkSent(ctx context.Context, id int64) error {
	f.marks = append(f.marks, markCall{id: id, sent: true})
	return nil
}

func (f *fakeDeliveryRepo) MarkFailed(ctx context.Context, id int64, attempts int, lastError string) error {
	f.marks = append(f.marks, markCall{id: id, attempts: attempts, lastError: lastError})
	return nil
}

func (f *fakeDeliveryRepo) CountSentToday(ctx context.Context) (int, error) {
	return 0, nil
}

func (f *fakeDeliveryRepo) CountFailedDeliveries(ctx context.Context) (int, error) {
	return 0, nil
}

func (f *fakeDeliveryRepo) ListFailed(ctx context.Context) ([]delivery.FailedDelivery, error) {
	return nil, nil
}

func TestProcessPending_DispatchesToCorrectNotifierByKind(t *testing.T) {
	messages := &fakeMessageRepo{messages: map[string]message.Message{
		"msg-1": {ID: "msg-1", Body: "oi"},
	}}
	subscribers := &fakeSubscriberRepo{subscribers: map[int64]subscriber.Subscriber{
		1: {ID: 1, Kind: "log"},
	}}
	deliveries := &fakeDeliveryRepo{pending: []delivery.Delivery{
		{ID: 100, MessageID: "msg-1", SubscriberID: 1, Status: "pending"},
	}}

	w := &worker.Worker{
		Messages:    messages,
		Subscribers: subscribers,
		Deliveries:  deliveries,
		Notifiers:   map[string]notifier.Notifier{"log": lognotifier.New(0)},
		Sleep:       func(time.Duration) {},
	}

	w.ProcessPending(context.Background())

	if len(deliveries.marks) != 1 {
		t.Fatalf("expected 1 delivery processed, got %d", len(deliveries.marks))
	}
	if !deliveries.marks[0].sent {
		t.Errorf("expected delivery to be marked sent, got %+v", deliveries.marks[0])
	}
}

func TestProcessPending_UnknownKind_MarksFailedWithoutCallingNotifier(t *testing.T) {
	messages := &fakeMessageRepo{messages: map[string]message.Message{
		"msg-1": {ID: "msg-1", Body: "oi"},
	}}
	subscribers := &fakeSubscriberRepo{subscribers: map[int64]subscriber.Subscriber{
		1: {ID: 1, Kind: "fcm"}, // sem notifier registrado
	}}
	deliveries := &fakeDeliveryRepo{pending: []delivery.Delivery{
		{ID: 100, MessageID: "msg-1", SubscriberID: 1, Status: "pending"},
	}}

	w := &worker.Worker{
		Messages:    messages,
		Subscribers: subscribers,
		Deliveries:  deliveries,
		Notifiers:   map[string]notifier.Notifier{}, // vazio de propósito
		Sleep:       func(time.Duration) {},
	}

	w.ProcessPending(context.Background())

	if len(deliveries.marks) != 1 {
		t.Fatalf("expected 1 delivery processed, got %d", len(deliveries.marks))
	}
	mark := deliveries.marks[0]
	if mark.sent {
		t.Fatal("expected delivery to be marked failed, not sent")
	}
	if mark.lastError == "" {
		t.Error("expected a last_error explaining missing notifier")
	}
}

func TestProcessPending_ExhaustedRetriesAreMarkedFailedWithLastError(t *testing.T) {
	messages := &fakeMessageRepo{messages: map[string]message.Message{
		"msg-1": {ID: "msg-1"},
	}}
	subscribers := &fakeSubscriberRepo{subscribers: map[int64]subscriber.Subscriber{
		1: {ID: 1, Kind: "log"},
	}}
	deliveries := &fakeDeliveryRepo{pending: []delivery.Delivery{
		{ID: 100, MessageID: "msg-1", SubscriberID: 1, Status: "pending"},
	}}

	w := &worker.Worker{
		Messages:    messages,
		Subscribers: subscribers,
		Deliveries:  deliveries,
		Notifiers:   map[string]notifier.Notifier{"log": lognotifier.New(10)}, // sempre falha
		Sleep:       func(time.Duration) {},
	}

	w.ProcessPending(context.Background())

	mark := deliveries.marks[0]
	if mark.sent {
		t.Fatal("expected delivery marked failed")
	}
	if mark.attempts != worker.MaxAttempts {
		t.Errorf("expected %d attempts recorded, got %d", worker.MaxAttempts, mark.attempts)
	}
	if mark.lastError == "" {
		t.Error("expected last_error to be recorded")
	}
}

func TestProcessPending_OneFailureDoesNotBlockOthers(t *testing.T) {
	messages := &fakeMessageRepo{messages: map[string]message.Message{
		"msg-fail": {ID: "msg-fail"},
		"msg-ok":   {ID: "msg-ok"},
	}}
	subscribers := &fakeSubscriberRepo{subscribers: map[int64]subscriber.Subscriber{
		// kinds diferentes só pra poder registrar dois lognotifier com
		// comportamentos diferentes (um sempre falha, outro sempre sucede)
		1: {ID: 1, Kind: "log-fail"},
		2: {ID: 2, Kind: "log-ok"},
	}}
	deliveries := &fakeDeliveryRepo{pending: []delivery.Delivery{
		{ID: 100, MessageID: "msg-fail", SubscriberID: 1, Status: "pending"},
		{ID: 101, MessageID: "msg-ok", SubscriberID: 2, Status: "pending"},
	}}

	w := &worker.Worker{
		Messages:    messages,
		Subscribers: subscribers,
		Deliveries:  deliveries,
		Notifiers: map[string]notifier.Notifier{
			"log-fail": lognotifier.New(10),
			"log-ok":   lognotifier.New(0),
		},
		Sleep: func(time.Duration) {},
	}

	w.ProcessPending(context.Background())

	if len(deliveries.marks) != 2 {
		t.Fatalf("expected both deliveries processed, got %d", len(deliveries.marks))
	}

	var failedMark, sentMark *markCall
	for i := range deliveries.marks {
		switch deliveries.marks[i].id {
		case 100:
			failedMark = &deliveries.marks[i]
		case 101:
			sentMark = &deliveries.marks[i]
		}
	}
	if failedMark == nil || failedMark.sent {
		t.Errorf("expected delivery 100 marked failed, got %+v", failedMark)
	}
	if sentMark == nil || !sentMark.sent {
		t.Errorf("expected delivery 101 marked sent, got %+v", sentMark)
	}
}

package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sprained/notify-system/internal/delivery"
	"github.com/Sprained/notify-system/internal/httpapi"
	"github.com/Sprained/notify-system/internal/message"
	"github.com/Sprained/notify-system/internal/route"
)

type fakeMessageRepo struct {
	inserted []message.Message
	err      error
}

func (f *fakeMessageRepo) Insert(ctx context.Context, msg message.Message) error {
	if f.err != nil {
		return f.err
	}
	f.inserted = append(f.inserted, msg)
	return nil
}

func (f *fakeMessageRepo) Get(ctx context.Context, id string) (message.Message, error) {
	return message.Message{}, nil
}

func (f *fakeMessageRepo) Recent(ctx context.Context, limit int) ([]message.Message, error) {
	return nil, nil
}

type fakeRouteRepo struct {
	subscribers []int64
	err         error
}

func (f *fakeRouteRepo) MatchingSubscribers(ctx context.Context, topic string, priority int) ([]int64, error) {
	return f.subscribers, f.err
}

func (f *fakeRouteRepo) List(ctx context.Context) ([]route.Route, error) {
	return nil, nil
}

func (f *fakeRouteRepo) Create(ctx context.Context, topic string, subscriberID int64, minPriority int) error {
	return nil
}

type deliveryInsert struct {
	MessageID    string
	SubscriberID int64
}

type fakeDeliveryRepo struct {
	inserted []deliveryInsert
	err      error
}

func (f *fakeDeliveryRepo) InsertPending(ctx context.Context, messageID string, subscriberID int64) error {
	if f.err != nil {
		return f.err
	}
	f.inserted = append(f.inserted, deliveryInsert{messageID, subscriberID})
	return nil
}

func (f *fakeDeliveryRepo) ListPending(ctx context.Context) ([]delivery.Delivery, error) {
	return nil, nil
}

func (f *fakeDeliveryRepo) MarkSent(ctx context.Context, id int64) error {
	return nil
}

func (f *fakeDeliveryRepo) MarkFailed(ctx context.Context, id int64, attempts int, lastError string) error {
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

func newRouter(messages *fakeMessageRepo, routes *fakeRouteRepo, deliveries *fakeDeliveryRepo) http.Handler {
	h := &httpapi.Handler{
		Messages:   messages,
		Routes:     routes,
		Deliveries: deliveries,
	}
	return httpapi.NewRouter(h)
}

func TestPublish_ValidRequest_CreatesMessageAndDeliveries(t *testing.T) {
	messages := &fakeMessageRepo{}
	deliveries := &fakeDeliveryRepo{}
	router := newRouter(messages, &fakeRouteRepo{subscribers: []int64{1, 2}}, deliveries)

	req := httptest.NewRequest(http.MethodPost, "/alerts", strings.NewReader("corpo da mensagem"))
	req.Header.Set("X-Title", "Título")
	req.Header.Set("X-Priority", "4")
	req.Header.Set("X-Tags", "casa,urgente")
	req.Header.Set("X-Click", "https://example.com")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(messages.inserted) != 1 {
		t.Fatalf("expected 1 message inserted, got %d", len(messages.inserted))
	}
	msg := messages.inserted[0]
	if msg.Topic != "alerts" || msg.Title != "Título" || msg.Body != "corpo da mensagem" || msg.Priority != 4 {
		t.Errorf("unexpected message: %+v", msg)
	}
	if len(deliveries.inserted) != 2 {
		t.Fatalf("expected 2 deliveries inserted, got %d", len(deliveries.inserted))
	}
}

func TestPublish_DefaultPriorityWhenHeaderMissing(t *testing.T) {
	messages := &fakeMessageRepo{}
	router := newRouter(messages, &fakeRouteRepo{}, &fakeDeliveryRepo{})

	req := httptest.NewRequest(http.MethodPost, "/alerts", strings.NewReader("corpo"))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if len(messages.inserted) != 1 {
		t.Fatalf("expected 1 message inserted, got %d", len(messages.inserted))
	}
	if messages.inserted[0].Priority != 3 {
		t.Errorf("expected default priority 3, got %d", messages.inserted[0].Priority)
	}
}

func TestPublish_NonNumericPriorityRejects400(t *testing.T) {
	messages := &fakeMessageRepo{}
	router := newRouter(messages, &fakeRouteRepo{}, &fakeDeliveryRepo{})

	req := httptest.NewRequest(http.MethodPost, "/alerts", strings.NewReader("corpo"))
	req.Header.Set("X-Priority", "aaaa")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if len(messages.inserted) != 0 {
		t.Errorf("expected no message inserted, got %d", len(messages.inserted))
	}
}

func TestPublish_TagsParsedFromCSV(t *testing.T) {
	messages := &fakeMessageRepo{}
	router := newRouter(messages, &fakeRouteRepo{}, &fakeDeliveryRepo{})

	req := httptest.NewRequest(http.MethodPost, "/alerts", strings.NewReader("corpo"))
	req.Header.Set("X-Tags", "casa,urgente")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	got := messages.inserted[0].Tags
	if len(got) != 2 || got[0] != "casa" || got[1] != "urgente" {
		t.Errorf("expected tags [casa urgente], got %v", got)
	}
}

func TestPublish_EmptyBodyRejects400(t *testing.T) {
	messages := &fakeMessageRepo{}
	router := newRouter(messages, &fakeRouteRepo{}, &fakeDeliveryRepo{})

	req := httptest.NewRequest(http.MethodPost, "/alerts", strings.NewReader(""))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if len(messages.inserted) != 0 {
		t.Errorf("expected no message inserted, got %d", len(messages.inserted))
	}
}

func TestPublish_NoMatchingRoutes_NoDeliveriesCreated(t *testing.T) {
	messages := &fakeMessageRepo{}
	deliveries := &fakeDeliveryRepo{}
	router := newRouter(messages, &fakeRouteRepo{subscribers: nil}, deliveries)

	req := httptest.NewRequest(http.MethodPost, "/alerts", strings.NewReader("corpo"))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if len(messages.inserted) != 1 {
		t.Errorf("expected message still inserted, got %d", len(messages.inserted))
	}
	if len(deliveries.inserted) != 0 {
		t.Errorf("expected no deliveries, got %d", len(deliveries.inserted))
	}
}

func TestPublish_ResponseIncludesMessageID(t *testing.T) {
	messages := &fakeMessageRepo{}
	router := newRouter(messages, &fakeRouteRepo{}, &fakeDeliveryRepo{})

	req := httptest.NewRequest(http.MethodPost, "/alerts", strings.NewReader("corpo"))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	var body struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.ID) != 26 {
		t.Errorf("expected ULID (26 chars) in response, got %q", body.ID)
	}
	if body.ID != messages.inserted[0].ID {
		t.Errorf("expected response id to match inserted message id")
	}
}

func TestPublish_JSONContentType_ExtractsMessageFieldAsBody(t *testing.T) {
	messages := &fakeMessageRepo{}
	router := newRouter(messages, &fakeRouteRepo{}, &fakeDeliveryRepo{})

	req := httptest.NewRequest(http.MethodPost, "/alerts", strings.NewReader(`{"message":"corpo via json"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Title", "Alerta")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	msg := messages.inserted[0]
	if msg.Body != "corpo via json" || msg.Title != "Alerta" {
		t.Errorf("unexpected message: %+v", msg)
	}
}

func TestPublish_JSONContentTypeWithCharset_StillParsed(t *testing.T) {
	messages := &fakeMessageRepo{}
	router := newRouter(messages, &fakeRouteRepo{}, &fakeDeliveryRepo{})

	req := httptest.NewRequest(http.MethodPost, "/alerts", strings.NewReader(`{"message":"corpo"}`))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if messages.inserted[0].Body != "corpo" {
		t.Errorf("expected body %q, got %q", "corpo", messages.inserted[0].Body)
	}
}

func TestPublish_JSONContentType_InvalidJSON_Rejects400(t *testing.T) {
	messages := &fakeMessageRepo{}
	router := newRouter(messages, &fakeRouteRepo{}, &fakeDeliveryRepo{})

	req := httptest.NewRequest(http.MethodPost, "/alerts", strings.NewReader(`not json`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if len(messages.inserted) != 0 {
		t.Errorf("expected no message inserted, got %d", len(messages.inserted))
	}
}

func TestPublish_JSONContentType_MissingMessageField_Rejects400(t *testing.T) {
	messages := &fakeMessageRepo{}
	router := newRouter(messages, &fakeRouteRepo{}, &fakeDeliveryRepo{})

	req := httptest.NewRequest(http.MethodPost, "/alerts", strings.NewReader(`{"foo":"bar"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestPublish_JSONContentType_EmptyMessageField_Rejects400(t *testing.T) {
	messages := &fakeMessageRepo{}
	router := newRouter(messages, &fakeRouteRepo{}, &fakeDeliveryRepo{})

	req := httptest.NewRequest(http.MethodPost, "/alerts", strings.NewReader(`{"message":""}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestPublish_NonJSONContentType_RawBodyUnaffected(t *testing.T) {
	messages := &fakeMessageRepo{}
	router := newRouter(messages, &fakeRouteRepo{}, &fakeDeliveryRepo{})

	raw := `{"message":"não deveria ser parseado"}`
	req := httptest.NewRequest(http.MethodPost, "/alerts", strings.NewReader(raw))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if messages.inserted[0].Body != raw {
		t.Errorf("expected raw body untouched, got %q", messages.inserted[0].Body)
	}
}

func TestPublish_JSONContentType_HeadersStillApplyOnTopOfJSONBody(t *testing.T) {
	messages := &fakeMessageRepo{}
	router := newRouter(messages, &fakeRouteRepo{}, &fakeDeliveryRepo{})

	req := httptest.NewRequest(http.MethodPost, "/alerts", strings.NewReader(`{"message":"corpo"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Priority", "5")
	req.Header.Set("X-Tags", "dozzle,producao")
	req.Header.Set("X-Click", "https://dozzle.example.com")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	msg := messages.inserted[0]
	if msg.Priority != 5 || msg.ClickURL != "https://dozzle.example.com" {
		t.Errorf("unexpected message: %+v", msg)
	}
	if len(msg.Tags) != 2 || msg.Tags[0] != "dozzle" || msg.Tags[1] != "producao" {
		t.Errorf("expected tags [dozzle producao], got %v", msg.Tags)
	}
}

func TestPublish_WrongMethodReturns405(t *testing.T) {
	router := newRouter(&fakeMessageRepo{}, &fakeRouteRepo{}, &fakeDeliveryRepo{})

	req := httptest.NewRequest(http.MethodGet, "/alerts", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

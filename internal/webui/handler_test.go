package webui_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sprained/notify-system/internal/delivery"
	"github.com/Sprained/notify-system/internal/message"
	"github.com/Sprained/notify-system/internal/route"
	"github.com/Sprained/notify-system/internal/subscriber"
	"github.com/Sprained/notify-system/internal/topic"
	"github.com/Sprained/notify-system/internal/webui"
)

type fakeMessageRepo struct{ recent []message.Message }

func (f *fakeMessageRepo) Insert(ctx context.Context, msg message.Message) error { return nil }
func (f *fakeMessageRepo) Get(ctx context.Context, id string) (message.Message, error) {
	return message.Message{}, nil
}
func (f *fakeMessageRepo) Recent(ctx context.Context, limit int) ([]message.Message, error) {
	return f.recent, nil
}

type fakeDeliveryRepo struct {
	pending    []delivery.Delivery
	sentToday  int
	failed     int
	failedList []delivery.FailedDelivery
}

func (f *fakeDeliveryRepo) InsertPending(ctx context.Context, messageID string, subscriberID int64) error {
	return nil
}
func (f *fakeDeliveryRepo) ListPending(ctx context.Context) ([]delivery.Delivery, error) {
	return f.pending, nil
}
func (f *fakeDeliveryRepo) MarkSent(ctx context.Context, id int64) error { return nil }
func (f *fakeDeliveryRepo) MarkFailed(ctx context.Context, id int64, attempts int, lastError string) error {
	return nil
}
func (f *fakeDeliveryRepo) CountSentToday(ctx context.Context) (int, error)       { return f.sentToday, nil }
func (f *fakeDeliveryRepo) CountFailedDeliveries(ctx context.Context) (int, error) { return f.failed, nil }
func (f *fakeDeliveryRepo) ListFailed(ctx context.Context) ([]delivery.FailedDelivery, error) {
	return f.failedList, nil
}

type fakeSubscriberRepo struct {
	active  int
	list    []subscriber.Subscriber
	created []struct {
		Kind   string
		Config map[string]string
		Label  string
	}
}

func (f *fakeSubscriberRepo) Get(ctx context.Context, id int64) (subscriber.Subscriber, error) {
	return subscriber.Subscriber{}, nil
}
func (f *fakeSubscriberRepo) CountActive(ctx context.Context) (int, error) { return f.active, nil }
func (f *fakeSubscriberRepo) List(ctx context.Context) ([]subscriber.Subscriber, error) {
	return f.list, nil
}
func (f *fakeSubscriberRepo) Create(ctx context.Context, kind string, config map[string]string, label string) error {
	f.created = append(f.created, struct {
		Kind   string
		Config map[string]string
		Label  string
	}{kind, config, label})
	return nil
}

type fakeRouteRepo struct {
	routes  []route.Route
	created []struct {
		Topic        string
		SubscriberID int64
		MinPriority  int
	}
}

func (f *fakeRouteRepo) MatchingSubscribers(ctx context.Context, topic string, priority int) ([]int64, error) {
	return nil, nil
}
func (f *fakeRouteRepo) List(ctx context.Context) ([]route.Route, error) {
	return f.routes, nil
}
func (f *fakeRouteRepo) Create(ctx context.Context, topic string, subscriberID int64, minPriority int) error {
	f.created = append(f.created, struct {
		Topic        string
		SubscriberID int64
		MinPriority  int
	}{topic, subscriberID, minPriority})
	return nil
}

type fakeTopicRepo struct {
	topics  []topic.Topic
	deleted []string
}

func (f *fakeTopicRepo) List(ctx context.Context) ([]topic.Topic, error) {
	return f.topics, nil
}
func (f *fakeTopicRepo) Delete(ctx context.Context, name string) error {
	f.deleted = append(f.deleted, name)
	return nil
}

func TestTopics_ListsWithCounts(t *testing.T) {
	h := &webui.Handler{
		Messages:    &fakeMessageRepo{},
		Deliveries:  &fakeDeliveryRepo{},
		Subscribers: &fakeSubscriberRepo{},
		Routes:      &fakeRouteRepo{},
		Topics: &fakeTopicRepo{topics: []topic.Topic{
			{Name: "alertas", MessageCount: 5, RouteCount: 2},
		}},
	}
	router := webui.NewRouter(h)

	req := httptest.NewRequest("GET", "/topicos", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("esperava 200, veio %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"alertas", "5", "2"} {
		if !strings.Contains(body, want) {
			t.Errorf("esperava %q no corpo", want)
		}
	}
}

func TestTopics_EmptyState_RendersWithoutError(t *testing.T) {
	h := &webui.Handler{
		Messages:    &fakeMessageRepo{},
		Deliveries:  &fakeDeliveryRepo{},
		Subscribers: &fakeSubscriberRepo{},
		Routes:      &fakeRouteRepo{},
		Topics:      &fakeTopicRepo{},
	}
	router := webui.NewRouter(h)

	req := httptest.NewRequest("GET", "/topicos", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("esperava 200 mesmo sem tópico nenhum, veio %d", rec.Code)
	}
}

func TestTopics_ShowsOrphanBadges(t *testing.T) {
	h := &webui.Handler{
		Messages:    &fakeMessageRepo{},
		Deliveries:  &fakeDeliveryRepo{},
		Subscribers: &fakeSubscriberRepo{},
		Routes:      &fakeRouteRepo{},
		Topics: &fakeTopicRepo{topics: []topic.Topic{
			{Name: "sem-rota-nenhuma", MessageCount: 3, RouteCount: 0},
			{Name: "nunca-usado", MessageCount: 0, RouteCount: 0},
			{Name: "saudavel", MessageCount: 3, RouteCount: 1},
		}},
	}
	router := webui.NewRouter(h)

	req := httptest.NewRequest("GET", "/topicos", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "sem rota") {
		t.Errorf("esperava badge \"sem rota\" no corpo, corpo: %s", body)
	}
	if !strings.Contains(body, "sem mensagem") {
		t.Errorf("esperava badge \"sem mensagem\" no corpo, corpo: %s", body)
	}
}

func TestTopics_DeleteButton_OnlyShownWhenSafeToDelete(t *testing.T) {
	h := &webui.Handler{
		Messages:    &fakeMessageRepo{},
		Deliveries:  &fakeDeliveryRepo{},
		Subscribers: &fakeSubscriberRepo{},
		Routes:      &fakeRouteRepo{},
		Topics: &fakeTopicRepo{topics: []topic.Topic{
			{Name: "com-mensagem", MessageCount: 1, RouteCount: 0},
			{Name: "vazio-de-verdade", MessageCount: 0, RouteCount: 0},
		}},
	}
	router := webui.NewRouter(h)

	req := httptest.NewRequest("GET", "/topicos", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	body := rec.Body.String()
	deleteFormsCount := strings.Count(body, "/topicos/apagar")
	if deleteFormsCount != 1 {
		t.Errorf("esperava 1 formulário de apagar (só pro tópico vazio), encontrado %d", deleteFormsCount)
	}
}

func TestDeleteTopic_ValidForm_DeletesAndRedirects(t *testing.T) {
	topics := &fakeTopicRepo{}
	h := &webui.Handler{
		Messages:    &fakeMessageRepo{},
		Deliveries:  &fakeDeliveryRepo{},
		Subscribers: &fakeSubscriberRepo{},
		Routes:      &fakeRouteRepo{},
		Topics:      topics,
	}
	router := webui.NewRouter(h)

	req := httptest.NewRequest("POST", "/topicos/apagar", strings.NewReader("name=descartavel"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("esperava 303, veio %d", rec.Code)
	}
	if rec.Header().Get("Location") != "/topicos" {
		t.Errorf("esperava redirect pra /topicos, veio %q", rec.Header().Get("Location"))
	}
	if len(topics.deleted) != 1 || topics.deleted[0] != "descartavel" {
		t.Errorf("esperava \"descartavel\" apagado, veio %v", topics.deleted)
	}
}

func TestSubscribersScreen_ListsWithMaskedConfig(t *testing.T) {
	h := &webui.Handler{
		Messages:   &fakeMessageRepo{},
		Deliveries: &fakeDeliveryRepo{},
		Routes:     &fakeRouteRepo{},
		Subscribers: &fakeSubscriberRepo{list: []subscriber.Subscriber{
			{Kind: "telegram", Label: "Gabriel", Config: map[string]string{"chat_id": "123456789"}, Enabled: true},
		}},
	}
	router := webui.NewRouter(h)

	req := httptest.NewRequest("GET", "/assinantes", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("esperava 200, veio %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Gabriel") {
		t.Errorf("esperava o rótulo no corpo")
	}
	if strings.Contains(body, "123456789") {
		t.Errorf("chat_id não deveria aparecer em texto puro no corpo")
	}
	if !strings.Contains(body, "789") {
		t.Errorf("esperava os últimos dígitos mascarados no corpo")
	}
}

func TestSubscribersScreen_EmptyState_RendersWithoutError(t *testing.T) {
	h := &webui.Handler{
		Messages:    &fakeMessageRepo{},
		Deliveries:  &fakeDeliveryRepo{},
		Routes:      &fakeRouteRepo{},
		Subscribers: &fakeSubscriberRepo{},
	}
	router := webui.NewRouter(h)

	req := httptest.NewRequest("GET", "/assinantes", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("esperava 200 mesmo sem assinante nenhum, veio %d", rec.Code)
	}
}

func TestCreateSubscriber_ValidForm_CreatesAndRedirects(t *testing.T) {
	subs := &fakeSubscriberRepo{}
	h := &webui.Handler{
		Messages:    &fakeMessageRepo{},
		Deliveries:  &fakeDeliveryRepo{},
		Routes:      &fakeRouteRepo{},
		Subscribers: subs,
	}
	router := webui.NewRouter(h)

	req := httptest.NewRequest("POST", "/assinantes", strings.NewReader("label=Gabriel&chat_id=123456789"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("esperava 303, veio %d", rec.Code)
	}
	if rec.Header().Get("Location") != "/assinantes" {
		t.Errorf("esperava redirect pra /assinantes, veio %q", rec.Header().Get("Location"))
	}
	if len(subs.created) != 1 {
		t.Fatalf("esperava 1 assinante criado, veio %d", len(subs.created))
	}
	created := subs.created[0]
	if created.Label != "Gabriel" || created.Kind != "telegram" || created.Config["chat_id"] != "123456789" {
		t.Errorf("assinante criado com dado errado: %+v", created)
	}
}

func TestRoutes_ListsExistingRoutes(t *testing.T) {
	h := &webui.Handler{
		Messages:    &fakeMessageRepo{},
		Deliveries:  &fakeDeliveryRepo{},
		Subscribers: &fakeSubscriberRepo{},
		Routes: &fakeRouteRepo{routes: []route.Route{
			{Topic: "alertas", SubscriberLabel: "Gabriel", MinPriority: 3, Enabled: true},
		}},
	}
	router := webui.NewRouter(h)

	req := httptest.NewRequest("GET", "/rotas", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("esperava 200, veio %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"alertas", "Gabriel", "3"} {
		if !strings.Contains(body, want) {
			t.Errorf("esperava %q no corpo", want)
		}
	}
}

func TestRoutes_EmptyState_RendersWithoutError(t *testing.T) {
	h := &webui.Handler{
		Messages:    &fakeMessageRepo{},
		Deliveries:  &fakeDeliveryRepo{},
		Subscribers: &fakeSubscriberRepo{},
		Routes:      &fakeRouteRepo{},
	}
	router := webui.NewRouter(h)

	req := httptest.NewRequest("GET", "/rotas", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("esperava 200 mesmo sem rota nenhuma, veio %d", rec.Code)
	}
}

func TestCreateRoute_ValidForm_CreatesAndRedirects(t *testing.T) {
	routes := &fakeRouteRepo{}
	h := &webui.Handler{
		Messages:    &fakeMessageRepo{},
		Deliveries:  &fakeDeliveryRepo{},
		Subscribers: &fakeSubscriberRepo{},
		Routes:      routes,
	}
	router := webui.NewRouter(h)

	req := httptest.NewRequest("POST", "/rotas", strings.NewReader("topic=alertas&subscriber_id=7&min_priority=4"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("esperava 303, veio %d", rec.Code)
	}
	if rec.Header().Get("Location") != "/rotas" {
		t.Errorf("esperava redirect pra /rotas, veio %q", rec.Header().Get("Location"))
	}
	if len(routes.created) != 1 {
		t.Fatalf("esperava 1 rota criada, veio %d", len(routes.created))
	}
	created := routes.created[0]
	if created.Topic != "alertas" || created.SubscriberID != 7 || created.MinPriority != 4 {
		t.Errorf("rota criada com dado errado: %+v", created)
	}
}

func TestOverview_ShowsCounts(t *testing.T) {
	h := &webui.Handler{
		Messages:    &fakeMessageRepo{},
		Deliveries:  &fakeDeliveryRepo{pending: []delivery.Delivery{{}, {}, {}}, sentToday: 17, failed: 4},
		Subscribers: &fakeSubscriberRepo{active: 9},
	}
	router := webui.NewRouter(h)

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("esperava 200, veio %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"3", "17", "4", "9"} {
		if !strings.Contains(body, want) {
			t.Errorf("esperava contador %q no corpo, não encontrado", want)
		}
	}
}

func TestOverview_ListsRecentMessages(t *testing.T) {
	h := &webui.Handler{
		Messages: &fakeMessageRepo{recent: []message.Message{
			{Topic: "alertas", Title: "Disco cheio", Priority: 5},
		}},
		Deliveries:  &fakeDeliveryRepo{},
		Subscribers: &fakeSubscriberRepo{},
	}
	router := webui.NewRouter(h)

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "Disco cheio") || !strings.Contains(body, "alertas") {
		t.Errorf("esperava a mensagem recente no corpo, corpo: %s", body)
	}
}

func TestPartialStats_ReturnsUpdatedCounts(t *testing.T) {
	h := &webui.Handler{
		Messages:    &fakeMessageRepo{},
		Deliveries:  &fakeDeliveryRepo{pending: []delivery.Delivery{{}}, sentToday: 8, failed: 1},
		Subscribers: &fakeSubscriberRepo{active: 2},
	}
	router := webui.NewRouter(h)

	req := httptest.NewRequest("GET", "/partials/stats", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("esperava 200, veio %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"1", "8", "2"} {
		if !strings.Contains(body, want) {
			t.Errorf("esperava %q no fragmento, corpo: %s", want, body)
		}
	}
}

func TestMessages_ListsAllMessages(t *testing.T) {
	h := &webui.Handler{
		Messages: &fakeMessageRepo{recent: []message.Message{
			{Topic: "alertas", Title: "Disco cheio", Priority: 5},
			{Topic: "backups", Title: "Backup diário concluído", Priority: 2},
		}},
		Deliveries:  &fakeDeliveryRepo{},
		Subscribers: &fakeSubscriberRepo{},
	}
	router := webui.NewRouter(h)

	req := httptest.NewRequest("GET", "/mensagens", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("esperava 200, veio %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"Disco cheio", "alertas", "Backup diário concluído", "backups"} {
		if !strings.Contains(body, want) {
			t.Errorf("esperava %q no corpo", want)
		}
	}
}

func TestMessages_EmptyState_RendersWithoutError(t *testing.T) {
	h := &webui.Handler{
		Messages:    &fakeMessageRepo{},
		Deliveries:  &fakeDeliveryRepo{},
		Subscribers: &fakeSubscriberRepo{},
	}
	router := webui.NewRouter(h)

	req := httptest.NewRequest("GET", "/mensagens", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("esperava 200, veio %d", rec.Code)
	}
}

func TestFailedDeliveries_ListsFailures(t *testing.T) {
	h := &webui.Handler{
		Messages: &fakeMessageRepo{},
		Deliveries: &fakeDeliveryRepo{failedList: []delivery.FailedDelivery{
			{MessageTitle: "Disco cheio", SubscriberLabel: "Gabriel", Attempts: 4, LastError: "telegram: 403 forbidden"},
		}},
		Subscribers: &fakeSubscriberRepo{},
	}
	router := webui.NewRouter(h)

	req := httptest.NewRequest("GET", "/entregas", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("esperava 200, veio %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"Disco cheio", "Gabriel", "4", "telegram: 403 forbidden"} {
		if !strings.Contains(body, want) {
			t.Errorf("esperava %q no corpo", want)
		}
	}
}

func TestFailedDeliveries_EmptyState_RendersWithoutError(t *testing.T) {
	h := &webui.Handler{
		Messages:    &fakeMessageRepo{},
		Deliveries:  &fakeDeliveryRepo{},
		Subscribers: &fakeSubscriberRepo{},
	}
	router := webui.NewRouter(h)

	req := httptest.NewRequest("GET", "/entregas", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("esperava 200 mesmo sem falha nenhuma, veio %d", rec.Code)
	}
}

func TestOverview_EmptyState_RendersWithoutError(t *testing.T) {
	h := &webui.Handler{
		Messages:    &fakeMessageRepo{},
		Deliveries:  &fakeDeliveryRepo{},
		Subscribers: &fakeSubscriberRepo{},
	}
	router := webui.NewRouter(h)

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("esperava 200 mesmo sem dado nenhum, veio %d", rec.Code)
	}
}

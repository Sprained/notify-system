package webui

import (
	"context"
	"embed"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"github.com/Sprained/notify-system/internal/delivery"
	"github.com/Sprained/notify-system/internal/message"
	"github.com/Sprained/notify-system/internal/route"
	"github.com/Sprained/notify-system/internal/subscriber"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed static/*
var staticFS embed.FS

func parsePage(page string) *template.Template {
	return template.Must(template.ParseFS(templatesFS, "templates/layout.html", "templates/stats.html", page))
}

var (
	overviewTmpl = parsePage("templates/overview.html")
	messagesTmpl = parsePage("templates/messages.html")
	failedTmpl   = parsePage("templates/failed.html")
	routesTmpl       = parsePage("templates/routes.html")
	subscribersTmpl  = parsePage("templates/subscribers.html")
	statsTmpl    = template.Must(template.ParseFS(templatesFS, "templates/stats.html"))
)

type Handler struct {
	Messages    message.Repository
	Deliveries  delivery.Repository
	Subscribers subscriber.Repository
	Routes      route.Repository
}

type shellData struct {
	ActiveNav string
	PageTitle string
}

type statsData struct {
	Pending    int
	SentToday  int
	Failed     int
	ActiveSubs int
}

type overviewData struct {
	shellData
	statsData
	Recent []message.Message
}

type messagesData struct {
	shellData
	Messages []message.Message
}

type failedData struct {
	shellData
	Failed []delivery.FailedDelivery
}

type routesData struct {
	shellData
	Routes      []route.Route
	Subscribers []subscriber.Subscriber
}

type subscriberView struct {
	Kind         string
	Label        string
	MaskedConfig string
	Enabled      bool
}

type subscribersData struct {
	shellData
	Subscribers []subscriberView
}

func maskConfig(sub subscriber.Subscriber) string {
	chatID, ok := sub.Config["chat_id"]
	if !ok || len(chatID) < 4 {
		return "chat_id: ••••••"
	}
	return "chat_id: " + strings.Repeat("•", len(chatID)-3) + chatID[len(chatID)-3:]
}

func NewRouter(h *Handler) *http.ServeMux {
	mux := http.NewServeMux()
	RegisterRoutes(mux, h)
	return mux
}

func RegisterRoutes(mux *http.ServeMux, h *Handler) {
	mux.HandleFunc("GET /{$}", h.handleOverview)
	mux.HandleFunc("GET /mensagens", h.handleMessages)
	mux.HandleFunc("GET /entregas", h.handleFailed)
	mux.HandleFunc("GET /rotas", h.handleRoutes)
	mux.HandleFunc("POST /rotas", h.handleCreateRoute)
	mux.HandleFunc("GET /assinantes", h.handleSubscribers)
	mux.HandleFunc("POST /assinantes", h.handleCreateSubscriber)
	mux.HandleFunc("GET /partials/stats", h.handleStatsPartial)
	mux.Handle("GET /static/", http.FileServerFS(staticFS))
}

func (h *Handler) loadStats(ctx context.Context) (statsData, error) {
	pending, err := h.Deliveries.ListPending(ctx)
	if err != nil {
		return statsData{}, err
	}
	sentToday, err := h.Deliveries.CountSentToday(ctx)
	if err != nil {
		return statsData{}, err
	}
	failed, err := h.Deliveries.CountFailedDeliveries(ctx)
	if err != nil {
		return statsData{}, err
	}
	activeSubs, err := h.Subscribers.CountActive(ctx)
	if err != nil {
		return statsData{}, err
	}
	return statsData{Pending: len(pending), SentToday: sentToday, Failed: failed, ActiveSubs: activeSubs}, nil
}

func (h *Handler) handleOverview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	stats, err := h.loadStats(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	recent, err := h.Messages.Recent(ctx, 10)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := overviewData{
		shellData: shellData{ActiveNav: "overview", PageTitle: "Visão geral"},
		statsData: stats,
		Recent:    recent,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := overviewTmpl.ExecuteTemplate(w, "layout", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *Handler) handleMessages(w http.ResponseWriter, r *http.Request) {
	recent, err := h.Messages.Recent(r.Context(), 50)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := messagesData{
		shellData: shellData{ActiveNav: "messages", PageTitle: "Mensagens"},
		Messages:  recent,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := messagesTmpl.ExecuteTemplate(w, "layout", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *Handler) handleFailed(w http.ResponseWriter, r *http.Request) {
	failed, err := h.Deliveries.ListFailed(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := failedData{
		shellData: shellData{ActiveNav: "failed", PageTitle: "Entregas falhadas"},
		Failed:    failed,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := failedTmpl.ExecuteTemplate(w, "layout", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *Handler) handleRoutes(w http.ResponseWriter, r *http.Request) {
	routes, err := h.Routes.List(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	subs, err := h.Subscribers.List(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := routesData{
		shellData:   shellData{ActiveNav: "routes", PageTitle: "Rotas"},
		Routes:      routes,
		Subscribers: subs,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := routesTmpl.ExecuteTemplate(w, "layout", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *Handler) handleCreateRoute(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "formulário inválido", http.StatusBadRequest)
		return
	}
	topic := r.FormValue("topic")
	subscriberID, err := strconv.ParseInt(r.FormValue("subscriber_id"), 10, 64)
	if err != nil {
		http.Error(w, "assinante inválido", http.StatusBadRequest)
		return
	}
	minPriority, err := strconv.Atoi(r.FormValue("min_priority"))
	if err != nil {
		http.Error(w, "prioridade mínima inválida", http.StatusBadRequest)
		return
	}

	if err := h.Routes.Create(r.Context(), topic, subscriberID, minPriority); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/rotas", http.StatusSeeOther)
}

func (h *Handler) handleSubscribers(w http.ResponseWriter, r *http.Request) {
	subs, err := h.Subscribers.List(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	views := make([]subscriberView, 0, len(subs))
	for _, sub := range subs {
		views = append(views, subscriberView{
			Kind:         sub.Kind,
			Label:        sub.Label,
			MaskedConfig: maskConfig(sub),
			Enabled:      sub.Enabled,
		})
	}

	data := subscribersData{
		shellData:   shellData{ActiveNav: "subscribers", PageTitle: "Assinantes"},
		Subscribers: views,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := subscribersTmpl.ExecuteTemplate(w, "layout", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *Handler) handleCreateSubscriber(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "formulário inválido", http.StatusBadRequest)
		return
	}
	label := r.FormValue("label")
	chatID := r.FormValue("chat_id")

	err := h.Subscribers.Create(r.Context(), "telegram", map[string]string{"chat_id": chatID}, label)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/assinantes", http.StatusSeeOther)
}

func (h *Handler) handleStatsPartial(w http.ResponseWriter, r *http.Request) {
	stats, err := h.loadStats(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := statsTmpl.ExecuteTemplate(w, "stats", stats); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

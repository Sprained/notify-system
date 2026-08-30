package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/Sprained/notify-system/internal/delivery"
	"github.com/Sprained/notify-system/internal/message"
	"github.com/Sprained/notify-system/internal/route"
)

const defaultPriority = 3

type Handler struct {
	Messages   message.Repository
	Routes     route.Repository
	Deliveries delivery.Repository
}

func NewRouter(h *Handler) *http.ServeMux {
	mux := http.NewServeMux()
	RegisterRoutes(mux, h)
	return mux
}

func RegisterRoutes(mux *http.ServeMux, h *Handler) {
	mux.HandleFunc("POST /{topic}", h.Publish)
}

func (h *Handler) Publish(w http.ResponseWriter, r *http.Request) {
	topic := r.PathValue("topic")

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	priority := defaultPriority
	if raw := r.Header.Get("X-Priority"); raw != "" {
		p, err := strconv.Atoi(raw)
		if err != nil {
			http.Error(w, "X-Priority must be a number", http.StatusBadRequest)
			return
		}
		priority = p
	}

	var tags []string
	if raw := r.Header.Get("X-Tags"); raw != "" {
		tags = strings.Split(raw, ",")
	}

	msg, err := message.New(topic, r.Header.Get("X-Title"), string(body), priority, tags, r.Header.Get("X-Click"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	if err := h.Messages.Insert(ctx, msg); err != nil {
		http.Error(w, "failed to save message", http.StatusInternalServerError)
		return
	}

	subscriberIDs, err := h.Routes.MatchingSubscribers(ctx, topic, msg.Priority)
	if err != nil {
		http.Error(w, "failed to resolve routes", http.StatusInternalServerError)
		return
	}

	for _, subID := range subscriberIDs {
		if err := h.Deliveries.InsertPending(ctx, msg.ID, subID); err != nil {
			http.Error(w, "failed to queue delivery", http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"id": msg.ID})
}

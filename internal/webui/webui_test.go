package webui_test

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sprained/notify-system/internal/webui"
)

func TestRouter_Root_ServesShell(t *testing.T) {
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
		t.Fatalf("esperava 200, veio %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "notify") {
		t.Errorf("esperava o shell mencionar 'notify', corpo: %s", rec.Body.String())
	}
}

func TestRouter_ServesStaticCSS(t *testing.T) {
	h := &webui.Handler{
		Messages:    &fakeMessageRepo{},
		Deliveries:  &fakeDeliveryRepo{},
		Subscribers: &fakeSubscriberRepo{},
	}
	router := webui.NewRouter(h)

	req := httptest.NewRequest("GET", "/static/styles.css", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("esperava 200, veio %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "--accent") {
		t.Errorf("esperava o CSS conter a variável --accent, corpo: %s", rec.Body.String())
	}
}

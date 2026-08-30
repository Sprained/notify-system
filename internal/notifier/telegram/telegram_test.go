package telegram_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sprained/notify-system/internal/message"
	"github.com/Sprained/notify-system/internal/notifier/telegram"
	"github.com/Sprained/notify-system/internal/subscriber"
)

type capturedRequest struct {
	Path string
	Body map[string]any
}

func newFakeTelegramServer(t *testing.T, statusCode int, capture *capturedRequest) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capture.Path = r.URL.Path
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		capture.Body = body
		w.WriteHeader(statusCode)
		w.Write([]byte(`{"ok":true}`))
	}))
}

func testSubscriber(chatID string) subscriber.Subscriber {
	return subscriber.Subscriber{
		ID:   1,
		Kind: "telegram",
		Config: map[string]string{
			"chat_id": chatID,
		},
	}
}

func TestKind(t *testing.T) {
	n := &telegram.Notifier{}
	if n.Kind() != "telegram" {
		t.Errorf("expected kind %q, got %q", "telegram", n.Kind())
	}
}

func TestSend_UsesChatIDAndTokenInURL(t *testing.T) {
	var captured capturedRequest
	server := newFakeTelegramServer(t, http.StatusOK, &captured)
	defer server.Close()

	n := &telegram.Notifier{Token: "abc123", BaseURL: server.URL}
	err := n.Send(context.Background(), testSubscriber("999"), message.Message{Body: "oi"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if captured.Path != "/botabc123/sendMessage" {
		t.Errorf("expected path with token, got %q", captured.Path)
	}
	if captured.Body["chat_id"] != "999" {
		t.Errorf("expected chat_id 999, got %v", captured.Body["chat_id"])
	}
}

func TestSend_PlainTextNoParseMode(t *testing.T) {
	var captured capturedRequest
	server := newFakeTelegramServer(t, http.StatusOK, &captured)
	defer server.Close()

	n := &telegram.Notifier{Token: "abc123", BaseURL: server.URL}
	err := n.Send(context.Background(), testSubscriber("999"), message.Message{Body: "oi"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, hasParseMode := captured.Body["parse_mode"]; hasParseMode {
		t.Error("expected no parse_mode field in request")
	}
	if captured.Body["text"] != "oi" {
		t.Errorf("expected plain text body, got %v", captured.Body["text"])
	}
}

func TestSend_ClickURLBecomesInlineButton(t *testing.T) {
	var captured capturedRequest
	server := newFakeTelegramServer(t, http.StatusOK, &captured)
	defer server.Close()

	n := &telegram.Notifier{Token: "abc123", BaseURL: server.URL}
	msg := message.Message{Body: "oi", ClickURL: "https://example.com"}
	err := n.Send(context.Background(), testSubscriber("999"), msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	replyMarkup, ok := captured.Body["reply_markup"].(map[string]any)
	if !ok {
		t.Fatal("expected reply_markup in request")
	}
	buttons, ok := replyMarkup["inline_keyboard"].([]any)
	if !ok || len(buttons) == 0 {
		t.Fatal("expected inline_keyboard with at least one row")
	}
}

func TestSend_NonOKResponseReturnsError(t *testing.T) {
	var captured capturedRequest
	server := newFakeTelegramServer(t, http.StatusForbidden, &captured)
	defer server.Close()

	n := &telegram.Notifier{Token: "abc123", BaseURL: server.URL}
	err := n.Send(context.Background(), testSubscriber("999"), message.Message{Body: "oi"})
	if err == nil {
		t.Fatal("expected error for non-200 response, got nil")
	}
}

func TestSend_MissingChatIDReturnsErrorWithoutCallingAPI(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	n := &telegram.Notifier{Token: "abc123", BaseURL: server.URL}
	sub := subscriber.Subscriber{ID: 1, Kind: "telegram", Config: map[string]string{}}
	err := n.Send(context.Background(), sub, message.Message{Body: "oi"})
	if err == nil {
		t.Fatal("expected error for missing chat_id, got nil")
	}
	if called {
		t.Error("expected API to not be called when chat_id is missing")
	}
}

func TestSend_RespectsCanceledContext(t *testing.T) {
	server := newFakeTelegramServer(t, http.StatusOK, &capturedRequest{})
	defer server.Close()

	n := &telegram.Notifier{Token: "abc123", BaseURL: server.URL}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := n.Send(ctx, testSubscriber("999"), message.Message{Body: "oi"})
	if err == nil {
		t.Fatal("expected error for canceled context, got nil")
	}
}

func TestSend_BodyOver4096CharsIsTruncated(t *testing.T) {
	var captured capturedRequest
	server := newFakeTelegramServer(t, http.StatusOK, &captured)
	defer server.Close()

	n := &telegram.Notifier{Token: "abc123", BaseURL: server.URL}
	longBody := strings.Repeat("a", 5000)
	err := n.Send(context.Background(), testSubscriber("999"), message.Message{Body: longBody})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sentText, _ := captured.Body["text"].(string)
	if len(sentText) > 4096 {
		t.Errorf("expected text truncated to 4096 chars, got %d", len(sentText))
	}
}

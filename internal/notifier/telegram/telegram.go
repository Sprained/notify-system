package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/Sprained/notify-system/internal/message"
	"github.com/Sprained/notify-system/internal/subscriber"
)

const (
	defaultBaseURL = "https://api.telegram.org"
	maxTextLength  = 4096
)

type Notifier struct {
	Token      string
	BaseURL    string
	HTTPClient *http.Client
}

func (n *Notifier) Kind() string {
	return "telegram"
}

type inlineButton struct {
	Text string `json:"text"`
	URL  string `json:"url"`
}

type replyMarkup struct {
	InlineKeyboard [][]inlineButton `json:"inline_keyboard"`
}

type sendMessageRequest struct {
	ChatID      string       `json:"chat_id"`
	Text        string       `json:"text"`
	ReplyMarkup *replyMarkup `json:"reply_markup,omitempty"`
}

func (n *Notifier) Send(ctx context.Context, sub subscriber.Subscriber, msg message.Message) error {
	chatID, ok := sub.Config["chat_id"]
	if !ok || chatID == "" {
		return errors.New("telegram: subscriber config missing chat_id")
	}

	text := msg.Body
	if len(text) > maxTextLength {
		text = text[:maxTextLength]
	}

	reqBody := sendMessageRequest{
		ChatID: chatID,
		Text:   text,
	}
	if msg.ClickURL != "" {
		reqBody.ReplyMarkup = &replyMarkup{
			InlineKeyboard: [][]inlineButton{
				{{Text: "Abrir", URL: msg.ClickURL}},
			},
		}
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("telegram: encode request: %w", err)
	}

	baseURL := n.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	url := fmt.Sprintf("%s/bot%s/sendMessage", baseURL, n.Token)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("telegram: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := n.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("telegram: send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram: unexpected status %d", resp.StatusCode)
	}

	return nil
}

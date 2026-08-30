package message

import (
	"errors"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
)

const (
	MinPriority = 1
	MaxPriority = 5
)

type Message struct {
	ID        string
	Topic     string
	Title     string
	Body      string
	Priority  int
	Tags      []string
	ClickURL  string
	CreatedAt time.Time
}

var ErrEmptyBody = errors.New("message: body cannot be empty")

func New(topic, title, body string, priority int, tags []string, clickURL string) (Message, error) {
	if strings.TrimSpace(body) == "" {
		return Message{}, ErrEmptyBody
	}

	if priority < MinPriority {
		priority = MinPriority
	}
	if priority > MaxPriority {
		priority = MaxPriority
	}

	return Message{
		ID:        ulid.Make().String(),
		Topic:     topic,
		Title:     title,
		Body:      body,
		Priority:  priority,
		Tags:      tags,
		ClickURL:  clickURL,
		CreatedAt: time.Now(),
	}, nil
}

package route

type Route struct {
	ID              int64
	Topic           string
	SubscriberID    int64
	SubscriberLabel string
	MinPriority     int
	Enabled         bool
}

package delivery

type FailedDelivery struct {
	ID              int64
	MessageTitle    string
	SubscriberLabel string
	Attempts        int
	LastError       string
}

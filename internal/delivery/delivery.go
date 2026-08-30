package delivery

type Delivery struct {
	ID           int64
	MessageID    string
	SubscriberID int64
	Status       string
	Attempts     int
}

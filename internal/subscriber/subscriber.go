package subscriber

type Subscriber struct {
	ID      int64
	Kind    string
	Config  map[string]string
	Label   string
	Enabled bool
}

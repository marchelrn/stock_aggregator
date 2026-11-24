package models

type TradeReview struct {
	ID         uint   `json:"id"`
	UserID     uint   `json:"-"`
	BrokerID   int    `json:"-"`
	BrokerName string `json:"broker"`
	MessageID  string `json:"-"`
	PartID     string `json:"-"`
	Digest     string `json:"-"`
	Filename   string `json:"filename"`
	State      string `json:"state"`
	Draft      string `json:"-"`
}

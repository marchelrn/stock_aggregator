package models

import "time"

type RightExercise struct {
	UserID                                                   uint
	BrokerID                                                 int
	Date                                                     time.Time
	Ticker, Amount, CreditRef, DebitRef, MessageID, Filename string
}

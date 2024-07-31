package eventmodels

import "time"

type OptionContract interface {
	GetLongEV() float64
	GetShortEV() float64
	GetLongExpectedProfit() float64
	GetShortExpectedProfit() float64
	GetExpiration() (time.Time, error)
	GetCreditReceived() *float64
	GetDebitPaid() *float64
}

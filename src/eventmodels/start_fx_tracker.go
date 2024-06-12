package eventmodels

import "time"

type StartFxTracker struct {
	Symbol    FxSymbol  `json:"symbol"`
	Timestamp time.Time `json:"timestamp"`
	Reason    string    `json:"reason"`
}

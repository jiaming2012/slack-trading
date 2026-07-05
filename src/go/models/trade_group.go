package models

import (
	"sync"
)

// TradeGroup is a legacy, non-colliding helper retained from the pre-merge
// models package. Trades is the canonical (merged) type.
type TradeGroup struct {
	Trades Trades
	mutex  sync.Mutex
}

package models

import (
	"time"

	"github.com/jiaming2012/slack-trading/src/go/eventmodels"
)

// ISignalRepository stores and retrieves TradeSignals.
// The repository maintains a single global signal stream (per D-01).
// Symbol is a filter attribute on TradeSignal, not a partition key.
type ISignalRepository interface {
	// Write stores a signal in the repository, maintaining timestamp sort order.
	Write(signal *eventmodels.TradeSignal) error

	// ReadPending returns signals with Timestamp <= upTo that have not yet been
	// delivered. Returns them in timestamp order. Advances the internal cursor
	// past returned signals so they are not re-delivered on subsequent calls.
	ReadPending(upTo time.Time) []*eventmodels.TradeSignal

	// GetAll returns all signals in the repository regardless of cursor position.
	// Used for batch persistence (e.g., ESDB write on SavePlayground).
	GetAll() []*eventmodels.TradeSignal
}

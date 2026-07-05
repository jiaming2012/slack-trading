package evtracker

import (
	"context"
	"time"
)

// TradeOutcome is a single closed-trade result fed into the EV computation. It
// carries only what the engine needs: which strategy produced it, the market
// regime it closed in, when it closed, and its realized pnl. The pnl is gross
// (pre-cost) — net-of-cost EV is owned by a separate change.
type TradeOutcome struct {
	StrategyID string
	Regime     string
	ClosedAt   time.Time
	PnL        float64
}

// TradeOutcomeRepository is the pluggable source of closed-trade outcomes. The
// engine reads trades through this interface so synthetic fixtures can drive it
// now and a live-trade-backed source can be supplied later without changing the
// engine. Implementations return every outcome at or before asOf that they know
// about; the engine applies its own window and as-of filtering on top.
type TradeOutcomeRepository interface {
	FetchTradeOutcomes(ctx context.Context, asOf time.Time) ([]TradeOutcome, error)
}

// InMemoryTradeOutcomeRepository is an in-memory TradeOutcomeRepository backed
// by a fixed slice of outcomes. It is used to drive the full recompute-and-
// persist path in tests without any live or external data source.
type InMemoryTradeOutcomeRepository struct {
	Trades []TradeOutcome
}

// NewInMemoryTradeOutcomeRepository returns an in-memory repository over a copy
// of the supplied trades, so later mutation of the caller's slice cannot alter
// what the repository returns.
func NewInMemoryTradeOutcomeRepository(trades []TradeOutcome) *InMemoryTradeOutcomeRepository {
	cp := make([]TradeOutcome, len(trades))
	copy(cp, trades)
	return &InMemoryTradeOutcomeRepository{Trades: cp}
}

// FetchTradeOutcomes returns a defensive copy of the backing trades. asOf is
// accepted for interface conformance; the engine performs as-of filtering, so
// this implementation returns all stored trades unmodified.
func (r *InMemoryTradeOutcomeRepository) FetchTradeOutcomes(_ context.Context, _ time.Time) ([]TradeOutcome, error) {
	out := make([]TradeOutcome, len(r.Trades))
	copy(out, r.Trades)
	return out, nil
}

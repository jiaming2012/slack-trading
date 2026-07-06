package fidelity

import "context"

// TradeSource supplies the sim-vs-live trade sets a scheduled fidelity run
// compares. It is injected at wiring time (mirroring the
// evtracker.TradeOutcomeRepository precedent) so an in-memory fixture source
// can drive the full monitor path in tests and a real live-trade-backed source
// can be supplied later without changing the monitor — a one-constructor swap
// at the cmd/main.go wiring point (design D2).
type TradeSource interface {
	// FetchTradeSet returns the live and simulator trades for the period.
	FetchTradeSet(ctx context.Context, period Period) (TradeSet, error)
}

// NoLiveTradesSource is the production trade source until live-trade ingestion
// and batch simulator re-runs exist (design D2). It always returns an empty
// trade set, so every scheduled run completes as a clean no_data outcome: no
// fidelity rows persisted, no alert state changed, no error raised — while the
// scheduling, persistence, alerting, and heartbeat plumbing stay fully real.
type NoLiveTradesSource struct{}

// FetchTradeSet returns an empty TradeSet: there are no live trades yet.
func (NoLiveTradesSource) FetchTradeSet(context.Context, Period) (TradeSet, error) {
	return TradeSet{}, nil
}

// StaticTradeSource is an in-memory fixture source: it returns a fixed
// TradeSet (or a fixed error) regardless of the requested period. It drives
// the monitor end to end in tests and demos with no external input.
type StaticTradeSource struct {
	Set TradeSet
	Err error
}

// FetchTradeSet returns the configured set or error.
func (s StaticTradeSource) FetchTradeSet(context.Context, Period) (TradeSet, error) {
	if s.Err != nil {
		return TradeSet{}, s.Err
	}
	return s.Set, nil
}

// SyntheticSource exposes the built-in synthetic sim-vs-live dataset (known
// drift by construction, including two breaching strategies) as a TradeSource
// for demos and tests.
func SyntheticSource() StaticTradeSource {
	set, _ := SyntheticTradeSet()
	return StaticTradeSource{Set: set}
}

package riskoverlay

import (
	"fmt"
	"math"
	"time"

	models "github.com/jiaming2012/slack-trading/src/go/backtester/models"
	coremodels "github.com/jiaming2012/slack-trading/src/go/models"
)

// BuildPortfolioSnapshot is the production PortfolioSnapshotFunc factory. It
// maps a live Simulation playground plus the incoming order into the pure
// engine's inputs:
//
//   - Positions: from the playground's in-memory position cache — signed
//     notional per instrument as quantity x current price (cost-basis fallback
//     when no current price is known), with the x100 contract multiplier for
//     option instruments; sector resolved through the SectorLookup seam (for
//     option instruments the UNDERLYING ticker is resolved, since the sector
//     of an OCC symbol is meaningless and the exposure is to the underlying).
//   - StrategyDeployed: each open (filled, unclosed-quantity) order's absolute
//     remaining notional attributed to a strategy identity derived from the
//     order's Tag, falling back to the playground's client ID (then the
//     playground ID) when the tag is empty.
//   - EquitySeries: the trailing 5-session series from the playground's
//     IN-MEMORY equity plot — one closing value per session date, ending with
//     current equity. No database read happens for equity or positions, so
//     equity can never be the fail-permissive trigger.
//   - EvWeights: through the EvWeightLookup seam.
//
// The returned scan-cycle time is wall-clock now: crowding metrics are
// produced by the live scanner pipeline on wall-clock scanned_at, so the
// crowding view "in effect now" is the correct one even while a Simulation
// playground's own clock replays history.
//
// Any error (or panic) inside the build surfaces as a snapshot error, which
// the gate handles fail-permissive (permit + Warn + degradation counter) —
// and reductions never reach this function at all (the gate short-circuits
// them side-first before any snapshot build or lookup).
func BuildPortfolioSnapshot(evWeights EvWeightLookup, sectors SectorLookup) PortfolioSnapshotFunc {
	return func(p *models.Playground, order *models.OrderRecord) (state PortfolioState, proposed ProposedOrder, scannedAt time.Time, err error) {
		// A malformed playground (nil account, nil internal caches) must
		// degrade to a fail-permissive permit upstream, never crash the
		// order-placement path.
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("BuildPortfolioSnapshot: panic while building snapshot: %v", r)
			}
		}()

		if p == nil {
			return PortfolioState{}, ProposedOrder{}, time.Time{}, fmt.Errorf("BuildPortfolioSnapshot: nil playground")
		}
		if order == nil {
			return PortfolioState{}, ProposedOrder{}, time.Time{}, fmt.Errorf("BuildPortfolioSnapshot: nil order")
		}

		posCache := p.GetPositionCache()
		if posCache == nil {
			return PortfolioState{}, ProposedOrder{}, time.Time{}, fmt.Errorf("BuildPortfolioSnapshot: playground %s has no position cache", p.ID)
		}

		// Positions with sector + signed notional.
		instruments, positions := posCache.List()
		state.Positions = make([]LogicalPosition, 0, len(positions))
		for i, pos := range positions {
			if pos == nil || pos.Quantity == 0 {
				continue
			}
			inst := instruments[i]
			ticker := inst.GetTicker()

			price := positionPrice(pos)
			notional := pos.Quantity * price
			if instrumentIsOption(inst) {
				notional *= optionContractMultiplier
			}

			// Position-ticker sector resolution: unknown resolves to the empty
			// sector (exempt from the sector-concentration family). Only the
			// PROPOSED ENTRY's unknown-sector resolution is recorded as
			// degradation (spec scenario); counting every sector-less holding
			// per evaluation would drown the counter in structural noise.
			sector, serr := sectors.SectorOf(sectorTickerForInstrument(inst))
			if serr != nil {
				return PortfolioState{}, ProposedOrder{}, time.Time{}, fmt.Errorf("BuildPortfolioSnapshot: sector lookup for position %q failed: %w", ticker, serr)
			}

			state.Positions = append(state.Positions, LogicalPosition{
				Ticker:         ticker,
				Sector:         sector,
				SignedNotional: notional,
			})
		}

		// Per-strategy deployed capital: every filled order with remaining open
		// quantity, attributed by its tag. This mirrors the open-orders-cache
		// derivation (SetOpenOrdersCache) but reads through the exported
		// GetAllOrders surface.
		state.StrategyDeployed = make(map[string]float64)
		for _, o := range p.GetAllOrders() {
			if o == nil || !o.Status.IsFilled() {
				continue
			}
			qty, qerr := o.GetRemainingOpenQuantity()
			if qerr != nil || qty == 0 {
				continue
			}

			price := positionPrice(posCache.Get(o.Symbol))
			if price <= 0 {
				price = o.GetAvgFillPrice()
			}
			notional := math.Abs(qty) * price
			if o.Class == models.OrderRecordClassOption {
				notional *= optionContractMultiplier
			}
			state.StrategyDeployed[strategyIdentity(p, o.Tag)] += notional
		}

		// Trailing 5-session equity series from the in-memory plot, ending with
		// current equity. The most recent session's stale close is replaced by
		// the up-to-the-moment equity; an empty plot yields a single-point
		// series, so the wired snapshot ALWAYS carries at least current equity
		// (the drawdown breaker's empty-series inactive state can only occur
		// pre-wiring or in fixtures).
		series := sessionCloses(p.GetEquityPlot(), 5)
		current := p.GetEquity(posCache)
		if len(series) == 0 {
			series = []float64{current}
		} else {
			series[len(series)-1] = current
		}
		state.EquitySeries = series

		// EV weights through the seam. An error is a snapshot error
		// (fail-permissive upstream); an EMPTY set is a legitimate
		// data-availability state that pins the allocation family inactive.
		weights, werr := evWeights.Latest()
		if werr != nil {
			return PortfolioState{}, ProposedOrder{}, time.Time{}, fmt.Errorf("BuildPortfolioSnapshot: EV-weight lookup failed: %w", werr)
		}
		state.EvWeights = weights

		// Complete the proposed order: notional/side/class from the mapping
		// (x100 for options), plus sector and strategy identity.
		proposed = MapProposedOrder(order)
		proposed.StrategyID = strategyIdentity(p, order.Tag)

		sector, serr := sectors.SectorOf(sectorTickerForOrder(order))
		if serr != nil {
			return PortfolioState{}, ProposedOrder{}, time.Time{}, fmt.Errorf("BuildPortfolioSnapshot: sector lookup for order %q failed: %w", order.Symbol, serr)
		}
		proposed.Sector = sector

		return state, proposed, time.Now().UTC(), nil
	}
}

// positionPrice is the effective per-unit price for a position: the current
// price when known, otherwise the cost basis.
func positionPrice(pos *models.Position) float64 {
	if pos == nil {
		return 0
	}
	if pos.CurrentPrice > 0 {
		return pos.CurrentPrice
	}
	return pos.CostBasis
}

// strategyIdentity derives the strategy attribution for an order tag: the tag
// itself, falling back to the playground's client ID, then the playground ID —
// so attribution is always non-empty and stable for a given playground.
func strategyIdentity(p *models.Playground, tag string) string {
	if tag != "" {
		return tag
	}
	if p.ClientID != nil && *p.ClientID != "" {
		return *p.ClientID
	}
	return p.ID.String()
}

// instrumentIsOption reports whether a position-cache instrument is an option
// (its notional carries the x100 contract multiplier).
func instrumentIsOption(inst coremodels.Instrument) bool {
	switch inst.(type) {
	case coremodels.OptionSymbol, *coremodels.OptionContractV3:
		return true
	default:
		return false
	}
}

// sectorTickerForInstrument picks the ticker whose sector meaningfully
// describes the exposure: the underlying for option instruments (an OCC
// symbol never appears in scan_results), the instrument's own ticker
// otherwise. A parse failure falls back to the raw ticker, which then simply
// resolves to the empty (unknown) sector.
func sectorTickerForInstrument(inst coremodels.Instrument) string {
	switch v := inst.(type) {
	case *coremodels.OptionContractV3:
		if v.UnderlyingSymbol != "" {
			return string(v.UnderlyingSymbol)
		}
	case coremodels.OptionSymbol:
		if c, err := v.Components(); err == nil {
			return c.Underlying
		}
	}
	return inst.GetTicker()
}

// sectorTickerForOrder picks the sector-resolution ticker for the proposed
// order: the underlying for option-class orders, the symbol itself otherwise.
func sectorTickerForOrder(order *models.OrderRecord) string {
	if order.Class == models.OrderRecordClassOption {
		if c, err := coremodels.OptionSymbol(order.Symbol).Components(); err == nil {
			return c.Underlying
		}
	}
	return order.Symbol
}

// sessionCloses collapses a chronological equity plot into one closing value
// per distinct session date, returning at most the last max sessions in
// chronological order.
func sessionCloses(plot []*coremodels.EquityPlot, max int) []float64 {
	if len(plot) == 0 || max <= 0 {
		return nil
	}

	// Walk backwards: the first point seen for a date is that session's close.
	reversed := make([]float64, 0, max)
	lastDate := ""
	for i := len(plot) - 1; i >= 0 && len(reversed) < max; i-- {
		if plot[i] == nil {
			continue
		}
		d := plot[i].Timestamp.Format("2006-01-02")
		if d == lastDate {
			continue
		}
		reversed = append(reversed, plot[i].Value)
		lastDate = d
	}

	out := make([]float64, len(reversed))
	for i, v := range reversed {
		out[len(reversed)-1-i] = v
	}
	return out
}

package riskoverlay

import (
	"fmt"
	"math"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	models "github.com/jiaming2012/slack-trading/src/go/backtester/models"
)

// RejectedError is returned by the gate when the risk overlay rejects an order.
// It carries the full breach list so the caller can surface every reason.
type RejectedError struct {
	Breaches []LimitBreach
}

// Error renders the breach types and reasons.
func (e *RejectedError) Error() string {
	parts := make([]string, 0, len(e.Breaches))
	for _, b := range e.Breaches {
		parts = append(parts, fmt.Sprintf("%s: %s", b.Type, b.Reason))
	}
	return "portfolio risk overlay rejected order [" + strings.Join(parts, "; ") + "]"
}

// PortfolioSnapshotFunc builds the engine inputs — a PortfolioState, the
// ProposedOrder, and the scan-cycle time used to resolve crowding — from a live
// playground and the incoming order. It is injected so the adapter stays
// testable without constructing a full Playground, and so the (deferred)
// end-to-end mapping from real playground state can evolve independently of the
// gate logic.
type PortfolioSnapshotFunc func(p *models.Playground, order *models.OrderRecord) (PortfolioState, ProposedOrder, time.Time, error)

// SimulationRiskGate is the impure adapter that adapts the pure engine to the
// Simulation order-placement path. It implements models.RiskGate. It is a no-op
// (permits every order) when disabled, when no snapshot builder is wired, or
// when the resulting Decision is Allowed.
//
// Enablement is off by default and, by construction, this adapter is only ever
// wired into the Simulation path (models.CheckRiskGate is called only for
// ModeSimulation); it never sees a Paper or Margin order.
type SimulationRiskGate struct {
	enabled  bool
	limits   RiskLimits
	lookup   CrowdingLookup
	snapshot PortfolioSnapshotFunc
}

// NewSimulationRiskGate constructs a gate. When enabled is false the gate
// permits every order (permissive-observe). lookup may be nil (treated as an
// unflagged crowding view); snapshot may be nil (the gate then permits every
// order, since it cannot build engine inputs).
func NewSimulationRiskGate(enabled bool, limits RiskLimits, lookup CrowdingLookup, snapshot PortfolioSnapshotFunc) *SimulationRiskGate {
	return &SimulationRiskGate{
		enabled:  enabled,
		limits:   limits,
		lookup:   lookup,
		snapshot: snapshot,
	}
}

// Enabled reports whether the gate enforces limits.
func (g *SimulationRiskGate) Enabled() bool {
	return g != nil && g.enabled
}

// EvaluateSimulationOrder implements models.RiskGate. It returns nil to permit
// and a *RejectedError to reject.
func (g *SimulationRiskGate) EvaluateSimulationOrder(p *models.Playground, order *models.OrderRecord) error {
	if !g.Enabled() || g.snapshot == nil {
		return nil
	}

	// Classify reduction side-first from the raw order — cheaply, before any
	// snapshot build or crowding lookup. A risk-reducing order ALWAYS passes
	// (this mirrors the engine's IsReduction short-circuit) and must never be
	// blocked by a transient snapshot- or crowding-lookup error. Resolving those
	// I/O paths before short-circuiting would let a DB hiccup reject a
	// risk-reducing exit mid-drawdown — the exact opposite of what the overlay is
	// for.
	if order != nil && isReductionSide(order.Side) {
		return nil
	}

	state, proposed, scannedAt, err := g.snapshot(p, order)
	if err != nil {
		// Fail PERMISSIVE: a risk gate that turns snapshot/DB hiccups into trading
		// halts is a new failure mode. Halting is the kill switch's job, not this
		// gate's — so we permit the (non-reducing) order and warn loudly.
		log.Warnf("riskoverlay gate: build snapshot failed for order into %q; permitting order (fail-permissive): %v", orderSymbol(order), err)
		return nil
	}
	return g.decide(state, proposed, scannedAt)
}

// orderSymbol renders the order's symbol for a log line, tolerating a nil order.
func orderSymbol(order *models.OrderRecord) string {
	if order == nil {
		return "<nil>"
	}
	return order.Symbol
}

// decide resolves the crowding view and runs the pure engine. It is separated
// from EvaluateSimulationOrder so the decision path is unit-testable without a
// Playground.
func (g *SimulationRiskGate) decide(state PortfolioState, proposed ProposedOrder, scannedAt time.Time) error {
	view := NewCrowdingView(false, nil)
	if g.lookup != nil {
		v, err := g.lookup.ViewForScanCycle(scannedAt)
		if err != nil {
			// Fail PERMISSIVE: this path is only reached for NON-reducing orders
			// (reductions short-circuit before any lookup). A crowding-DB error
			// must not become a trading halt — the kill switch owns halting, not
			// this gate — so we permit and warn loudly.
			log.Warnf("riskoverlay gate: resolve crowding view failed for %q; permitting order (fail-permissive): %v", proposed.Ticker, err)
			return nil
		}
		view = v
	}

	decision, err := Evaluate(state, proposed, g.limits, view)
	if err != nil {
		return fmt.Errorf("riskoverlay gate: evaluate: %w", err)
	}
	if !decision.Allowed {
		return &RejectedError{Breaches: decision.Breaches}
	}
	return nil
}

// MapProposedOrder derives a ProposedOrder from an OrderRecord: the ticker and a
// signed notional (positive for buy/long-opening sides, negative for
// sell/short-opening sides), and whether the order reduces an existing position
// (any *_to_close, plain sell, or buy_to_cover side). Sector and strategy
// attribution from the order's tags is left to the (deferred) end-to-end
// snapshot wiring and is not populated here.
//
// It is a building block for a PortfolioSnapshotFunc; the portfolio-state half
// (positions, deployed capital, equity series) is deferred per the change's
// design and supplied by the wiring code.
func MapProposedOrder(order *models.OrderRecord) ProposedOrder {
	price := order.RequestedPrice
	if order.Price != nil && *order.Price > 0 {
		price = *order.Price
	}
	notional := math.Abs(order.AbsoluteQuantity) * price

	reduction := isReductionSide(order.Side)

	signed := notional
	if !buySide(order.Side) {
		signed = -notional
	}

	return ProposedOrder{
		Ticker:         order.Symbol,
		SignedNotional: signed,
		IsReduction:    reduction,
	}
}

// isReductionSide reports whether a side reduces or closes an existing logical
// position rather than opening or increasing one.
func isReductionSide(side models.TradierOrderSide) bool {
	switch side {
	case models.TradierOrderSideSell,
		models.TradierOrderSideSellToClose,
		models.TradierOrderSideBuyToClose,
		models.TradierOrderSideBuyToCover:
		return true
	default:
		return false
	}
}

// buySide reports whether a side increases long exposure (positive signed
// notional). Buy-to-cover closes a short and is treated as a reduction, so it is
// classified via isReductionSide, not here.
func buySide(side models.TradierOrderSide) bool {
	switch side {
	case models.TradierOrderSideBuy,
		models.TradierOrderSideBuyToOpen,
		models.TradierOrderSideBuyToClose,
		models.TradierOrderSideBuyToCover:
		return true
	default:
		return false
	}
}

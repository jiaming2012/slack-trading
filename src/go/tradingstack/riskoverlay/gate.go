package riskoverlay

import (
	"fmt"
	"math"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	models "github.com/jiaming2012/slack-trading/src/go/backtester/models"
	"github.com/jiaming2012/slack-trading/src/go/telemetry"
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
// Enablement defaults to true for Simulation (wire-risk-overlay-state) with
// permissive default limits and, by construction, this adapter is only ever
// wired into the Simulation path (models.CheckRiskGate is called only for
// ModeSimulation); it never sees a Paper or Margin order.
//
// Observability states (nit e wording):
//   - DISABLED gate = permissive-BLIND: no evaluation happens and NOTHING is
//     recorded — no degradation, no rejections, no gauges.
//   - Enabled gate on a fail-permissive path (snapshot error, crowding-lookup
//     error, unknown sector) = permissive-OBSERVED: the order is permitted,
//     a Warn is logged, AND grodt.riskoverlay.degraded is incremented with a
//     {reason} label, so gate blindness is alertable without any external
//     observability stack.
type SimulationRiskGate struct {
	enabled  bool
	limits   RiskLimits
	lookup   CrowdingLookup
	snapshot PortfolioSnapshotFunc
}

// NewSimulationRiskGate constructs a gate. When enabled is false the gate
// permits every order without evaluating or recording anything
// (permissive-blind). lookup may be nil (treated as an unflagged crowding
// view); snapshot may be nil (the gate then permits every order, since it
// cannot build engine inputs).
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

	// Classify side-first from the raw order — cheaply, before any snapshot
	// build or crowding lookup. A risk-reducing order ALWAYS passes (this
	// mirrors the engine's IsReduction short-circuit) and must never be
	// blocked by a transient snapshot- or crowding-lookup error. Resolving those
	// I/O paths before short-circuiting would let a DB hiccup reject a
	// risk-reducing exit mid-drawdown — the exact opposite of what the overlay is
	// for.
	//
	// SYSTEM-GENERATED orders bypass the overlay the same way (adversarial
	// review, MAJOR 1): an ITM exercise settlement leg is emitted as a plain
	// `buy` with IsSystemOrder=true, which side-classification alone would read
	// as a NEW entry — under an operator-narrowed limit the overlay would
	// reject the settlement, error the tick with no deferral, and wedge every
	// subsequent tick on the unsettled expiry. Settlement and auto-close
	// placements are mechanical consequences of positions ALREADY held, not new
	// risk decisions; the instrument to stop them is the kill switch (whose own
	// system-order semantics are untouched — CheckOrderGate still runs first
	// for every order in every mode), never this gate.
	if order != nil && (order.IsSystemOrder || isReductionSide(order.Side)) {
		return nil
	}

	state, proposed, scannedAt, err := g.snapshot(p, order)
	if err != nil {
		// Fail PERMISSIVE, but OBSERVED: a risk gate that turns snapshot/DB
		// hiccups into trading halts is a new failure mode. Halting is the kill
		// switch's job, not this gate's — so we permit the (non-reducing) order,
		// warn loudly, and count the degradation so a blind-but-permitting gate
		// pages the operator.
		log.Warnf("riskoverlay gate: build snapshot failed for order into %q; permitting order (fail-permissive): %v", orderSymbol(order), err)
		telemetry.RiskOverlayDegraded.Add(1, telemetry.Label{Key: "reason", Value: DegradedReasonSnapshotError})
		return nil
	}

	// The EV-family pin state comes from every EV lookup result the gate
	// evaluates with: 1 = allocation family enforcing, 0 = pinned inactive
	// (empty EV set — alert-worthy while the gate is enabled).
	if len(state.EvWeights) > 0 {
		telemetry.RiskOverlayEvFamilyActive.Set(1)
	} else {
		telemetry.RiskOverlayEvFamilyActive.Set(0)
	}

	decisionErr := g.decide(state, proposed, scannedAt)

	// An unknown sector leaves the proposed entry exempt from the
	// sector-concentration family: still evaluated, but partially blind. The
	// spec scopes degradation to PERMITS (the gate letting through something it
	// could not fully see), so a rejected order does not count — only a
	// permitted one (adversarial review, minor 3).
	if decisionErr == nil && proposed.Sector == "" {
		telemetry.RiskOverlayDegraded.Add(1, telemetry.Label{Key: "reason", Value: DegradedReasonSectorUnknown})
	}

	return decisionErr
}

// Degradation reasons for the grodt.riskoverlay.degraded counter's {reason}
// label.
const (
	DegradedReasonSnapshotError       = "snapshot_error"
	DegradedReasonCrowdingLookupError = "crowding_lookup_error"
	DegradedReasonSectorUnknown       = "sector_unknown"
)

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
			// Fail PERMISSIVE, but OBSERVED: this path is only reached for
			// NON-reducing orders (reductions short-circuit before any lookup).
			// A crowding-DB error must not become a trading halt — the kill
			// switch owns halting, not this gate — so we permit, warn loudly,
			// and count the degradation.
			log.Warnf("riskoverlay gate: resolve crowding view failed for %q; permitting order (fail-permissive): %v", proposed.Ticker, err)
			telemetry.RiskOverlayDegraded.Add(1, telemetry.Label{Key: "reason", Value: DegradedReasonCrowdingLookupError})
			return nil
		}
		view = v
	}

	decision, err := Evaluate(state, proposed, g.limits, view)
	if err != nil {
		return fmt.Errorf("riskoverlay gate: evaluate: %w", err)
	}
	if !decision.Allowed {
		// One increment per breached limit family, so the operator can see
		// WHAT the overlay is blocking, not just that it blocks.
		for _, b := range decision.Breaches {
			telemetry.RiskOverlayRejections.Add(1, telemetry.Label{Key: "limit_type", Value: string(b.Type)})
		}
		return &RejectedError{Breaches: decision.Breaches}
	}
	return nil
}

// optionContractMultiplier converts option contracts to underlying-share
// notional: one standard contract controls 100 shares.
const optionContractMultiplier = 100.0

// MapProposedOrder derives a ProposedOrder from an OrderRecord: the ticker and a
// signed notional (positive for buy/long-opening sides, negative for
// sell/short-opening sides), and whether the order reduces an existing position
// (any *_to_close, plain sell, or buy_to_cover side). Option-class orders carry
// the x100 contract multiplier in their notional (|quantity| x price x 100);
// equity and empty-class orders (historically defaulting to equity) do not.
// Sector and strategy attribution are populated by the snapshot builder
// (BuildPortfolioSnapshot), which completes the ProposedOrder.
func MapProposedOrder(order *models.OrderRecord) ProposedOrder {
	price := order.RequestedPrice
	if order.Price != nil && *order.Price > 0 {
		price = *order.Price
	}
	notional := math.Abs(order.AbsoluteQuantity) * price
	if order.Class == models.OrderRecordClassOption {
		// Nit c (wire-risk-overlay-state): without the contract multiplier an
		// option order's exposure is understated by 100x. The snapshot builder
		// applies the same multiplier to option positions so both sides of
		// every exposure comparison agree.
		notional *= optionContractMultiplier
	}

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

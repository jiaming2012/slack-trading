package safety

import (
	"context"
	"fmt"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"

	models "github.com/jiaming2012/slack-trading/src/go/backtester/models"
	"github.com/jiaming2012/slack-trading/src/go/telemetry"
)

// CompanionStopTag marks broker orders as companion stops. Every companion
// stop's tag is either exactly this value or CompanionStopTagForEntry's
// "companion-stop-<entryOrderID>" form; the tag is the recursion guard (a
// companion stop's own fill must never spawn another companion stop) and the
// durable per-entry association used for idempotency across restarts.
const CompanionStopTag = "companion-stop"

// CompanionStopTagForEntry returns the tag carrying the durable association
// between a companion stop and the entry order it protects.
func CompanionStopTagForEntry(entryOrderID uint) string {
	return fmt.Sprintf("%s-%d", CompanionStopTag, entryOrderID)
}

// IsCompanionStopOrderTag reports whether a tag identifies a companion-stop
// order (either the bare tag or the per-entry form).
func IsCompanionStopOrderTag(tag string) bool {
	return tag == CompanionStopTag || strings.HasPrefix(tag, CompanionStopTag+"-")
}

// CompanionStopEligible reports whether a committed live fill must trigger
// companion-stop placement. Eligible fills are realtime-Mode (Paper/Margin)
// equity ENTRY fills that open or increase a position:
//
//   - realtime Mode only — Simulation exits are handled by the simulated fill
//     engine, and reconciliation containers (which carry no Mode) are netting
//     artifacts, not exposure decisions;
//   - equity class only — the placement routine supports equity buy/sell_short;
//   - entry side only (buy or sell_short) with no close linkage — closes,
//     adjustments, and system auto-closes reduce exposure and get no stop;
//   - not a companion stop itself (tag check) — the stop must never spawn a
//     stop, or the order-update stream would recurse unboundedly.
func CompanionStopEligible(meta models.Meta, order *models.OrderRecord) bool {
	if order == nil {
		return false
	}

	if !meta.Mode.IsRealtime() || meta.IsReconciliation() {
		return false
	}

	if order.Class != models.OrderRecordClassEquity {
		return false
	}

	if order.Side != models.TradierOrderSideBuy && order.Side != models.TradierOrderSideSellShort {
		return false
	}

	if order.IsClose || order.CloseOrderId != nil {
		return false
	}

	if order.IsAdjustment || order.IsSystemOrder {
		return false
	}

	if IsCompanionStopOrderTag(order.Tag) {
		return false
	}

	return true
}

// CompanionStopper owns companion-stop placement for the live fill pipeline:
// the opt-in configuration, per-entry idempotency, and the loud failure path
// (log + internal-registry counters + unprotected-position records consumed by
// the alert engine).
type CompanionStopper struct {
	cfg CompanionStopConfig

	mu sync.Mutex
	// placed is the in-memory idempotency set keyed by entry order ID: the
	// Tradier order-update stream can redeliver fill events (reconnects,
	// restarts, poller laps), and a redelivered fill must not double the
	// protective size into an actual short.
	placed map[uint]struct{}
	// unprotected records placement failures — live positions with no
	// broker-held exit — keyed by entry order ID for the alert engine.
	unprotected map[uint]telemetry.UnprotectedPosition
}

// NewCompanionStopper builds a stopper for the given (already validated)
// configuration.
func NewCompanionStopper(cfg CompanionStopConfig) *CompanionStopper {
	return &CompanionStopper{
		cfg:         cfg,
		placed:      make(map[uint]struct{}),
		unprotected: make(map[uint]telemetry.UnprotectedPosition),
	}
}

// Config returns the stopper's stop-distance configuration.
func (s *CompanionStopper) Config() CompanionStopConfig {
	return s.cfg
}

// alreadyPlaced consults the in-memory placed-set first, then — the restart
// window, where the set died with the previous process — the durable
// association: broker orders tagged with this entry's companion-stop tag. A
// broker lookup failure is treated as NOT placed: the residual risk of a
// duplicate stop is protective-side (it flattens, never reverses past the tag
// guard), while skipping placement on a failed lookup could leave the position
// with no stop at all.
func (s *CompanionStopper) alreadyPlaced(ctx context.Context, broker models.IBroker, entryOrderID uint) bool {
	s.mu.Lock()
	_, ok := s.placed[entryOrderID]
	s.mu.Unlock()
	if ok {
		return true
	}

	if broker == nil {
		return false
	}

	orders, err := broker.FetchOrders(ctx)
	if err != nil {
		log.Warnf("companion stop: could not check broker orders for an existing stop for entry order %d (placing anyway — a duplicate stop fails protective-side): %v", entryOrderID, err)
		return false
	}

	tag := CompanionStopTagForEntry(entryOrderID)
	for _, o := range orders {
		if o != nil && o.Tag == tag {
			s.markPlaced(entryOrderID)
			log.Infof("companion stop: entry order %d already has companion stop at the broker (tag %q, broker order %d) — skipping duplicate placement", entryOrderID, tag, o.ID)
			return true
		}
	}

	return false
}

func (s *CompanionStopper) markPlaced(entryOrderID uint) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.placed[entryOrderID] = struct{}{}
	delete(s.unprotected, entryOrderID)
}

// recordFailure records a live position left without its protective exit:
// Error log with the entry-order context, failure counter on the internal
// registry (ADR-0005: no OpenTelemetry), and an unprotected-position record
// the alert engine turns into a "position UNPROTECTED" Alert.
func (s *CompanionStopper) recordFailure(fill EntryFill, cause error) {
	telemetry.CompanionStopFailures.Add(1)

	log.Errorf("companion stop FAILED — position UNPROTECTED: %s qty %d (entry order %d, fill price %v) has NO broker-held exit: %v", fill.Symbol, fill.Quantity, fill.EntryOrderID, fill.FillPrice, cause)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.unprotected[fill.EntryOrderID] = telemetry.UnprotectedPosition{
		EntryOrderID: fill.EntryOrderID,
		Symbol:       fill.Symbol,
		Quantity:     fill.Quantity,
		Reason:       cause.Error(),
	}
}

// UnprotectedPositions returns the positions whose companion stops failed to
// place, for the alert engine's unprotected_position rule.
func (s *CompanionStopper) UnprotectedPositions() []telemetry.UnprotectedPosition {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]telemetry.UnprotectedPosition, 0, len(s.unprotected))
	for _, u := range s.unprotected {
		out = append(out, u)
	}
	return out
}

// PlaceForFill places the companion stop for one committed, eligible live
// entry fill, idempotently per entry order. It NEVER returns an error to the
// caller's control flow decision — the fill already happened at the broker and
// must stay committed — but reports (placed, err) so tests and logs can see
// the outcome. All failures run through the loud recordFailure path.
//
// The placement goes through the IBroker seam DIRECTLY (PlaceCompanionStop),
// below the halt-gated Playground.PlaceOrder path, so an engaged kill switch
// can never strand the position without its protective exit — the halt bypass
// holds by routing, not by an exemption in the gate.
func (s *CompanionStopper) PlaceForFill(ctx context.Context, broker models.IBroker, mode models.Mode, fill EntryFill) (bool, error) {
	if fill.EntryOrderID == 0 {
		err := fmt.Errorf("entry order ID is required for idempotent companion-stop placement")
		s.recordFailure(fill, err)
		return false, err
	}

	if s.alreadyPlaced(ctx, broker, fill.EntryOrderID) {
		return false, nil
	}

	req, err := PlaceCompanionStop(ctx, broker, mode, fill, s.cfg)
	if err != nil {
		s.recordFailure(fill, err)
		return false, err
	}

	if req == nil {
		// Simulation-Mode no-op (defensive: eligibility excludes Simulation).
		return false, nil
	}

	s.markPlaced(fill.EntryOrderID)
	telemetry.CompanionStopsPlaced.Add(1)
	log.Infof("companion stop placed for entry order %d: %s %s qty %d @ stop %v (tag %q)", fill.EntryOrderID, fill.Symbol, req.Sides[0], fill.Quantity, *req.StopPrice, req.Tag)

	return true, nil
}

// The companion stopper is a package-level hook, mirroring models.SetOrderGate
// and SetGuardRegistry: wired once by main at process startup when
// COMPANION_STOP_DISTANCE is configured, nil everywhere else. A nil stopper
// keeps the live fill pipeline byte-for-byte unaffected — the feature is
// completely inert until deliberately armed by the operator.
var (
	companionStopperMu sync.RWMutex
	companionStopper   *CompanionStopper
)

// SetCompanionStopper installs the process-wide companion stopper. Passing nil
// removes it (used to reset in tests). Safe for concurrent use.
func SetCompanionStopper(s *CompanionStopper) {
	companionStopperMu.Lock()
	defer companionStopperMu.Unlock()
	companionStopper = s
}

// ActiveCompanionStopper returns the installed stopper, or nil when
// companion stops are unarmed.
func ActiveCompanionStopper() *CompanionStopper {
	companionStopperMu.RLock()
	defer companionStopperMu.RUnlock()
	return companionStopper
}

// UnprotectedPositions is the alert-engine provider: the installed stopper's
// failure records, or nothing when companion stops are unarmed.
func UnprotectedPositions() []telemetry.UnprotectedPosition {
	s := ActiveCompanionStopper()
	if s == nil {
		return nil
	}
	return s.UnprotectedPositions()
}

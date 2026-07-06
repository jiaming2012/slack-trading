package services

import (
	"context"
	"math"

	backtester_models "github.com/jiaming2012/slack-trading/src/go/backtester/models"
	"github.com/jiaming2012/slack-trading/src/go/backtester/safety"
)

// companionStopBroker resolves the IBroker seam for companion-stop placement:
// the playground's own live account, falling back to the reconcile
// playground's. A nil result flows into PlaceForFill, whose validation turns
// it into the loud unprotected-position failure path rather than a panic.
func companionStopBroker(playground *backtester_models.Playground) backtester_models.IBroker {
	if la := playground.GetLiveAccount(); la != nil {
		return la.GetBroker()
	}

	if rp := playground.GetReconcilePlayground(); rp != nil {
		if la := rp.GetLiveAccount(); la != nil {
			return la.GetBroker()
		}
	}

	return nil
}

// maybePlaceCompanionStop runs at the end of fillPendingOrder, AFTER a live
// fill has committed and its order record has been re-saved
// (wire-companion-stops). For every eligible Paper/Margin equity entry fill it
// places the broker-held protective stop through the IBroker seam DIRECTLY —
// below the halt-gated Playground.PlaceOrder path — so an engaged kill switch
// can never strand the just-committed position without its protective exit
// (the halt bypass holds by routing, never by an exemption in the gate).
//
// It NEVER propagates an error: the fill already happened at the broker, and
// failing our bookkeeping would desync us from reality. Placement failures are
// loud instead — Error log, failure counter, and a "position UNPROTECTED"
// alert — all inside the stopper. A nil stopper (COMPANION_STOP_DISTANCE
// unset) keeps this a strict no-op, and Simulation fills are excluded by
// eligibility, so replay behavior is byte-for-byte unchanged.
func maybePlaceCompanionStop(playground *backtester_models.Playground, order *backtester_models.OrderRecord, fillPrice, fillQuantity float64) {
	stopper := safety.ActiveCompanionStopper()
	if stopper == nil {
		return
	}

	meta := playground.GetMeta()
	if !safety.CompanionStopEligible(meta, order) {
		return
	}

	// One strategy order can net into MULTIPLE broker trades (buy 10 →
	// buy_to_cover 4 + buy 6), each committing through fillPendingOrder
	// separately. The stopper sizes each placement to the still-unprotected
	// delta against the entry's CUMULATIVE filled quantity, so every trade
	// tops up coverage and the total protective size tracks the total filled
	// size (never the requested total — an over-sized stop reverses instead
	// of flattening).
	totalFilled := math.Abs(order.GetFilledVolume())
	if totalFilled <= 0 {
		// Defensive: the trade link should already be committed by now; fall
		// back to this trade's own quantity rather than skipping protection.
		totalFilled = math.Abs(fillQuantity)
	}

	fill := safety.EntryFill{
		Symbol:              order.Symbol,
		EntrySide:           order.Side,
		FillPrice:           fillPrice,
		EntryOrderID:        order.ID,
		TotalFilledQuantity: totalFilled,
	}

	// Errors are fully handled (and made loud) inside PlaceForFill; the fill's
	// control flow must not depend on the stop placement outcome.
	_, _ = stopper.PlaceForFill(context.Background(), companionStopBroker(playground), meta.Mode, fill)
}

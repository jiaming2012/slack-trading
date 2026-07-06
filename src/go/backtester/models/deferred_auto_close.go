package models

import (
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/jiaming2012/slack-trading/src/go/telemetry"
)

// DeferredAutoClose retains an option-assignment / option-expiration auto-close
// that could not be placed because the kill-switch order gate was engaged
// (wire-companion-stops, review nit e). Assignment and expiration events fire
// exactly once, so a halted Tick must not drop the close — the constructed
// close request and its fill parameters are retained here and retried on each
// subsequent Tick until the halt clears, at which point the close is placed and
// committed exactly as it would have been originally.
type DeferredAutoClose struct {
	// Request is the fully constructed close order request from the original
	// assignment/expiration event.
	Request *CreateOrderRequest

	// SourceOrderID is the open order the auto-close is closing. It guards
	// against a later event (e.g. expiration following a deferred assignment
	// close across ticks) constructing a second close for the same order while
	// the first is still deferred.
	SourceOrderID uint

	// FillTime is the playground time at which the close would originally have
	// filled — the retained fill parameter, so a retried commit reproduces the
	// original execution.
	FillTime time.Time

	// EmitAssignmentEvent replays the expiration branch's follow-on behavior:
	// an expired ITM option that closes into an equity leg emits an
	// OptionAssigned event for the placed close order.
	EmitAssignmentEvent bool

	// Reason records the halt error that forced the deferral.
	Reason string
}

// GetDeferredAutoCloses returns the currently outstanding deferred auto-closes
// (exposure the operator is alerted about while the halt is engaged).
func (p *Playground) GetDeferredAutoCloses() []*DeferredAutoClose {
	return p.deferredAutoCloses
}

// hasDeferredAutoCloseFor reports whether a deferred auto-close is already
// outstanding for the given source order, so assignment/expiration events
// arriving on later ticks cannot double-close an order whose first auto-close
// is still deferred.
func (p *Playground) hasDeferredAutoCloseFor(sourceOrderID uint) bool {
	for _, d := range p.deferredAutoCloses {
		if d.SourceOrderID == sourceOrderID {
			return true
		}
	}
	return false
}

// deferAutoClose queues an auto-close for retry on subsequent ticks, logs the
// deferral loudly, and updates the internal-registry gauge (ADR-0005: internal
// telemetry only) that the alert engine's deferred-auto-close rule reads.
func (p *Playground) deferAutoClose(d *DeferredAutoClose) {
	p.deferredAutoCloses = append(p.deferredAutoCloses, d)
	p.updateDeferredAutoCloseGauge()

	log.Warnf("postTickProcessing: option auto-close for order %d (%s, tag %q) DEFERRED — kill switch is engaged (%s); the close is retained and will be retried each tick until the halt clears (%d deferred close(s) outstanding)",
		d.SourceOrderID, d.Request.Symbol, d.Request.Tag, d.Reason, len(p.deferredAutoCloses))
}

// updateDeferredAutoCloseGauge publishes the outstanding deferral count for
// this playground. Nil-instrument recording is a no-op, so this is safe before
// telemetry.Init.
func (p *Playground) updateDeferredAutoCloseGauge() {
	telemetry.DeferredAutoCloses.Set(float64(len(p.deferredAutoCloses)),
		telemetry.Label{Key: "playground_id", Value: p.ID.String()})
}

// placeAutoClose places one option auto-close through the database service and
// stages its execution-fill request for this tick's CommitOrderQueue, exactly
// as the original inline postTickProcessing block did. When emitAssignmentEvent
// is set (expiration into an equity leg) the follow-on OptionAssigned event is
// constructed and returned; the caller records it on the playground and the
// tick delta.
func (p *Playground) placeAutoClose(dbService IDatabaseService, req *CreateOrderRequest, fillTime time.Time, emitAssignmentEvent bool, executionRequests map[*OrderRecord]ExecutionFillRequest) (*TickDeltaEvent, error) {
	placeOrderResults, err := dbService.PlaceOrders(p.ID, []*CreateOrderRequest{req})
	if err != nil {
		return nil, fmt.Errorf("failed to place close order: %w", err)
	}

	placeOrderResult := placeOrderResults[0]

	executionRequests[placeOrderResult] = ExecutionFillRequest{
		Price:    placeOrderResult.RequestedPrice,
		Time:     fillTime,
		Quantity: placeOrderResult.GetQuantity(),
	}

	if !emitAssignmentEvent {
		return nil, nil
	}

	multiplier := 1.0
	if req.Side == TradierOrderSideSell || req.Side == TradierOrderSideSellShort {
		multiplier = -1.0
	}

	return &TickDeltaEvent{
		Type: TickDeltaEventTypeOptionAssigned,
		OptionAssignmentEvent: &OptionAssignmentEvent{
			OrderId:          placeOrderResult.ID,
			Symbol:           placeOrderResult.GetInstrument(),
			AssignedQuantity: req.Quantity * multiplier,
			AssignedPrice:    req.RequestedPrice,
			Timestamp:        fillTime,
		},
	}, nil
}

// placeOrDeferAutoClose consults the kill-switch order gate BEFORE placing an
// auto-close: while the halt is engaged the constructed request is deferred
// (retained with its fill parameters) instead of erroring the tick; while
// clear it is placed exactly as today.
func (p *Playground) placeOrDeferAutoClose(dbService IDatabaseService, req *CreateOrderRequest, sourceOrderID uint, emitAssignmentEvent bool, executionRequests map[*OrderRecord]ExecutionFillRequest) (*TickDeltaEvent, error) {
	if gateErr := CheckOrderGate(); gateErr != nil {
		p.deferAutoClose(&DeferredAutoClose{
			Request:             req,
			SourceOrderID:       sourceOrderID,
			FillTime:            p.GetCurrentTime(),
			EmitAssignmentEvent: emitAssignmentEvent,
			Reason:              gateErr.Error(),
		})
		return nil, nil
	}

	return p.placeAutoClose(dbService, req, p.GetCurrentTime(), emitAssignmentEvent, executionRequests)
}

// retryDeferredAutoCloses runs at the start of every postTickProcessing pass:
// while the halt is still engaged the deferrals are kept (and the standing
// warning re-logged); once the gate clears, each retained close is placed and
// staged for this tick's commit with its original fill parameters. A non-halt
// placement failure keeps the remaining deferrals (nothing is dropped) and
// errors the tick, matching today's loud behavior for auto-close failures.
func (p *Playground) retryDeferredAutoCloses(dbService IDatabaseService, executionRequests map[*OrderRecord]ExecutionFillRequest) ([]*TickDeltaEvent, error) {
	if len(p.deferredAutoCloses) == 0 {
		return nil, nil
	}

	if gateErr := CheckOrderGate(); gateErr != nil {
		log.Warnf("postTickProcessing: %d option auto-close(s) still DEFERRED — kill switch remains engaged (%v)", len(p.deferredAutoCloses), gateErr)
		p.updateDeferredAutoCloseGauge()
		return nil, nil
	}

	var events []*TickDeltaEvent
	for len(p.deferredAutoCloses) > 0 {
		d := p.deferredAutoCloses[0]

		event, err := p.placeAutoClose(dbService, d.Request, d.FillTime, d.EmitAssignmentEvent, executionRequests)
		if err != nil {
			p.updateDeferredAutoCloseGauge()
			return events, fmt.Errorf("failed to place deferred auto-close for order %d: %w", d.SourceOrderID, err)
		}

		p.deferredAutoCloses = p.deferredAutoCloses[1:]
		if event != nil {
			events = append(events, event)
		}

		log.Infof("postTickProcessing: deferred option auto-close for order %d (%s) placed after halt release (%d still outstanding)", d.SourceOrderID, d.Request.Symbol, len(p.deferredAutoCloses))
	}

	p.updateDeferredAutoCloseGauge()
	return events, nil
}

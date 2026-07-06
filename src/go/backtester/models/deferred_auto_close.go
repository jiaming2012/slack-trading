package models

import (
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"

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

	// RecordID is the persisted deferred_auto_closes row backing this deferral
	// (0 = not persisted). Deferrals originate from drain-once events, so they
	// are written to the database on deferral and deleted on successful
	// placement — a halt followed by a process restart must not silently drop
	// an auto-close.
	RecordID uint
}

// DeferredAutoCloseRecord is the persisted form of a DeferredAutoClose
// (adversarial-review finding 2): written on deferral, deleted on successful
// placement, and reloaded at playground load so deferrals survive a restart.
type DeferredAutoCloseRecord struct {
	gorm.Model
	PlaygroundID        uuid.UUID `gorm:"column:playground_id;type:uuid;index:idx_deferred_auto_close_playground;not null"`
	SourceOrderID       uint      `gorm:"column:source_order_id;not null"`
	FillTime            time.Time `gorm:"column:fill_time;type:timestamptz;not null"`
	EmitAssignmentEvent bool      `gorm:"column:emit_assignment_event;not null"`
	Reason              string    `gorm:"column:reason;type:text"`
	RequestJSON         string    `gorm:"column:request_json;type:jsonb;not null"`
}

// TableName pins the persisted deferral table.
func (DeferredAutoCloseRecord) TableName() string {
	return "deferred_auto_closes"
}

// ToRecord serializes the deferral for persistence.
func (d *DeferredAutoClose) ToRecord(playgroundID uuid.UUID) (*DeferredAutoCloseRecord, error) {
	if d.Request == nil {
		return nil, fmt.Errorf("DeferredAutoClose.ToRecord: request is nil")
	}

	reqJSON, err := json.Marshal(d.Request)
	if err != nil {
		return nil, fmt.Errorf("DeferredAutoClose.ToRecord: failed to serialize close request: %w", err)
	}

	rec := &DeferredAutoCloseRecord{
		PlaygroundID:        playgroundID,
		SourceOrderID:       d.SourceOrderID,
		FillTime:            d.FillTime,
		EmitAssignmentEvent: d.EmitAssignmentEvent,
		Reason:              d.Reason,
		RequestJSON:         string(reqJSON),
	}
	rec.ID = d.RecordID

	return rec, nil
}

// ToDeferredAutoClose deserializes a persisted deferral.
func (r *DeferredAutoCloseRecord) ToDeferredAutoClose() (*DeferredAutoClose, error) {
	var req CreateOrderRequest
	if err := json.Unmarshal([]byte(r.RequestJSON), &req); err != nil {
		return nil, fmt.Errorf("DeferredAutoCloseRecord.ToDeferredAutoClose: failed to deserialize close request (row %d): %w", r.ID, err)
	}

	return &DeferredAutoClose{
		Request:             &req,
		SourceOrderID:       r.SourceOrderID,
		FillTime:            r.FillTime,
		EmitAssignmentEvent: r.EmitAssignmentEvent,
		Reason:              r.Reason,
		RecordID:            r.ID,
	}, nil
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

// hasOrderWithTag reports whether any order in the playground carries the
// given tag (used to detect an already-placed exercise leg at restore time).
func (p *Playground) hasOrderWithTag(tag string) bool {
	for _, o := range p.GetAllOrders() {
		if o.Tag == tag {
			return true
		}
	}
	return false
}

// deferAutoClose queues an auto-close for retry on subsequent ticks, persists
// it (deferrals originate from drain-once events and must survive a restart),
// logs the deferral loudly, and updates the internal-registry gauge
// (ADR-0005: internal telemetry only).
func (p *Playground) deferAutoClose(dbService IDatabaseService, d *DeferredAutoClose) {
	if dbService != nil {
		if err := dbService.SaveDeferredAutoClose(p.ID, d); err != nil {
			log.Errorf("deferAutoClose: failed to PERSIST deferred auto-close for order %d — the close will be retried while this process lives but will be LOST if it restarts before the halt clears: %v", d.SourceOrderID, err)
		}
	}

	p.deferredAutoCloses = append(p.deferredAutoCloses, d)
	p.updateDeferredAutoCloseGauge()

	log.Warnf("postTickProcessing: option auto-close for order %d (%s, tag %q) DEFERRED — kill switch is engaged (%s); the close is retained and will be retried each tick until the halt clears (%d deferred close(s) outstanding)",
		d.SourceOrderID, d.Request.Symbol, d.Request.Tag, d.Reason, len(p.deferredAutoCloses))
}

// RestoreDeferredAutoCloses rehydrates the deferred-auto-close list from
// persisted records at playground load, so a halt followed by a restart does
// not drop the closes. Staleness is judged per request kind:
//
//   - CLOSE requests (CloseOrderId set) are stale when the source order no
//     longer has remaining open quantity — the close already committed some
//     other way, and replaying it would double-close.
//   - EXERCISE legs (no CloseOrderId — the exercised stock delivery of an
//     assigned/expired option) are INDEPENDENT of the source option order's
//     remaining quantity: the option close may have committed while the stock
//     leg is still owed. They are stale only when an order carrying the leg's
//     own exercise tag already exists — i.e. the leg itself was already
//     placed; replaying THAT would double the stock delivery.
func (p *Playground) RestoreDeferredAutoCloses(deferrals []*DeferredAutoClose) (stale []*DeferredAutoClose) {
	for _, d := range deferrals {
		if d.Request != nil && d.Request.CloseOrderId == nil {
			// Exercise leg.
			if d.Request.Tag != "" && p.hasOrderWithTag(d.Request.Tag) {
				log.Warnf("RestoreDeferredAutoCloses: deferred exercise leg row %d (order %d, tag %q) was already placed — treating as stale", d.RecordID, d.SourceOrderID, d.Request.Tag)
				stale = append(stale, d)
				continue
			}
			if d.Request.Tag == "" {
				log.Warnf("RestoreDeferredAutoCloses: deferred exercise leg row %d (order %d) carries no tag — restoring and retrying (retry-biased; verify no duplicate stock delivery)", d.RecordID, d.SourceOrderID)
			}

			p.deferredAutoCloses = append(p.deferredAutoCloses, d)
			continue
		}

		order, err := p.GetOrder(d.SourceOrderID)
		if err != nil {
			log.Warnf("RestoreDeferredAutoCloses: deferred auto-close row %d references missing order %d — treating as stale", d.RecordID, d.SourceOrderID)
			stale = append(stale, d)
			continue
		}

		remaining, err := order.GetRemainingOpenQuantity()
		if err != nil || math.Abs(remaining) <= 0 {
			log.Warnf("RestoreDeferredAutoCloses: deferred auto-close row %d for order %d has no remaining open quantity — treating as stale (already closed)", d.RecordID, d.SourceOrderID)
			stale = append(stale, d)
			continue
		}

		p.deferredAutoCloses = append(p.deferredAutoCloses, d)
	}

	if n := len(p.deferredAutoCloses); n > 0 {
		log.Warnf("RestoreDeferredAutoCloses: playground %s restored %d deferred option auto-close(s) from the database — open exposure retained across the restart; they will be retried each tick until the halt clears", p.ID, n)
	}

	p.updateDeferredAutoCloseGauge()
	return stale
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
		p.deferAutoClose(dbService, &DeferredAutoClose{
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

		// The close committed: remove its persisted row so a restart cannot
		// replay it. A failed delete is loud — a resurrected row would be
		// rejected by the pending-close quantity check at placement rather
		// than double-closing, but it would fail ticks until cleaned up.
		if err := dbService.DeleteDeferredAutoClose(d.RecordID); err != nil {
			log.Errorf("retryDeferredAutoCloses: deferred auto-close for order %d committed but its persisted row %d could not be deleted — clean it up manually or the restart-time restore will report it stale: %v", d.SourceOrderID, d.RecordID, err)
		}

		log.Infof("postTickProcessing: deferred option auto-close for order %d (%s) placed after halt release (%d still outstanding)", d.SourceOrderID, d.Request.Symbol, len(p.deferredAutoCloses))
	}

	p.updateDeferredAutoCloseGauge()
	return events, nil
}

package models

import (
	"fmt"
	"time"
)

// ReconcileBroker is the Broker adapter for reconcile playgrounds — the
// internal netting containers that hold the NET of all logical positions
// sharing one broker account. Only netting adjustment orders are placed
// here; ticking is not supported.
type ReconcileBroker struct{}

func (ReconcileBroker) PlaceOrder(p *Playground, order *OrderRecord) ([]*PlaceOrderChanges, error) {
	return p.placeReconcileAdjustmentOrder(order)
}

func (ReconcileBroker) Tick(p *Playground, d time.Duration, isPreview bool) (*TickDelta, error) {
	return nil, fmt.Errorf("tick is not supported in environment: %s", p.Meta.LegacyEnv)
}

func (p *Playground) placeReconcileAdjustmentOrder(order *OrderRecord) ([]*PlaceOrderChanges, error) {
	if !p.Meta.IsReconciliation() {
		return nil, fmt.Errorf("place order is not supported in %s environment", p.Meta.LegacyEnv)
	}

	changes, err := p.placeOrder(order)
	if err != nil {
		return nil, fmt.Errorf("failed to place adjustment order in reconcile playground: %w", err)
	}

	changes = append(changes, &PlaceOrderChanges{
		SaveIntents: []OrderSaveIntent{{Order: order, ForceNew: false}},
		Info:        "update live order record",
	})

	return changes, nil
}

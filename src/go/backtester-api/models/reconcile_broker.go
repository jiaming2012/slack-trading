package models

import (
	"fmt"
	"time"

	"gorm.io/gorm"
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
	return nil, fmt.Errorf("tick is not supported in environment: %s", p.Meta.Environment)
}

func (p *Playground) placeReconcileAdjustmentOrder(order *OrderRecord) ([]*PlaceOrderChanges, error) {
	if p.Meta.Environment != PlaygroundEnvironmentReconcile {
		return nil, fmt.Errorf("place order is not supported in %s environment", p.Meta.Environment)
	}

	changes, err := p.placeOrder(order)
	if err != nil {
		return nil, fmt.Errorf("failed to place adjustment order in reconcile playground: %w", err)
	}

	changes = append(changes, &PlaceOrderChanges{
		Commit: func(tx *gorm.DB) error {
			if err := p.GetLiveAccount().GetDatabase().SaveOrderRecordTx(tx, order, false); err != nil {
				return fmt.Errorf("failed to update live order record: %w", err)
			}

			return nil
		},
		Info: "update live order record",
	})

	return changes, nil
}

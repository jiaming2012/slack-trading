package models

import (
	"fmt"
	"time"
)

// Broker is the seam through which every order in every mode is placed and
// every tick is executed (ADR-0001). One adapter exists per mode —
// SimulatedBroker for Simulation, LiveBroker for Paper and Margin — plus the
// internal ReconcileBroker for reconciliation containers. Adapters are
// stateless; all account state lives on the Playground they operate on.
type Broker interface {
	PlaceOrder(p *Playground, order *OrderRecord) ([]*PlaceOrderChanges, error)
	Tick(p *Playground, d time.Duration, isPreview bool) (*TickDelta, error)
}

func brokerFor(meta *Meta) (Broker, error) {
	if meta.IsReconciliation() {
		return ReconcileBroker{}, nil
	}

	switch meta.Mode {
	case ModeSimulation:
		return SimulatedBroker{}, nil
	case ModePaper, ModeMargin:
		return LiveBroker{}, nil
	default:
		return nil, fmt.Errorf("unsupported playground mode: %s", meta.Mode)
	}
}

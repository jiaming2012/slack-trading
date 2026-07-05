package models

import (
	"fmt"
	"time"
)

// Broker is the seam through which every order in every mode is placed and
// every tick is executed (ADR-0001). One adapter exists per playground
// environment: SimulatedBroker, LiveBroker, ReconcileBroker. Adapters are
// stateless; all account state lives on the Playground they operate on.
type Broker interface {
	PlaceOrder(p *Playground, order *OrderRecord) ([]*PlaceOrderChanges, error)
	Tick(p *Playground, d time.Duration, isPreview bool) (*TickDelta, error)
}

func brokerFor(env PlaygroundEnvironment) (Broker, error) {
	switch env {
	case PlaygroundEnvironmentSimulator:
		return SimulatedBroker{}, nil
	case PlaygroundEnvironmentLive:
		return LiveBroker{}, nil
	case PlaygroundEnvironmentReconcile:
		return ReconcileBroker{}, nil
	default:
		return nil, fmt.Errorf("unsupported playground environment: %s", env)
	}
}

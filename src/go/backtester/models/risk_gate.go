package models

import "sync"

// RiskGate decides whether a proposed Simulation-mode order passes the
// portfolio risk overlay. It is a SEPARATE safety concern from the kill-switch
// OrderGate (order_gate.go): the kill switch is consulted for every order in
// every Mode at the Broker seam, whereas the risk gate is consulted only on the
// Simulation order-placement path and never on Paper or Margin. The two compose
// — CheckOrderGate runs first (all modes), then CheckRiskGate runs (Simulation
// only) — so installing a risk gate can never disable or bypass the kill
// switch.
//
// Like OrderGate this is a package-level hook rather than a struct field, so it
// can be wired once at startup. It defaults to nil, and a nil gate permits
// every order — so the risk overlay is completely inert (and the
// simulation / model-diff paths are byte-for-byte unaffected) unless explicitly
// wired.
type RiskGate interface {
	// EvaluateSimulationOrder returns nil when the order is permitted and a
	// non-nil error (carrying the limit breaches) when it is rejected.
	EvaluateSimulationOrder(p *Playground, order *OrderRecord) error
}

var (
	riskGateMu sync.RWMutex
	riskGate   RiskGate
)

// SetRiskGate installs the process-wide Simulation risk gate. Passing nil
// removes it (used to reset in tests). It is safe for concurrent use.
func SetRiskGate(g RiskGate) {
	riskGateMu.Lock()
	defer riskGateMu.Unlock()
	riskGate = g
}

// CheckRiskGate consults the installed risk gate for a Simulation-mode order.
// With no gate installed it returns nil (permit), so absence of the risk-overlay
// wiring can never block trading. The caller (Playground.PlaceOrder) invokes it
// only on the Simulation path.
func CheckRiskGate(p *Playground, order *OrderRecord) error {
	riskGateMu.RLock()
	g := riskGate
	riskGateMu.RUnlock()
	if g == nil {
		return nil
	}
	return g.EvaluateSimulationOrder(p, order)
}

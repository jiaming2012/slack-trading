package models

import "sync"

// OrderGate decides whether order submission is currently permitted. It is the
// single safety chokepoint consulted at the Broker seam (Playground.PlaceOrder)
// before any order in any Mode reaches a Broker. The safety package's
// HaltController implements it.
//
// The gate is a package-level hook rather than a field so it can be wired once
// at process startup without threading a controller through every construction
// path. Crucially it defaults to nil, and a nil gate permits every order — so
// the halt feature is completely inert (and the simulation / model-diff paths
// are byte-for-byte unaffected) unless explicitly wired by main at startup.
type OrderGate interface {
	// AllowOrder returns nil when submission is permitted and a non-nil error
	// (the halt rejection) when it is not.
	AllowOrder() error
}

var (
	orderGateMu sync.RWMutex
	orderGate   OrderGate
)

// SetOrderGate installs the process-wide order gate. Passing nil removes the
// gate (used to reset in tests). It is safe for concurrent use.
func SetOrderGate(g OrderGate) {
	orderGateMu.Lock()
	defer orderGateMu.Unlock()
	orderGate = g
}

// CheckOrderGate consults the installed gate. With no gate installed it returns
// nil (permit), so absence of the safety wiring can never block trading. It is
// exported so the RPC layer can reject synchronously as well, sharing the exact
// same single source of truth as the Broker-seam check.
func CheckOrderGate() error {
	orderGateMu.RLock()
	g := orderGate
	orderGateMu.RUnlock()
	if g == nil {
		return nil
	}
	return g.AllowOrder()
}

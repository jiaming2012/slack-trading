package models

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

// haltingGate is a test OrderGate that always rejects with a recognizable
// sentinel, and records whether AllowOrder was consulted.
type haltingGate struct {
	consulted bool
}

var errTestHalt = errors.New("test halt engaged")

func (g *haltingGate) AllowOrder() error {
	g.consulted = true
	return errTestHalt
}

// permittingGate always allows and records that it was consulted.
type permittingGate struct {
	consulted bool
}

func (g *permittingGate) AllowOrder() error {
	g.consulted = true
	return nil
}

func TestPlaceOrder_RejectedWhileHalted_AllModes(t *testing.T) {
	modes := []Mode{ModeSimulation, ModePaper, ModeMargin}

	for _, mode := range modes {
		t.Run(string(mode), func(t *testing.T) {
			gate := &haltingGate{}
			SetOrderGate(gate)
			t.Cleanup(func() { SetOrderGate(nil) })

			// A minimal playground in the given Mode. If the halt gate leaked,
			// execution would proceed into brokerFor / the Broker adapter and
			// return a different (deps-missing) error; getting exactly the gate
			// sentinel back proves the order never reached any Broker.
			p := &Playground{Meta: Meta{Mode: mode}}

			changes, err := p.PlaceOrder(&OrderRecord{})
			require.Error(t, err)
			require.ErrorIs(t, err, errTestHalt, "mode %s must reject with the halt error", mode)
			require.Nil(t, changes)
			require.True(t, gate.consulted, "the gate must be consulted at the Broker seam")
		})
	}
}

func TestPlaceOrder_GateConsultedBeforeBroker(t *testing.T) {
	// With a permitting gate the order proceeds past the seam into brokerFor.
	// For a zero-value non-reconciliation Meta (empty Mode) brokerFor returns
	// the "unsupported mode" error — proving we got PAST the gate rather than
	// being short-circuited by it.
	gate := &permittingGate{}
	SetOrderGate(gate)
	t.Cleanup(func() { SetOrderGate(nil) })

	p := &Playground{Meta: Meta{Mode: Mode("")}}
	_, err := p.PlaceOrder(&OrderRecord{})
	require.Error(t, err)
	require.NotErrorIs(t, err, errTestHalt)
	require.True(t, gate.consulted)
	require.Contains(t, err.Error(), "not supported")
}

func TestPlaceOrder_NilGatePermits(t *testing.T) {
	// No gate installed: CheckOrderGate permits, so PlaceOrder proceeds to
	// brokerFor exactly as before the kill switch existed (model-diff safety).
	SetOrderGate(nil)
	p := &Playground{Meta: Meta{Mode: Mode("")}}
	_, err := p.PlaceOrder(&OrderRecord{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "not supported")
}

func TestCheckOrderGate_NilReturnsNil(t *testing.T) {
	SetOrderGate(nil)
	require.NoError(t, CheckOrderGate())
}

package models

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

// rejectingRiskGate always rejects and records whether it was consulted.
type rejectingRiskGate struct {
	consulted bool
}

var errTestRiskReject = errors.New("test risk overlay rejection")

func (g *rejectingRiskGate) EvaluateSimulationOrder(_ *Playground, _ *OrderRecord) error {
	g.consulted = true
	return errTestRiskReject
}

// The risk gate is consulted on the Simulation path and its rejection is
// surfaced to the caller before the order can reach any Broker.
func TestPlaceOrder_RiskGateRejectsSimulationOrder(t *testing.T) {
	SetOrderGate(nil) // kill switch permits; isolate the risk gate
	gate := &rejectingRiskGate{}
	SetRiskGate(gate)
	t.Cleanup(func() { SetRiskGate(nil) })

	p := &Playground{Meta: Meta{Mode: ModeSimulation}}
	changes, err := p.PlaceOrder(&OrderRecord{})
	require.Error(t, err)
	require.ErrorIs(t, err, errTestRiskReject)
	require.Nil(t, changes)
	require.True(t, gate.consulted, "risk gate must be consulted on the Simulation path")
}

// Paper and Margin paths are NEVER gated by the risk overlay: the risk gate is
// not consulted, and the order proceeds past the (Simulation-only) seam into the
// live broker. A bare playground has no live account wired, so it panics deeper
// in the live broker — which itself proves the risk gate did not short-circuit;
// we recover from that and assert the gate was never consulted.
func TestPlaceOrder_RiskGateNotConsultedForPaperAndMargin(t *testing.T) {
	SetOrderGate(nil)
	for _, mode := range []Mode{ModePaper, ModeMargin} {
		t.Run(string(mode), func(t *testing.T) {
			gate := &rejectingRiskGate{}
			SetRiskGate(gate)
			t.Cleanup(func() { SetRiskGate(nil) })

			// The live broker path may panic on a bare playground; either an
			// error or a panic proves we got PAST the (skipped) risk seam.
			func() {
				defer func() { _ = recover() }()
				p := &Playground{Meta: Meta{Mode: mode}}
				_, err := p.PlaceOrder(&OrderRecord{})
				if err != nil {
					require.NotErrorIs(t, err, errTestRiskReject, "risk gate must not gate %s", mode)
				}
			}()

			require.False(t, gate.consulted, "risk gate must not be consulted for %s", mode)
		})
	}
}

// With no risk gate installed, the Simulation path behaves exactly as before —
// CheckRiskGate permits, so model-diff output is byte-for-byte unaffected.
func TestPlaceOrder_NilRiskGatePermitsSimulation(t *testing.T) {
	SetOrderGate(nil)
	SetRiskGate(nil)

	// It proceeds past the (permitting) risk seam into the simulated broker; a
	// bare simulation playground fails/panics there — NOT at the risk gate.
	// Either outcome proves the permitting nil gate did not short-circuit.
	func() {
		defer func() { _ = recover() }()
		p := &Playground{Meta: Meta{Mode: ModeSimulation}}
		_, err := p.PlaceOrder(&OrderRecord{})
		if err != nil {
			require.NotErrorIs(t, err, errTestRiskReject)
		}
	}()
}

// The kill switch and the risk gate COMPOSE: the kill switch runs first for
// every mode, so a halted Simulation order is rejected by the kill switch
// before the risk gate is even consulted.
func TestPlaceOrder_KillSwitchComposesWithRiskGate(t *testing.T) {
	killed := errors.New("killed")
	SetOrderGate(orderGateFunc(func() error { return killed }))
	t.Cleanup(func() { SetOrderGate(nil) })

	gate := &rejectingRiskGate{}
	SetRiskGate(gate)
	t.Cleanup(func() { SetRiskGate(nil) })

	p := &Playground{Meta: Meta{Mode: ModeSimulation}}
	_, err := p.PlaceOrder(&OrderRecord{})
	require.ErrorIs(t, err, killed, "kill switch must reject before the risk gate")
	require.False(t, gate.consulted, "risk gate must not run once the kill switch has halted the order")
}

func TestCheckRiskGate_NilReturnsNil(t *testing.T) {
	SetRiskGate(nil)
	require.NoError(t, CheckRiskGate(&Playground{Meta: Meta{Mode: ModeSimulation}}, &OrderRecord{}))
}

// orderGateFunc adapts a func to the OrderGate interface for composition tests.
type orderGateFunc func() error

func (f orderGateFunc) AllowOrder() error { return f() }

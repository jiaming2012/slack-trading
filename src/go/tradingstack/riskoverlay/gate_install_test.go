package riskoverlay

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	models "github.com/jiaming2012/slack-trading/src/go/backtester/models"
)

// countingSnapshot wraps a PortfolioSnapshotFunc and counts invocations, so
// composition tests can prove the overlay was (not) consulted.
type countingSnapshot struct {
	inner PortfolioSnapshotFunc
	calls int
}

func (c *countingSnapshot) fn() PortfolioSnapshotFunc {
	return func(p *models.Playground, order *models.OrderRecord) (PortfolioState, ProposedOrder, time.Time, error) {
		c.calls++
		return c.inner(p, order)
	}
}

// haltedOrderGate stands in for an engaged kill switch.
type haltedOrderGate struct{ err error }

func (g haltedOrderGate) AllowOrder() error { return g.err }

// newInstalledDefaultGate installs a REAL SimulationRiskGate — default
// (permissive, enabled) limits, real snapshot builder over seeded fakes — via
// models.SetRiskGate, returning the snapshot spy. Cleanup uninstalls.
func newInstalledDefaultGate(t *testing.T) *countingSnapshot {
	t.Helper()

	spy := &countingSnapshot{inner: BuildPortfolioSnapshot(
		NewFakeEvWeightLookup(map[string]float64{"cc-v7": 0.75, "wheel": 0.25}),
		NewFakeSectorLookup(map[string]string{"AAPL": "technology", "KO": "staples", "MSFT": "technology"}),
	)}
	gate := NewSimulationRiskGate(DefaultRiskLimits.Enabled, DefaultRiskLimits, NewFakeCrowdingLookup(), spy.fn())
	require.True(t, gate.Enabled(), "DefaultRiskLimits must default-enable the gate")

	models.SetRiskGate(gate)
	t.Cleanup(func() { models.SetRiskGate(nil) })
	return spy
}

// 6.4 — with the default (permissive, enabled) configuration installed at the
// models.SetRiskGate seam, an ordinary Simulation entry is permitted by the
// overlay: the gate IS consulted and changes nothing.
func TestInstalledGate_DefaultConfigPermitsOrdinarySimulationOrder(t *testing.T) {
	models.SetOrderGate(nil) // kill switch clear
	spy := newInstalledDefaultGate(t)
	p := newSnapshotFixture(t)

	entry := buyEntry("MSFT")
	entry.Tag = "cc-v7"

	require.NoError(t, models.CheckRiskGate(p, entry), "permissive defaults must not change the order outcome")
	require.Equal(t, 1, spy.calls, "the enabled gate must actually be consulted on the Simulation path")
}

// 6.4 — an engaged kill switch rejects BEFORE the overlay is consulted,
// regardless of the overlay's configuration: halts always win.
func TestInstalledGate_KillSwitchRejectsBeforeOverlay(t *testing.T) {
	killed := errors.New("kill switch engaged")
	models.SetOrderGate(haltedOrderGate{err: killed})
	t.Cleanup(func() { models.SetOrderGate(nil) })

	spy := newInstalledDefaultGate(t)
	p := newSnapshotFixture(t)

	_, err := p.PlaceOrder(buyEntry("MSFT"))
	require.ErrorIs(t, err, killed, "the kill switch must reject first")
	require.Equal(t, 0, spy.calls, "the overlay must never run once the kill switch has halted the order")
}

// 6.4 — Paper and Margin order-placement paths NEVER invoke the installed
// gate: the seam is structurally Simulation-only.
func TestInstalledGate_PaperAndMarginPathsNeverConsultIt(t *testing.T) {
	models.SetOrderGate(nil)
	spy := newInstalledDefaultGate(t)

	for _, mode := range []models.Mode{models.ModePaper, models.ModeMargin} {
		t.Run(string(mode), func(t *testing.T) {
			// A bare live-mode playground errors or panics DEEPER in the live
			// broker path — which itself proves the (skipped) risk seam did not
			// short-circuit. Only the consultation count matters here.
			func() {
				defer func() { _ = recover() }()
				p := &models.Playground{Meta: models.Meta{Mode: mode}}
				_, _ = p.PlaceOrder(buyEntry("MSFT"))
			}()
			require.Equal(t, 0, spy.calls, "risk gate must not be consulted for %s", mode)
		})
	}
}

// 6.4 — a rejection from a narrowed limit surfaces through the seam as a
// typed *RejectedError carrying the full breach list (this is what an
// operator-tightened config produces; the shipped defaults never reject).
func TestInstalledGate_NarrowedLimitRejectsThroughSeam(t *testing.T) {
	models.SetOrderGate(nil)

	limits := DefaultRiskLimits
	limits.MaxGrossExposure = 20_000 // fixture gross is 19_000 open + 40k order

	gate := NewSimulationRiskGate(true, limits, NewFakeCrowdingLookup(), BuildPortfolioSnapshot(
		NewFakeEvWeightLookup(map[string]float64{"cc-v7": 1}),
		NewFakeSectorLookup(map[string]string{"MSFT": "technology"}),
	))
	models.SetRiskGate(gate)
	t.Cleanup(func() { models.SetRiskGate(nil) })

	p := newSnapshotFixture(t)
	entry := &models.OrderRecord{
		Class:            models.OrderRecordClassEquity,
		Symbol:           "MSFT",
		Side:             models.TradierOrderSideBuyToOpen,
		AbsoluteQuantity: 100,
		RequestedPrice:   400,
		Tag:              "cc-v7",
	}

	err := models.CheckRiskGate(p, entry)
	require.Error(t, err)
	var rejected *RejectedError
	require.True(t, errors.As(err, &rejected))
	require.True(t, Decision{Breaches: rejected.Breaches}.HasBreach(LimitGrossExposure))
}

// Adversarial review MAJOR 1 — a system-generated ITM exercise settlement leg
// (plain `buy`, IsSystemOrder=true, exactly the shape CreateCloseOrderRequests
// emits) passes the installed gate even under an operator-NARROWED gross
// exposure limit, while the identical non-system buy is rejected. Without the
// bypass, a held call expiring ITM would breach the narrowed limit, error the
// tick with no deferral, and wedge every subsequent tick.
func TestInstalledGate_SystemSettlementLegBypassesNarrowedLimit(t *testing.T) {
	models.SetOrderGate(nil)

	limits := DefaultRiskLimits
	limits.MaxGrossExposure = 20_000 // fixture gross is 19_000 open; the 23k settlement leg would breach

	gate := NewSimulationRiskGate(true, limits, NewFakeCrowdingLookup(), BuildPortfolioSnapshot(
		NewFakeEvWeightLookup(map[string]float64{"cc-v7": 1}),
		NewFakeSectorLookup(map[string]string{"AAPL": "technology"}),
	))
	models.SetRiskGate(gate)
	t.Cleanup(func() { models.SetRiskGate(nil) })

	p := newSnapshotFixture(t)

	// The settlement leg CreateCloseOrderRequests emits for an exercised ITM
	// call: underlying stock delivery at the strike, side=buy, system order.
	settlement := &models.OrderRecord{
		Class:            models.OrderRecordClassEquity,
		Symbol:           "AAPL",
		Side:             models.TradierOrderSideBuy,
		AbsoluteQuantity: 100,
		RequestedPrice:   230, // strike
		Tag:              "exercise-call-option-1",
		IsSystemOrder:    true,
	}
	require.NoError(t, models.CheckRiskGate(p, settlement), "a system settlement leg must bypass the overlay so the tick can complete")

	// The identical order WITHOUT the system flag is a real entry and is
	// rejected by the narrowed limit — proving the bypass is the system flag,
	// not a hole in the limit.
	entry := *settlement
	entry.IsSystemOrder = false
	entry.Tag = "cc-v7"
	err := models.CheckRiskGate(p, &entry)
	require.Error(t, err)
	var rejected *RejectedError
	require.True(t, errors.As(err, &rejected))
}

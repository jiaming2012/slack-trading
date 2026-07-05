package safety

import (
	"github.com/jiaming2012/slack-trading/src/go/telemetry"
)

// InstallHaltTelemetry wires the halt controller to the internal telemetry
// registry (ADR-0005: internal module only, no OpenTelemetry): the
// safety_halt_engaged gauge is set from the restored state immediately (the
// startup-restore reading) and on every subsequent controller transition via
// the transition listener. Recording through nil instruments is a no-op, so
// this is safe to call before telemetry.Init in tests.
func InstallHaltTelemetry(c *HaltController) {
	update := func(st HaltState) {
		v := 0.0
		if st.Engaged {
			v = 1.0
		}
		telemetry.HaltEngaged.Set(v)
	}
	c.SetTransitionListener(update)
	update(c.Status())
}

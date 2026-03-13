package eventmodels

type TrackerType string

const (
	TrackerTypeStart   TrackerType = "start"
	TrackerTypeStop    TrackerType = "stop"
	TrackerTypeSignal  TrackerType = "signal"
	TrackerTypeStartFx TrackerType = "start_fx"
)

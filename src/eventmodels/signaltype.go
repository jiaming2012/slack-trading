package eventmodels

type SignalType int

const (
	SignalTypeEntry SignalType = iota
	SignalTypeExit
	SignalTypeReset
)

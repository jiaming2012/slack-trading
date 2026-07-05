package models

type SignalType int

const (
	SignalTypeEntry SignalType = iota
	SignalTypeExit
	SignalTypeReset
)

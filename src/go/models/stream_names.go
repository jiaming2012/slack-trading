package models

import "fmt"

type StreamName string

const (
	AccountsStream        StreamName = "accounts"
	OptionAlertsStream    StreamName = "option-alerts"
	OptionChainTickStream StreamName = "option-chain-ticks"
	StockTickStream       StreamName = "stock-ticks"
	OptionContractStream  StreamName = "option-contracts"
	FxTicksStream         StreamName = "fx-ticks"
	TrackersStream        StreamName = "trackers"
	CandleStream          StreamName = "candles"
	TradeSignalStream     StreamName = "trade-signals"
)

func NewCandleStreamName(symbol string) StreamName {
	return StreamName(fmt.Sprintf("%s-%s", CandleStream, symbol))
}

func NewStockTickStreamName(name string) StreamName {
	return StreamName(fmt.Sprintf("%s-%s", StockTickStream, name))
}

func NewOptionChainTickStreamName(name OptionSymbol) StreamName {
	return StreamName(fmt.Sprintf("%s-%s", OptionChainTickStream, name))
}

func NewFxTickStreamName(symbol FxSymbol) StreamName {
	return StreamName(fmt.Sprintf("%s-%s", FxTicksStream, symbol))
}

// NewSimSignalStreamName returns an opaque stream name for persisted sim signals.
// Sim signals go to per-playground streams, NOT the global trade-signals stream.
func NewSimSignalStreamName(playgroundID string) StreamName {
	return StreamName(fmt.Sprintf("%s-sim-%s", TradeSignalStream, playgroundID))
}


package eventmodels

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
)

func NewStockTickStreamName(name string) StreamName {
	return StreamName(fmt.Sprintf("%s-%s", StockTickStream, name))
}

func NewOptionChainTickStreamName(name OptionSymbol) StreamName {
	return StreamName(fmt.Sprintf("%s-%s", OptionChainTickStream, name))
}

func NewFxTickStreamName(symbol FxSymbol) StreamName {
	return StreamName(fmt.Sprintf("%s-%s", FxTicksStream, symbol))
}

package eventmodels

type StreamName string

const (
	AccountsStream        StreamName = "accounts"
	OptionAlertsStream    StreamName = "option-alerts"
	OptionChainTickStream StreamName = "option-chain-ticks"
	StockTickStream       StreamName = "stock-ticks"
	OptionContractStream  StreamName = "option-contracts"
	TrackersStream        StreamName = "trackers"
)

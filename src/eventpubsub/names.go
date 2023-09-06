package eventpubsub

const (
	NewTickEvent             = "NewTickEvent"
	NewCandleEvent           = "NewCandleEvent"
	RsiTradeSignal           = "RsiTradeSignal"
	BotTradeRequestEvent     = "BotTradeRequestEvent"
	TradeRequestEvent        = "TradeRequestEvent"
	TradeFulfilledEvent      = "TradeFulfilledEvent"
	VolumeRequestEvent       = "VolumeRequestEvent"
	VolumeResultEvent        = "VolumeResultEvent"
	BalanceRequestEvent      = "BalanceRequestEvent"
	BalanceResultEvent       = "BalanceResultEvent"
	GetAccountsRequestEvent  = "GetAccountsRequestEvent"
	GetAccountsResponseEvent = "GetAccountsResponseEvent"
	AddAccountRequestEvent   = "AddAccountRequestEvent"
	AddAccountResponseEvent  = "AddAccountResponseEvent"
	SupportBreakSignal       = "SupportBreakSignal"
	ResistanceBreakSignal    = "ResistanceBreakSignal"
	TrendlineBreakSignal     = "TrendlineBreakSignal"
	AddStrategyRequest       = "AddStrategyRequest"
	Error                    = "DefaultError"
)

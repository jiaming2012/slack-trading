package eventpubsub

type EventName string

const (
	NewTickEvent              EventName = "NewTickEvent"
	NewCandleEvent            EventName = "NewCandleEvent"
	RsiTradeSignal            EventName = "RsiTradeSignal"
	BotTradeRequestEvent      EventName = "BotTradeRequestEvent"
	TradeRequestEvent         EventName = "TradeRequestEvent"
	TradeFulfilledEvent       EventName = "TradeFulfilledEvent"
	VolumeRequestEvent        EventName = "VolumeRequestEvent"
	VolumeResultEvent         EventName = "VolumeResultEvent"
	BalanceRequestEvent       EventName = "BalanceRequestEvent"
	BalanceResultEvent        EventName = "BalanceResultEvent"
	GetAccountsRequestEvent   EventName = "GetAccountsRequestEvent"
	GetAccountsResponseEvent  EventName = "GetAccountsResponseEvent"
	AddAccountRequestEvent    EventName = "AddAccountRequestEvent"
	AddAccountResponseEvent   EventName = "AddAccountResponseEvent"
	SupportBreakSignal        EventName = "SupportBreakSignal"
	ResistanceBreakSignal     EventName = "ResistanceBreakSignal"
	TrendlineBreakSignal      EventName = "TrendlineBreakSignal"
	AddStrategyRequest        EventName = "AddStrategyRequest"
	NewCloseTradesRequest     EventName = "NewCloseTradesRequest"
	ExecuteOpenTradeRequest   EventName = "ExecuteOpenTradeRequest"
	ExecuteCloseTradesRequest EventName = "ExecuteCloseTradesRequest"
	ExecuteOpenTradeResult    EventName = "ExecuteOpenTradeResult"
	ExecuteCloseTradesResult  EventName = "ExecuteCloseTradesResult"
	NewOpenTradeRequest       EventName = "NewOpenTradeRequest"
	FetchTradesRequest        EventName = "FetchTradesRequest"
	FetchTradesResult         EventName = "FetchTradesResult"
	Error                     EventName = "DefaultError"
)

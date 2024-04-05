package eventmodels

const (
	NewTickEventName                      EventName = "NewTickEvent"
	NewCandleEventName                    EventName = "NewCandleEvent"
	RsiTradeSignalEventName               EventName = "RsiTradeSignal"
	BotTradeRequestEventName              EventName = "BotTradeRequestEvent"
	TradeRequestEventName                 EventName = "TradeRequestEvent"
	TradeFulfilledEventName               EventName = "TradeFulfilledEvent"
	VolumeRequestEventName                EventName = "VolumeRequestEvent"
	VolumeResultEventName                 EventName = "VolumeResultEvent"
	BalanceRequestEventName               EventName = "BalanceRequestEvent"
	BalanceResultEventName                EventName = "BalanceResultEvent"
	GetAccountsRequestEventName           EventName = "GetAccountsRequestEvent"
	CreateAccountRequestEventName         EventName = "CreateAccountRequestEvent"
	AddAccountRequestEventName            EventName = "AddAccountRequestEvent"
	AddAccountResponseEventEventName      EventName = "AddAccountResponseEvent"
	SupportBreakSignalEventName           EventName = "SupportBreakSignal"
	ResistanceBreakSignalEventName        EventName = "ResistanceBreakSignal"
	TrendlineBreakSignalEventName         EventName = "TrendlineBreakSignal"
	AddStrategyRequestEventName           EventName = "AddStrategyRequest"
	CloseTradesRequestEventName           EventName = "CloseTradesRequest"
	ExecuteOpenTradeRequestEventName      EventName = "ExecuteOpenTradeRequest"
	ExecuteCloseTradeRequestEventName     EventName = "ExecuteCloseTradeRequest"
	ExecuteCloseTradesRequestEventName    EventName = "ExecuteCloseTradesRequest"
	ExecuteCloseTradesResultEventName     EventName = "ExecuteCloseTradesResult"
	CreateTradeRequestEventName           EventName = "NewOpenTradeRequest"
	NewGetStatsRequestEventName           EventName = "NewGetStatsRequest"
	GetStatsResultEventName               EventName = "GetStatsResult"
	FetchTradesRequestEventName           EventName = "FetchTradesRequest"
	FetchTradesResultEventName            EventName = "FetchTradesResult"
	CreateSignalRequestEventName          EventName = "NewSignalRequestEvent"
	CreateSignalResponseEventName         EventName = "NewSignalResultEvent"
	ManualDatafeedUpdateRequestEventName  EventName = "ManualDatafeedUpdateRequest"
	ManualDatafeedUpdateResultEventName   EventName = "ManualDatafeedUpdateResult"
	AutoExecuteTradeEventName             EventName = "AutoExecuteTrade"
	GetStrategiesRequestEventName         EventName = "GetStrategiesRequestEvent"
	CreateAccountStrategyRequestEventName EventName = "CreateAccountStrategyRequestEvent"
	ProcessRequestCompleteEventName       EventName = "ProcessRequestComplete"
	OpenTradeRequestEventName             EventName = "OpenTradeRequest"
	CloseTradeRequestEventName            EventName = "CloseTradeRequest"
	GetOptionAlertRequestEventName        EventName = "GetOptionAlertRequestEvent"
	CreateOptionAlertRequestEventName     EventName = "CreateOptionAlertRequestEvent"
	DeleteOptionAlertRequestEventName     EventName = "DeleteOptionAlertRequestEvent"
	OptionAlertUpdateEventName            EventName = "OptionAlertUpdateEvent"
	CreateNewOptionChainTickEvent         EventName = "CreateNewOptionChainTickEvent"
	CreateNewStockTickEvent               EventName = "CreateNewStockTickEvent"
	TerminalErrorName                     EventName = "TerminalError"
	Error                                 EventName = "DefaultError"
)

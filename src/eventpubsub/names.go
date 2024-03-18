package eventpubsub

type EventName string

const (
	NewTickEvent                                   EventName = "NewTickEvent"
	NewCandleEvent                                 EventName = "NewCandleEvent"
	RsiTradeSignal                                 EventName = "RsiTradeSignal"
	BotTradeRequestEvent                           EventName = "BotTradeRequestEvent"
	TradeRequestEvent                              EventName = "TradeRequestEvent"
	TradeFulfilledEvent                            EventName = "TradeFulfilledEvent"
	VolumeRequestEvent                             EventName = "VolumeRequestEvent"
	VolumeResultEvent                              EventName = "VolumeResultEvent"
	BalanceRequestEvent                            EventName = "BalanceRequestEvent"
	BalanceResultEvent                             EventName = "BalanceResultEvent"
	GetAccountsRequestEvent                        EventName = "GetAccountsRequestEvent"
	GetAccountsResponseEvent                       EventName = "GetAccountsResponseEvent"
	CreateAccountRequestEvent                      EventName = "CreateAccountRequestEvent"
	CreateAccountResponseEvent                     EventName = "CreateAccountResponseEvent"
	AddAccountRequestEvent                         EventName = "AddAccountRequestEvent"
	AddAccountResponseEvent                        EventName = "AddAccountResponseEvent"
	SupportBreakSignal                             EventName = "SupportBreakSignal"
	ResistanceBreakSignal                          EventName = "ResistanceBreakSignal"
	TrendlineBreakSignal                           EventName = "TrendlineBreakSignal"
	AddStrategyRequest                             EventName = "AddStrategyRequest"
	CloseTradesRequest                             EventName = "CloseTradesRequest"
	ExecuteOpenTradeRequest                        EventName = "ExecuteOpenTradeRequest"
	ExecuteCloseTradeRequest                       EventName = "ExecuteCloseTradeRequest"
	ExecuteCloseTradesRequest                      EventName = "ExecuteCloseTradesRequest"
	ExecuteOpenTradeResult                         EventName = "ExecuteOpenTradeResult"
	ExecuteCloseTradesResult                       EventName = "ExecuteCloseTradesResult"
	NewOpenTradeRequest                            EventName = "NewOpenTradeRequest"
	NewGetStatsRequest                             EventName = "NewGetStatsRequest"
	GetStatsResult                                 EventName = "GetStatsResult"
	FetchTradesRequest                             EventName = "FetchTradesRequest"
	FetchTradesResult                              EventName = "FetchTradesResult"
	NewSignalRequestEvent                          EventName = "NewSignalRequestEvent"
	NewSignalResultEvent                           EventName = "NewSignalResultEvent"
	ManualDatafeedUpdateRequest                    EventName = "ManualDatafeedUpdateRequest"
	ManualDatafeedUpdateResult                     EventName = "ManualDatafeedUpdateResult"
	AutoExecuteTrade                               EventName = "AutoExecuteTrade"
	GetStrategiesRequestEvent                      EventName = "GetStrategiesRequestEvent"
	CreateAccountStrategyRequestEvent              EventName = "CreateAccountStrategyRequestEvent"
	CreateStrategyResponseEvent                    EventName = "CreateStrategyResponseEvent"
	CreateAccountRequestEventStoredSuccess         EventName = "CreateAccountRequestEventStoredSuccess"
	CreateAccountStrategyRequestEventStoredSuccess EventName = "CreateAccountStrategyRequestEventStoredSuccess"
	NewSignalRequestEventStoredSuccess             EventName = "NewSignalRequestEventStoredSuccess"
	// RequestCompletedEvent                          EventName = "RequestCompletedEvent"
	ProcessRequestComplete         EventName = "ProcessRequestComplete"
	OpenTradeRequest               EventName = "OpenTradeRequest"
	CloseTradeRequest              EventName = "CloseTradeRequest"
	GetOptionAlertRequestEvent     EventName = "GetOptionAlertRequestEvent"
	GetOptionAlertResponseEvent    EventName = "GetOptionAlertResponseEvent"
	CreateOptionAlertRequestEvent  EventName = "CreateOptionAlertRequestEvent"
	CreateOptionAlertResponseEvent EventName = "CreateOptionAlertResponseEvent"
	DeleteOptionAlertRequestEvent  EventName = "DeleteOptionAlertRequestEvent"
	DeleteOptionAlertResponseEvent EventName = "DeleteOptionAlertResponseEvent"
	Error                          EventName = "DefaultError"
)

package models

// type IPlayground interface {
// 	GetMeta() *PlaygroundMeta
// 	GetId() uuid.UUID
// 	GetReconcilePlayground() IReconcilePlayground
// 	GetClientId() *string
// 	GetBalance() float64
// 	GetEquity(positions map[eventmodels.Instrument]*Position) float64
// 	GetEquityPlot() []*eventmodels.EquityPlot
// 	GetOrders() []*OrderRecord
// 	GetPosition(symbol eventmodels.Instrument, checkExists bool) (Position, error)
// 	GetPositions() (map[eventmodels.Instrument]*Position, error)
// 	GetRepositories() []*CandleRepository
// 	GetCandle(symbol eventmodels.Instrument, period time.Duration) (*eventmodels.PolygonAggregateBarV2, error)
// 	GetFreeMargin() (float64, error)
// 	PlaceOrder(order *OrderRecord) ([]*PlaceOrderChanges, error)
// 	Tick(d time.Duration, isPreview bool) (*TickDelta, error)
// 	GetFreeMarginFromPositionMap(positions map[eventmodels.Instrument]*Position) float64
// 	GetOpenOrders(symbol eventmodels.Instrument) []*OrderRecord
// 	GetCurrentTime() time.Time
// 	NextOrderID() uint
// 	FetchCandles(symbol eventmodels.Instrument, period time.Duration, from time.Time, to time.Time) ([]*eventmodels.AggregateBarWithIndicators, error)
// 	CommitPendingOrder(order *OrderRecord, startingPositions map[eventmodels.Instrument]*Position, orderFillRequest ExecutionFillRequest, performChecks bool) (newTrade *TradeRecord, invalidOrder *OrderRecord, err error)
// 	// RejectOrder(order *OrderRecord, reason string, database IDatabaseService) error
// 	SetEquityPlot(equityPlot []*eventmodels.EquityPlot)
// 	GetLiveAccountType() LiveAccountType
// 	SetOpenOrdersCache() error
// }

// type Playground struct {
// 	Meta               *PlaygroundMeta
// 	ID                 uuid.UUID
// 	ClientID           *string
// 	account            *BacktesterAccount
// 	clock              *Clock
// 	repos              map[eventmodels.Instrument]map[time.Duration]*CandleRepository
// 	isBacktestComplete bool
// 	positionCache     map[eventmodels.Instrument]*Position
// 	openOrdersCache    map[eventmodels.Instrument][]*OrderRecord
// 	minimumPeriod      time.Duration
// 	Broker             IBroker
// }

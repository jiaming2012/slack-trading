package models



// type LivePlayground struct {
// 	playground      *Playground
// 	liveAccount     *LiveAccount
	
// }

// func (p *LivePlayground) GetReconciliationOrders() []*OrderRecord {
// 	return p.GetReconcilePlayground().GetOrders()
// }

// func (p *LivePlayground) GetReconcilePlayground() IReconcilePlayground {
// 	return p.liveAccount.GetReconcilePlayground()
// }

// func (p *LivePlayground) GetClientId() *string {
// 	return p.playground.GetClientId()
// }

// func (p *LivePlayground) GetLiveAccountType() LiveAccountType {
// 	return p.liveAccount.Source.GetAccountType()
// }

// func (p *LivePlayground) SetOpenOrdersCache() error {
// 	return p.playground.SetOpenOrdersCache()
// }


// // func (p *LivePlayground) GetAccount() *LiveAccount {
// // 	return p.account
// // }

// func (p *LivePlayground) GetRepositories() []*CandleRepository {
// 	return p.playground.GetRepositories()
// }

// func (p *LivePlayground) GetMeta() *PlaygroundMeta {
// 	return p.playground.GetMeta()
// }

// func (p *LivePlayground) GetId() uuid.UUID {
// 	return p.playground.GetId()
// }

// func (p *LivePlayground) GetBalance() float64 {
// 	return p.playground.GetBalance()
// }

// func (p *LivePlayground) GetEquity(positions map[eventmodels.Instrument]*Position) float64 {
// 	return p.playground.GetEquity(positions)
// }

// func (p *LivePlayground) GetOrders() []*OrderRecord {
// 	return p.playground.GetOrders()
// }

// func (p *LivePlayground) GetPosition(symbol eventmodels.Instrument, checkExists bool) (Position, error) {
// 	return p.playground.GetPosition(symbol, checkExists)
// }

// func (p *LivePlayground) GetPositions() (map[eventmodels.Instrument]*Position, error) {
// 	return p.playground.GetPositions()
// }

// func (p *LivePlayground) GetCandle(symbol eventmodels.Instrument, period time.Duration) (*eventmodels.PolygonAggregateBarV2, error) {
// 	return p.playground.GetCandle(symbol, period)
// }

// func (p *LivePlayground) GetFreeMargin() (float64, error) {
// 	return p.playground.GetFreeMargin()
// }

// func (p *LivePlayground) CommitPendingOrder(order *OrderRecord, startingPositions map[eventmodels.Instrument]*Position, orderFillRequest ExecutionFillRequest, performChecks bool) (newTrade *TradeRecord, invalidOrder *OrderRecord, err error) {
// 	return p.playground.CommitPendingOrder(order, startingPositions, orderFillRequest, performChecks)
// }

// func (p *LivePlayground) SetEquityPlot(plot []*eventmodels.EquityPlot) {
// 	p.playground.SetEquityPlot(plot)
// }

// func (p *LivePlayground) GetEquityPlot() []*eventmodels.EquityPlot {
// 	return p.playground.GetEquityPlot()
// }





// func (p *LivePlayground) GetFreeMarginFromPositionMap(positions map[eventmodels.Instrument]*Position) float64 {
// 	return p.playground.GetFreeMarginFromPositionMap(positions)
// }

// func (p *LivePlayground) GetOpenOrders(symbol eventmodels.Instrument) []*OrderRecord {
// 	return p.playground.GetOpenOrders(symbol)
// }

// func (p *LivePlayground) GetCurrentTime() time.Time {
// 	return time.Now()
// }

// func (p *LivePlayground) NextOrderID() uint {
// 	return p.playground.NextOrderID()
// }

// func (p *LivePlayground) FetchCandles(symbol eventmodels.Instrument, period time.Duration, from, to time.Time) ([]*eventmodels.AggregateBarWithIndicators, error) {
// 	return p.playground.FetchCandles(symbol, period, from, to)
// }

// func (p *LivePlayground) RejectOrder(order *OrderRecord, reason string) error {
// 	return p.playground.RejectOrder(order, reason, p.database)
// }

// func NewLivePlayground(playgroundID *uuid.UUID, database IDatabaseService, clientID *string, liveAccount *LiveAccount, startingBalance float64, repositories []*CandleRepository, newCandlesQueue *eventmodels.FIFOQueue[*BacktesterCandle], newTradesQueue *eventmodels.FIFOQueue[*TradeRecord], orders []*OrderRecord, now time.Time, tags []string) (*LivePlayground, error) {
// 	playground, err := NewPlayground(playgroundID, clientID, startingBalance, startingBalance, nil, orders, PlaygroundEnvironmentLive, now, tags, repositories...)
// 	if err != nil {
// 		return nil, fmt.Errorf("NewLivePlayground: failed to create playground: %w", err)
// 	}

// 	playground.SetBroker(liveAccount.Broker)

// 	playground.Meta.SourceBroker = liveAccount.Source.GetBroker()
// 	playground.Meta.SourceAccountId = liveAccount.Source.GetAccountID()
// 	playground.Meta.LiveAccountType = liveAccount.Source.GetAccountType()

// 	log.Debugf("adding newCandlesQueue(%p) to NewLivePlayground", newCandlesQueue)

// 	return &LivePlayground{
// 		playground:      playground,
// 		database:        database,
// 		liveAccount:     liveAccount,
// 		newCandlesQueue: newCandlesQueue,
// 		newTradesQueue:  newTradesQueue,
// 	}, nil
// }
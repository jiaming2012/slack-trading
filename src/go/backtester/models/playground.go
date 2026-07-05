package models

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/jiaming2012/slack-trading/src/go/models"
	"github.com/jiaming2012/slack-trading/src/go/utils"
)

// "github.com/lib/pq"
type Playground struct {
	gorm.Model
	Meta
	ID                          uuid.UUID                            `gorm:"type:uuid;default:uuid_generate_v4();primaryKey"`
	account                     *BacktesterAccount                   `gorm:"-"`
	clock                       *Clock                               `gorm:"-"`
	ClientID                    *string                              `gorm:"column:client_id;type:text;unique"`
	Balance                     float64                              `gorm:"column:balance;type:numeric;not null"`
	BrokerName                  *string                              `gorm:"column:broker;type:text"`
	AccountID                   *string                              `gorm:"column:account_id;type:text"`
	Orders                      []*OrderRecord                       `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
	EquityPlotRecords           []EquityPlotRecord                   `gorm:"foreignKey:PlaygroundID;references:ID"`
	ParentID                    *uuid.UUID                           `gorm:"column:parent_id;type:uuid;index:idx_parent_id"`
	Repositories                CandleRepositoryRecord               `gorm:"type:json"`
	ReconcilePlaygroundID       *uuid.UUID                           `gorm:"column:reconcile_playground_id;type:uuid;index:idx_reconcile_playground_id"`
	LiveAccountID               *uint                                `gorm:"column:live_account_id;type:bigint;index:idx_live_account_id"`
	LiveAccount                 ILiveAccount                         `gorm:"-"`
	ReconcilePlayground         IReconcilePlayground                 `gorm:"-"`
	repos                       *CandleMasterRepository              `gorm:"-"`
	isBacktestComplete          bool                                 `gorm:"-"`
	Events                      []*TickDeltaEvent                    `gorm:"-"`
	OptionsBroker               IOptionsBroker                       `gorm:"-"`
	positionCache               *PositionsCache                      `gorm:"-"`
	openOrdersCache             *OpenOrdersCache                     `gorm:"-"`
	newCandlesQueue             *models.FIFOQueue[*BacktesterCandle] `json:"-" gorm:"-"`
	newTradesQueue              *models.FIFOQueue[*TradeRecord]      `json:"-" gorm:"-"`
	invalidOrdersQueue          *models.FIFOQueue[*OrderRecord]      `json:"-" gorm:"-"`
	minimumPeriod               time.Duration                        `gorm:"-"` // This is a new field
	placeOrderMutex             *sync.Mutex                          `json:"-" gorm:"-"`
	newOrdersQueueMutex         *sync.Mutex                          `json:"-" gorm:"-"`
	pendingOrdersQueueMutex     *sync.Mutex                          `json:"-" gorm:"-"`
	exerciseOptionsRequestQueue *ExerciseOptionRequestQueue          `json:"-" gorm:"-"`
	signalRepo                  ISignalRepository                    `json:"-" gorm:"-"`
}

func (p *Playground) ExerciseOption(orderId uint, assignedQuantity, assignedPrice float64) error {
	if p.OptionsBroker == nil {
		return fmt.Errorf("no options broker available")
	}

	order, err := p.GetOrder(orderId)
	if err != nil {
		return fmt.Errorf("failed to get order: %w: %w", err, ErrOptionAssignmentOrderNotFound)
	}

	if assignedQuantity <= 0 {
		return fmt.Errorf("assigned quantity must be greater than zero")
	}

	remainingQty, err := order.GetRemainingOpenQuantity()
	if err != nil {
		return fmt.Errorf("failed to get remaining open quantity: %w", err)
	}

	if assignedQuantity > math.Abs(remainingQty) {
		return fmt.Errorf("assigned quantity exceeds remaining open quantity: %w", ErrOptionAssignmentInvalidQuantity)
	}

	optionContract, ok := order.GetInstrument().(*models.OptionContractV3)
	if !ok {
		return fmt.Errorf("order instrument is not an option contract")
	}

	switch optionContract.OptionType {
	case models.OptionTypeCall:
		if assignedPrice < optionContract.Strike {
			return fmt.Errorf("assigned price %.2f is below strike price %.2f for a call option: %w", assignedPrice, optionContract.Strike, ErrOptionAssignmentInvalidPrice)
		}
	case models.OptionTypePut:
		if assignedPrice > optionContract.Strike {
			return fmt.Errorf("assigned price %.2f is above strike price %.2f for a put option: %w", assignedPrice, optionContract.Strike, ErrOptionAssignmentInvalidPrice)
		}
	default:
		return fmt.Errorf("exercise option not yet implemented for %s", optionContract.OptionType)
	}

	p.exerciseOptionsRequestQueue.Enqueue(&models.ExerciseOptionRequest{
		Order:            order,
		AssignedQuantity: assignedQuantity,
		AssignmentPrice:  assignedPrice,
	})

	return nil
}

func (p *Playground) GetPlaceOrderLock() *sync.Mutex {
	return p.placeOrderMutex
}

func (p *Playground) GetOrder(orderID uint) (*OrderRecord, error) {
	for _, o := range p.GetAllOrders() {
		if o.ID == orderID {
			return o, nil
		}
	}

	return nil, fmt.Errorf("order %d not found", orderID)
}

func (p *Playground) GetEquityReportItems(dbService IDatabaseService) ([]LiveAccountPlot, error) {
	items, err := dbService.GetEquityPlots(p.ID)
	if err != nil {
		return nil, fmt.Errorf("GetEquityReport: error getting equity plots: %v", err)
	}

	return items, nil
}

func (p *Playground) AddToPendingOrdersQueue(order *OrderRecord) {
	p.pendingOrdersQueueMutex.Lock()
	defer p.pendingOrdersQueueMutex.Unlock()

	p.newOrdersQueueMutex.Lock()
	defer p.newOrdersQueueMutex.Unlock()

	for i, o := range p.account.NewOrders {
		if o.ID == order.ID {
			// remove order from new orders queue
			p.account.NewOrders = append(p.account.NewOrders[:i], p.account.NewOrders[i+1:]...)
			log.Debugf("order %d found in new orders queue. Moved to pending queue", order.ID)
		}
	}

	for _, o := range p.account.PendingOrders {
		if o.ID == order.ID {
			if o.ExternalOrderID != nil && order.ExternalOrderID != nil && *o.ExternalOrderID == *order.ExternalOrderID {
				log.Warnf("order %d already in pending orders queue", order.ID)
				return
			}
		}
	}

	order.Status = OrderRecordStatusPending
	p.account.PendingOrders = append(p.account.PendingOrders, order)
}

func (p *Playground) AddToNewOrdersQueue(order *OrderRecord) {
	p.newOrdersQueueMutex.Lock()
	defer p.newOrdersQueueMutex.Unlock()

	for _, o := range p.account.NewOrders {
		if o.ID == order.ID {
			log.Warnf("AddToNewOrdersQueue: order %d already in new orders queue. Returning ...", order.ID)
			return
		}
	}

	order.Status = OrderRecordStatusNew
	p.account.NewOrders = append(p.account.NewOrders, order)
	log.Infof("AddToNewOrdersQueue: order %d added to new orders queue", order.ID)
}

func (p *Playground) GetSource() (CreateAccountRequestSource, error) {
	if p.BrokerName == nil {
		return CreateAccountRequestSource{}, fmt.Errorf("Playground.GetSource: broker name is nil")
	}

	if p.AccountID == nil {
		return CreateAccountRequestSource{}, fmt.Errorf("Playground.GetSource: account id is nil")
	}

	return CreateAccountRequestSource{
		Broker:      *p.BrokerName,
		AccountID:   *p.AccountID,
		AccountRole: p.Meta.Role,
	}, nil
}

func (p *Playground) ResetStatusToPending(order *OrderRecord, dbService IDatabaseService) (commit func() error, dbCommit func() error, msg string) {
	var orderCommit func() error
	orderCommit, dbCommit, msg = order.ResetStatusToPending(dbService)
	if orderCommit != nil {
		commit = func() error {
			index := -1

			// find order in pending orders
			for i, o := range p.account.PendingOrders {
				if o.ID == order.ID {
					index = i
					break
				}
			}

			if index == -1 {
				for i, o := range p.account.Orders {
					if o.ID == order.ID {
						index = i
						break
					}
				}

				if index == -1 {
					return fmt.Errorf("order %d not found in orders/pending orders", order.ID)
				}

				// remove order from order queue
				p.account.Orders = append(p.account.Orders[:index], p.account.Orders[index+1:]...)

				// add order to pending orders
				p.account.PendingOrders = append(p.account.PendingOrders, order)
			}

			return orderCommit()
		}
	}

	return commit, dbCommit, msg
}

func (p *Playground) SetReconcilePlayground(playground IReconcilePlayground) {
	p.ReconcilePlayground = playground

	id := playground.GetId()
	p.ReconcilePlaygroundID = &id
}

func (p *Playground) TableName() string {
	return "playground_sessions"
}

func (p *Playground) GetReconcilePlayground() IReconcilePlayground {
	return p.ReconcilePlayground
}

func (p *Playground) SetBalance(balance float64) {
	p.account.Balance = balance
}

func (p *Playground) GetClientId() *string {
	return p.ClientID
}

func (p *Playground) GetMode() Mode {
	return p.Meta.Mode
}

func (p *Playground) GetAccountRole() AccountRole {
	return p.Meta.Role
}

func (p *Playground) SetEquityPlot(equityPlot []*models.EquityPlot) {
	p.account.EquityPlot = equityPlot
}

func (p *Playground) GetEquityPlot() []*models.EquityPlot {
	return p.account.EquityPlot
}

func (p *Playground) appendStat(currentTime time.Time, positionCache *PositionsCache) (*models.EquityPlot, error) {
	plot := &models.EquityPlot{
		Timestamp: currentTime,
		Value:     p.GetEquity(positionCache),
	}

	p.account.EquityPlot = append(p.account.EquityPlot, plot)

	return plot, nil
}

func (p *Playground) AddToOrderQueue(order *OrderRecord) error {
	index := -1

	// find order in pending orders
	for i, o := range p.account.PendingOrders {
		if o.ID == order.ID {
			index = i
			break
		}
	}

	newOrdersIndex := -1
	if index == -1 {
		log.Tracef("order %d not found in pending orders, check new orders queue ...", order.ID)

		for i, o := range p.account.NewOrders {
			if o.ID == order.ID {
				newOrdersIndex = i
				break
			}
		}

		if newOrdersIndex == -1 {
			for i := 0; i < len(p.account.Orders); i++ {
				if p.account.Orders[i].ID == order.ID {
					log.Tracef("order already %d found in order queue", order.ID)
					return nil
				}
			}

			return fmt.Errorf("order %d not found in pending or new orders queue", order.ID)
		}
	}

	// add order to order queue
	p.account.Orders = append(p.account.Orders, order)

	if index >= 0 {
		// remove order from pending orders
		p.account.PendingOrders = append(p.account.PendingOrders[:index], p.account.PendingOrders[index+1:]...)
	} else if newOrdersIndex >= 0 {
		// remove order from new orders
		p.account.NewOrders = append(p.account.NewOrders[:newOrdersIndex], p.account.NewOrders[newOrdersIndex+1:]...)
	} else {
		return fmt.Errorf("unexpected: order %d not found in pending, orders, or new queue", order.ID)
	}

	return nil
}

func (p *Playground) GetId() uuid.UUID {
	return p.ID
}

func (p *Playground) GetRepositories() []*CandleRepository {
	repos := make([]*CandleRepository, 0)
	for _, periodRepoMap := range p.repos.Iter() {
		for _, repo := range periodRepoMap {
			repos = append(repos, repo)
		}
	}

	return repos
}

func (p *Playground) GetOpenOrders(symbol models.Instrument) []*OrderRecord {
	return p.openOrdersCache.Get(symbol.GetTicker())
}

func (p *Playground) GetOpenOrder(id uint) *OrderRecord {
	cache, done := p.openOrdersCache.Iter()
	defer done()

	for _, orders := range cache {
		for _, order := range orders {
			if order.ID == id {
				return order
			}
		}
	}

	return nil
}

func (p *Playground) commitTradableOrderToOrderQueue(order *OrderRecord, positionCache *PositionsCache, orderFillEntry ExecutionFillRequest, performChecks bool) error {
	if !order.Status.IsTradingAllowed() {
		err := fmt.Errorf("order %d status is %s, which is no longer tradable", order.ID, order.Status)
		order.Reject(err)
		return fmt.Errorf("commitTradableOrderToOrderQueue: %w", err)
	}

	// check if the order is valid
	orderQuantity := order.GetQuantity()
	if performChecks {
		if orderQuantity == 0 {
			order.Reject(ErrInvalidOrderVolumeZero)
			return fmt.Errorf("commitTradableOrderToOrderQueue: order %d has zero quantity", order.ID)
		}
	}

	var freeMargin, initialMargin float64
	performMarginCheck := performChecks

	if performChecks {
		// margin check
		freeMargin = p.GetFreeMarginFromPositionMap(positionCache)
		initialMargin = calculateInitialMarginRequirement(orderQuantity, orderFillEntry.Price)
		position := positionCache.Get(order.GetInstrument().GetTicker())

		if position != nil {
			if position.Quantity < 0 && orderQuantity > 0 {
				if orderQuantity > math.Abs(position.Quantity) {
					if performChecks {
						order.Reject(ErrInvalidOrderVolumeLongVolume)
						return fmt.Errorf("commitTradableOrderToOrderQueue: order quantity %.2f exceeds short position of %.2f", orderQuantity, math.Abs(position.Quantity))
					}
				} else {
					performMarginCheck = false
				}
			} else if position.Quantity > 0 && orderQuantity < 0 {
				if math.Abs(orderQuantity) > position.Quantity {
					if performChecks {
						order.Reject(ErrInvalidOrderVolumeShortVolume)
						return fmt.Errorf("commitTradableOrderToOrderQueue: order quantity %.2f exceeds long position of %.2f", math.Abs(orderQuantity), position.Quantity)
					}
				} else {
					performMarginCheck = false
				}
			}
		}
	}

	// check if the order can be filled
	if performMarginCheck && freeMargin <= initialMargin {
		order.Reject(fmt.Errorf("%s: free_margin (%.2f) <= initial_margin (%.2f)", ErrInsufficientFreeMargin.Error(), freeMargin, initialMargin))
		return fmt.Errorf("commitTradableOrderToOrderQueue: order %d has insufficient free margin", order.ID)
	}

	return nil
}

func (p *Playground) CommitPendingOrder(order *OrderRecord, positionCache *PositionsCache, executionFillRequest ExecutionFillRequest, performChecks bool) (newOrder *OrderRecord, newTrade *TradeRecord, invalidOrder *OrderRecord, err error) {
	for _, o := range p.account.PendingOrders {
		if o.ID == order.ID {
			order = o
			if err := p.commitTradableOrderToOrderQueue(order, positionCache, executionFillRequest, performChecks); err != nil {
				order.Reject(err)
				invalidOrder = order

				if err := p.AddToOrderQueue(order); err != nil {
					return nil, nil, nil, fmt.Errorf("CommitPendingOrder: error adding order to order queue after commitTradableOrderToOrderQueue(): %v", err)
				}

				return nil, nil, invalidOrder, nil
			}

			newTrade, orderIsFilled, err := p.fillOrder(order, performChecks, executionFillRequest)
			if err != nil {
				order.Reject(err)
				invalidOrder = order
				log.Errorf("CommitPendingOrder: error filling order: %v", err)

				if err2 := p.AddToOrderQueue(order); err2 != nil {
					return nil, nil, nil, fmt.Errorf("CommitPendingOrder: error adding order to order queue after fillOrder(): %v", err2)
				}

				return nil, nil, invalidOrder, nil
			}

			if orderIsFilled {
				if err := p.AddToOrderQueue(order); err != nil {
					return nil, nil, nil, fmt.Errorf("commitPendingOrders: error adding order to order queue: %v", err)
				}
			}

			return order, newTrade, nil, nil
		}
	}

	for _, o := range p.account.Orders {
		if o.ID == order.ID {
			return nil, nil, nil, fmt.Errorf("order %d is already in order queue: %w", order.ID, ErrOrderAlreadyFilled)
		}
	}

	return nil, nil, nil, fmt.Errorf("order %d not found in pending orders", order.ID)
}

func (p *Playground) commitPendingOrders(executionFillMap map[*OrderRecord]ExecutionFillRequest, performChecks bool) (newTrades []*TradeRecord, invalidOrders []*OrderRecord, err error) {
	pendingOrders := make([]*OrderRecord, len(p.account.PendingOrders))

	copy(pendingOrders, p.account.PendingOrders)

	for _, order := range pendingOrders {
		// commented out bc the program already skips till the next market open before placing a trade, hence
		// this check is not reqired

		// calendar := p.clock.GetCalendar(p.GetCurrentTime())
		// if calendar == nil {
		// 	return nil, nil, fmt.Errorf("calendar not found for time %s", p.GetCurrentTime())
		// }

		// if !calendar.IsBetweenMarketHours(p.GetCurrentTime()) {
		// 	log.Debugf("order %d not filled because it is not between market hours", order.ID)
		// 	continue
		// }
		if !order.Status.IsTradingAllowed() {
			invalidOrders = append(invalidOrders, order)
			continue
		}

		orderFillEntry, found := executionFillMap[order]
		if !found {
			log.Warnf("error finding order filled entry price for order: %v", order.ID)
			continue
		}

		// positionCache, err := p.UpdatePositionCachePositions()
		positionCache := p.GetPositionCache()
		// if err != nil {
		// 	order.Reject(err)
		// 	invalidOrders = append(invalidOrders, order)
		// 	log.Errorf("error updating position cache: %v", err)
		// 	if err := p.AddToOrderQueue(order); err != nil {
		// 		return nil, nil, fmt.Errorf("commitPendingOrders: error adding order to order queue after UpdatePositionCachePositions(): %v", err)
		// 	}
		// 	continue
		// }

		if order.GetIsSystemOrder() {
			performChecks = false
		}

		if err := p.commitTradableOrderToOrderQueue(order, positionCache, orderFillEntry, performChecks); err != nil {
			order.Reject(err)
			invalidOrders = append(invalidOrders, order)
			log.Errorf("error committing pending order: %v", err)

			if err := p.AddToOrderQueue(order); err != nil {
				return nil, nil, fmt.Errorf("commitPendingOrders: error adding order to order queue after commitTradableOrderToOrderQueue(): %v", err)
			}

			continue
		}

		newTrade, orderIsFilled, err := p.fillOrder(order, performChecks, orderFillEntry)
		if err != nil {
			order.Reject(err)
			invalidOrders = append(invalidOrders, order)
			log.Errorf("error filling order: %v", err)

			if err := p.AddToOrderQueue(order); err != nil {
				return nil, nil, fmt.Errorf("commitPendingOrders: error adding order to order queue after fillOrder(): %v", err)
			}

			continue
		}

		newTrades = append(newTrades, newTrade)

		if orderIsFilled {
			if err := p.AddToOrderQueue(order); err != nil {
				return nil, nil, fmt.Errorf("commitPendingOrders: error adding order to order queue: %v", err)
			}
		}
	}

	return
}

func (p *Playground) updateOpenOrdersCache(openOrdersCache *OpenOrdersCache, newOrder *OrderRecord) error {
	// Check for close of open orders — iterate the COPY's cache directly
	// (not the live p.openOrdersCache) to avoid stale state from shared
	// order pointers between live and copy. We access the copy's internal
	// map without locks since the copy is not shared with other goroutines.
	for symbol, orders := range openOrdersCache.cache {
		var remaining []*OrderRecord
		for _, order := range orders {
			qty, err := order.GetRemainingOpenQuantity()
			if err != nil {
				return fmt.Errorf("updateOpenOrdersCache: error getting remaining open quantity: %w", err)
			}

			if math.Abs(qty) > 0 {
				remaining = append(remaining, order)
			}
		}
		openOrdersCache.cache[symbol] = remaining
	}

	// check for new open orders
	isOpen := newOrder.Side == TradierOrderSideBuy || newOrder.Side == TradierOrderSideSellShort || newOrder.Side == TradierOrderSideBuyToOpen || newOrder.Side == TradierOrderSideSellToOpen
	if isOpen {
		openOrdersCache.Add(newOrder)
	}

	return nil
}

func calcVwap(orders []*OrderRecord) float64 {
	var totalQuantity float64
	var totalValue float64

	for _, order := range orders {
		vol := order.GetFilledVolume()
		totalQuantity += vol
		totalValue += order.GetAvgFillPrice() * vol
	}

	if totalQuantity == 0 {
		return 0
	}

	return totalValue / totalQuantity
}

func (p *Playground) updatePositionsCache(openOrdersCache *OpenOrdersCache, positionCache *PositionsCache, symbol models.Instrument, trade *TradeRecord, isClose bool) {
	position := positionCache.Get(symbol.GetTicker())

	totalQuantity := position.Quantity + trade.Quantity

	// update the cost basis
	if totalQuantity == 0 {
		positionCache.Delete(symbol)
	} else {
		if !isClose {
			position.CostBasis = calcVwap(openOrdersCache.Get(symbol.GetTicker()))
		}

		// update the quantity
		position.Quantity = totalQuantity

		// update the maintenance margin
		position.MaintenanceMargin = calculateMaintenanceRequirement(position.Quantity, position.CostBasis)

		positionCache.Set(symbol, position)
	}
}

func (p *Playground) getPriceAt(symbol models.Instrument, timestamp time.Time) (float64, error) {
	repo, ok := p.repos.Get(symbol, p.minimumPeriod)
	if !ok {
		return 0, fmt.Errorf("getPriceAt: no repository found for symbol %s and period %s", symbol, p.minimumPeriod)
	}

	candle, err := repo.GetCandleAt(timestamp, 2*p.minimumPeriod)
	if err != nil {
		return 0, fmt.Errorf("getPriceAt: error getting candle at %s for symbol %s: %w", timestamp, symbol, err)
	}

	return candle.Close, nil
}

// todo: test this
func (p *Playground) populateRepo(symbol models.OptionSymbol, from time.Time, to *time.Time) (*CandleRepository, error) {
	candles, err := p.OptionsBroker.GetCandles(p.ID, symbol, p.minimumPeriod, from, to)
	if err != nil {
		return nil, fmt.Errorf("populateRepo: error getting option candles: %w", err)
	}

	var repo *CandleRepository
	var ok bool
	if p.repos.HasInstrument(symbol) {
		repo, ok = p.repos.Get(symbol, p.minimumPeriod)
		if !ok {
			return nil, fmt.Errorf("populateRepo: error getting candle repository: %w", err)
		}

		if err := repo.AddCandles(candles); err != nil {
			return nil, fmt.Errorf("populateRepo: error adding candles to repository: %w", err)
		}
	} else {
		var barsWithIndicators []*models.PolygonAggregateBarV2
		for _, c := range candles {
			barsWithIndicators = append(barsWithIndicators, c.ToPolygonAggregateBarV2())
		}

		indicators := []string{}
		historyInDays := uint32(0)
		repoSource := models.CandleRepositorySource{Type: "polygon"}

		repo, err = NewCandleRepository(symbol, p.minimumPeriod, barsWithIndicators, indicators, nil, historyInDays, repoSource)
		if err != nil {
			return nil, fmt.Errorf("populateRepo: error creating candle repository: %w", err)
		}

		repo.SetStartingPosition(p.GetCurrentTime(), p.Meta.Mode, nil)

		p.repos.Add(symbol, p.minimumPeriod, repo)
	}

	return repo, nil
}

func (p *Playground) getCurrentPrices(symbols []models.Instrument) (map[string]*Tick, error) {
	result := make(map[string]*Tick)

	if p.Meta.IsReconciliation() {
		if len(symbols) == 0 {
			return map[string]*Tick{}, nil
		}

		broker := p.GetLiveAccount().GetBroker()
		if broker == nil {
			return nil, errors.New("getCurrentPrice: broker is nil")
		}

		quotes, err := broker.FetchQuotes(context.Background(), symbols)
		if err != nil {
			return nil, fmt.Errorf("getCurrentPrice: error fetching quotes: %w", err)
		}

		for _, q := range quotes {
			ts := time.Unix(q.TradeDate, 0)
			result[q.Symbol] = &Tick{
				Symbol:    q.Symbol,
				Timestamp: ts,
				Value:     q.Last,
			}
		}

		return result, nil
	} else {
		for _, symbol := range symbols {
			var repo *CandleRepository
			var ok bool
			var optionSymbol models.OptionSymbol
			var expiration time.Time

			switch s := symbol.(type) {
			case models.StockSymbol:
				repo, ok = p.repos.Get(s, p.minimumPeriod)
				if !ok {
					return nil, fmt.Errorf("getCurrentPrice: no repository found for symbol %s and period %s", s, p.minimumPeriod)
				}

			case models.OptionSymbol:
				optionSymbol = s
				components, err := s.Components()
				if err != nil {
					return nil, fmt.Errorf("getCurrentPrice: error getting option components: %w", err)
				}

				expiration = components.Expiration

			case *models.OptionContractV3:
				optionSymbol = s.Symbol
				expiration = s.Expiration

			default:
				return nil, fmt.Errorf("getCurrentPrice: unsupported symbol type: %T", symbol)
			}

			// for option symbols, check if repo exists, if not, populate it
			if repo == nil {
				repo, ok = p.repos.Get(optionSymbol, p.minimumPeriod)
				if !ok {
					from := expiration.Add(-7 * 24 * time.Hour)
					if from.After(p.GetCurrentTime()) {
						from = p.GetCurrentTime()
					}

					to := expiration.Add(24 * time.Hour)

					var err error
					if repo, err = p.populateRepo(optionSymbol, from, &to); err != nil {
						return nil, fmt.Errorf("getCurrentPrice: error populating repo: %w", err)
					}
				}
			}

			candle, err := repo.GetCurrentCandle()
			if err != nil {
				return nil, fmt.Errorf("getCurrentPrice: error getting current candle: %w", err)
			}

			if candle == nil {
				log.Warnf("getCurrentPrice: current candle is nil for symbol %s", symbol.GetTicker())
				continue
				// return nil, ErrCurrentPriceNotSet
			}

			ticker := symbol.GetTicker()
			result[ticker] = &Tick{
				Symbol:    ticker,
				Timestamp: candle.Timestamp,
				Value:     candle.Close,
			}
		}

		return result, nil
	}
}

func (p *Playground) SetPositionCache(cache *PositionsCache) error {
	p.positionCache = cache
	return nil
}

func (p *Playground) GetPositionCache() *PositionsCache {
	return p.positionCache
}

func (p *Playground) SetOpenOrdersCache() error {
	p.openOrdersCache = NewOpenOrdersCache()
	for _, o := range p.account.Orders {
		if !o.Status.IsFilled() {
			continue
		}

		qty, err := o.GetRemainingOpenQuantity()
		if err != nil {
			continue
		}

		if math.Abs(qty) > 0 {
			p.openOrdersCache.Add(o)
		}
	}

	return nil
}

// func (p *Playground) addToOpenOrdersCache(order *OrderRecord) {
// 	p.addToCache(p.openOrdersCache, order)
// }

func (p *Playground) addToCache(cache map[models.Instrument][]*OrderRecord, order *OrderRecord) {
	openOrders, found := cache[order.GetInstrument()]
	if !found {
		openOrders = []*OrderRecord{}
	}

	cache[order.GetInstrument()] = append(openOrders, order)
}

// func (p *Playground) deleteFromOpenOrdersCache(symbol models.Instrument, index int) {
// 	p.openOrdersCache[symbol] = append(p.openOrdersCache[symbol][:index], p.openOrdersCache[symbol][index+1:]...)
// }

func (p *Playground) CancelOrder(order *OrderRecord, database IDatabaseService) error {
	if order.Status == OrderRecordStatusCanceled {
		log.Warnf("CancelOrder: order %d is already canceled. Returning ...", order.ID)
		return nil
	}

	if order.Status != OrderRecordStatusPending {
		return fmt.Errorf("order is not pending")
	}

	// The DB writes (order row + reconciled order rows) live behind a narrow
	// store method that owns its transaction; the in-memory queue mutations
	// below run after the store commit, outside any DB handle.
	if err := database.SaveCanceledOrderWithReconciles(order); err != nil {
		return fmt.Errorf("CancelOrder: failed to cancel order: %w", err)
	}

	err := func() error {
		for _, o := range order.Reconciles {
			// TODO: this should use the saga pattern - maybe temporal - so that all orders that were reconciled
			// are rolled back if any of them fail
			pg, e := database.GetPlayground(o.PlaygroundID)
			if e != nil {
				return fmt.Errorf("CancelOrder order.Reconciles: failed to get playground: %w", e)
			}

			o.Cancel()

			if e = pg.AddToOrderQueue(o); e != nil {
				return fmt.Errorf("CancelOrder order.Reconciles: failed to add order to order queue: %w", e)
			}
		}

		order.Cancel()

		if err := p.AddToOrderQueue(order); err != nil {
			return fmt.Errorf("CancelOrder: failed to add order to order queue: %w", err)
		}

		return nil
	}()

	if err != nil {
		return fmt.Errorf("CancelOrder: failed to cancel order: %w", err)
	}

	return nil
}

func (p *Playground) RejectOrder(order *OrderRecord, reason string, database IDatabaseService) error {
	p.account.mutex.Lock()
	defer p.account.mutex.Unlock()

	if order.Status == OrderRecordStatusRejected {
		log.Warnf("RejectOrder: order %d is already rejected. Returning ...", order.ID)
		return nil
	}

	if order.Status != OrderRecordStatusPending {
		return fmt.Errorf("order is not pending")
	}

	// The DB writes (order row + reconciled order rows) live behind a narrow
	// store method that owns its transaction; the in-memory queue mutations
	// below run after the store commit, outside any DB handle.
	if err := database.SaveRejectedOrderWithReconciles(order); err != nil {
		return fmt.Errorf("RejectOrder: failed to reject order: %w", err)
	}

	err := func() error {
		cause := fmt.Errorf(reason)

		for _, o := range order.Reconciles {
			// TODO: this should use the saga pattern - maybe temporal - so that all orders that were reconciled
			// are rolled back if any of them fail
			pg, e := database.GetPlayground(o.PlaygroundID)
			if e != nil {
				return fmt.Errorf("RejectOrder order.Reconciles: failed to get playground: %w", e)
			}

			o.Reject(cause)

			if e = pg.AddToOrderQueue(o); e != nil {
				return fmt.Errorf("RejectOrder order.Reconciles: failed to add order to order queue: %w", e)
			}
		}

		order.Reject(cause)

		if err := p.AddToOrderQueue(order); err != nil {
			return fmt.Errorf("RejectOrder: failed to add order to order queue: %w", err)
		}

		return nil
	}()

	if err != nil {
		return fmt.Errorf("RejectOrder: failed to reject order: %w", err)
	}

	return nil
}

func (p *Playground) closeOpenOrder(order *OrderRecord, openOrder *OrderRecord, pendingCloses map[*OrderRecord]float64, closeVolume float64) (float64, error) {
	qty, err := openOrder.GetRemainingOpenQuantity()
	if err != nil {
		return 0, fmt.Errorf("addClosesInfoToOrder: error getting remaining open quantity: %w", err)
	}

	remainingOpenQuantity := math.Abs(qty)
	if volume, found := pendingCloses[openOrder]; found {
		remainingOpenQuantity -= volume
	}

	if remainingOpenQuantity <= 0 {
		return 0, nil
	}

	volumeToClose := math.Min(closeVolume, remainingOpenQuantity)

	if _, found := pendingCloses[openOrder]; !found {
		pendingCloses[openOrder] = 0
	}
	pendingCloses[openOrder] += volumeToClose

	order.Closes = append(order.Closes, openOrder)

	return volumeToClose, nil
}

func (p *Playground) setCloseInfoToOrder(order *OrderRecord, position *Position) error {
	orderQty := order.GetQuantity()

	// check if the order is a close order — must have a close side AND
	// be reducing an existing position (not opening a new one on the opposite side)
	order.IsClose = false
	if position != nil && order.Side.IsCloseSide() {
		if position.Quantity > 0 && orderQty < 0 {
			order.IsClose = true
		} else if position.Quantity < 0 && orderQty > 0 {
			order.IsClose = true
		}
	}

	// add the order to the closes list of the open order
	if order.IsClose {
		closeVolume := math.Abs(orderQty)
		openOrders := p.GetOpenOrders(order.GetInstrument())
		pendingCloses := make(map[*OrderRecord]float64)

		if order.CloseOrderId == nil {
			for _, openOrder := range openOrders {
				if closeVolume <= 0 {
					break
				}

				volumeToClose, err := p.closeOpenOrder(order, openOrder, pendingCloses, closeVolume)
				if err != nil {
					return fmt.Errorf("addClosesInfoToOrder: error closing open order: %w", err)
				}

				closeVolume -= volumeToClose
			}
		} else {
			openOrders := p.GetOpenOrders(order.GetInstrument())
			foundOpenOrder := false

			for _, openOrder := range openOrders {
				if *order.CloseOrderId == openOrder.ID {
					remaining_volume, err := openOrder.GetRemainingOpenQuantity()
					if err != nil {
						return fmt.Errorf("addClosesInfoToOrder: error getting remaining open quantity: %w", err)
					}

					if closeVolume > math.Abs(remaining_volume) {
						return fmt.Errorf("addClosesInfoToOrder: close volume exceeds open order quantity for open order id %d", openOrder.ID)
					}

					if _, err := p.closeOpenOrder(order, openOrder, pendingCloses, closeVolume); err != nil {
						return fmt.Errorf("addClosesInfoToOrder: error closing open order: %w", err)
					}

					foundOpenOrder = true
				}
			}

			if !foundOpenOrder {
				return fmt.Errorf("addClosesInfoToOrder: open order id %d not found in open orders", *order.CloseOrderId)
			}
		}
	}

	return nil
}

func (p *Playground) validateCache(openOrdersCache *OpenOrdersCache, positionCache *PositionsCache) error {
	if openOrdersCache == nil {
		return fmt.Errorf("open orders cache is nil")
	}

	if positionCache == nil {
		return fmt.Errorf("positions cache is nil")
	}

	if openOrdersCache.Len() != positionCache.Len() {
		return fmt.Errorf("open orders cache length %d does not match positions cache length %d", openOrdersCache.Len(), positionCache.Len())
	}

	position := make(map[string]Position)
	cache, done := openOrdersCache.Iter()
	defer done()

	for symbol, orders := range cache {
		for _, order := range orders {
			if order == nil {
				continue
			}
			// if order.ID == 0 {
			// 	continue
			// }

			if _, ok := position[order.Symbol]; !ok {
				position[order.Symbol] = Position{}
			}

			pos := position[order.Symbol]

			openQty, err := order.GetRemainingOpenQuantity()
			if err != nil {
				return fmt.Errorf("open orders cache symbol %s: error getting remaining open quantity: %w", order.Symbol, err)
			}

			pos.Quantity += openQty
			position[order.Symbol] = pos
		}

		positionCache := positionCache.Get(symbol)
		fmt.Printf("position: %+v, positionCache: %+v\n", position[symbol], positionCache)
		if positionCache.Quantity != position[symbol].Quantity {
			return fmt.Errorf("open orders cache symbol %s quantity %f does not match positions cache quantity %f", symbol, positionCache.Quantity, position[symbol].Quantity)
		}
	}

	return nil
}

func (p *Playground) validateBalance() error {
	realized_pl := 0.0
	for _, order := range p.getAllOrders() {
		if order.Status == OrderRecordStatusFilled && order.IsClose {
			pl := order.CalcRealizedPL()
			realized_pl += pl
		}
	}

	balance := p.InitialBalance + realized_pl

	if math.Abs(balance-p.account.Balance) > 0.01 {
		return fmt.Errorf("balance validation failed: calculated balance %.2f does not match account balance %.2f", balance, p.account.Balance)
	}

	return nil
}

func (p *Playground) fillOrder(order *OrderRecord, performChecks bool, orderFillEntry ExecutionFillRequest) (*TradeRecord, bool, error) {
	position := p.positionCache.Get(order.GetInstrument().GetTicker())

	if performChecks {
		orderStatus := order.GetStatus()
		if !orderStatus.IsTradingAllowed() {
			return nil, false, fmt.Errorf("fillOrder: trading is not allowed for order with status %s", orderStatus)
		}

		if order.OrderType != Market {
			return nil, false, fmt.Errorf("fillOrder: %d is not market, found %s", order.ID, order.OrderType)
		}

		if order.Class != OrderRecordClassEquity && order.Class != OrderRecordClassOption {
			log.Errorf("fillOrders: only equity and option orders are supported")
			return nil, false, fmt.Errorf("fillOrders: only equity and option orders are supported")
		}

		if err := p.isSideAllowed(order.GetInstrument(), order.Side, position.Quantity, false); err != nil {
			order.Status = OrderRecordStatusRejected
			return nil, false, fmt.Errorf("fillOrders: error checking side allowed: %v", err)
		}
	}

	// commit the trade
	var trade *TradeRecord
	if orderFillEntry.Trade != nil {
		trade = orderFillEntry.Trade
	} else {
		trade = NewTradeRecord(order, orderFillEntry.Time, orderFillEntry.Quantity, orderFillEntry.Price)
	}

	// Assign trade ID for simulator playgrounds (no DB auto-increment)
	if p.Meta.Mode == ModeSimulation && trade.ID == 0 {
		trade.ID = p.NextTradeID()
	}

	orderIsFilled, err := order.Fill(trade)
	if err != nil {
		if errors.Is(err, ErrOrderAlreadyFilled) {
			log.Warnf("order %d already filled", order.ID)
			return nil, true, nil
		}

		return nil, false, fmt.Errorf("fillOrder: error filling order: %w", err)
	}

	setCloseInfo := true
	closeByRequests, err := p.getCloseByRequests(order, position, setCloseInfo)
	if err != nil {
		return nil, false, fmt.Errorf("fillOrder: error getting close by requests: %w", err)
	}

	// close the open orders
	for _, req := range closeByRequests {
		// todo: used to reflect req.Quantity, but it should be trade.Quantity
		// closeBy := NewTradeRecord(order, orderFillEntry.Time, req.Quantity, orderFillEntry.Price)
		if req.Quantity != trade.Quantity {
			partialTrade := NewTradeRecord(order, orderFillEntry.Time, req.Quantity, orderFillEntry.Price)
			partialTrade.ParentTrade = trade
			if p.Meta.Mode == ModeSimulation {
				partialTrade.ID = p.NextTradeID()
			}
			req.Order.ClosedBy = append(req.Order.ClosedBy, partialTrade)
		} else {
			req.Order.ClosedBy = append(req.Order.ClosedBy, trade)
		}
	}

	openOrdersCacheCopy := p.openOrdersCache.Copy()

	// update copy: remove the open orders that were closed in cache
	p.updateOpenOrdersCache(openOrdersCacheCopy, order)

	positionCacheCopy := p.positionCache.Copy()

	// update the copy positions cache
	p.updatePositionsCache(openOrdersCacheCopy, positionCacheCopy, order.GetInstrument(), trade, order.IsClose)

	if performChecks {
		if err := p.validateCache(openOrdersCacheCopy, positionCacheCopy); err != nil {
			log.Debugf("rolling back order %v", order.ID)

			order.Rollback(trade)

			return nil, false, fmt.Errorf("fillOrder: error validating cache after filling order %d: %w", order.ID, err)
		}
	}

	// only update the position on the first fill
	if len(order.Trades) == 1 || len(order.ReconcileTrades) == 1 {
		order.PreviousPosition = *position
	}

	if order.IsClose {
		previousBalance := p.account.Balance
		order.PreviousBalance = &previousBalance
	}

	// update the account balance before updating the positions cache
	p.updateBalance(order.GetInstrument(), trade, p.positionCache)

	if err := p.validateBalance(); err != nil {
		log.Warnf("balance validation failed: %v", err)
	}

	// update the caches
	p.openOrdersCache.Commit(openOrdersCacheCopy)

	p.positionCache.Commit(positionCacheCopy)

	if orderFillEntry.Trade != nil {
		orderFillEntry.Trade = trade
	}

	return trade, orderIsFilled, nil
}

func (p *Playground) updateBalance(symbol models.Instrument, trade *TradeRecord, previousPositionCache *PositionsCache) {
	previousPosition := previousPositionCache.Get(symbol.GetTicker())

	isOptionContract := false
	switch symbol.(type) {
	case models.OptionSymbol, *models.OptionContractV3:
		isOptionContract = true
	}

	if previousPosition.Quantity > 0 {
		if trade.Quantity < 0 {
			closeQuantity := math.Min(previousPosition.Quantity, math.Abs(trade.Quantity))
			if isOptionContract {
				closeQuantity *= 100
			}

			pl := (trade.Price - previousPosition.CostBasis) * closeQuantity
			log.Debugf("(%.2f, %.2f, %.2f) [SELL] pl: %.2f, balance %f -> %f", trade.Price, previousPosition.CostBasis, closeQuantity, pl, p.account.Balance, p.account.Balance+pl)
			p.account.Balance += pl
		}
	} else if previousPosition.Quantity < 0 {
		if trade.Quantity > 0 {
			closeQuantity := math.Min(math.Abs(previousPosition.Quantity), trade.Quantity)
			if isOptionContract {
				closeQuantity *= 100
			}

			pl := (previousPosition.CostBasis - trade.Price) * closeQuantity
			log.Debugf("(%.2f, %.2f, %.2f) [COVER] pl: %.2f, balance %f -> %f", trade.Price, previousPosition.CostBasis, closeQuantity, pl, p.account.Balance, p.account.Balance+pl)
			p.account.Balance += pl
		}
	}
}

func (p *Playground) GetCurrentTime() time.Time {
	if p.Meta.Mode.IsRealtime() || p.Meta.IsReconciliation() {
		return time.Now()
	}

	return p.clock.CurrentTime
}

func (p *Playground) FetchCurrentPrice(ctx context.Context, symbol models.Instrument) (float64, error) {
	lastCandle, err := p.FetchCurrentCandle(ctx, symbol)
	if err != nil {
		return 0, fmt.Errorf("error fetching current candle: %w", err)
	}

	return lastCandle.Close, nil
}

func (p *Playground) FetchCurrentCandle(ctx context.Context, symbol models.Instrument) (*models.AggregateBarWithIndicators, error) {
	var optionSymbol *models.OptionSymbol

	switch s := symbol.(type) {
	case models.StockSymbol:
		result, err := p.getCurrentPrices([]models.Instrument{symbol})
		if err != nil {
			return nil, fmt.Errorf("error fetching current price: %w", err)
		}

		tick, found := result[s.GetTicker()]
		if !found {
			return nil, fmt.Errorf("symbol %s not found in result", symbol)
		}

		// todo: should be refactor to use a stockBroker interface to get candles
		return &models.AggregateBarWithIndicators{
			Timestamp: tick.Timestamp,
			Open:      tick.Value,
			High:      tick.Value,
			Low:       tick.Value,
			Close:     tick.Value,
			Volume:    0,
		}, nil
	case models.OptionSymbol:
		optionSymbol = &s
	case *models.OptionContractV3:
		optionSymbol = &s.Symbol
	}

	if optionSymbol != nil {
		// Start of trading day 9:30est
		tz, err := time.LoadLocation("America/New_York")
		if err != nil {
			return nil, fmt.Errorf("error loading timezone: %w", err)
		}
		from := time.Date(p.clock.CurrentTime.Year(), p.clock.CurrentTime.Month(), p.clock.CurrentTime.Day(), 9, 30, 0, 0, tz)
		to := time.Date(p.clock.CurrentTime.Year(), p.clock.CurrentTime.Month(), p.clock.CurrentTime.Day(), 16, 0, 0, 0, tz)
		candles, err := p.OptionsBroker.GetCandles(p.ID, *optionSymbol, p.minimumPeriod, from, &to)
		if err != nil {
			return nil, fmt.Errorf("error fetching option candles for %s: %w", symbol, err)
		}

		if len(candles) == 0 {
			return nil, fmt.Errorf("no candles found for option %s: %w", symbol, models.ErrNoCandlesFound)
		}

		latestCandle := candles[len(candles)-1]
		return latestCandle, nil
	}

	return nil, fmt.Errorf("fetchCurrentPrice: unsupported symbol type %T", symbol)
}

func (p *Playground) performLiquidations(symbol models.Instrument, position *Position, tag string) (*OrderRecord, error) {
	var order *OrderRecord

	requestedPrice, err := p.FetchCurrentPrice(context.Background(), symbol)
	if err != nil {
		return nil, fmt.Errorf("error fetching price: %w", err)
	}

	isSystemOrder := true
	if position.Quantity > 0 {
		id := p.account.NextOrderID()
		order, err = NewOrderRecord(id, nil, nil, p.ID, OrderRecordClassEquity, p.Meta.Role, p.clock.CurrentTime, symbol.GetTicker(), TradierOrderSideSell, position.Quantity, Market, Day, requestedPrice, nil, nil, OrderRecordStatusPending, tag, nil, isSystemOrder, nil, nil)
		if err != nil {
			return nil, fmt.Errorf("error creating order record: %w", err)
		}
	} else if position.Quantity < 0 {
		id := p.account.NextOrderID()
		order, err = NewOrderRecord(id, nil, nil, p.ID, OrderRecordClassEquity, p.Meta.Role, p.clock.CurrentTime, symbol.GetTicker(), TradierOrderSideBuyToCover, math.Abs(position.Quantity), Market, Day, requestedPrice, nil, nil, OrderRecordStatusPending, tag, nil, isSystemOrder, nil, nil)
		if err != nil {
			return nil, fmt.Errorf("error creating order record: %w", err)
		}
	} else {
		return nil, nil
	}

	p.AddToPendingOrdersQueue(order)

	orderFillPriceMap := map[*OrderRecord]ExecutionFillRequest{}

	for _, order := range p.account.PendingOrders {
		price, err := p.FetchCurrentPrice(context.Background(), order.GetInstrument())
		if err != nil {
			return nil, fmt.Errorf("error fetching price: %w", err)
		}

		orderFillPriceMap[order] = ExecutionFillRequest{
			Price:    price,
			Quantity: order.GetQuantity(),
			Time:     p.clock.CurrentTime,
		}
	}

	_, invalidOrders, _, err := p.CommitOrderQueue(orderFillPriceMap)
	if err != nil {
		return nil, fmt.Errorf("performLiquidations: error committing pending orders: %w", err)
	}

	if len(invalidOrders) > 0 {
		return nil, fmt.Errorf("performLiquidations: error committing pending orders: %d invalid orders: %v", len(invalidOrders), invalidOrders)
	}

	return order, nil
}

func (p *Playground) NextOrderID() uint {
	return p.account.NextOrderID()
}

func (p *Playground) NextTradeID() uint {
	return p.account.NextTradeID()
}

// checkForLiquidations checks for liquidations and returns a LiquidationEvent if liquidations are necessary
// Liquidations are performed in the following order:
// 1. Sort positions by position size (quantity * cost_basis) in descending order
// 2. Liquidate positions until equity reaches above maintenance margin or until all positions have been liquidated
func (p *Playground) checkForLiquidations(positionCache *PositionsCache) (*TickDeltaEvent, error) {
	equity := p.GetEquity(positionCache)
	maintenanceMargin := p.getMaintenanceMargin(positionCache)
	var liquidatedOrders []*OrderRecord
	for equity < maintenanceMargin && positionCache.Len() > 0 {
		sortedSymbols, sortedPositions := sortPositionsByQuantityDesc(positionCache)

		tag := fmt.Sprintf("liquidation - equity of %.2f < %.2f (maintenance margin)", equity, maintenanceMargin)

		order, err := p.performLiquidations(sortedSymbols[0], sortedPositions[0], tag)
		if err != nil {
			return nil, fmt.Errorf("error performing liquidations: %w", err)
		}

		if order != nil {
			liquidatedOrders = append(liquidatedOrders, order)
		}

		order.Attributes.Add("system_liquidation", "true")
		order.Attributes.Add("action", "liquidation")

		positionCache, err = p.UpdatePricesAndGetPositionCache()
		if err != nil {
			return nil, fmt.Errorf("error getting positions: %w", err)
		}

		maintenanceMargin = p.getMaintenanceMargin(positionCache)
	}

	if equity < maintenanceMargin {
		log.Warnf("equity, %.2f, still below maintenance margin, %.2f, after liquidating all positions", equity, maintenanceMargin)
	}

	if len(liquidatedOrders) > 0 {
		return &TickDeltaEvent{
			Type: TickDeltaEventTypeLiquidation,
			LiquidationEvent: &LiquidationEvent{
				OrdersPlaced: liquidatedOrders,
			},
		}, nil
	}

	return nil, nil
}

func (p *Playground) FetchCandles(symbol models.Instrument, period time.Duration, from time.Time, to *time.Time) ([]*models.AggregateBarWithIndicators, error) {
	repo, ok := p.repos.Get(symbol, period)
	if !ok {
		return nil, fmt.Errorf("period %s not found in repos", period)
	}

	candles, err := repo.FetchCandles(from, to)
	if err != nil {
		return nil, fmt.Errorf("error fetching candles: %w", err)
	}

	return candles, nil
}

func (p *Playground) updateAccountStats(currentTime time.Time) (*models.EquityPlot, error) {
	positions, err := p.UpdatePricesAndGetPositionCache()
	if err != nil {
		return nil, fmt.Errorf("error getting positions: %w", err)
	}

	return p.appendStat(currentTime, positions)
}

func (p *Playground) DeleteRepository(symbol models.Instrument) {
	p.repos.Delete(symbol)
}

func (p *Playground) CommitOrderQueue(orderExecutionRequests map[*OrderRecord]ExecutionFillRequest) ([]*TradeRecord, []*OrderRecord, *PositionsCache, error) {
	newTrades, invalidOrdersDTO, err := p.commitPendingOrders(orderExecutionRequests, true)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("error committing pending orders: %w", err)
	}

	positionCache, err := p.UpdatePositionCachePositions()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("error getting position cache: %w", err)
	}

	return newTrades, invalidOrdersDTO, positionCache, nil
}

func (p *Playground) Tick(d time.Duration, isPreview bool, dbService IDatabaseService) (*TickDelta, error) {
	broker, err := brokerFor(&p.Meta)
	if err != nil {
		return nil, fmt.Errorf("tick is not supported in environment: %s", p.Meta.LegacyEnv)
	}

	delta, err := broker.Tick(p, d, isPreview)
	if err != nil {
		return nil, err
	}

	if dbService != nil { // todo: make dbService non-nilable: currently nil to not break old tests
		delta, err = p.postTickProcessing(delta, dbService)
		if err != nil {
			return nil, fmt.Errorf("error in post tick processing: %w", err)
		}
	} else {
		log.Warnf("dbService is nil in %s tick", p.Meta.LegacyEnv)
	}

	return delta, nil
}

func (p *Playground) postTickProcessing(tickDelta *TickDelta, dbService IDatabaseService) (*TickDelta, error) {
	// Record all events
	p.Events = append(p.Events, tickDelta.Events...)
	var optionAssignmentEvents []*TickDeltaEvent

	// Close expired option contracts repos
	executionRequests := make(map[*OrderRecord]ExecutionFillRequest)
	// Track orders that already have close orders pending to prevent double-close
	// when both assignment and expiration events fire for the same order in one tick
	closedOrderIds := make(map[uint]bool)
	for _, event := range tickDelta.Events {
		if event.Type == TickDeltaEventTypeOptionAssigned {
			o := p.GetOpenOrder(event.OptionAssignmentEvent.OrderId)
			if o == nil {
				return nil, fmt.Errorf("failed to find open order for option assignment event: %d", event.OptionAssignmentEvent.OrderId)
			}

			remainingQty, err := o.GetRemainingOpenQuantity()
			if err != nil {
				return nil, fmt.Errorf("failed to get remaining open quantity for assigned order %d: %w", o.ID, err)
			}

			if math.Abs(remainingQty) <= 0 {
				log.Infof("skipping option assignment for order %d (%s): already fully closed (remaining qty=%.4f)", o.ID, o.Symbol, remainingQty)
				continue
			}

			requestedPrice := event.OptionAssignmentEvent.AssignedPrice
			requestedQuantity := event.OptionAssignmentEvent.AssignedQuantity
			exercisedOptionOrderRequests, err := o.CreateCloseOrderRequests(p.positionCache, p.GetCurrentTime(), requestedPrice, &requestedQuantity, "auto-closed-on-early-assignment")
			if err != nil {
				return nil, fmt.Errorf("failed to create close order request: %w", err)
			}

			for _, orderRequest := range exercisedOptionOrderRequests {
				placeOrderResults, placeOrderErr := dbService.PlaceOrders(p.ID, []*CreateOrderRequest{orderRequest})
				if placeOrderErr != nil {
					return nil, fmt.Errorf("failed to place close order: %w", placeOrderErr)
				}

				placeOrderResult := placeOrderResults[0]

				executionRequests[placeOrderResult] = ExecutionFillRequest{
					Price:    placeOrderResult.RequestedPrice,
					Time:     p.GetCurrentTime(),
					Quantity: placeOrderResult.GetQuantity(),
				}
			}

			closedOrderIds[o.ID] = true
		}

		if event.Type == TickDeltaEventTypeOptionExpired {
			openOrders := p.GetOpenOrders(event.OptionExpirationEvent.Symbol)
			for _, o := range openOrders {
				if closedOrderIds[o.ID] {
					log.Infof("skipping expiration close for order %d (%s): already closed by assignment", o.ID, o.Symbol)
					continue
				}

				remainingQty, err := o.GetRemainingOpenQuantity()
				if err != nil {
					return nil, fmt.Errorf("failed to get remaining open quantity for order %d: %w", o.ID, err)
				}

				if math.Abs(remainingQty) <= 0 {
					log.Infof("skipping expired option order %d (%s): already fully closed (remaining qty=%.4f)", o.ID, o.Symbol, remainingQty)
					continue
				}

				components, err := event.OptionExpirationEvent.Symbol.Components()
				if err != nil {
					return nil, fmt.Errorf("failed to get symbol components: %w", err)
				}

				var closePriceAtExpiration float64
				switch components.OptionType {
				case models.OptionTypeCall:
					if event.OptionExpirationEvent.UnderlyingPriceAtExpiry > components.StrikePrice {
						closePriceAtExpiration = event.OptionExpirationEvent.UnderlyingPriceAtExpiry - components.StrikePrice
					} else {
						closePriceAtExpiration = 0
					}

				case models.OptionTypePut:
					if event.OptionExpirationEvent.UnderlyingPriceAtExpiry < components.StrikePrice {
						closePriceAtExpiration = components.StrikePrice - event.OptionExpirationEvent.UnderlyingPriceAtExpiry
					} else {
						closePriceAtExpiration = 0
					}

				default:
					return nil, fmt.Errorf("unknown option type: %s", components.OptionType)
				}

				exercisedOptionOrderRequests, err := o.CreateCloseOrderRequests(p.positionCache, p.GetCurrentTime(), closePriceAtExpiration, nil, "auto-closed-on-expiration")
				if err != nil {
					return nil, fmt.Errorf("failed to create close order request: %w", err)
				}

				for _, orderRequest := range exercisedOptionOrderRequests {
					placeOrderResults, placeOrderErr := dbService.PlaceOrders(p.ID, []*CreateOrderRequest{orderRequest})
					if placeOrderErr != nil {
						return nil, fmt.Errorf("failed to place close order: %w", placeOrderErr)
					}

					placeOrderResult := placeOrderResults[0]

					executionRequests[placeOrderResult] = ExecutionFillRequest{
						Price:    placeOrderResult.RequestedPrice,
						Time:     p.GetCurrentTime(),
						Quantity: placeOrderResult.GetQuantity(),
					}

					if orderRequest.Class == OrderRecordClassEquity {
						multiplier := 1.0
						if orderRequest.Side == TradierOrderSideSell || orderRequest.Side == TradierOrderSideSellShort {
							multiplier = -1.0
						}

						optionAssignmentEvents = append(optionAssignmentEvents, &TickDeltaEvent{
							Type: TickDeltaEventTypeOptionAssigned,
							OptionAssignmentEvent: &OptionAssignmentEvent{
								OrderId:          placeOrderResult.ID,
								Symbol:           placeOrderResult.GetInstrument(),
								AssignedQuantity: orderRequest.Quantity * multiplier,
								AssignedPrice:    orderRequest.RequestedPrice,
								Timestamp:        p.GetCurrentTime(),
							},
						})

						p.Events = append(p.Events, optionAssignmentEvents[len(optionAssignmentEvents)-1])
					}
				}

				closedOrderIds[o.ID] = true
			}

			p.DeleteRepository(event.OptionExpirationEvent.Symbol)
		}

		// todo: in order to handle multiple events in a single tick, we need to commit after each event
	}

	// Append all option assignment events to the playground events
	tickDelta.Events = append(tickDelta.Events, optionAssignmentEvents...)

	newTrades, invalidOrders, _, err := p.CommitOrderQueue(executionRequests)
	if err != nil {
		return nil, fmt.Errorf("failed to commit order queue: %w", err)
	}

	tickDelta.NewTrades = append(tickDelta.NewTrades, newTrades...)
	tickDelta.InvalidOrders = append(tickDelta.InvalidOrders, invalidOrders...)

	if p.GetMeta().Mode.IsRealtime() && tickDelta.EquityPlot != nil {
		if err := dbService.SaveEquityPlotRecord(p.ID, tickDelta.EquityPlot.Timestamp, tickDelta.EquityPlot.Value); err != nil {
			return nil, fmt.Errorf("failed to save equity plot record: %v", err)
		}
	}

	return tickDelta, nil
}

func (p *Playground) GetMeta() Meta {
	return p.Meta
}

// AfterFind hydrates the in-memory Mode from the persisted legacy
// environment / live_account_type columns on every GORM load, so playground
// rows written before the Mode migration (including internal reconcile
// containers) load without mutation of stored data. An unrecognized legacy
// combination fails the load explicitly rather than defaulting.
func (p *Playground) AfterFind(tx *gorm.DB) error {
	if err := p.Meta.HydrateMode(); err != nil {
		return fmt.Errorf("Playground.AfterFind: playground %s: %w", p.ID, err)
	}

	return nil
}

func (p *Playground) GetBalance() float64 {
	balance := 0.0
	for _, order := range p.getAllOrders() {
		if order.Status == OrderRecordStatusFilled && order.IsClose {
			pl := order.CalcRealizedPL()
			balance += pl
		}
	}

	return p.InitialBalance + balance
}

func (p *Playground) GetEquity(positionCache *PositionsCache) float64 {
	equity := p.GetBalance()

	for _, position := range positionCache.Iter() {
		equity += position.PL
	}

	return equity
}

func (p *Playground) GetPendingOrders() []*OrderRecord {
	return p.account.PendingOrders
}

func (p *Playground) getAllOrders() []*OrderRecord {
	result := append(p.account.Orders, p.account.PendingOrders...)
	result = append(result, p.account.NewOrders...)
	if len(result) == 0 {
		return make([]*OrderRecord, 0)
	}
	return result
}

func (p *Playground) GetAllOrders() []*OrderRecord {
	p.account.mutex.Lock()
	defer p.account.mutex.Unlock()

	return p.getAllOrders()
}

func (p *Playground) GetPosition(symbol models.Instrument, checkExists bool) (Position, error) {
	positionCache, err := p.UpdatePricesAndGetPositionCache()
	if err != nil {
		return Position{}, fmt.Errorf("error getting positions: %w", err)
	}

	if checkExists {
		if exists := positionCache.Exists(symbol); !exists {
			return Position{}, fmt.Errorf("position not found for symbol %s", symbol)
		}
	}

	position := positionCache.Get(symbol.GetTicker())

	return *position, nil
}

func (p *Playground) getNetTrades(trades []*TradeRecord) []*TradeRecord {
	netTrades := []*TradeRecord{}
	direction := 0
	totalQuantity := 0.0

	for _, trade := range trades {
		if direction > 0 {
			if totalQuantity+trade.Quantity < 0 {
				netTrades = []*TradeRecord{
					netTrades[len(netTrades)-1],
					trade,
				}

				direction = -1
				totalQuantity = trade.Quantity

				continue
			} else if totalQuantity+trade.Quantity == 0 {
				direction = 0
				netTrades = []*TradeRecord{}

				continue
			}

			totalQuantity += trade.Quantity
		} else if direction < 0 {
			if totalQuantity+trade.Quantity > 0 {
				netTrades = []*TradeRecord{
					netTrades[len(netTrades)-1],
					trade,
				}

				direction = 1
				totalQuantity = trade.Quantity

				continue
			} else if totalQuantity+trade.Quantity == 0 {
				direction = 0
				netTrades = []*TradeRecord{}

				continue
			}

			totalQuantity += trade.Quantity
		} else {
			if trade.Quantity > 0 {
				direction = 1
			} else if trade.Quantity < 0 {
				direction = -1
			}

			totalQuantity = trade.Quantity
		}

		netTrades = append(netTrades, trade)
	}

	return netTrades
}

func (p *Playground) UpdatePricesAndGetPositionCache() (*PositionsCache, error) {
	if p.positionCache != nil {
		// update pl
		instruments, _ := p.positionCache.List()
		currentPrices, err := p.getCurrentPrices(instruments)
		if err != nil {
			return nil, fmt.Errorf("getCurrentPrice: %w", err)
		}

		for symbol, position := range p.positionCache.Iter() {
			currentPrice, found := currentPrices[symbol]
			if !found {
				return nil, fmt.Errorf("current price not found for symbol %s", symbol)
			}

			pl := (currentPrice.Value - position.CostBasis) * position.Quantity
			if isOptionSymbol(symbol) {
				pl *= 100.0
			}

			p.positionCache.Update(symbol, pl, currentPrice.Value, currentPrice.Timestamp)
		}

		return p.positionCache, nil
	}

	return nil, fmt.Errorf("position cache is nil")
}

func (p *Playground) UpdatePositionCachePositions() (*PositionsCache, error) {
	positions := make(map[string]*Position)
	instruments := make([]models.Instrument, 0)
	uniqueInstruments := make(map[string]models.Instrument)
	positionToInstrumentMap := make(map[*Position]models.Instrument)

	allTrades := make(map[string][]*TradeRecord)
	for _, order := range p.account.Orders {
		instrument := order.GetInstrument()
		ticker := instrument.GetTicker()

		if _, exists := uniqueInstruments[ticker]; !exists {
			instruments = append(instruments, instrument)
			uniqueInstruments[ticker] = instrument
		}

		_, ok := positions[ticker]
		if !ok {
			positions[ticker] = &Position{}
		}

		positionToInstrumentMap[positions[ticker]] = instrument

		orderStatus := order.GetStatus()
		if orderStatus.IsFilled() {
			filledQty := order.GetFilledVolume()
			positions[ticker].Quantity += filledQty
			allTrades[ticker] = append(allTrades[ticker], order.Trades...)
		}
	}

	// remove positions with zero quantity
	var markForDeletion []int
	for k, instrument := range instruments {
		symbol := instrument.GetTicker()
		position, found := positions[symbol]
		if !found {
			return nil, fmt.Errorf("position not found for symbol %s", symbol)
		}

		if position.Quantity == 0 {
			delete(positions, symbol)
			markForDeletion = append(markForDeletion, k)
		}
	}

	// delete instruments with zero quantity positions
	for i := len(markForDeletion) - 1; i >= 0; i-- {
		idx := markForDeletion[i]
		instruments = append(instruments[:idx], instruments[idx+1:]...)
	}

	vwapMap := make(map[string]float64)
	totalQuantityMap := make(map[string]float64)
	totalOpenQuantityMap := make(map[string]float64)
	for _, order := range p.account.Orders {
		ticker := order.GetInstrument().GetTicker()
		_, ok := vwapMap[ticker]
		if !ok {
			vwapMap[ticker] = 0.0
			totalQuantityMap[ticker] = 0.0
			totalOpenQuantityMap[ticker] = 0.0
		}

		orderStatus := order.GetStatus()
		if orderStatus == OrderRecordStatusFilled || orderStatus == OrderRecordStatusPartiallyFilled {
			totalQuantityMap[ticker] += order.GetFilledVolume()

			if totalQuantityMap[ticker] != 0 {
				if order.Side == TradierOrderSideBuy || order.Side == TradierOrderSideSellShort || order.Side == TradierOrderSideBuyToOpen || order.Side == TradierOrderSideSellToOpen {
					filledVolume := order.GetFilledVolume()
					vwapMap[ticker] += order.GetAvgFillPrice() * filledVolume
					totalOpenQuantityMap[ticker] += filledVolume
				}
			} else {
				vwapMap[ticker] = 0
				totalOpenQuantityMap[ticker] = 0
			}
		}
	}

	// calculate positions
	currentPrices, err := p.getCurrentPrices(instruments)
	if err != nil {
		return nil, fmt.Errorf("getCurrentPrice: %w", err)
	}

	for symbol, vwap := range vwapMap {
		var costBasis float64
		totalQuantity := totalOpenQuantityMap[symbol]

		// calculate cost basis
		if totalQuantity != 0 {
			costBasis = vwap / totalQuantity
		}

		if _, ok := positions[symbol]; !ok {
			continue
		}

		positions[symbol].CostBasis = costBasis

		// calculate maintenance margin
		positions[symbol].MaintenanceMargin = calculateMaintenanceRequirement(totalQuantity, costBasis)

		// calculate pl
		tick, found := currentPrices[symbol]
		if found {
			instrument, ok := uniqueInstruments[symbol]
			if !ok {
				return nil, fmt.Errorf("instrument not found for symbol %s", symbol)
			}

			switch instrument.(type) {
			case *models.OptionContractV3, models.OptionSymbol:
				positions[symbol].PL = (tick.Value - costBasis) * positions[symbol].Quantity * 100
			case models.StockSymbol:
				positions[symbol].PL = (tick.Value - costBasis) * positions[symbol].Quantity
			default:
				return nil, fmt.Errorf("unsupported instrument type for symbol %s", symbol)
			}

			positions[symbol].CurrentPrice = tick.Value
			positions[symbol].Timestamp = p.GetCurrentTime().Format(time.RFC3339)
		} else {
			log.Warnf("getCurrentPrice [%s]: not found", symbol)
			// positions[symbol].PL = 0
		}
	}

	// set the cache
	p.positionCache.SetCache(positions, positionToInstrumentMap)

	return p.positionCache, nil
}

func (p *Playground) GetCandle(symbol models.Instrument, period time.Duration) (*models.PolygonAggregateBarV2, error) {
	repo, ok := p.repos.Get(symbol, period)
	if !ok {
		return nil, fmt.Errorf("GetTick: symbol %s not found in repos", symbol)
	}

	candle, err := repo.GetCurrentCandle()
	if err != nil {
		return nil, fmt.Errorf("GetTick: no more candles for %s", symbol)
	}

	if candle != nil {
		return candle.ToPolygonAggregateBarV2(), nil
	}

	return nil, nil
}

func (p *Playground) isSideAllowed(symbol models.Instrument, side TradierOrderSide, positionQuantity float64, includePendingOrders bool) error {
	if includePendingOrders {
		for _, o := range p.account.PendingOrders {
			if o.GetInstrument() == symbol {
				positionQuantity += o.GetQuantity()
			}
		}
	}

	if err := side.Validate(GetClass(symbol)); err != nil {
		return fmt.Errorf("invalid side: %w", err)
	}

	if positionQuantity > 0 {
		if side == TradierOrderSideBuyToCover {
			return fmt.Errorf("cannot buy to cover when long position of %.2f exists: must sell to close", positionQuantity)
		}

		if side == TradierOrderSideSellShort {
			return fmt.Errorf("cannot sell short when long position of %.2f exists: must sell to close", positionQuantity)
		}

		if side == TradierOrderSideBuyToClose {
			return fmt.Errorf("cannot buy to close when long position of %.2f exists: must sell to close", positionQuantity)
		}

		if side == TradierOrderSideSellToOpen {
			return fmt.Errorf("cannot sell to open when long position of %.2f exists: must sell to close", positionQuantity)
		}
	} else if positionQuantity < 0 {
		if side == TradierOrderSideBuy {
			return fmt.Errorf("cannot buy when short position of %.2f exists: must buy to cover", positionQuantity)
		}

		if side == TradierOrderSideSell {
			return fmt.Errorf("cannot sell when short position of %.2f exists: must buy to cover", positionQuantity)
		}

		if side == TradierOrderSideSellToClose {
			return fmt.Errorf("cannot sell to close when short position of %.2f exists: must buy to close", positionQuantity)
		}

		if side == TradierOrderSideBuyToOpen {
			return fmt.Errorf("cannot buy to open when short position of %.2f exists: must buy to close", positionQuantity)
		}
	} else {
		if side == TradierOrderSideSell {
			return fmt.Errorf("cannot sell when no position exists: must sell short")
		}

		if side == TradierOrderSideBuyToCover {
			return fmt.Errorf("cannot buy to cover when no position exists")
		}

		if side == TradierOrderSideSellToClose {
			return fmt.Errorf("cannot sell to close when no position exists")
		}

		if side == TradierOrderSideBuyToClose {
			return fmt.Errorf("cannot buy to close when no position exists")
		}
	}

	return nil
}

// max returns the maximum of two float64 values
func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func (p *Playground) getMaintenanceMargin(positionCache *PositionsCache) float64 {
	maintenanceMargin := 0.0
	for _, position := range positionCache.Iter() {
		maintenanceMargin += calculateMaintenanceRequirement(position.Quantity, position.CostBasis)
	}

	return maintenanceMargin
}

// todo: fix - free margin should be calculated on each open order, not the total position
func (p *Playground) GetFreeMarginFromPositionMap(positionCache *PositionsCache) float64 {
	pl := 0.0
	for _, position := range positionCache.Iter() {
		pl += position.PL
	}

	freeMargin := (p.account.Balance + pl)

	for _, position := range positionCache.Iter() {
		freeMargin -= calculateMaintenanceRequirement(position.Quantity, position.CostBasis)
	}

	return freeMargin
}

func (p *Playground) GetFreeMargin() (float64, error) {
	positions, err := p.UpdatePricesAndGetPositionCache()
	if err != nil {
		return 0, fmt.Errorf("error getting positions: %w", err)
	}

	return p.GetFreeMarginFromPositionMap(positions), nil
}

func (p *Playground) SetNewCandlesQueue(queue *models.FIFOQueue[*BacktesterCandle]) {
	p.newCandlesQueue = queue
}

func (p *Playground) GetNewCandlesQueue() *models.FIFOQueue[*BacktesterCandle] {
	return p.newCandlesQueue
}

func (p *Playground) SetNewTradesQueue(queue *models.FIFOQueue[*TradeRecord]) {
	p.newTradesQueue = queue
}

func (p *Playground) GetNewTradesQueue() *models.FIFOQueue[*TradeRecord] {
	return p.newTradesQueue
}

func (p *Playground) SetSignalRepo(repo ISignalRepository) {
	p.signalRepo = repo
}

func (p *Playground) GetSignalRepo() ISignalRepository {
	return p.signalRepo
}

func (p *Playground) SetInvalidOrdersQueue(queue *models.FIFOQueue[*OrderRecord]) {
	p.invalidOrdersQueue = queue
}

func (p *Playground) GetInvalidOrdersQueue() *models.FIFOQueue[*OrderRecord] {
	return p.invalidOrdersQueue
}

func (p *Playground) getCloseByRequests(order *OrderRecord, position *Position, setCloseInfo bool) ([]*CloseByRequest, error) {
	var closeByRequests []*CloseByRequest

	// reconciliation playgrounds do not have close orders
	if order.AccountRole != AccountRoleReconcilation {
		if setCloseInfo {
			// mutates the order to add closes info
			if err := p.setCloseInfoToOrder(order, position); err != nil {
				return nil, fmt.Errorf("placeOrder: error adding closes info to order: %w", err)
			}
		}

		if order.IsClose {
			volumeToClose := math.Abs(order.GetQuantity())

			if order.CloseOrderId == nil {
				openOrders := p.GetOpenOrders(order.GetInstrument())

				// calculate the volume to close
				for _, o := range openOrders {
					if volumeToClose <= 0 {
						break
					}

					// Only match open orders whose side is compatible with this close order.
					// e.g. buy_to_close should only close sell_to_open, not buy_to_open.
					if !order.Side.ClosesOpenSide(o.Side) {
						continue
					}

					qty, err := o.GetRemainingOpenQuantity()
					if err != nil {
						return nil, fmt.Errorf("placeOrder: error getting remaining open quantity: %w", err)
					}

					remainingOpenQuantity := math.Abs(qty)
					if remainingOpenQuantity <= 0 {
						continue
					}

					quantity := math.Min(volumeToClose, remainingOpenQuantity)
					volumeToClose -= quantity

					sign := 1.0
					if o.Side.IsLongOpen() {
						sign = -1.0
					}

					closeByRequests = append(closeByRequests, &CloseByRequest{
						Order:    o,
						Quantity: quantity * sign,
					})
				}
			} else {
				o := p.GetOpenOrder(*order.CloseOrderId)
				if o == nil {
					return nil, fmt.Errorf("placeOrder: open order %d not found in open orders", *order.CloseOrderId)
				}

				qty, err := o.GetRemainingOpenQuantity()
				if err != nil {
					return nil, fmt.Errorf("placeOrder: error getting remaining open quantity: %w", err)
				}

				remainingOpenQuantity := math.Abs(qty)
				if remainingOpenQuantity <= 0 {
					return nil, fmt.Errorf("placeOrder: open order %d has no remaining open quantity", *order.CloseOrderId)
				}

				quantity := math.Min(volumeToClose, remainingOpenQuantity)
				volumeToClose -= quantity

				sign := 1.0
				if o.Side.IsLongOpen() {
					sign = -1.0
				}

				closeByRequests = append(closeByRequests, &CloseByRequest{
					Order:    o,
					Quantity: quantity * sign,
				})
			}

			// check if the volume to close is valid
			if volumeToClose < 0 {
				return nil, fmt.Errorf("placeOrder: volume to close cannot be negative")
			}

			if volumeToClose > 0 {
				return nil, fmt.Errorf("placeOrder: volume to close exceeds open volume")
			}
		}
	}

	return closeByRequests, nil
}

func (p *Playground) placeOrder(order *OrderRecord) ([]*PlaceOrderChanges, error) {
	p.account.mutex.Lock()
	defer p.account.mutex.Unlock()

	if order.Class != OrderRecordClassEquity && order.Class != OrderRecordClassOption {
		return nil, fmt.Errorf("only equity and option orders are supported")
	}

	if !p.Meta.IsReconciliation() {
		if ok := p.repos.HasInstrument(order.GetInstrument()); !ok && order.Class == OrderRecordClassEquity {
			return nil, fmt.Errorf("symbol %s not found in repos", order.GetInstrument())
		}
	}

	positionCache, err := p.UpdatePricesAndGetPositionCache()
	if err != nil {
		return nil, fmt.Errorf("error getting positions: %w", err)
	}

	position := positionCache.Get(order.GetInstrument().GetTicker())

	if !p.Meta.IsReconciliation() {
		if err := p.isSideAllowed(order.GetInstrument(), order.Side, position.Quantity, true); err != nil {
			return nil, fmt.Errorf("PlaceOrder: side not allowed: %w", err)
		}
	}

	if order.IsAdjustment {
		if position.CostBasis == 0 {
			if position.CurrentPrice > 0 {
				order.RequestedPrice = position.CurrentPrice
			} else {
				prc, err := p.FetchCurrentPrice(context.Background(), order.GetInstrument())
				if err != nil {
					return nil, fmt.Errorf("error current fetching price: %w", err)
				}

				order.RequestedPrice = prc
			}
		} else {
			order.RequestedPrice = math.Abs(position.CostBasis)
		}
	} else {
		if order.RequestedPrice < 0 {
			return nil, fmt.Errorf("requested price must be greater than 0")
		}
	}

	if order.Price != nil && *order.Price <= 0 {
		return nil, fmt.Errorf("price must be greater than 0")
	}

	if order.AbsoluteQuantity <= 0 {
		return nil, fmt.Errorf("quantity must be greater than 0")
	}

	if err := utils.ValidateTag(order.Tag); err != nil {
		return nil, fmt.Errorf("invalid tag: %w", err)
	}

	setCloseInfo := false
	if _, err := p.getCloseByRequests(order, position, setCloseInfo); err != nil {
		return nil, fmt.Errorf("error getting close by requests: %w", err)
	}

	// order.ID can be zero if the order is a pending live order
	if order.ID > 0 {
		for _, o := range p.account.Orders {
			if o.ID == order.ID {
				return nil, fmt.Errorf("order with id %d already exists in orders", order.ID)
			}
		}
	}

	// order.ID can be zero if the order is a pending live order
	if order.ID > 0 {
		for _, o := range p.account.PendingOrders {
			if o.ID == order.ID {
				return nil, fmt.Errorf("order with id %d already exists in pending orders", order.ID)
			}
		}
	}

	return []*PlaceOrderChanges{
		{
			Commit: func() error {
				p.account.mutex.Lock()
				defer p.account.mutex.Unlock()

				p.AddToPendingOrdersQueue(order)

				return nil
			},
			Info: fmt.Sprintf("Add PendingOrders field to playground %s", p.ID),
		},
	}, nil
}

func (p *Playground) PlaceOrder(order *OrderRecord) ([]*PlaceOrderChanges, error) {
	// Kill-switch chokepoint: this is the single Broker seam through which every
	// order in every Mode is placed (broker_seam.go). Consulting the gate here,
	// immediately before an order can reach any Broker, guarantees a halt cannot
	// be bypassed by any Mode or code path. A nil gate (the default, and the
	// state in all simulation/model-diff paths) permits the order.
	if err := CheckOrderGate(); err != nil {
		return nil, err
	}

	broker, err := brokerFor(&p.Meta)
	if err != nil {
		return nil, fmt.Errorf("place order is not supported in %s environment", p.Meta.LegacyEnv)
	}

	return broker.PlaceOrder(p, order)
}

func (p *Playground) GetLiveAccount() ILiveAccount {
	return p.LiveAccount
}

func (p *Playground) GetCalendarRepository() models.CalendarRepository {
	return p.clock.GetCalendarRepository()
}

func (p *Playground) SetLiveAccount(account ILiveAccount) {
	p.LiveAccount = account

	id := account.GetId()
	p.LiveAccountID = &id
}

func PopulatePlayground(playground *Playground, req *PopulatePlaygroundRequest, clock *Clock, now time.Time, newTradesQueue *models.FIFOQueue[*TradeRecord], invalidOrdersQueue *models.FIFOQueue[*OrderRecord], calendar *models.MarketCalendar, feeds ...(*CandleRepository)) error {
	source := req.Account.Source
	clientID := req.ClientID
	balance := req.Account.Balance
	initialBalance := req.InitialBalance
	orders := req.BackfillOrders
	tags := req.Tags
	mode := req.Mode
	isReconciliation := req.Reconciliation

	repos := make(map[models.Instrument]map[time.Duration]*CandleRepository)
	var symbols []string
	var minimumPeriod time.Duration
	var startAt time.Time
	var endAt *time.Time
	var accountRole AccountRole
	var repositories []CandleRepositoryDTO

	var meta Meta
	if isReconciliation {
		meta = *NewReconciliationMeta(tags)
	} else {
		if err := mode.Validate(); err != nil {
			return fmt.Errorf("PopulatePlayground: error validating mode: %w", err)
		}

		meta = *NewMeta(mode, tags)
	}

	brokerName := "tradier"
	var accountID *string

	if isReconciliation || mode.IsRealtime() {
		if source == nil {
			return fmt.Errorf("source is required")
		}

		if playground.LiveAccount == nil {
			if req.LiveAccount == nil {
				return fmt.Errorf("live account is required")
			}

			playground.SetLiveAccount(req.LiveAccount)
		}

		if mode.IsRealtime() {
			playground.SetReconcilePlayground(req.ReconcilePlayground)

			if newTradesQueue == nil {
				return fmt.Errorf("newTradesQueue is required")
			}

			playground.SetNewTradesQueue(newTradesQueue)
			playground.SetInvalidOrdersQueue(invalidOrdersQueue)
		}

		accountID = &source.AccountID

		accountRole = source.AccountRole

		meta.SourceBroker = brokerName
		meta.SourceAccountId = source.AccountID
	} else {
		accountRole = AccountRoleSimulator
	}

	meta.Role = accountRole

	if isReconciliation {
		startAt = now
	} else {
		// set the clock
		if clock != nil {
			startAt = clock.CurrentTime
			endAt = &clock.EndTime
		} else {
			startAt = now
		}

		// set the feeds
		for _, feed := range feeds {
			feed.Sort()

			if err := feed.SetStartingPosition(startAt, mode, calendar); err != nil {
				return fmt.Errorf("error setting starting position for feed %v: %w", feed, err)
			}

			symbol := feed.GetSymbol()

			if optionSymbol, ok := symbol.(models.OptionSymbol); ok {
				components, err := optionSymbol.Components()
				if err != nil {
					return fmt.Errorf("error getting option components: %w", err)
				}

				symbol = &models.OptionContractV3{
					Symbol:           optionSymbol,
					UnderlyingSymbol: models.StockSymbol(components.Underlying),
					Expiration:       components.Expiration,
					ExpirationDate:   models.ExpirationDate(components.Expiration.Format("%Y-%m-%d")),
					Strike:           components.StrikePrice,
					OptionType:       components.OptionType,
				}
			}

			// todo: remove antipattern of using map for repo. use a list instead
			if _, found := repos[symbol]; !found {
				symbols = append(symbols, symbol.GetTicker())
				repo := make(map[time.Duration]*CandleRepository)
				repos[symbol] = repo
			}

			repos[symbol][feed.GetPeriod()] = feed
			repositories = append(repositories, feed.ToDTO())

			if minimumPeriod == 0 || feed.GetPeriod() < minimumPeriod {
				minimumPeriod = feed.GetPeriod()
			}
		}
	}

	meta.Symbols = symbols
	meta.InitialBalance = initialBalance
	meta.StartAt = startAt
	meta.EndAt = endAt
	meta.ClientID = clientID

	var id uuid.UUID
	if req.ID != nil {
		id = *req.ID
	} else {
		id = uuid.New()
	}

	meta.PlaygroundId = id.String()
	if req.ReconcilePlayground != nil {
		_id := req.ReconcilePlayground.GetPlayground().GetId().String()
		meta.ReconcilePlaygroundId = &_id
	}

	playground.Meta = meta
	playground.ID = id
	playground.Balance = balance
	playground.ClientID = clientID
	playground.account = NewBacktesterAccount(balance, orders)
	playground.clock = clock
	playground.repos = NewCandleMasterRepository(repos)
	playground.Repositories = CandleRepositoryRecord(repositories)
	playground.openOrdersCache = NewOpenOrdersCache()
	playground.positionCache = NewPositionCache()
	playground.minimumPeriod = minimumPeriod
	playground.AccountID = accountID
	playground.BrokerName = &brokerName
	playground.placeOrderMutex = &sync.Mutex{}
	playground.newOrdersQueueMutex = &sync.Mutex{}
	playground.pendingOrdersQueueMutex = &sync.Mutex{}
	playground.OptionsBroker = req.OptionsBroker
	playground.exerciseOptionsRequestQueue = NewExerciseOptionRequestQueue()

	if _, err := playground.UpdatePositionCachePositions(); err != nil {
		return fmt.Errorf("error getting positions: %w", err)
	}

	return nil
}

// PlaygroundConfig configures NewPlayground. Zero values apply defaults:
// InitialBalance defaults to Balance and Mode defaults to Simulation.
type PlaygroundConfig struct {
	ID             *uuid.UUID
	Source         *CreateAccountRequestSource
	ClientID       *string
	Balance        float64
	InitialBalance float64
	Clock          *Clock
	BackfillOrders []*OrderRecord
	Mode           Mode
	Now            time.Time
	Tags           []string
	OptionsBroker  IOptionsBroker
	Feeds          []*CandleRepository
}

// todo: change repository on playground to BacktesterCandleRepository
func NewPlayground(cfg PlaygroundConfig) (*Playground, error) {
	if cfg.InitialBalance == 0 {
		cfg.InitialBalance = cfg.Balance
	}

	if cfg.Mode == "" {
		cfg.Mode = ModeSimulation
	}

	playground := new(Playground)

	if cfg.ID != nil {
		playground.ID = *cfg.ID
	}

	req := &PopulatePlaygroundRequest{
		Account: CreateAccountRequest{
			Source:  cfg.Source,
			Balance: cfg.Balance,
		},
		Mode:           cfg.Mode,
		ClientID:       cfg.ClientID,
		InitialBalance: cfg.InitialBalance,
		BackfillOrders: cfg.BackfillOrders,
		OptionsBroker:  cfg.OptionsBroker,
		Tags:           cfg.Tags,
	}

	if err := PopulatePlayground(playground, req, cfg.Clock, cfg.Now, nil, nil, nil, cfg.Feeds...); err != nil {
		return nil, fmt.Errorf("error populating playground: %w", err)
	}

	return playground, nil
}

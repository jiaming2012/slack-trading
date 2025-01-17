package models

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

type IPlayground interface {
	GetMeta() *PlaygroundMeta
	GetId() uuid.UUID
	GetBalance() float64
	GetEquity(positions map[eventmodels.Instrument]*Position) float64
	GetOrders() []*BacktesterOrder
	GetPosition(symbol eventmodels.Instrument) Position
	GetPositions() map[eventmodels.Instrument]*Position
	GetRepositories() []*CandleRepository
	GetCandle(symbol eventmodels.Instrument, period time.Duration) (*eventmodels.PolygonAggregateBarV2, error)
	GetFreeMargin() float64
	PlaceOrder(order *BacktesterOrder) (*PlaceOrderChanges, error)
	Tick(d time.Duration, isPreview bool) (*TickDelta, error)
	GetFreeMarginFromPositionMap(positions map[eventmodels.Instrument]*Position) float64
	GetOpenOrders(symbol eventmodels.Instrument) []*BacktesterOrder
	GetCurrentTime() time.Time
	NextOrderID() uint
	FetchCandles(symbol eventmodels.Instrument, period time.Duration, from time.Time, to time.Time) ([]*eventmodels.AggregateBarWithIndicators, error)
	// CommitPendingOrderToOrderQueue(order *BacktesterOrder, startingPositions map[eventmodels.Instrument]*Position, orderFillEntry OrderFillEntry, performChecks bool) error
	FillOrder(order *BacktesterOrder, performChecks bool, orderFillEntry OrderExecutionRequest, positionsMap map[eventmodels.Instrument]*Position) (*BacktesterTrade, error)
	CommitPendingOrders(startingPositions map[eventmodels.Instrument]*Position, orderFillPricesMap map[uint]OrderExecutionRequest, performChecks bool) (newTrades []*BacktesterTrade, invalidOrders []*BacktesterOrder, err error)
	RejectOrder(order *BacktesterOrder, reason string) error
}

type Playground struct {
	Meta               *PlaygroundMeta
	ID                 uuid.UUID
	account            *BacktesterAccount
	clock              *Clock
	repos              map[eventmodels.Instrument]map[time.Duration]*CandleRepository
	isBacktestComplete bool
	positionsCache     map[eventmodels.Instrument]*Position
	openOrdersCache    map[eventmodels.Instrument][]*BacktesterOrder
	minimumPeriod      time.Duration
	Env                PlaygroundEnvironment
}

func (p *Playground) AddToOrderQueue(order *BacktesterOrder) error {
	index := -1

	// find order in pending orders
	for i, o := range p.account.PendingOrders {
		if o.ID == order.ID {
			index = i
			break
		}
	}

	if index == -1 {
		return fmt.Errorf("order %d not found in pending orders", order.ID)
	}

	// add order to order queue
	p.account.Orders = append(p.account.Orders, order)

	// remove order from pending orders
	p.account.PendingOrders = append(p.account.PendingOrders[:index], p.account.PendingOrders[index+1:]...)

	return nil
}

func (p *Playground) GetId() uuid.UUID {
	return p.ID
}

func (p *Playground) GetRepositories() []*CandleRepository {
	repos := make([]*CandleRepository, 0)
	for _, periodRepoMap := range p.repos {
		for _, repo := range periodRepoMap {
			repos = append(repos, repo)
		}
	}

	return repos
}

func (p *Playground) GetOpenOrders(symbol eventmodels.Instrument) []*BacktesterOrder {
	openOrders, found := p.openOrdersCache[symbol]
	if !found {
		return make([]*BacktesterOrder, 0)
	}

	return openOrders
}

func (p *Playground) CommitPendingOrderToOrderQueue(order *BacktesterOrder, startingPositions map[eventmodels.Instrument]*Position, orderFillEntry OrderExecutionRequest, performChecks bool) error {
	if order.Status != BacktesterOrderStatusPending {
		return fmt.Errorf("commitPendingOrders: order %d status is %s, not pending", order.ID, order.Status)
	}

	// check if the order is valid
	orderQuantity := order.GetQuantity()
	if performChecks {
		if orderQuantity == 0 {
			order.Status = BacktesterOrderStatusRejected
			rejectReason := ErrInvalidOrderVolumeZero.Error()
			order.RejectReason = &rejectReason

			if err := p.AddToOrderQueue(order); err != nil {
				log.Fatalf("position 1: error adding order to order queue: %v", err)
			}

			return fmt.Errorf("commitPendingOrders: order %d has zero quantity", order.ID)
		}
	}

	var freeMargin, initialMargin float64
	isCloseOrder := false
	performMarginCheck := performChecks

	if performChecks {
		// perform margin check
		freeMargin = p.GetFreeMarginFromPositionMap(startingPositions)
		initialMargin = calculateInitialMarginRequirement(orderQuantity, orderFillEntry.Price)
		position := startingPositions[order.Symbol]

		if position != nil {
			if position.Quantity <= 0 && orderQuantity > 0 {
				if orderQuantity > math.Abs(position.Quantity) {
					if performChecks {
						order.Status = BacktesterOrderStatusRejected
						rejectReason := ErrInvalidOrderVolumeLongVolume.Error()
						order.RejectReason = &rejectReason

						// append the order to the queue
						if err := p.AddToOrderQueue(order); err != nil {
							log.Fatalf("position 2: error adding order to order queue: %v", err)
						}

						return fmt.Errorf("commitPendingOrders: order %d volume exceeds long volume", order.ID)
					}
				} else {
					performMarginCheck = false
					isCloseOrder = true
				}
			} else if position.Quantity >= 0 && orderQuantity < 0 {
				if math.Abs(orderQuantity) > position.Quantity {
					if performChecks {
						order.Status = BacktesterOrderStatusRejected
						rejectReason := ErrInvalidOrderVolumeShortVolume.Error()
						order.RejectReason = &rejectReason

						// append the order to the queue
						if err := p.AddToOrderQueue(order); err != nil {
							log.Fatalf("position 3: error adding order to order queue: %v", err)
						}

						return fmt.Errorf("commitPendingOrders: order %d volume exceeds short volume", order.ID)
					}
				} else {
					performMarginCheck = false
					isCloseOrder = true
				}
			}
		}
	}

	// check if the order can be filled
	if performMarginCheck && freeMargin <= initialMargin {
		order.Status = BacktesterOrderStatusRejected
		rejectReason := fmt.Sprintf("%s: free_margin (%.2f) <= initial_margin (%.2f)", ErrInsufficientFreeMargin.Error(), freeMargin, initialMargin)
		order.RejectReason = &rejectReason

		// append the order to the queue
		if err := p.AddToOrderQueue(order); err != nil {
			log.Fatalf("position 4: error adding order to order queue: %v", err)
		}

		return fmt.Errorf("commitPendingOrders: order %d has insufficient margin", order.ID)

	} else if isCloseOrder {
		closeVolume := math.Abs(orderQuantity)
		openOrders := p.GetOpenOrders(order.Symbol)
		pendingCloses := make(map[*BacktesterOrder]float64)

		for _, openOrder := range openOrders {
			if closeVolume <= 0 {
				break
			}

			remainingOpenQuantity := math.Abs(openOrder.GetRemainingOpenQuantity())
			if volume, found := pendingCloses[openOrder]; found {
				remainingOpenQuantity -= volume
			}

			if remainingOpenQuantity <= 0 {
				continue
			}

			volumeToClose := math.Min(closeVolume, remainingOpenQuantity)
			closeVolume -= volumeToClose

			if _, found := pendingCloses[openOrder]; !found {
				pendingCloses[openOrder] = 0
			}
			pendingCloses[openOrder] += volumeToClose

			order.Closes = append(order.Closes, openOrder)
		}
	}

	// append the order to the queue
	if err := p.AddToOrderQueue(order); err != nil {
		log.Fatalf("position 4: error adding order to order queue: %v", err)
	}

	return nil
}

func (p *Playground) CommitPendingOrders(startingPositions map[eventmodels.Instrument]*Position, orderFillEntryMap map[uint]OrderExecutionRequest, performChecks bool) (newTrades []*BacktesterTrade, invalidOrders []*BacktesterOrder, err error) {
	for _, order := range p.account.PendingOrders {
		orderFillEntry, found := orderFillEntryMap[order.ID]
		if !found {
			log.Warnf("error finding order filled entry price for order: %v", order.ID)
			continue
		}

		err := p.CommitPendingOrderToOrderQueue(order, startingPositions, orderFillEntry, performChecks)
		if err != nil {
			log.Errorf("error committing pending order: %v", err)
		}

		newTrade, err := p.FillOrder(order, performChecks, orderFillEntry, startingPositions)
		if err != nil {
			return nil, nil, fmt.Errorf("error filling order: %w", err)
		}

		// remove the open orders that were closed in cache
		p.updateOpenOrdersCache()

		// update the account balance before updating the positions cache
		p.updateBalance(newTrades, startingPositions)

		// update the positions cache
		p.updatePositionsCache(newTrade)
	}

	return
}

func (p *Playground) updateOpenOrdersCache() {
	for symbol, orders := range p.openOrdersCache {
		for i := len(orders) - 1; i >= 0; i-- {
			remaining_open_qty := math.Abs(orders[i].GetRemainingOpenQuantity())
			if remaining_open_qty <= 0 {
				p.deleteFromOpenOrdersCache(symbol, i)
			}
		}
	}
}

func (p *Playground) updatePositionsCache(trade *BacktesterTrade) {
	position, ok := p.positionsCache[trade.Symbol]
	if !ok {
		position = &Position{}
	}

	totalQuantity := position.Quantity + trade.Quantity

	// update the cost basis
	if totalQuantity == 0 {
		delete(p.positionsCache, trade.Symbol)
	} else {
		position.CostBasis = (position.CostBasis*position.Quantity + trade.Price*trade.Quantity) / totalQuantity

		// update the quantity
		position.Quantity = totalQuantity

		// update the maintenance margin
		position.MaintenanceMargin = calculateMaintenanceRequirement(position.Quantity, position.CostBasis)

		p.positionsCache[trade.Symbol] = position
	}
}

func (p *Playground) getCurrentPrice(symbol eventmodels.Instrument, period time.Duration) (float64, error) {
	repo, ok := p.repos[symbol][period]
	if !ok {
		return 0, fmt.Errorf("getCurrentPrice: symbol %s not found in repos", symbol)
	}

	candle := repo.GetCurrentCandle()
	if candle == nil {
		return 0, fmt.Errorf("getCurrentPrice: no more candles")
	}

	return candle.Close, nil
}

func (p *Playground) addToOpenOrdersCache(order *BacktesterOrder) {
	openOrders, found := p.openOrdersCache[order.Symbol]
	if !found {
		openOrders = []*BacktesterOrder{}
	}

	p.openOrdersCache[order.Symbol] = append(openOrders, order)
}

func (p *Playground) deleteFromOpenOrdersCache(symbol eventmodels.Instrument, index int) {
	p.openOrdersCache[symbol] = append(p.openOrdersCache[symbol][:index], p.openOrdersCache[symbol][index+1:]...)
}

func (p *Playground) RejectOrder(order *BacktesterOrder, reason string) error {
	if order.Status != BacktesterOrderStatusPending {
		return fmt.Errorf("order is not pending")
	}

	order.Status = BacktesterOrderStatusRejected
	order.RejectReason = &reason

	return nil
}

func (p *Playground) FillOrder(order *BacktesterOrder, performChecks bool, orderFillEntry OrderExecutionRequest, positionsMap map[eventmodels.Instrument]*Position) (*BacktesterTrade, error) {
	position, ok := positionsMap[order.Symbol]
	if !ok {
		position = &Position{Quantity: 0}
		positionsMap[order.Symbol] = position
	}

	if performChecks {
		if !order.GetStatus().IsTradingAllowed() && order.Type == Market {
			return nil, fmt.Errorf("fillOrder: trading is not allowed for %d", order.ID)
		}

		if order.Type != Market {
			return nil, fmt.Errorf("fillOrder: %d is not market, found %s", order.ID, order.Type)
		}

		if order.Class != BacktesterOrderClassEquity {
			log.Errorf("fillOrders: only equity orders are supported")
			return nil, fmt.Errorf("fillOrders: only equity orders are supported")
		}

		if err := p.isSideAllowed(order.Symbol, order.Side, position.Quantity); err != nil {
			order.Status = BacktesterOrderStatusRejected
			return nil, fmt.Errorf("fillOrders: error checking side allowed: %v", err)
		}
	}

	trade := NewBacktesterTrade(order.Symbol, orderFillEntry.Time, orderFillEntry.Quantity, orderFillEntry.Price)

	if err := order.Fill(trade); err != nil {
		return nil, fmt.Errorf("fillOrder: error filling order: %w", err)
	}

	position.Quantity += orderFillEntry.Quantity

	return trade, nil
}

func (p *Playground) updateTrade(trade *BacktesterTrade, startingPositions map[eventmodels.Instrument]*Position) {
	position, ok := startingPositions[trade.Symbol]
	if !ok {
		position = &Position{}
	}

	if position.Quantity > 0 {
		if trade.Quantity < 0 {
			closeQuantity := math.Min(position.Quantity, math.Abs(trade.Quantity))
			pl := (trade.Price - position.CostBasis) * closeQuantity
			position.PL += pl
		}
	} else if position.Quantity < 0 {
		if trade.Quantity > 0 {
			closeQuantity := math.Min(math.Abs(position.Quantity), trade.Quantity)
			pl := (position.CostBasis - trade.Price) * closeQuantity
			position.PL += pl
		}
	}

	position.Quantity += trade.Quantity
	startingPositions[trade.Symbol] = position
}

func (p *Playground) fillOrdersDeprecated(ordersToOpen []*BacktesterOrder, startingPositions map[eventmodels.Instrument]*Position, orderFillEntryMap map[uint]OrderExecutionRequest, performChecks bool) ([]*BacktesterTrade, error) {
	var trades []*BacktesterTrade

	positionsCopy := make(map[eventmodels.Instrument]*Position)
	for symbol, position := range startingPositions {
		positionsCopy[symbol] = &Position{
			Quantity: position.Quantity,
		}
	}

	for _, order := range ordersToOpen {
		orderFillEntry, found := orderFillEntryMap[order.ID]
		if !found {
			return nil, fmt.Errorf("fillOrders: order %d not found in order fill entry map", order.ID)
		}

		trade, err := p.FillOrder(order, performChecks, orderFillEntry, positionsCopy)
		if err != nil {
			return nil, fmt.Errorf("fillOrders: error filling order: %w", err)
		}

		trades = append(trades, trade)
	}

	return trades, nil
}

func (p *Playground) updateBalance(newTrades []*BacktesterTrade, startingPositions map[eventmodels.Instrument]*Position) {
	for _, trade := range newTrades {
		currentPosition, ok := startingPositions[trade.Symbol]
		if !ok {
			currentPosition = &Position{}
		}

		if currentPosition.Quantity > 0 {
			if trade.Quantity < 0 {
				closeQuantity := math.Min(currentPosition.Quantity, math.Abs(trade.Quantity))
				pl := (trade.Price - currentPosition.CostBasis) * closeQuantity
				p.account.Balance += pl
			}
		} else if currentPosition.Quantity < 0 {
			if trade.Quantity > 0 {
				closeQuantity := math.Min(math.Abs(currentPosition.Quantity), trade.Quantity)
				pl := (currentPosition.CostBasis - trade.Price) * closeQuantity
				p.account.Balance += pl
			}
		}
	}
}

func (p *Playground) GetCurrentTime() time.Time {
	return p.clock.CurrentTime
}

func (p *Playground) fetchCurrentPrice(ctx context.Context, symbol eventmodels.Instrument) (float64, error) {
	return p.getCurrentPrice(symbol, p.minimumPeriod)
}

func (p *Playground) performLiquidations(symbol eventmodels.Instrument, position *Position, tag string) (*BacktesterOrder, error) {
	var order *BacktesterOrder

	if position.Quantity > 0 {
		order = NewBacktesterOrder(p.account.NextOrderID(), BacktesterOrderClassEquity, p.clock.CurrentTime, symbol, TradierOrderSideSell, position.Quantity, Market, Day, nil, nil, BacktesterOrderStatusPending, tag)
	} else if position.Quantity < 0 {
		order = NewBacktesterOrder(p.account.NextOrderID(), BacktesterOrderClassEquity, p.clock.CurrentTime, symbol, TradierOrderSideBuyToCover, math.Abs(position.Quantity), Market, Day, nil, nil, BacktesterOrderStatusPending, tag)
	} else {
		return nil, nil
	}

	p.account.PendingOrders = append(p.account.PendingOrders, order)

	orderFillPriceMap := map[uint]OrderExecutionRequest{}

	for _, order := range p.account.PendingOrders {
		price, err := p.fetchCurrentPrice(context.Background(), order.Symbol)
		if err != nil {
			return nil, fmt.Errorf("error fetching price: %w", err)
		}

		orderFillPriceMap[order.ID] = OrderExecutionRequest{
			Price: price,
			Time:  p.clock.CurrentTime,
		}
	}

	_, invalidOrders, err := p.CommitPendingOrders(map[eventmodels.Instrument]*Position{symbol: position}, orderFillPriceMap, true)
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

// checkForLiquidations checks for liquidations and returns a LiquidationEvent if liquidations are necessary
// Liquidations are performed in the following order:
// 1. Sort positions by position size (quantity * cost_basis) in descending order
// 2. Liquidate positions until equity reaches above maintenance margin or until all positions have been liquidated
func (p *Playground) checkForLiquidations(positions map[eventmodels.Instrument]*Position) (*TickDeltaEvent, error) {
	equity := p.GetEquity(positions)
	maintenanceMargin := p.getMaintenanceMargin(positions)

	var liquidatedOrders []*BacktesterOrder
	for equity < maintenanceMargin && len(positions) > 0 {
		sortedSymbols, sortedPositions := sortPositionsByQuantityDesc(positions)

		tag := fmt.Sprintf("liquidation - equity @ %.2f < maintenance margin @ %.2f", equity, maintenanceMargin)

		order, err := p.performLiquidations(sortedSymbols[0], sortedPositions[0], tag)
		if err != nil {
			return nil, fmt.Errorf("error performing liquidations: %w", err)
		}

		if order != nil {
			liquidatedOrders = append(liquidatedOrders, order)
		}

		positions = p.GetPositions()
		maintenanceMargin = p.getMaintenanceMargin(positions)
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

func (p *Playground) FetchCandles(symbol eventmodels.Instrument, period time.Duration, from time.Time, to time.Time) ([]*eventmodels.AggregateBarWithIndicators, error) {
	repo, ok := p.repos[symbol][period]
	if !ok {
		return nil, fmt.Errorf("symbol %s not found in repos", symbol)
	}

	candles, err := repo.FetchCandles(from, to)
	if err != nil {
		return nil, fmt.Errorf("error fetching candles: %w", err)
	}

	return candles, nil
}

func (p *Playground) Tick(d time.Duration, isPreview bool) (*TickDelta, error) {
	// Preview
	if isPreview {
		nextTick := p.clock.GetNext(p.clock.CurrentTime, d)

		var newCandles []*BacktesterCandle
		for instrument, periodRepoMap := range p.repos {
			for period, repo := range periodRepoMap {
				newCandle, err := repo.FetchCandlesAtOrAfter(nextTick)
				if err != nil {
					log.Warnf("repo.FetchCandlesAtOrAfter [%s]: %v", instrument, err)
					return nil, fmt.Errorf("backtest complete: no more ticks")
				}

				if newCandle != nil {
					newCandles = append(newCandles, &BacktesterCandle{
						Symbol: instrument,
						Period: period,
						Bar:    newCandle,
					})
				}
			}
		}

		isBacktestComplete := p.clock.IsTimeExpired(nextTick)

		return &TickDelta{
			NewCandles:         newCandles,
			CurrentTime:        nextTick.Format(time.RFC3339),
			IsBacktestComplete: isBacktestComplete,
		}, nil
	}

	// Update the account
	p.account.mutex.Lock()
	defer p.account.mutex.Unlock()

	// Check for liquidations
	var tickDeltaEvents []*TickDeltaEvent

	startingPositions := p.GetPositions()

	liquidationEvents, err := p.checkForLiquidations(startingPositions)
	if err != nil {
		return nil, fmt.Errorf("error checking for liquidations: %w", err)
	}

	if liquidationEvents != nil {
		tickDeltaEvents = append(tickDeltaEvents, liquidationEvents)
	}

	orderExecutionRequests := make(map[uint]OrderExecutionRequest)
	for _, order := range p.account.PendingOrders {
		price, err := p.fetchCurrentPrice(context.Background(), order.Symbol)
		if err != nil {
			return nil, fmt.Errorf("error fetching price: %w", err)
		}

		orderExecutionRequests[order.ID] = OrderExecutionRequest{
			Price:    price,
			Time:     p.clock.CurrentTime,
			Quantity: order.GetQuantity(),
		}
	}

	// Commit pending orders
	newTrades, invalidOrdersDTO, err := p.CommitPendingOrders(startingPositions, orderExecutionRequests, true)
	if err != nil {
		return nil, fmt.Errorf("error committing pending orders: %w", err)
	}

	// Update the clock
	if !p.clock.IsExpired() {
		p.clock.Add(d)
	}

	if p.clock.IsExpired() {
		if p.isBacktestComplete {
			return nil, fmt.Errorf("backtest complete: clock expired")
		}

		p.isBacktestComplete = true

		log.Infof("setting status -> backtest complete: clock expired")

		return &TickDelta{
			IsBacktestComplete: true,
		}, nil
	}

	// Update the candle repos
	var newCandles []*BacktesterCandle
	for instrument, periodRepoMap := range p.repos {
		for period, repo := range periodRepoMap {
			newCandle, err := repo.Update(p.clock.CurrentTime)
			if err != nil {
				log.Warnf("repo.Next [%s]: %v", instrument, err)
				return nil, fmt.Errorf("backtest complete: no more ticks")
			}

			if newCandle != nil {
				newCandles = append(newCandles, &BacktesterCandle{
					Symbol: instrument,
					Period: period,
					Bar:    newCandle,
				})
			}
		}
	}

	return &TickDelta{
		NewTrades:     newTrades,
		NewCandles:    newCandles,
		CurrentTime:   p.clock.CurrentTime.Format(time.RFC3339),
		InvalidOrders: invalidOrdersDTO,
		Events:        tickDeltaEvents,
	}, nil
}

func (p *Playground) GetMeta() *PlaygroundMeta {
	return p.Meta
}

func (p *Playground) GetBalance() float64 {
	return p.account.Balance
}

func (p *Playground) GetEquity(positions map[eventmodels.Instrument]*Position) float64 {
	equity := p.GetBalance()

	for _, position := range positions {
		equity += position.PL
	}

	return equity
}

func (p *Playground) GetOrders() []*BacktesterOrder {
	p.account.mutex.Lock()
	defer p.account.mutex.Unlock()

	result := append(p.account.Orders, p.account.PendingOrders...)
	if len(result) == 0 {
		return make([]*BacktesterOrder, 0)
	}
	return result
}

func (p *Playground) GetPosition(symbol eventmodels.Instrument) Position {
	position, ok := p.GetPositions()[symbol]
	if !ok {
		return Position{}
	}

	return *position
}

func (p *Playground) getNetTrades(trades []*BacktesterTrade) []*BacktesterTrade {
	netTrades := []*BacktesterTrade{}
	direction := 0
	totalQuantity := 0.0

	for _, trade := range trades {
		if direction > 0 {
			if totalQuantity+trade.Quantity < 0 {
				netTrades = []*BacktesterTrade{
					netTrades[len(netTrades)-1],
					trade,
				}

				direction = -1
				totalQuantity = trade.Quantity

				continue
			} else if totalQuantity+trade.Quantity == 0 {
				direction = 0
				netTrades = []*BacktesterTrade{}

				continue
			}

			totalQuantity += trade.Quantity
		} else if direction < 0 {
			if totalQuantity+trade.Quantity > 0 {
				netTrades = []*BacktesterTrade{
					netTrades[len(netTrades)-1],
					trade,
				}

				direction = 1
				totalQuantity = trade.Quantity

				continue
			} else if totalQuantity+trade.Quantity == 0 {
				direction = 0
				netTrades = []*BacktesterTrade{}

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

func (p *Playground) GetPositions() map[eventmodels.Instrument]*Position {
	if p.positionsCache != nil {
		// update pl
		for symbol, position := range p.positionsCache {
			close, err := p.getCurrentPrice(symbol, p.minimumPeriod)
			if err == nil {
				p.positionsCache[symbol].PL = (close - position.CostBasis) * position.Quantity
			} else {
				log.Warnf("getCurrentPrice [%s]: %v", symbol, err)
				p.positionsCache[symbol].PL = 0
			}
		}

		return p.positionsCache
	}

	positions := make(map[eventmodels.Instrument]*Position)

	allTrades := make(map[eventmodels.Instrument][]*BacktesterTrade)
	for _, order := range p.account.Orders {
		_, ok := positions[order.Symbol]
		if !ok {
			positions[order.Symbol] = &Position{}
		}

		orderStatus := order.GetStatus()
		if orderStatus.IsFilled() {
			filledQty := order.GetFilledVolume()
			positions[order.Symbol].Quantity += filledQty
			allTrades[order.Symbol] = append(allTrades[order.Symbol], order.Trades...)
		}
	}

	// remove positions with zero quantity
	for symbol, position := range positions {
		if position.Quantity == 0 {
			delete(positions, symbol)
		}
	}

	// adjust open trades
	// for symbol, position := range positions {
	// 	position.OpenTrades = p.getNetTrades(allTrades[symbol])
	// }

	vwapMap := make(map[eventmodels.Instrument]float64)
	totalQuantityMap := make(map[eventmodels.Instrument]float64)
	for _, order := range p.account.Orders {
		_, ok := vwapMap[order.Symbol]
		if !ok {
			vwapMap[order.Symbol] = 0.0
			totalQuantityMap[order.Symbol] = 0.0
		}

		orderStatus := order.GetStatus()
		if orderStatus == BacktesterOrderStatusFilled || orderStatus == BacktesterOrderStatusPartiallyFilled {
			totalQuantityMap[order.Symbol] += order.GetFilledVolume()

			if totalQuantityMap[order.Symbol] != 0 {
				vwapMap[order.Symbol] += order.GetAvgFillPrice() * order.GetFilledVolume()
			} else {
				vwapMap[order.Symbol] = 0
			}
		}
	}

	for symbol, vwap := range vwapMap {
		var costBasis float64
		totalQuantity := totalQuantityMap[symbol]

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
		close, err := p.getCurrentPrice(symbol, p.minimumPeriod)
		if err == nil {
			positions[symbol].PL = (close - costBasis) * positions[symbol].Quantity
		} else {
			log.Warnf("getCurrentPrice [%s]: %v", symbol, err)
			positions[symbol].PL = 0
		}
	}

	// set the cache
	p.positionsCache = positions

	return positions
}

func (p *Playground) GetCandle(symbol eventmodels.Instrument, period time.Duration) (*eventmodels.PolygonAggregateBarV2, error) {
	repo, ok := p.repos[symbol][period]
	if !ok {
		return nil, fmt.Errorf("GetTick: symbol %s not found in repos", symbol)
	}

	candle := repo.GetCurrentCandle()
	if candle == nil {
		return nil, fmt.Errorf("GetTick: no more candles for %s", symbol)
	}

	return candle.ToPolygonAggregateBarV2(), nil
}

func (p *Playground) isSideAllowed(symbol eventmodels.Instrument, side TradierOrderSide, positionQuantity float64) error {
	if positionQuantity > 0 {
		if side == TradierOrderSideBuyToCover {
			return fmt.Errorf("cannot buy to cover when long position exists: must sell to close")
		}

		if side == TradierOrderSideSellShort {
			return fmt.Errorf("cannot sell short when long position exists: must sell to close")
		}
	} else if positionQuantity < 0 {
		if side == TradierOrderSideBuy {
			return fmt.Errorf("cannot buy when short position exists: must sell to close")
		}

		if side == TradierOrderSideSell {
			return fmt.Errorf("cannot sell to close when short position exists: must buy to cover")
		}
	} else {
		if side == TradierOrderSideSell {
			return fmt.Errorf("cannot sell when no position exists: must sell short")
		}

		if side == TradierOrderSideBuyToCover {
			return fmt.Errorf("cannot buy to cover when no position exists")
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

func (p *Playground) getMaintenanceMargin(positions map[eventmodels.Instrument]*Position) float64 {
	maintenanceMargin := 0.0
	for _, position := range positions {
		maintenanceMargin += calculateMaintenanceRequirement(position.Quantity, position.CostBasis)
	}

	return maintenanceMargin
}

// todo: fix - free margin should be calculated on each open order, not the total position
func (p *Playground) GetFreeMarginFromPositionMap(positions map[eventmodels.Instrument]*Position) float64 {
	pl := 0.0
	for _, position := range positions {
		pl += position.PL
	}

	freeMargin := (p.account.Balance + pl)

	for _, position := range positions {
		freeMargin -= calculateInitialMarginRequirement(position.Quantity, position.CostBasis)
	}

	return freeMargin
}

func (p *Playground) GetFreeMargin() float64 {
	positions := p.GetPositions()
	return p.GetFreeMarginFromPositionMap(positions)
}

type PlaceOrderChanges struct {
	Commit func() error
}

func (p *Playground) PlaceOrder(order *BacktesterOrder) (*PlaceOrderChanges, error) {
	if order.Class != BacktesterOrderClassEquity {
		return nil, fmt.Errorf("only equity orders are supported")
	}

	if _, ok := p.repos[order.Symbol]; !ok {
		return nil, fmt.Errorf("symbol %s not found in repos", order.Symbol)
	}

	positions := p.GetPositions()
	positionQty := 0.0
	if position, ok := positions[order.Symbol]; ok {
		positionQty = position.Quantity
	}

	if err := p.isSideAllowed(order.Symbol, order.Side, positionQty); err != nil {
		return nil, fmt.Errorf("PlaceOrder: side not allowed: %w", err)
	}

	if order.Price != nil && *order.Price <= 0 {
		return nil, fmt.Errorf("price must be greater than 0")
	}

	if order.AbsoluteQuantity <= 0 {
		return nil, fmt.Errorf("quantity must be greater than 0")
	}

	p.account.mutex.Lock()
	defer p.account.mutex.Unlock()

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

	return &PlaceOrderChanges{
		Commit: func() error {
			p.account.mutex.Lock()
			defer p.account.mutex.Unlock()

			order.Status = BacktesterOrderStatusPending

			p.account.PendingOrders = append(p.account.PendingOrders, order)

			return nil
		},
	}, nil
}

// todo: change repository on playground to BacktesterCandleRepository
func NewPlayground(playgroundId *uuid.UUID, balance float64, clock *Clock, orders []*BacktesterOrder, env PlaygroundEnvironment, source *PlaygroundSource, now time.Time, feeds ...(*CandleRepository)) (*Playground, error) {
	repos := make(map[eventmodels.Instrument]map[time.Duration]*CandleRepository)
	var symbols []string
	var minimumPeriod time.Duration

	for _, feed := range feeds {
		symbol := feed.GetSymbol()

		if _, found := repos[symbol]; !found {
			symbols = append(symbols, symbol.GetTicker())
			repo := make(map[time.Duration]*CandleRepository)
			repos[symbol] = repo
		}

		repos[symbol][feed.GetPeriod()] = feed

		if minimumPeriod == 0 || feed.GetPeriod() < minimumPeriod {
			minimumPeriod = feed.GetPeriod()
		}
	}

	var startAt time.Time
	var endAt *time.Time

	if clock != nil {
		startAt = clock.CurrentTime
		endAt = &clock.EndTime
	} else {
		startAt = now
	}

	
	meta := &PlaygroundMeta{
		Symbols:          symbols,
		StartingBalance:  balance,
		Environment:      env,
		StartAt:          startAt,
		EndAt:            endAt,
	}

	if env == PlaygroundEnvironmentLive {
		if source == nil {
			return nil, fmt.Errorf("source is required")
		}

		meta.SourceBroker = source.Broker
		meta.SourceAccountId = source.AccountID
		meta.SourceApiKeyName = source.ApiKeyName
	}

	var id uuid.UUID
	if playgroundId != nil {
		id = *playgroundId
	} else {
		id = uuid.New()
	}

	return &Playground{
		Meta:            meta,
		ID:              id,
		account:         NewBacktesterAccount(balance, orders),
		clock:           clock,
		repos:           repos,
		positionsCache:  nil,
		openOrdersCache: make(map[eventmodels.Instrument][]*BacktesterOrder),
		minimumPeriod:   minimumPeriod,
	}, nil
}

// note: this is only here for testing
func NewPlaygroundDeprecated(balance float64, clock *Clock, env PlaygroundEnvironment, feeds ...BacktesterDataFeed) (*Playground, error) {
	repos := make(map[eventmodels.Instrument]map[time.Duration]*CandleRepository)
	var symbols []string
	var minimumPeriod time.Duration

	for _, feed := range feeds {
		symbol := feed.GetSymbol()

		if _, found := repos[symbol]; !found {
			symbols = append(symbols, symbol.GetTicker())
			repo := make(map[time.Duration]*CandleRepository)
			repos[symbol] = repo
		}

		candles, err := feed.FetchCandles(clock.CurrentTime, clock.EndTime)
		if err != nil {
			return nil, fmt.Errorf("error fetching candles: %w", err)
		}

		var data []*eventmodels.PolygonAggregateBarV2
		for _, candle := range candles {
			data = append(data, candle.ToPolygonAggregateBarV2())
		}

		source := eventmodels.CandleRepositorySource{
			Type: feed.GetSource(),
		}

		repos[symbol][feed.GetPeriod()], err = NewCandleRepository(symbol, feed.GetPeriod(), data, []string{}, nil, 0, 0, source)
		if err != nil {
			return nil, fmt.Errorf("error creating repository: %w", err)
		}

		if minimumPeriod == 0 || feed.GetPeriod() < minimumPeriod {
			minimumPeriod = feed.GetPeriod()
		}
	}

	var endAt *time.Time
	if clock != nil {
		endAt = &clock.EndTime
	}

	return &Playground{
		Meta: &PlaygroundMeta{
			Symbols:         symbols,
			StartAt:         clock.CurrentTime,
			EndAt:           endAt,
			StartingBalance: balance,
			Environment:     env,
		},
		ID:              uuid.New(),
		account:         NewBacktesterAccount(balance, []*BacktesterOrder{}),
		clock:           clock,
		repos:           repos,
		positionsCache:  nil,
		openOrdersCache: make(map[eventmodels.Instrument][]*BacktesterOrder),
		minimumPeriod:   minimumPeriod,
	}, nil
}

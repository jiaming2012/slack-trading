package models

import (
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

type Playground struct {
	Meta               *PlaygroundMeta
	ID                 uuid.UUID
	account            *BacktesterAccount
	clock              *Clock
	repos              map[eventmodels.Instrument]map[time.Duration]*BacktesterCandleRepository
	isBacktestComplete bool
	positionsCache     map[eventmodels.Instrument]*Position
	openOrdersCache    map[eventmodels.Instrument][]*BacktesterOrder
	minimumPeriod      time.Duration
	Env                PlaygroundEnvironment
}

func (p *Playground) GetOpenOrders(symbol eventmodels.Instrument) []*BacktesterOrder {
	openOrders, found := p.openOrdersCache[symbol]
	if !found {
		return make([]*BacktesterOrder, 0)
	}

	return openOrders
}

func (p *Playground) commitPendingOrders(startingPositions map[eventmodels.Instrument]*Position) (newTrades []*BacktesterTrade, invalidOrders []*BacktesterOrder, err error) {
	invalidOrders = []*BacktesterOrder{}
	pendingCloses := make(map[*BacktesterOrder]float64)
	var ordersToOpen []*BacktesterOrder

	for _, order := range p.account.PendingOrders {
		if order.Status != BacktesterOrderStatusPending {
			log.Warnf("commitPendingOrders: order %d status is %s, not pending", order.ID, order.Status)
			continue
		}

		// append the order to the queue
		p.account.Orders = append(p.account.Orders, order)

		// check if the order is valid
		orderQuantity := order.GetQuantity()
		if orderQuantity == 0 {
			invalidOrders = append(invalidOrders, order)
			order.Status = BacktesterOrderStatusRejected
			rejectReason := ErrInvalidOrderVolumeZero.Error()
			order.RejectReason = &rejectReason
			continue
		}

		isCloseOrder := false
		currentPrice, err := p.getCurrentPrice(order.Symbol, p.minimumPeriod)
		if err != nil {
			invalidOrders = append(invalidOrders, order)
			order.Status = BacktesterOrderStatusRejected
			rejectReason := ErrNoPriceAvailable.Error()
			order.RejectReason = &rejectReason
			continue
		} else {
			// perform margin check
			freeMargin := p.GetFreeMarginFromPositionMap(startingPositions)
			initialMargin := calculateInitialMarginRequirement(orderQuantity, currentPrice)
			position := startingPositions[order.Symbol]

			performMarginCheck := true
			if position != nil {
				if position.Quantity <= 0 && orderQuantity > 0 {
					if orderQuantity > math.Abs(position.Quantity) {
						order.Status = BacktesterOrderStatusRejected
						rejectReason := ErrInvalidOrderVolumeLongVolume.Error()
						order.RejectReason = &rejectReason
						invalidOrders = append(invalidOrders, order)
						continue
					} else {
						performMarginCheck = false
						isCloseOrder = true
					}
				} else if position.Quantity >= 0 && orderQuantity < 0 {
					if math.Abs(orderQuantity) > position.Quantity {
						order.Status = BacktesterOrderStatusRejected
						rejectReason := ErrInvalidOrderVolumeShortVolume.Error()
						order.RejectReason = &rejectReason
						invalidOrders = append(invalidOrders, order)
						continue
					} else {
						performMarginCheck = false
						isCloseOrder = true
					}
				}
			}

			// check if the order can be filled
			if performMarginCheck && freeMargin <= initialMargin {
				order.Status = BacktesterOrderStatusRejected
				rejectReason := fmt.Sprintf("%s: free_margin (%.2f) <= initial_margin (%.2f)", ErrInsufficientFreeMargin.Error(), freeMargin, initialMargin)
				order.RejectReason = &rejectReason
				invalidOrders = append(invalidOrders, order)
			} else if isCloseOrder {
				closeVolume := math.Abs(orderQuantity)
				openOrders := p.GetOpenOrders(order.Symbol)
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

			ordersToOpen = append(ordersToOpen, order)
		}
	}

	// empty the pending orders
	p.account.PendingOrders = make([]*BacktesterOrder, 0)

	newTrades, err = p.updateTrades(ordersToOpen, startingPositions)
	if err != nil {
		err = fmt.Errorf("error updating trades: %w", err)
		return
	}

	// remove the open orders that were closed in cache
	p.updateOpenOrdersCache()

	// update the account balance before updating the positions cache
	p.updateBalance(newTrades, startingPositions)

	for _, trade := range newTrades {
		p.updatePositionsCache(trade)
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

func (p *Playground) updateTrades(ordersToOpen []*BacktesterOrder, startingPositions map[eventmodels.Instrument]*Position) ([]*BacktesterTrade, error) {
	var trades []*BacktesterTrade

	positionsCopy := make(map[eventmodels.Instrument]*Position)
	for symbol, position := range startingPositions {
		positionsCopy[symbol] = &Position{
			Quantity: position.Quantity,
		}
	}

	for _, order := range ordersToOpen {
		orderStatus := order.GetStatus()
		if orderStatus.IsTradingAllowed() && order.Type == Market {
			if order.Class != Equity {
				log.Errorf("updateTrades: only equity orders are supported")
				continue
			}

			position, ok := positionsCopy[order.Symbol]
			if !ok {
				position = &Position{Quantity: 0}
				positionsCopy[order.Symbol] = position
			}

			if err := p.isSideAllowed(order.Symbol, order.Side, position.Quantity); err != nil {
				order.Status = BacktesterOrderStatusRejected
				log.Errorf("updateTrades: error checking side allowed: %v", err)
				continue
			}

			price, err := p.getCurrentPrice(order.Symbol, p.minimumPeriod)
			if err != nil {
				log.Errorf("updateTrades: error fetching price: %v", err)
				continue
			}

			quantity := order.GetQuantity()

			trade := NewBacktesterTrade(p.account.NextTradeID(), order.Symbol, p.clock.CurrentTime, quantity, price)

			if err := order.Fill(trade); err != nil {
				log.Errorf("updateTrades: error filling order: %v", err)
				continue
			}

			order.Status = BacktesterOrderStatusFilled

			if order.Side == BacktesterOrderSideBuy || order.Side == BacktesterOrderSideSellShort {
				p.addToOpenOrdersCache(order)
			}

			// mark the open orders that this trade closes
			for _, o := range order.Closes {
				o.ClosedBy = append(o.ClosedBy, trade)
			}

			trades = append(trades, trade)

			position.Quantity += quantity
		}
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

func (p *Playground) performLiquidations(symbol eventmodels.Instrument, position *Position, tag string) (*BacktesterOrder, error) {
	var order *BacktesterOrder

	if position.Quantity > 0 {
		order = NewBacktesterOrder(p.account.NextOrderID(), Equity, p.clock.CurrentTime, symbol, BacktesterOrderSideSell, position.Quantity, Market, Day, nil, nil, BacktesterOrderStatusPending, tag)
	} else if position.Quantity < 0 {
		order = NewBacktesterOrder(p.account.NextOrderID(), Equity, p.clock.CurrentTime, symbol, BacktesterOrderSideBuyToCover, math.Abs(position.Quantity), Market, Day, nil, nil, BacktesterOrderStatusPending, tag)
	} else {
		return nil, nil
	}

	p.account.PendingOrders = append(p.account.PendingOrders, order)

	_, invalidOrders, err := p.commitPendingOrders(map[eventmodels.Instrument]*Position{symbol: position})
	if err != nil {
		return nil, fmt.Errorf("error committing pending orders: %w", err)
	}

	if len(invalidOrders) > 0 {
		return nil, fmt.Errorf("error committing pending orders: %d invalid orders: %v", len(invalidOrders), invalidOrders)
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
		if p.Env == PlaygroundEnvironmentLive {
			return nil, fmt.Errorf("preview not supported in live environment")
		}

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

	// Commit pending orders
	newTrades, invalidOrdersDTO, err := p.commitPendingOrders(startingPositions)
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

func (p *Playground) isSideAllowed(symbol eventmodels.Instrument, side BacktesterOrderSide, positionQuantity float64) error {
	if positionQuantity > 0 {
		if side == BacktesterOrderSideBuyToCover {
			return fmt.Errorf("cannot buy to cover when long position exists: must sell to close")
		}

		if side == BacktesterOrderSideSellShort {
			return fmt.Errorf("cannot sell short when long position exists: must sell to close")
		}
	} else if positionQuantity < 0 {
		if side == BacktesterOrderSideBuy {
			return fmt.Errorf("cannot buy when short position exists: must sell to close")
		}

		if side == BacktesterOrderSideSell {
			return fmt.Errorf("cannot sell to close when short position exists: must buy to cover")
		}
	} else {
		if side == BacktesterOrderSideSell {
			return fmt.Errorf("cannot sell when no position exists: must sell short")
		}

		if side == BacktesterOrderSideBuyToCover {
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

func (p *Playground) PlaceOrder(order *BacktesterOrder) error {
	if order.Class != Equity {
		return fmt.Errorf("only equity orders are supported")
	}

	if _, ok := p.repos[order.Symbol]; !ok {
		return fmt.Errorf("symbol %s not found in repos", order.Symbol)
	}

	positions := p.GetPositions()
	positionQty := 0.0
	if position, ok := positions[order.Symbol]; ok {
		positionQty = position.Quantity
	}

	if err := p.isSideAllowed(order.Symbol, order.Side, positionQty); err != nil {
		return fmt.Errorf("PlaceOrder: side not allowed: %w", err)
	}

	if order.Price != nil && *order.Price <= 0 {
		return fmt.Errorf("price must be greater than 0")
	}

	if order.AbsoluteQuantity <= 0 {
		return fmt.Errorf("quantity must be greater than 0")
	}

	p.account.mutex.Lock()
	defer p.account.mutex.Unlock()

	for _, o := range p.account.Orders {
		if o.ID == order.ID {
			return fmt.Errorf("order with id %d already exists in orders", order.ID)
		}
	}

	for _, o := range p.account.PendingOrders {
		if o.ID == order.ID {
			return fmt.Errorf("order with id %d already exists in pending orders", order.ID)
		}
	}

	order.Status = BacktesterOrderStatusPending

	p.account.PendingOrders = append(p.account.PendingOrders, order)

	return nil
}

// todo: change repository on playground to BacktesterCandleRepository
func NewPlayground(balance float64, clock *Clock, env PlaygroundEnvironment, feeds ...(*BacktesterCandleRepository)) (*Playground, error) {
	repos := make(map[eventmodels.Instrument]map[time.Duration]*BacktesterCandleRepository)
	var symbols []string
	var minimumPeriod time.Duration

	for _, feed := range feeds {
		symbol := feed.GetSymbol()

		if _, found := repos[symbol]; !found {
			symbols = append(symbols, symbol.GetTicker())
			repo := make(map[time.Duration]*BacktesterCandleRepository)
			repos[symbol] = repo
		}

		repos[symbol][feed.GetPeriod()] = feed

		if minimumPeriod == 0 || feed.GetPeriod() < minimumPeriod {
			minimumPeriod = feed.GetPeriod()
		}
	}

	return &Playground{
		Meta: &PlaygroundMeta{
			Symbols:         symbols,
			StartDate:       clock.CurrentTime.Format(time.RFC3339),
			EndDate:         clock.EndTime.Format(time.RFC3339),
			StartingBalance: balance,
			Environment:     env,
		},
		ID:              uuid.New(),
		account:         NewBacktesterAccount(balance),
		clock:           clock,
		repos:           repos,
		positionsCache:  nil,
		openOrdersCache: make(map[eventmodels.Instrument][]*BacktesterOrder),
		minimumPeriod:   minimumPeriod,
	}, nil
}

// note: this is only here for testing
func NewPlaygroundDeprecated(balance float64, clock *Clock, env PlaygroundEnvironment, feeds ...BacktesterDataFeed) (*Playground, error) {
	repos := make(map[eventmodels.Instrument]map[time.Duration]*BacktesterCandleRepository)
	var symbols []string
	var minimumPeriod time.Duration

	for _, feed := range feeds {
		symbol := feed.GetSymbol()

		if _, found := repos[symbol]; !found {
			symbols = append(symbols, symbol.GetTicker())
			repo := make(map[time.Duration]*BacktesterCandleRepository)
			repos[symbol] = repo
		}

		candles, err := feed.FetchCandles(clock.CurrentTime, clock.EndTime)
		if err != nil {
			return nil, fmt.Errorf("error fetching candles: %w", err)
		}

		repos[symbol][feed.GetPeriod()] = NewBacktesterCandleRepository(symbol, feed.GetPeriod(), candles, 0)

		if minimumPeriod == 0 || feed.GetPeriod() < minimumPeriod {
			minimumPeriod = feed.GetPeriod()
		}
	}

	return &Playground{
		Meta: &PlaygroundMeta{
			Symbols:         symbols,
			StartDate:       clock.CurrentTime.Format(time.RFC3339),
			EndDate:         clock.EndTime.Format(time.RFC3339),
			StartingBalance: balance,
			Environment:     env,
		},
		ID:              uuid.New(),
		account:         NewBacktesterAccount(balance),
		clock:           clock,
		repos:           repos,
		positionsCache:  nil,
		openOrdersCache: make(map[eventmodels.Instrument][]*BacktesterOrder),
		minimumPeriod:   minimumPeriod,
	}, nil
}

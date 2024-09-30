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
	ID                 uuid.UUID
	account            BacktesterAccount
	clock              *Clock
	repos              map[eventmodels.Instrument]*BacktesterCandleRepository
	isBacktestComplete bool
}

func (p *Playground) addPendingOrdersToOrders() {
	p.account.Orders = append(p.account.Orders, p.account.PendingOrders...)
	p.account.PendingOrders = []*BacktesterOrder{}
}

func (p *Playground) getCurrentPrice(symbol eventmodels.Instrument) (float64, error) {
	repo, ok := p.repos[symbol]
	if !ok {
		return 0, fmt.Errorf("getCurrentPrice: symbol %s not found in repos", symbol)
	}

	candle := repo.GetCurrentCandle()
	if candle == nil {
		return 0, fmt.Errorf("getCurrentPrice: no more candles")
	}

	return candle.Close, nil
}

func (p *Playground) updateTrades() ([]*BacktesterTrade, error) {
	var trades []*BacktesterTrade

	for _, order := range p.account.Orders {
		orderStatus := order.GetStatus()
		if orderStatus.IsTradingAllowed() && order.Type == Market {
			if order.Class != Equity {
				return nil, fmt.Errorf("updateTrades: only equity orders are supported")
			}

			price, err := p.getCurrentPrice(order.Symbol)
			if err != nil {
				return nil, fmt.Errorf("updateTrades: error fetching price: %w", err)
			}

			quantity := order.GetQuantity()

			trade := NewBacktesterTrade(order.Symbol, p.clock.CurrentTime, quantity, price)

			if err := order.Fill(trade); err != nil {
				return nil, fmt.Errorf("updateTrades: error filling order: %w", err)
			}

			order.status = BacktesterOrderStatusFilled

			trades = append(trades, trade)
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

func (p *Playground) Tick(d time.Duration) (*StateChange, error) {
	if !p.clock.IsExpired() {
		p.clock.Add(d)
	}

	if p.clock.IsExpired() {
		if p.isBacktestComplete {
			return nil, fmt.Errorf("backtest complete: clock expired")
		}

		p.isBacktestComplete = true

		return &StateChange{
			IsBacktestComplete: true,
		}, nil
	}

	var newCandles []*BacktesterCandle

	for instrument, repo := range p.repos {
		newCandle, err := repo.Update(p.clock.CurrentTime)
		if err != nil {
			log.Warnf("repo.Next [%s]: %v", instrument, err)
			return nil, fmt.Errorf("backtest complete: no more ticks")
		}

		if newCandle != nil {
			newCandles = append(newCandles, &BacktesterCandle{
				Symbol: instrument,
				Candle: newCandle,
			})
		}
	}

	startingPositions := p.GetPositions()

	p.addPendingOrdersToOrders()

	newTrades, err := p.updateTrades()
	if err != nil {
		return nil, fmt.Errorf("error checking for new trades: %w", err)
	}

	p.updateBalance(newTrades, startingPositions)

	return &StateChange{
		NewTrades:  newTrades,
		NewCandles: newCandles,
	}, nil
}

func (p *Playground) GetBalance() float64 {
	return p.account.Balance
}

func (p *Playground) GetOrders() []*BacktesterOrder {
	return p.account.Orders
}

func (p *Playground) GetPosition(symbol eventmodels.Instrument) Position {
	position, ok := p.GetPositions()[symbol]
	if !ok {
		return Position{}
	}

	return *position
}

func (p *Playground) GetPositions() map[eventmodels.Instrument]*Position {
	postions := make(map[eventmodels.Instrument]*Position)

	for _, order := range p.account.Orders {
		_, ok := postions[order.Symbol]
		if !ok {
			postions[order.Symbol] = &Position{}
		}

		orderStatus := order.GetStatus()
		if orderStatus.IsFilled() {
			postions[order.Symbol].Quantity += order.GetFilledQuantity()
		}
	}

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
			totalQuantityMap[order.Symbol] += order.GetFilledQuantity()
			vwapMap[order.Symbol] += order.GetAvgFillPrice() * order.GetFilledQuantity()
		}
	}

	for symbol, vwap := range vwapMap {
		var avgPrice float64
		totalQuantity := totalQuantityMap[symbol]
		if totalQuantity != 0 {
			avgPrice = vwap / totalQuantity
		}

		postions[symbol].CostBasis = avgPrice
	}

	return postions
}

func (p *Playground) GetCandle(symbol eventmodels.Instrument) (*eventmodels.PolygonAggregateBarV2, error) {
	repo, ok := p.repos[symbol]
	if !ok {
		return nil, fmt.Errorf("GetTick: symbol %s not found in repos", symbol)
	}

	candle := repo.GetCurrentCandle()
	if candle == nil {
		return nil, fmt.Errorf("GetTick: no more candles for %s", symbol)
	}

	return candle, nil
}

func (p *Playground) PlaceOrder(order *BacktesterOrder) error {
	if order.Class != Equity {
		return fmt.Errorf("only equity orders are supported")
	}

	if _, ok := p.repos[order.Symbol]; !ok {
		return fmt.Errorf("symbol %s not found in repos", order.Symbol)
	}

	position := p.GetPosition(order.Symbol)

	if position.Quantity > 0 {
		if order.Side == BacktesterOrderSideBuyToCover {
			return fmt.Errorf("cannot buy to cover when long position exists: must sell to close")
		}

		if order.Side == BacktesterOrderSideSellShort {
			return fmt.Errorf("cannot sell short when long position exists: must sell to close")
		}
	} else if position.Quantity < 0 {
		if order.Side == BacktesterOrderSideBuy {
			return fmt.Errorf("cannot buy when short position exists: must sell to close")
		}

		if order.Side == BacktesterOrderSideSell {
			return fmt.Errorf("cannot sell to close when short position exists: must buy to cover")
		}
	} else {
		if order.Side == BacktesterOrderSideSell {
			return fmt.Errorf("cannot sell when no position exists: must sell short")
		}

		if order.Side == BacktesterOrderSideBuyToCover {
			return fmt.Errorf("cannot buy to cover when no position exists")
		}
	}

	if order.Price != nil && *order.Price <= 0 {
		return fmt.Errorf("price must be greater than 0")
	}

	if order.AbsoluteQuantity <= 0 {
		return fmt.Errorf("quantity must be greater than 0")
	}

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

	order.status = BacktesterOrderStatusPending

	p.account.PendingOrders = append(p.account.PendingOrders, order)

	return nil
}

func NewPlaygroundMultipleFeeds(balance float64, clock *Clock, feeds ...BacktesterDataFeed) (*Playground, error) {
	repos := make(map[eventmodels.Instrument]*BacktesterCandleRepository)

	for _, feed := range feeds {
		candles, err := feed.FetchRange(clock.CurrentTime, clock.EndTime)
		if err != nil {
			return nil, fmt.Errorf("NewPlaygroundMultipleFeeds: error fetching candles: %w", err)
		}

		repo := NewBacktesterCandleRepository(feed.GetSymbol(), candles)

		repos[feed.GetSymbol()] = repo
	}

	return &Playground{
		ID: uuid.New(),
		account: BacktesterAccount{
			Balance: balance,
		},
		clock: clock,
		repos: repos,
	}, nil
}

func NewPlayground(balance float64, clock *Clock, feed BacktesterDataFeed) (*Playground, error) {
	repos := make(map[eventmodels.Instrument]*BacktesterCandleRepository)

	candles, err := feed.FetchRange(clock.CurrentTime, clock.EndTime)
	if err != nil {
		return nil, fmt.Errorf("NewPlayground: error fetching candles: %w", err)
	}

	symbol := feed.GetSymbol()

	repos[symbol] = NewBacktesterCandleRepository(symbol, candles)

	return &Playground{
		ID: uuid.New(),
		account: BacktesterAccount{
			Balance: balance,
		},
		clock: clock,
		repos: repos,
	}, nil
}

package models

import (
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

type Playground struct {
	ID                 uuid.UUID
	account            BacktesterAccount
	clock              *Clock
	datafeed           BacktesterDataFeed
	isBacktestComplete bool
}

func (p *Playground) addPendingOrdersToOrders() {
	p.account.Orders = append(p.account.Orders, p.account.PendingOrders...)
	p.account.PendingOrders = []*BacktesterOrder{}
}

func (p *Playground) checkForNewTrades() ([]*BacktesterTrade, error) {
	var trades []*BacktesterTrade

	for _, order := range p.account.Orders {
		orderStatus := order.GetStatus()
		if orderStatus.IsTradingAllowed() && order.Type == Market {
			if order.Class != Equity {
				return nil, fmt.Errorf("checkForNewTrades: only equity orders are supported")
			}

			price, err := p.datafeed.FetchStockPrice(p.clock.CurrentTime, eventmodels.StockSymbol(order.Symbol))
			if err != nil {
				return nil, fmt.Errorf("error fetching price: %w", err)
			}

			quantity := order.Quantity
			if order.Side == BacktesterOrderSideSell || order.Side == BacktesterOrderSideSellShort {
				quantity *= -1
			}

			trade := NewBacktesterTrade(order.Symbol, p.clock.CurrentTime, quantity, price)

			if err = order.Fill(trade); err != nil {
				return nil, fmt.Errorf("error filling order: %w", err)
			}

			status := BacktesterOrderStatusFilled

			order.status = &status

			trades = append(trades, trade)
		}
	}

	return trades, nil
}

func (p *Playground) updateBalance(newTrades []*BacktesterTrade, startingPositions map[string]*Position) {
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
	if !p.clock.IsFinished() {
		p.clock.Add(d)
	}
	
	if p.clock.IsFinished() {
		if p.isBacktestComplete {
			return nil, fmt.Errorf("backtest is already complete")
		}

		p.isBacktestComplete = true

		return &StateChange{
			IsBacktestComplete: true,
		}, nil
	}

	startingPositions := p.GetPositions()

	p.addPendingOrdersToOrders()

	newTrades, err := p.checkForNewTrades()
	if err != nil {
		return nil, fmt.Errorf("error checking for new trades: %w", err)
	}

	p.updateBalance(newTrades, startingPositions)

	return &StateChange{
		NewTrades: newTrades,
	}, nil
}

func (p *Playground) GetAccountBalance() float64 {
	return p.account.Balance
}

func (p *Playground) GetOrders() []*BacktesterOrder {
	return p.account.Orders
}

func (p *Playground) GetPosition(symbol string) Position {
	position, ok := p.GetPositions()[symbol]
	if !ok {
		return Position{}
	}

	return *position
}

func (p *Playground) GetPositions() map[string]*Position {
	postions := make(map[string]*Position)

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

	vwapMap := make(map[string]float64)
	totalQuantityMap := make(map[string]float64)
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

func (p *Playground) AddOrder(order *BacktesterOrder) error {
	if order.Class != Equity {
		return fmt.Errorf("only equity orders are supported")
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

	if order.Quantity <= 0 {
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

	p.account.PendingOrders = append(p.account.PendingOrders, order)

	return nil
}

func NewPlayground(balance float64, clock *Clock, feed BacktesterDataFeed) *Playground {
	return &Playground{
		ID: uuid.New(),
		account: BacktesterAccount{
			Balance: balance,
		},
		clock:    clock,
		datafeed: feed,
	}
}

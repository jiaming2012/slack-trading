package models

import (
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

type Playground struct {
	ID       uuid.UUID
	account  BacktesterAccount
	clock    time.Time
	datafeed BacktesterDataFeed
}

func (p *Playground) checkForNewTrades() ([]BacktesterTrade, error) {
	var trades []BacktesterTrade

	for _, order := range p.account.Orders {
		orderStatus := order.GetStatus()
		if orderStatus.IsTradeable() && order.Type == Market {
			if order.Class != Equity {
				return nil, fmt.Errorf("checkForNewTrades: only equity orders are supported")
			}

			price, err := p.datafeed.FetchStockPrice(p.clock, eventmodels.StockSymbol(order.Symbol))
			if err != nil {
				return nil, fmt.Errorf("error fetching price: %w", err)
			}

			trade := BacktesterTrade{
				TransactionDate: p.clock,
				Quantity:        order.Quantity,
				Price:           price,
			}

			if err = order.Fill(&trade); err != nil {
				return nil, fmt.Errorf("error filling order: %w", err)
			}

			trades = append(trades, trade)
		}
	}

	return trades, nil
}

func (p *Playground) Tick(d time.Duration) (*StateChange, error) {
	p.clock = p.clock.Add(d)

	trades, err := p.checkForNewTrades()
	if err != nil {
		return nil, fmt.Errorf("error checking for new trades: %w", err)
	}

	return &StateChange{
		NewTrades: trades,
	}, nil
}

func (p *Playground) GetAccountBalance() float64 {
	return p.account.Amount
}

func (p *Playground) GetOrders() []BacktesterOrder {
	return p.account.Orders
}

func (p *Playground) AddOrder(order BacktesterOrder) error {
	if order.Class != Equity {
		return fmt.Errorf("only equity orders are supported")
	}

	if order.Price != nil && *order.Price <= 0 {
		return fmt.Errorf("price must be greater than 0")
	}

	if order.Quantity <= 0 {
		return fmt.Errorf("quantity must be greater than 0")
	}

	p.account.Orders = append(p.account.Orders, order)

	return nil
}

func NewPlayground(balance float64, startTime time.Time, feed BacktesterDataFeed) *Playground {
	return &Playground{
		ID: uuid.New(),
		account: BacktesterAccount{
			Amount: balance,
		},
		clock:    startTime,
		datafeed: feed,
	}
}

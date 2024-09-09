package main

import (
	"fmt"
	"sync"
	"time"
)

// Define the types of instruments
type InstrumentType int

const (
	Stock InstrumentType = iota
	Option
)

// Define a Trade struct
type Trade struct {
	Instrument     string
	InstrumentType InstrumentType
	Price          float64
	Quantity       int
	Buy            bool
}

// Define a Position struct
type Position struct {
	Instrument     string
	InstrumentType InstrumentType
	Quantity       int
	AvgPrice       float64
}

// Define an Account struct
type Account struct {
	Balance   float64
	Equity    float64
	Positions map[string]*Position
	Margin    float64
	mu        sync.Mutex
}

// NewAccount creates a new trading account
func NewAccount(initialBalance float64) *Account {
	return &Account{
		Balance:   initialBalance,
		Equity:    initialBalance,
		Positions: make(map[string]*Position),
		Margin:    0.0,
	}
}

// PlaceTrade processes a trade and updates the account
func (a *Account) PlaceTrade(t Trade) {
	a.mu.Lock()
	defer a.mu.Unlock()

	position, exists := a.Positions[t.Instrument]
	if !exists {
		position = &Position{
			Instrument:     t.Instrument,
			InstrumentType: t.InstrumentType,
			Quantity:       0,
			AvgPrice:       0.0,
		}
		a.Positions[t.Instrument] = position
	}

	if t.Buy {
		totalCost := t.Price * float64(t.Quantity)
		a.Balance -= totalCost
		newQty := position.Quantity + t.Quantity
		position.AvgPrice = (position.AvgPrice*float64(position.Quantity) + totalCost) / float64(newQty)
		position.Quantity = newQty
	} else {
		totalProceeds := t.Price * float64(t.Quantity)
		a.Balance += totalProceeds
		position.Quantity -= t.Quantity
		if position.Quantity == 0 {
			delete(a.Positions, t.Instrument)
		}
	}

	a.updateEquity()
	a.updateMargin()
}

// updateEquity updates the account's equity based on the positions
func (a *Account) updateEquity() {
	equity := a.Balance
	for _, pos := range a.Positions {
		equity += pos.AvgPrice * float64(pos.Quantity)
	}
	a.Equity = equity
}

// updateMargin updates the account's margin based on the positions
func (a *Account) updateMargin() {
	// Implement margin calculations here
	a.Margin = 0.0
}

// QueryBalance returns the current balance
func (a *Account) QueryBalance() float64 {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.Balance
}

// QueryEquity returns the current equity
func (a *Account) QueryEquity() float64 {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.Equity
}

// QueryPositions returns the current positions
func (a *Account) QueryPositions() map[string]*Position {
	a.mu.Lock()
	defer a.mu.Unlock()
	positionsCopy := make(map[string]*Position)
	for k, v := range a.Positions {
		positionsCopy[k] = &Position{
			Instrument:     v.Instrument,
			InstrumentType: v.InstrumentType,
			Quantity:       v.Quantity,
			AvgPrice:       v.AvgPrice,
		}
	}
	return positionsCopy
}

// QueryMargin returns the current margin
func (a *Account) QueryMargin() float64 {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.Margin
}

type Tick struct {
	Time       time.Time
	Instrument string
	Price      float64
}

// Example of feeding prices and placing trades
func main() {
	account := NewAccount(100000) // Start with $100,000

	// Example trade: Buying 10 shares of a stock at $100 each
	account.PlaceTrade(Trade{
		Instrument:     "AAPL",
		InstrumentType: Stock,
		Price:          100.0,
		Quantity:       10,
		Buy:            true,
	})

	// Example trade: Selling 5 shares of a stock at $110 each
	account.PlaceTrade(Trade{
		Instrument:     "AAPL",
		InstrumentType: Stock,
		Price:          110.0,
		Quantity:       5,
		Buy:            false,
	})

	fmt.Printf("Balance: $%.2f\n", account.QueryBalance())
	fmt.Printf("Equity: $%.2f\n", account.QueryEquity())
	fmt.Printf("Positions: %+v\n", account.QueryPositions())
	fmt.Printf("Margin: $%.2f\n", account.QueryMargin())
}

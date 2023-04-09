package money_management

import (
	"fmt"
	"slack-trading/src/models"
)

type Account struct {
	Balance           float64
	MaxLossPercentage float64
	PriceLevels       PriceLevels
}

type PriceLevel struct {
	Price             float64
	NoOfTrades        int
	AllocationPercent float64 // the amount of Account.Balance allocated to this price level
	Trades            models.Trades
}

type PriceLevels struct {
	Values []PriceLevel
}

type BalanceLevelStats struct {
	TotalBalance float64
	UsedBalance  float64
}

var BalanceOutOfRangeErr = fmt.Errorf("balance is out of range")
var LevelsNotSetErr = fmt.Errorf("at least two price levels must be set")
var MaxLossPercentErr = fmt.Errorf("maxLossPercentage must be a value between 0 and 1")
var PriceLevelsNotSortedErr = fmt.Errorf("price levels are not sorted")
var PriceOutsideLimitsErr = fmt.Errorf("price is outside price limits")

func (a *Account) Update() error {

}

func (a *Account) CanPlaceTrade(trade models.Trade) error {
	for i, _ := range a.PriceLevels.Values[:len(a.PriceLevels.Values)-1] {

		if trade.RequestedPrice >= a.PriceLevels.Values[i].Price && trade.RequestedPrice < a.PriceLevels.Values[i+1].Price {
			return nil
		}
	}

	return PriceOutsideLimitsErr
}

func NewAccount(balance float64, maxLossPercentage float64, priceLevels PriceLevels) (*Account, error) {
	if len(priceLevels.Values) < 2 {
		return nil, LevelsNotSetErr
	}

	if maxLossPercentage < 0 || maxLossPercentage > 1 {
		return nil, MaxLossPercentErr
	}

	prevPrice := 0.0
	for _, level := range priceLevels.Values {
		if level.Price <= 0 {
			return nil, fmt.Errorf("invalid price level, %v", level.Price)
		}

		if level.Price < prevPrice {
			return nil, PriceLevelsNotSortedErr
		}

		prevPrice = level.Price
	}

	return &Account{
		Balance:           balance,
		MaxLossPercentage: maxLossPercentage,
		PriceLevels:       priceLevels,
	}, nil
}

func (a *Account) GetBalanceAtLevel(price float64) BalanceLevelStats {
	return BalanceLevelStats{}
}

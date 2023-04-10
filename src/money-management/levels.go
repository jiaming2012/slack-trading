package money_management

import (
	"fmt"
	"math"
	"slack-trading/src/models"
	"sync"
	"time"
)

type Account struct {
	Balance           float64
	MaxLossPercentage float64
	PriceLevels       *PriceLevels
	mutex             sync.Mutex
}

type PriceLevel struct {
	Price             float64
	NoOfTrades        int
	AllocationPercent float64 // the amount of Account.Balance allocated to this price level
	Trades            models.Trades
}

type PriceLevels struct {
	Values []*PriceLevel
}

type BalanceLevelStats struct {
	TotalBalance float64
	UsedBalance  float64
}

//func (a *Account) Update() error {
//
//}

func (a *Account) GetTrades() *models.Trades {
	trades := models.Trades{}
	for _, level := range a.PriceLevels.Values {
		for _, tr := range level.Trades {
			trades = append(trades, tr)
		}
	}

	return &trades
}

func (a *Account) findPriceLevel(price float64) *PriceLevel {
	for i, priceLevel := range a.PriceLevels.Values[:len(a.PriceLevels.Values)-1] {

		if price >= a.PriceLevels.Values[i].Price && price < a.PriceLevels.Values[i+1].Price {
			return priceLevel
		}
	}

	return nil
}

func (a *Account) PlaceOrder(tradeType models.TradeType, currentPrice float64, stopLoss float64) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	priceLevel := a.findPriceLevel(currentPrice)
	if len(priceLevel.Trades) >= priceLevel.NoOfTrades {
		return models.MaxTradesPerPriceLevelErr
	}

	maxLoss := a.Balance * a.MaxLossPercentage * (priceLevel.AllocationPercent / float64(priceLevel.NoOfTrades))

	var volume float64
	if tradeType == models.TradeTypeBuy {
		volume = (currentPrice - stopLoss) / maxLoss
	} else if tradeType == models.TradeTypeSell {
		volume = (stopLoss - currentPrice) / maxLoss
	} else {
		return fmt.Errorf("invalid trade type %v", tradeType)
	}

	newTrade := models.Trade{
		Symbol:         "BTCUSD",
		Time:           time.Now(),
		RequestedPrice: currentPrice,
		StopLoss:       stopLoss,
		Volume:         volume,
	}

	if err := newTrade.Validate(); err != nil {
		return err
	}

	priceLevel.Trades.Add(&newTrade)

	return nil
}

func (a *Account) CanPlaceTrade(trade models.Trade) error {
	priceLevel := a.findPriceLevel(trade.RequestedPrice)
	if priceLevel == nil {
		return models.PriceOutsideLimitsErr
	}

	return nil
}

func NewAccount(balance float64, maxLossPercentage float64, priceLevels PriceLevels) (*Account, error) {
	if len(priceLevels.Values) < 2 {
		return nil, models.LevelsNotSetErr
	}

	if maxLossPercentage < 0 || maxLossPercentage > 1 {
		return nil, models.MaxLossPercentErr
	}

	if err := priceLevels.Validate(); err != nil {
		return nil, fmt.Errorf("failed to validate price levels: %w", err)
	}

	prevPrice := 0.0
	for _, level := range priceLevels.Values {
		if level.Price <= 0 {
			return nil, fmt.Errorf("invalid price level, %v", level.Price)
		}

		if level.Price < prevPrice {
			return nil, models.PriceLevelsNotSortedErr
		}

		prevPrice = level.Price
	}

	return &Account{
		Balance:           balance,
		MaxLossPercentage: maxLossPercentage,
		PriceLevels:       &priceLevels,
	}, nil
}

func (a *Account) GetBalanceAtLevel(price float64) BalanceLevelStats {
	return BalanceLevelStats{}
}

func (levels PriceLevels) Validate() error {
	total := 0.0
	for _, lvl := range levels.Values {
		total += lvl.AllocationPercent
	}

	if math.Abs(1-total) > 0.001 {
		return fmt.Errorf("%w: allocation total is %v", models.PriceLevelsAllocationErr, total)
	}

	return nil
}

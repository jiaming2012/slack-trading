package money_management

import (
	"fmt"
	"github.com/google/uuid"
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
	Trades            *models.Trades
}

func (p *PriceLevel) NewTradesRemaining() (int, models.TradeType) {
	buysCount := 0
	buyVolume := 0.0
	sellVolume := 0.0
	sellsCount := 0

	// todo: null pointer check
	for _, t := range *p.Trades {
		if t.Side() == models.TradeTypeBuy {
			buysCount += 1
			buyVolume += t.Volume
		} else if t.Side() == models.TradeTypeSell {
			sellsCount += 1
			sellVolume += math.Abs(t.Volume)
		}
	}

	var diff = int(math.Abs(float64(buysCount) - float64(sellsCount)))
	if buyVolume > sellVolume {
		for _, t := range *p.Trades {
			if t.Side() == models.TradeTypeBuy {
				tradeVolume := t.Volume
				if sellVolume > 0 {
					delta := math.Min(sellVolume, tradeVolume)
					remainingVolume := tradeVolume - delta
					if remainingVolume > 0 {
						buysCount += 1
					}
					sellVolume -= delta
				}
				buysCount += 1
			}
		}
	} else if buyVolume < sellVolume {

	} else {

	}

	var side models.TradeType
	if sellVolume > buyVolume {
		side = models.TradeTypeSell
	} else {
		side = models.TradeTypeBuy
	}

	return p.NoOfTrades - diff, side
}

type PriceLevels struct {
	Values []*PriceLevel
}

// todo: reeval if we need this
type BalanceLevelStats struct {
	TotalBalance float64
	UsedBalance  float64
}

type CloseTradeRequest struct {
	Trades models.Trades
}

func (a *Account) Update(price float64) *CloseTradeRequest {
	request := CloseTradeRequest{}
	for _, trade := range *a.GetTrades() {
		if trade.Side() == models.TradeTypeBuy {
			if price <= trade.StopLoss {
				request.Trades = append(request.Trades, trade)
			}
		}

		if trade.Side() == models.TradeTypeSell {
			if price >= trade.StopLoss {
				request.Trades = append(request.Trades, trade)
			}
		}
	}

	if len(request.Trades) > 0 {
		return &request
	}

	return nil
}

func (a *Account) GetTrades() *models.Trades {
	trades := models.Trades{}
	for _, level := range a.PriceLevels.Values {
		for _, tr := range *level.Trades {
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

func (a *Account) TradesRemaining(price float64) (int, models.TradeType) {
	lvl := a.findPriceLevel(price)
	if lvl != nil {
		return lvl.NewTradesRemaining()
	}
	return 0, models.TradeTypeBuy
}

func (a *Account) PlaceOrder(tradeType models.TradeType, currentPrice float64, stopLoss float64, closePercent float64) (*models.Trade, error) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	newTrade := models.Trade{
		ID:             uuid.New(),
		Symbol:         "BTCUSD",
		Time:           time.Now(),
		RequestedPrice: currentPrice,
	}

	if err := a.CanPlaceTrade(newTrade); err != nil {
		return nil, err
	}

	priceLevel := a.findPriceLevel(currentPrice)
	if priceLevel.NoOfTrades <= 0 {
		return nil, models.MaxTradesPerPriceLevelErr
	}

	tradesRemaining, side := priceLevel.NewTradesRemaining()
	if tradeType == models.TradeTypeBuy {
		if side == models.TradeTypeBuy && tradesRemaining <= 0 {
			return nil, models.MaxTradesPerPriceLevelErr
		}
	} else if tradeType == models.TradeTypeSell {
		if side == models.TradeTypeSell && tradesRemaining <= 0 {
			return nil, models.MaxTradesPerPriceLevelErr
		}
	}

	maxLoss := a.Balance * a.MaxLossPercentage * (priceLevel.AllocationPercent / float64(priceLevel.NoOfTrades))

	_, accountVolume, realizedPL := a.GetTrades().Vwap()
	var volume float64
	if tradeType == models.TradeTypeBuy {
		if accountVolume >= 0 {
			if stopLoss >= currentPrice {
				return nil, fmt.Errorf("%w: stopLoss of %v is above current price of %v", models.InvalidStopLossErr, stopLoss, currentPrice)
			}

			volume = (maxLoss + float64(realizedPL)) / (currentPrice - stopLoss)
		} else {
			if err := models.ClosePercent(closePercent).Validate(); err != nil {
				return nil, err
			}
			volume = float64(accountVolume) * closePercent * -1
		}
	} else if tradeType == models.TradeTypeSell {
		if accountVolume <= 0 {
			if stopLoss <= currentPrice {
				return nil, fmt.Errorf("%w: stopLoss of %v is below current price of %v", models.InvalidStopLossErr, stopLoss, currentPrice)
			}

			volume = (maxLoss + float64(realizedPL)) / (currentPrice - stopLoss)
		} else {
			if err := models.ClosePercent(closePercent).Validate(); err != nil {
				return nil, err
			}
			volume = float64(accountVolume) * closePercent * -1
		}
	} else {
		return nil, fmt.Errorf("invalid trade type %v", tradeType)
	}

	newTrade.StopLoss = stopLoss
	newTrade.Volume = volume

	if err := newTrade.Validate(false); err != nil {
		return nil, err
	}

	priceLevel.Trades.Add(&newTrade)

	return &newTrade, nil
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

		if level.AllocationPercent > 0 && level.NoOfTrades <= 0 {
			return nil, models.NoOfTradeMustBeNonzeroErr
		}

		if level.AllocationPercent == 0 && level.NoOfTrades > 0 {
			return nil, models.NoOfTradesMustBeZeroErr
		}
		level.Trades = &models.Trades{}
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

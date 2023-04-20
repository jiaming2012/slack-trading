package models

import (
	"fmt"
	"github.com/google/uuid"
	"math"
	"sync"
	"time"
)

type Account struct {
	Balance           float64
	MaxLossPercentage float64
	PriceLevels       *PriceLevels
	mutex             sync.Mutex
}

func (a *Account) GetTrades() *Trades {
	trades := Trades{}
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

func (a *Account) Update(price float64) *CloseTradeRequest {
	request := CloseTradeRequest{}
	for _, trade := range *a.GetTrades() {
		if trade.Side() == TradeTypeBuy {
			if price <= trade.StopLoss {
				request.Trades = append(request.Trades, trade)
			}
		}

		if trade.Side() == TradeTypeSell {
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

func (a *Account) TradesRemaining(price float64) (int, TradeType) {
	lvl := a.findPriceLevel(price)
	if lvl != nil {
		return lvl.NewTradesRemaining()
	}
	return 0, TradeTypeBuy
}

func (a *Account) PlaceOrder(tradeType TradeType, currentPrice float64, stopLoss float64, closePercent float64) (*Trade, error) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	// todo: refactor PlaceOrder parameters to pass in a trade request
	tradeReq := TradeRequest{
		Type:     tradeType,
		Price:    currentPrice,
		StopLoss: stopLoss,
	}

	// tradeReq.Validate()

	newTrade := Trade{
		ID:             uuid.New(),
		Symbol:         "BTCUSD",
		Time:           time.Now(),
		RequestedPrice: currentPrice,
	}

	tradeParams, err := a.CanPlaceTrade(tradeReq)
	if err != nil {
		return nil, err
	}

	_, accountVolume, realizedPL := a.GetTrades().Vwap()
	var volume float64
	if tradeType == TradeTypeBuy {
		if accountVolume >= 0 {
			if stopLoss >= currentPrice {
				return nil, fmt.Errorf("%w: stopLoss of %v is above current price of %v", InvalidStopLossErr, stopLoss, currentPrice)
			}

			volume = (tradeParams.MaxLoss + float64(realizedPL)) / (currentPrice - stopLoss)
		} else {
			if err = ClosePercent(closePercent).Validate(); err != nil {
				return nil, err
			}
			volume = float64(accountVolume) * closePercent * -1
		}
	} else if tradeType == TradeTypeSell {
		if accountVolume <= 0 {
			if stopLoss <= currentPrice {
				return nil, fmt.Errorf("%w: stopLoss of %v is below current price of %v", InvalidStopLossErr, stopLoss, currentPrice)
			}

			volume = (tradeParams.MaxLoss + float64(realizedPL)) / (currentPrice - stopLoss)
		} else {
			if err = ClosePercent(closePercent).Validate(); err != nil {
				return nil, err
			}
			volume = float64(accountVolume) * closePercent * -1
		}
	} else {
		return nil, fmt.Errorf("invalid trade type %v", tradeType)
	}

	// volume will eventually reach
	if math.Abs(volume) < math.SmallestNonzeroFloat64 {
		return nil, TradeVolumeIsZeroErr
	}

	newTrade.StopLoss = stopLoss
	newTrade.Volume = volume

	if err = newTrade.Validate(false); err != nil {
		return nil, err
	}

	tradeParams.PriceLevel.Trades.Add(&newTrade)

	return &newTrade, nil
}

func (a *Account) CanPlaceTrade(tradeReq TradeRequest) (*TradeParameters, error) {
	priceLevel := a.findPriceLevel(tradeReq.Price)
	if priceLevel == nil {
		return nil, PriceOutsideLimitsErr
	}

	if priceLevel.NoOfTrades <= 0 {
		return nil, MaxTradesPerPriceLevelErr
	}

	tradesRemaining, side := priceLevel.NewTradesRemaining()
	if tradeReq.Type == TradeTypeBuy {
		if side == TradeTypeBuy && tradesRemaining <= 0 {
			return nil, MaxTradesPerPriceLevelErr
		}
	} else if tradeReq.Type == TradeTypeSell {
		if side == TradeTypeSell && tradesRemaining <= 0 {
			return nil, MaxTradesPerPriceLevelErr
		}
	}

	_, _, realizedPL := priceLevel.Trades.Vwap()

	maxPriceLevelLoss := a.Balance * a.MaxLossPercentage * priceLevel.AllocationPercent
	maxTradeLoss := maxPriceLevelLoss / float64(priceLevel.NoOfTrades)

	if float64(realizedPL)+maxTradeLoss > maxPriceLevelLoss {
		return nil, MaxLossPriceBandErr
	}

	return &TradeParameters{
		PriceLevel: priceLevel,
		MaxLoss:    maxTradeLoss,
	}, nil
}

func NewAccount(balance float64, maxLossPercentage float64, priceLevels PriceLevels) (*Account, error) {
	if len(priceLevels.Values) < 2 {
		return nil, LevelsNotSetErr
	}

	if maxLossPercentage < 0 || maxLossPercentage > 1 {
		return nil, MaxLossPercentErr
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
			return nil, PriceLevelsNotSortedErr
		}

		if level.AllocationPercent > 0 && level.NoOfTrades <= 0 {
			return nil, NoOfTradeMustBeNonzeroErr
		}

		if level.AllocationPercent == 0 && level.NoOfTrades > 0 {
			return nil, NoOfTradesMustBeZeroErr
		}
		level.Trades = &Trades{}
		prevPrice = level.Price
	}

	if priceLevels.Values[len(priceLevels.Values)-1].AllocationPercent != 0 {
		return nil, PriceLevelsLastAllocationErr
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

package models

import (
	"fmt"
	"math"
	"sync"
)

const SmallRoundingError = 0.000001

type PriceLevel struct {
	Price                float64
	MinimumTradeDistance float64 // the minimum distance of the requested price of two trades in the same price band
	MaxNoOfTrades        int
	AllocationPercent    float64 // the amount of Account.Balance allocated to this price level
	Trades               *Trades
	StopLoss             float64
	mutex                sync.Mutex
}

func (p *PriceLevel) canAddTrade(trade *Trade) error {
	if trade.Type == TradeTypeBuy || trade.Type == TradeTypeSell {
		openTrades := p.Trades.OpenTrades()
		for _, open := range *openTrades {
			t := math.Abs(trade.RequestedPrice-open.RequestedPrice) + SmallRoundingError
			if t < p.MinimumTradeDistance {
				return fmt.Errorf("PriceLevel.canAddTrade: request price of %v is too close to request price of previously open trade %v with minimum distance of %v: %w", trade.RequestedPrice, open.RequestedPrice, p.MinimumTradeDistance, PriceLevelMinimumDistanceNotSatisfiedError)
			}
		}
	}

	return nil
}

func (p *PriceLevel) Add(trade *Trade, executedPrice float64, executedVolume float64) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if err := p.canAddTrade(trade); err != nil {
		return err
	}

	partialCloseItems, err := trade.PreparePartialCloseItems(executedPrice, executedVolume)
	if err != nil {
		return fmt.Errorf("Trade.Execute: failed to modify trades to add partial close items: %w", err)
	}

	if err = trade.Validate(partialCloseItems); err != nil {
		return fmt.Errorf("PriceLevel.Add: trade is not valid: %w", err)
	}

	if err = trade.Execute(executedPrice, executedVolume, partialCloseItems); err != nil {
		return fmt.Errorf("PriceLevel.Add: failed to execute trade: %w", err)
	}

	p.Trades.Add(trade)

	return nil
}

func (p *PriceLevel) Validate() error {
	if p.MinimumTradeDistance < 0 {
		return PriceLevelMinimumDistanceNotSatisfiedError
	}

	if p.AllocationPercent < 0 || p.AllocationPercent > 1 {
		return InvalidAllocationPercentErr
	}

	if p.AllocationPercent > 0 && p.StopLoss <= 0 {
		return NonPositiveStopLoss
	}

	if p.MaxNoOfTrades < 0 {
		return InvalidMaxTradesErr
	}

	if p.Price < 0 {
		return NegativePriceErr
	}

	return nil
}

func (p *PriceLevel) NewTradesRemaining() (int, TradeType) {
	// todo: make this part of GetTradeStats, along with GetOpenTrades
	openTrades := p.Trades.OpenTrades()

	if openTrades == nil || len(*openTrades) == 0 {
		return p.MaxNoOfTrades, TradeTypeNone
	}

	remaining := math.Max(float64(p.MaxNoOfTrades-len(*openTrades)), 0)

	return int(remaining), (*openTrades)[0].Type
}
